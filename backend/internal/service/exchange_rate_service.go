package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"golang.org/x/sync/singleflight"
)

type ResolvedExchangeRate struct {
	LiveRate        *float64
	LastSuccessRate *float64
	FloorRate       float64
	SafetyMargin    float64
	EffectiveRate   float64
	Source          string
	FetchedAt       time.Time
}

type UsageChargeSnapshot struct {
	ChargedAmountCNY float64
	FXRateUSDCNY     float64
	FXRateSource     string
	FXFetchedAt      *time.Time
	FXSafetyMargin   float64
}

type ExchangeRateService interface {
	ResolveUSDCNYRate(ctx context.Context) (*ResolvedExchangeRate, error)
}

type exchangeRateProvider interface {
	Name() string
	FetchUSDCNYRate(ctx context.Context) (float64, time.Time, error)
}

type staticExchangeRateService struct {
	cfg *config.Config
}

func NewStaticExchangeRateService(cfg *config.Config) ExchangeRateService {
	return &staticExchangeRateService{cfg: cfg}
}

func (s *staticExchangeRateService) ResolveUSDCNYRate(ctx context.Context) (*ResolvedExchangeRate, error) {
	settings := defaultBillingFXSettingsFromConfig(s.cfg)
	now := time.Now().UTC()
	return &ResolvedExchangeRate{
		FloorRate:     settings.FallbackRate,
		SafetyMargin:  settings.SafetyMargin,
		EffectiveRate: applySafetyMargin(settings.FallbackRate, settings.SafetyMargin),
		Source:        "fallback_floor",
		FetchedAt:     now,
	}, nil
}

type exchangeRateService struct {
	store     *SettingService
	cfg       *config.Config
	providers map[string]exchangeRateProvider

	cacheMu sync.RWMutex
	cache   *cachedResolvedExchangeRate
	sf      singleflight.Group
}

type cachedResolvedExchangeRate struct {
	value     *ResolvedExchangeRate
	expiresAt time.Time
}

type exchangeRateAPIProvider struct {
	client *http.Client
	url    string
}

type exchangeRateAPIResponse struct {
	Result             string             `json:"result"`
	TimeLastUpdateUnix int64              `json:"time_last_update_unix"`
	Rates              map[string]float64 `json:"rates"`
}

type billingFXDefaults struct {
	Enabled         bool
	Provider        string
	FallbackRate    float64
	CacheTTLSeconds int
	TimeoutMS       int
	SafetyMargin    float64
	LastSuccessRate *float64
	LastSuccessAt   *time.Time
	LiveURL         string
}

func NewExchangeRateService(store *SettingService, cfg *config.Config) ExchangeRateService {
	settings := defaultBillingFXSettingsFromConfig(cfg)
	return &exchangeRateService{
		store: store,
		cfg:   cfg,
		providers: map[string]exchangeRateProvider{
			"default": &exchangeRateAPIProvider{
				client: &http.Client{},
				url:    settings.LiveURL,
			},
		},
	}
}

func (s *exchangeRateService) ResolveUSDCNYRate(ctx context.Context) (*ResolvedExchangeRate, error) {
	settings := defaultBillingFXSettingsFromConfig(s.cfg)
	if s != nil && s.store != nil {
		settings = mergeBillingFXSettings(settings, s.store.GetBillingFXSettings(ctx))
	}
	if !settings.Enabled {
		return resolveFallbackExchangeRate(settings, "disabled"), nil
	}
	if cached := s.getCached(); cached != nil {
		return cached, nil
	}

	value, err, _ := s.sf.Do("usd_cny", func() (any, error) {
		if cached := s.getCached(); cached != nil {
			return cached, nil
		}
		resolved, resolveErr := s.resolveWithProvider(ctx, settings)
		if resolved != nil {
			s.setCached(resolved, settings.CacheTTLSeconds)
		}
		return resolved, resolveErr
	})
	if err != nil {
		return nil, err
	}
	if resolved, ok := value.(*ResolvedExchangeRate); ok && resolved != nil {
		return resolved, nil
	}
	return resolveFallbackExchangeRate(settings, "fallback_floor"), nil
}

func (s *exchangeRateService) resolveWithProvider(ctx context.Context, settings billingFXDefaults) (*ResolvedExchangeRate, error) {
	providerName := normalizeFXProvider(settings.Provider)
	provider, ok := s.providers[providerName]
	if !ok {
		slog.Warn("billing fx provider not found, fallback used", "provider", providerName)
		return resolveFallbackExchangeRate(settings, "fallback_floor"), nil
	}

	fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Duration(settings.TimeoutMS)*time.Millisecond)
	defer cancel()

	liveRate, fetchedAt, err := provider.FetchUSDCNYRate(fetchCtx)
	if err != nil {
		slog.Warn("billing fx live fetch failed, fallback used", "provider", provider.Name(), "error", err)
		return resolveFallbackExchangeRate(settings, ""), nil
	}
	liveRate = clampPositiveRate(liveRate)
	if liveRate <= 0 {
		slog.Warn("billing fx live rate invalid, fallback used", "provider", provider.Name(), "rate", liveRate)
		return resolveFallbackExchangeRate(settings, ""), nil
	}
	if fetchedAt.IsZero() {
		fetchedAt = time.Now().UTC()
	}

	baseRate := maxExchangeRateFloat(liveRate, settings.FallbackRate)
	if settings.LastSuccessRate != nil {
		baseRate = maxExchangeRateFloat(baseRate, *settings.LastSuccessRate)
	}

	resolved := &ResolvedExchangeRate{
		LiveRate:        &liveRate,
		LastSuccessRate: settings.LastSuccessRate,
		FloorRate:       settings.FallbackRate,
		SafetyMargin:    settings.SafetyMargin,
		EffectiveRate:   applySafetyMargin(baseRate, settings.SafetyMargin),
		Source:          "live:" + provider.Name(),
		FetchedAt:       fetchedAt.UTC(),
	}

	if s.store != nil {
		persistCtx, persistCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer persistCancel()
		if persistErr := s.store.UpdateBillingFXLastSuccess(persistCtx, liveRate, fetchedAt.UTC()); persistErr != nil {
			slog.Warn("billing fx persist last success failed", "provider", provider.Name(), "error", persistErr)
		}
	}

	return resolved, nil
}

func (s *exchangeRateService) getCached() *ResolvedExchangeRate {
	if s == nil {
		return nil
	}
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	if s.cache == nil || s.cache.value == nil {
		return nil
	}
	if time.Now().After(s.cache.expiresAt) {
		return nil
	}
	return cloneResolvedExchangeRate(s.cache.value)
}

func (s *exchangeRateService) setCached(value *ResolvedExchangeRate, ttlSeconds int) {
	if s == nil || value == nil || ttlSeconds <= 0 {
		return
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cache = &cachedResolvedExchangeRate{
		value:     cloneResolvedExchangeRate(value),
		expiresAt: time.Now().Add(time.Duration(ttlSeconds) * time.Second),
	}
}

func (p *exchangeRateAPIProvider) Name() string {
	return "default"
}

func (p *exchangeRateAPIProvider) FetchUSDCNYRate(ctx context.Context) (float64, time.Time, error) {
	if p == nil || p.client == nil {
		return 0, time.Time{}, fmt.Errorf("billing fx http client is nil")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return 0, time.Time{}, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, time.Time{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, time.Time{}, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var payload exchangeRateAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, time.Time{}, err
	}
	rate := payload.Rates["CNY"]
	if rate <= 0 {
		return 0, time.Time{}, fmt.Errorf("missing CNY rate")
	}
	if payload.TimeLastUpdateUnix > 0 {
		return rate, time.Unix(payload.TimeLastUpdateUnix, 0).UTC(), nil
	}
	return rate, time.Now().UTC(), nil
}

func defaultBillingFXSettingsFromConfig(cfg *config.Config) billingFXDefaults {
	settings := billingFXDefaults{
		Enabled:         true,
		Provider:        "default",
		FallbackRate:    7.2,
		CacheTTLSeconds: 600,
		TimeoutMS:       3000,
		SafetyMargin:    0.02,
		LiveURL:         "https://open.er-api.com/v6/latest/USD",
	}
	if cfg == nil {
		return settings
	}
	if !cfg.Billing.FX.Enabled && strings.TrimSpace(cfg.Billing.FX.Provider) == "" &&
		cfg.Billing.FX.FallbackRate == 0 && cfg.Billing.FX.CacheTTLSeconds == 0 &&
		cfg.Billing.FX.TimeoutMS == 0 && cfg.Billing.FX.SafetyMargin == 0 &&
		strings.TrimSpace(cfg.Billing.FX.DefaultLiveURL) == "" {
		return settings
	}
	settings.Enabled = cfg.Billing.FX.Enabled
	if raw := strings.TrimSpace(cfg.Billing.FX.Provider); raw != "" {
		settings.Provider = raw
	}
	if cfg.Billing.FX.FallbackRate > 0 {
		settings.FallbackRate = cfg.Billing.FX.FallbackRate
	}
	if cfg.Billing.FX.CacheTTLSeconds > 0 {
		settings.CacheTTLSeconds = cfg.Billing.FX.CacheTTLSeconds
	}
	if cfg.Billing.FX.TimeoutMS > 0 {
		settings.TimeoutMS = cfg.Billing.FX.TimeoutMS
	}
	if cfg.Billing.FX.SafetyMargin >= 0 {
		settings.SafetyMargin = cfg.Billing.FX.SafetyMargin
	}
	if raw := strings.TrimSpace(cfg.Billing.FX.DefaultLiveURL); raw != "" {
		settings.LiveURL = raw
	}
	return settings
}

func mergeBillingFXSettings(defaults billingFXDefaults, runtime BillingFXSettings) billingFXDefaults {
	defaults.Enabled = runtime.Enabled
	if runtime.Provider != "" {
		defaults.Provider = runtime.Provider
	}
	if runtime.FallbackRate > 0 {
		defaults.FallbackRate = runtime.FallbackRate
	}
	if runtime.CacheTTLSeconds > 0 {
		defaults.CacheTTLSeconds = runtime.CacheTTLSeconds
	}
	if runtime.TimeoutMS > 0 {
		defaults.TimeoutMS = runtime.TimeoutMS
	}
	if runtime.SafetyMargin >= 0 {
		defaults.SafetyMargin = runtime.SafetyMargin
	}
	defaults.LastSuccessRate = runtime.LastSuccessRate
	defaults.LastSuccessAt = runtime.LastSuccessAt
	return defaults
}

func resolveFallbackExchangeRate(settings billingFXDefaults, forcedSource string) *ResolvedExchangeRate {
	baseRate := settings.FallbackRate
	source := forcedSource
	if settings.LastSuccessRate != nil && *settings.LastSuccessRate > baseRate {
		baseRate = *settings.LastSuccessRate
		if source == "" {
			source = "last_success"
		}
	}
	if source == "" {
		source = "fallback_floor"
	}
	fetchedAt := time.Now().UTC()
	if source == "last_success" && settings.LastSuccessAt != nil && !settings.LastSuccessAt.IsZero() {
		fetchedAt = settings.LastSuccessAt.UTC()
	}
	return &ResolvedExchangeRate{
		LastSuccessRate: settings.LastSuccessRate,
		FloorRate:       settings.FallbackRate,
		SafetyMargin:    settings.SafetyMargin,
		EffectiveRate:   applySafetyMargin(baseRate, settings.SafetyMargin),
		Source:          source,
		FetchedAt:       fetchedAt,
	}
}

func BuildUsageChargeSnapshot(costUSD float64, rate *ResolvedExchangeRate) *UsageChargeSnapshot {
	if costUSD <= 0 || rate == nil || rate.EffectiveRate <= 0 {
		return nil
	}
	return BuildUsageChargeSnapshotFromCNY(costUSD*rate.EffectiveRate, rate)
}

func BuildUsageChargeSnapshotFromCNY(amountCNY float64, rate *ResolvedExchangeRate) *UsageChargeSnapshot {
	if amountCNY <= 0 || rate == nil || rate.EffectiveRate <= 0 {
		return nil
	}
	fetchedAt := rate.FetchedAt.UTC()
	return &UsageChargeSnapshot{
		ChargedAmountCNY: roundTo(amountCNY, 8),
		FXRateUSDCNY:     roundTo(rate.EffectiveRate, 10),
		FXRateSource:     strings.TrimSpace(rate.Source),
		FXFetchedAt:      &fetchedAt,
		FXSafetyMargin:   roundTo(rate.SafetyMargin, 6),
	}
}

func cloneResolvedExchangeRate(src *ResolvedExchangeRate) *ResolvedExchangeRate {
	if src == nil {
		return nil
	}
	out := *src
	if src.LiveRate != nil {
		v := *src.LiveRate
		out.LiveRate = &v
	}
	if src.LastSuccessRate != nil {
		v := *src.LastSuccessRate
		out.LastSuccessRate = &v
	}
	return &out
}

func normalizeFXProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return "default"
	}
	return provider
}

func clampPositiveRate(rate float64) float64 {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 {
		return 0
	}
	return rate
}

func applySafetyMargin(rate float64, margin float64) float64 {
	return roundTo(rate*(1+margin), 10)
}

func roundTo(value float64, precision int) float64 {
	if precision <= 0 {
		return math.Round(value)
	}
	pow := math.Pow10(precision)
	return math.Round(value*pow) / pow
}

func maxExchangeRateFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
