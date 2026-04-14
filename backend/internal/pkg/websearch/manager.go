package websearch

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyutil"
	"github.com/redis/go-redis/v9"
)

type ProviderConfig struct {
	Type         string `json:"type"`
	APIKey       string `json:"api_key"`
	QuotaLimit   int64  `json:"quota_limit"`
	SubscribedAt *int64 `json:"subscribed_at,omitempty"`
	ProxyURL     string `json:"-"`
	ProxyID      int64  `json:"-"`
	ExpiresAt    *int64 `json:"expires_at,omitempty"`
}

type Manager struct {
	configs []ProviderConfig
	redis   *redis.Client

	clientMu    sync.Mutex
	clientCache map[string]*http.Client
}

const (
	proxyDialTimeout     = 3 * time.Second
	proxyTLSTimeout      = 3 * time.Second
	searchDataTimeout    = 60 * time.Second
	searchRequestTimeout = searchDataTimeout + proxyDialTimeout

	quotaKeyPrefix      = "websearch:quota:"
	proxyUnavailableKey = "websearch:proxy_unavailable:%d"
	proxyUnavailableTTL = 5 * time.Minute
	quotaTTLBuffer      = 24 * time.Hour
	defaultQuotaTTL     = 31*24*time.Hour + quotaTTLBuffer
	maxCachedClients    = 100
)

var ErrProxyUnavailable = errors.New("websearch: proxy unavailable")

var quotaIncrScript = redis.NewScript(`
local val = redis.call('INCR', KEYS[1])
if val == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
else
  local ttl = redis.call('TTL', KEYS[1])
  if ttl == -1 then
    redis.call('EXPIRE', KEYS[1], ARGV[1])
  end
end
return val
`)

func NewManager(configs []ProviderConfig, redisClient *redis.Client) *Manager {
	copied := make([]ProviderConfig, len(configs))
	copy(copied, configs)
	return &Manager{
		configs:     copied,
		redis:       redisClient,
		clientCache: make(map[string]*http.Client),
	}
}

func (m *Manager) SearchWithBestProvider(ctx context.Context, req SearchRequest) (*SearchResponse, string, error) {
	if strings.TrimSpace(req.Query) == "" {
		return nil, "", fmt.Errorf("websearch: empty search query")
	}

	candidates := m.filterAvailableProviders(ctx, req.ProxyURL)
	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("websearch: no available provider (all exhausted, expired, or proxy unavailable)")
	}

	selected := m.selectByQuotaWeight(ctx, candidates)
	for _, cfg := range selected {
		allowed, incremented := m.tryReserveQuota(ctx, cfg)
		if !allowed {
			continue
		}
		resp, err := m.executeSearch(ctx, cfg, req)
		if err != nil {
			if incremented {
				m.rollbackQuota(ctx, cfg)
			}
			if isProxyError(err) {
				m.markProxyUnavailable(ctx, cfg, req.ProxyURL)
				if req.ProxyURL != "" {
					slog.Warn("websearch: account proxy error, aborting failover", "provider", cfg.Type, "error", err)
					return nil, "", fmt.Errorf("%w: %s", ErrProxyUnavailable, err.Error())
				}
				slog.Warn("websearch: provider proxy error, trying next provider", "provider", cfg.Type, "error", err)
				continue
			}
			slog.Warn("websearch: provider search failed", "provider", cfg.Type, "error", err)
			continue
		}
		return resp, cfg.Type, nil
	}
	return nil, "", fmt.Errorf("websearch: no available provider (all exhausted or failed)")
}

func (m *Manager) filterAvailableProviders(ctx context.Context, accountProxyURL string) []ProviderConfig {
	var out []ProviderConfig
	for _, cfg := range m.configs {
		if !m.isProviderAvailable(cfg) {
			continue
		}
		proxyID := resolveProxyID(cfg, accountProxyURL)
		if proxyID > 0 && !m.isProxyAvailable(ctx, proxyID) {
			slog.Debug("websearch: proxy marked unavailable, skipping", "provider", cfg.Type, "proxy_id", proxyID)
			continue
		}
		out = append(out, cfg)
	}
	return out
}

type weighted struct {
	cfg    ProviderConfig
	weight int64
}

func (m *Manager) selectByQuotaWeight(ctx context.Context, candidates []ProviderConfig) []ProviderConfig {
	items := m.computeWeights(ctx, candidates)
	withQuota, withoutQuota := partitionByQuota(items)
	sortByStableRandomWeight(withQuota)
	return mergeWeightedResults(withQuota, withoutQuota, len(candidates))
}

func (m *Manager) computeWeights(ctx context.Context, candidates []ProviderConfig) []weighted {
	items := make([]weighted, 0, len(candidates))
	for _, cfg := range candidates {
		var weight int64
		if cfg.QuotaLimit > 0 {
			used, _ := m.GetUsage(ctx, cfg.Type)
			if remaining := cfg.QuotaLimit - used; remaining > 0 {
				weight = remaining
			}
		}
		items = append(items, weighted{cfg: cfg, weight: weight})
	}
	return items
}

func partitionByQuota(items []weighted) (withQuota, withoutQuota []weighted) {
	for _, item := range items {
		if item.weight > 0 {
			withQuota = append(withQuota, item)
		} else {
			withoutQuota = append(withoutQuota, item)
		}
	}
	return
}

func sortByStableRandomWeight(items []weighted) {
	if len(items) <= 1 {
		return
	}
	type entry struct {
		item   weighted
		factor float64
	}
	entries := make([]entry, len(items))
	for i, item := range items {
		entries[i] = entry{item: item, factor: float64(item.weight) * (0.5 + rand.Float64())}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].factor > entries[j].factor
	})
	for i, entry := range entries {
		items[i] = entry.item
	}
}

func mergeWeightedResults(withQuota, withoutQuota []weighted, capacity int) []ProviderConfig {
	result := make([]ProviderConfig, 0, capacity)
	for _, item := range withQuota {
		result = append(result, item.cfg)
	}
	for _, item := range withoutQuota {
		result = append(result, item.cfg)
	}
	return result
}

func (m *Manager) isProviderAvailable(cfg ProviderConfig) bool {
	if cfg.APIKey == "" {
		return false
	}
	if cfg.ExpiresAt != nil && time.Now().Unix() > *cfg.ExpiresAt {
		slog.Info("websearch: provider expired, skipping", "provider", cfg.Type, "expires_at", *cfg.ExpiresAt)
		return false
	}
	return true
}

func (m *Manager) markProxyUnavailable(ctx context.Context, cfg ProviderConfig, accountProxyURL string) {
	proxyID := resolveProxyID(cfg, accountProxyURL)
	if proxyID <= 0 || m.redis == nil {
		return
	}
	key := fmt.Sprintf(proxyUnavailableKey, proxyID)
	if err := m.redis.Set(ctx, key, "1", proxyUnavailableTTL).Err(); err != nil {
		slog.Warn("websearch: failed to mark proxy unavailable", "proxy_id", proxyID, "error", err)
	}
}

func (m *Manager) isProxyAvailable(ctx context.Context, proxyID int64) bool {
	if m.redis == nil || proxyID <= 0 {
		return true
	}
	key := fmt.Sprintf(proxyUnavailableKey, proxyID)
	val, err := m.redis.Get(ctx, key).Result()
	if err != nil {
		return true
	}
	return val == ""
}

func resolveProxyID(cfg ProviderConfig, accountProxyURL string) int64 {
	if accountProxyURL != "" {
		return 0
	}
	return cfg.ProxyID
}

func isProxyError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var tlsErr *tls.RecordHeaderError
	if errors.As(err, &tlsErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "proxy") ||
		strings.Contains(msg, "socks") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "tls handshake") ||
		strings.Contains(msg, "certificate")
}

func (m *Manager) tryReserveQuota(ctx context.Context, cfg ProviderConfig) (bool, bool) {
	if cfg.QuotaLimit <= 0 {
		return true, false
	}
	if m.redis == nil {
		slog.Warn("websearch: Redis unavailable, quota check skipped", "provider", cfg.Type)
		return true, false
	}
	key := quotaRedisKey(cfg.Type)
	ttlSec := int(quotaTTLFromSubscription(cfg.SubscribedAt).Seconds())
	newVal, err := quotaIncrScript.Run(ctx, m.redis, []string{key}, ttlSec).Int64()
	if err != nil {
		slog.Warn("websearch: quota Lua INCR failed, allowing request", "provider", cfg.Type, "error", err)
		return true, false
	}
	if newVal > cfg.QuotaLimit {
		if decrErr := m.redis.Decr(ctx, key).Err(); decrErr != nil {
			slog.Warn("websearch: quota over-limit DECR failed", "provider", cfg.Type, "error", decrErr)
		}
		slog.Info("websearch: provider quota exhausted", "provider", cfg.Type, "used", newVal, "limit", cfg.QuotaLimit)
		return false, false
	}
	return true, true
}

func (m *Manager) rollbackQuota(ctx context.Context, cfg ProviderConfig) {
	if cfg.QuotaLimit <= 0 || m.redis == nil {
		return
	}
	key := quotaRedisKey(cfg.Type)
	if err := m.redis.Decr(ctx, key).Err(); err != nil {
		slog.Warn("websearch: quota rollback DECR failed", "provider", cfg.Type, "error", err)
	}
}

func (m *Manager) TestSearch(ctx context.Context, req SearchRequest) (*SearchResponse, string, error) {
	if strings.TrimSpace(req.Query) == "" {
		return nil, "", fmt.Errorf("websearch: empty search query")
	}
	for _, cfg := range m.configs {
		if !m.isProviderAvailable(cfg) {
			continue
		}
		resp, err := m.executeSearch(ctx, cfg, req)
		if err != nil {
			continue
		}
		return resp, cfg.Type, nil
	}
	return nil, "", fmt.Errorf("websearch: no available provider")
}

func (m *Manager) executeSearch(ctx context.Context, cfg ProviderConfig, req SearchRequest) (*SearchResponse, error) {
	proxyURL := cfg.ProxyURL
	if req.ProxyURL != "" {
		proxyURL = req.ProxyURL
	}
	client, err := m.getOrCreateHTTPClient(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("websearch: %w", err)
	}
	provider := m.buildProvider(cfg, client)
	return provider.Search(ctx, req)
}

func (m *Manager) getOrCreateHTTPClient(proxyURL string) (*http.Client, error) {
	m.clientMu.Lock()
	defer m.clientMu.Unlock()

	if client, ok := m.clientCache[proxyURL]; ok {
		return client, nil
	}
	if len(m.clientCache) >= maxCachedClients {
		m.clientCache = make(map[string]*http.Client)
	}
	client, err := newHTTPClient(proxyURL)
	if err != nil {
		return nil, err
	}
	m.clientCache[proxyURL] = client
	return client, nil
}

func newHTTPClient(proxyURL string) (*http.Client, error) {
	transport := &http.Transport{
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext:           (&net.Dialer{Timeout: proxyDialTimeout}).DialContext,
		TLSHandshakeTimeout:   proxyTLSTimeout,
		ResponseHeaderTimeout: searchDataTimeout,
	}
	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL %q: %w", proxyURL, err)
		}
		if err := proxyutil.ConfigureTransportProxy(transport, parsed); err != nil {
			return nil, fmt.Errorf("configure proxy: %w", err)
		}
	}
	return &http.Client{Transport: transport, Timeout: searchRequestTimeout}, nil
}

func (m *Manager) GetUsage(ctx context.Context, providerType string) (int64, error) {
	if m.redis == nil {
		return 0, nil
	}
	key := quotaRedisKey(providerType)
	val, err := m.redis.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

func (m *Manager) GetAllUsage(ctx context.Context) map[string]int64 {
	result := make(map[string]int64, len(m.configs))
	for _, cfg := range m.configs {
		used, _ := m.GetUsage(ctx, cfg.Type)
		result[cfg.Type] = used
	}
	return result
}

func (m *Manager) ResetUsage(ctx context.Context, providerType string) error {
	if m.redis == nil {
		return nil
	}
	key := quotaRedisKey(providerType)
	return m.redis.Del(ctx, key).Err()
}

func (m *Manager) buildProvider(cfg ProviderConfig, client *http.Client) Provider {
	switch cfg.Type {
	case braveProviderName:
		return NewBraveProvider(cfg.APIKey, client)
	case tavilyProviderName:
		return NewTavilyProvider(cfg.APIKey, client)
	default:
		slog.Warn("websearch: unknown provider type, falling back to brave", "type", cfg.Type)
		return NewBraveProvider(cfg.APIKey, client)
	}
}

func quotaRedisKey(providerType string) string {
	return quotaKeyPrefix + providerType
}

func quotaTTLFromSubscription(subscribedAt *int64) time.Duration {
	if subscribedAt == nil || *subscribedAt == 0 {
		return defaultQuotaTTL
	}
	next := nextMonthlyReset(time.Unix(*subscribedAt, 0).UTC())
	ttl := time.Until(next) + quotaTTLBuffer
	if ttl <= quotaTTLBuffer {
		ttl = defaultQuotaTTL
	}
	return ttl
}

func nextMonthlyReset(subscribedAt time.Time) time.Time {
	now := time.Now().UTC()
	if subscribedAt.IsZero() {
		return now.AddDate(0, 1, 0)
	}
	months := (now.Year()-subscribedAt.Year())*12 + int(now.Month()-subscribedAt.Month())
	if months < 0 {
		months = 0
	}
	candidate := addMonthsClamped(subscribedAt, months)
	if candidate.After(now) {
		return candidate
	}
	return addMonthsClamped(subscribedAt, months+1)
}

func addMonthsClamped(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	targetMonth := time.Month(int(m) + months)
	targetYear := y + int(targetMonth-1)/12
	targetMonth = (targetMonth-1)%12 + 1
	lastDay := time.Date(targetYear, targetMonth+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if d > lastDay {
		d = lastDay
	}
	return time.Date(targetYear, targetMonth, d, 0, 0, 0, 0, time.UTC)
}
