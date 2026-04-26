package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
)

const proxyFailoverLoopTick = time.Minute

type proxyFailureState struct {
	failCount        int
	windowStartedAt  time.Time
	lastFailureAt    time.Time
	lastSuccessAt    time.Time
	lastMessage      string
	lastStatusCode   int
	lastProbeFailed  bool
	distinctAccounts map[int64]time.Time
	unhealthyUntil   time.Time
	migrationRunning bool
}

type proxyFailoverCandidate struct {
	proxy      ProxyWithAccountCount
	score      int
	latencyMs  int64
	sameRegion bool
}

type ProxyFailoverService struct {
	settingService    *SettingService
	accountRepo       AccountRepository
	proxyRepo         ProxyRepository
	proxyProber       ProxyExitInfoProber
	proxyLatencyCache ProxyLatencyCache
	tempUnschedCache  TempUnschedCache
	schedulerSnapshot *SchedulerSnapshotService

	stopCh chan struct{}
	doneCh chan struct{}

	mu           sync.Mutex
	states       map[int64]*proxyFailureState
	lastProbeRun time.Time
}

func NewProxyFailoverService(
	settingService *SettingService,
	accountRepo AccountRepository,
	proxyRepo ProxyRepository,
	proxyProber ProxyExitInfoProber,
	proxyLatencyCache ProxyLatencyCache,
	tempUnschedCache TempUnschedCache,
	schedulerSnapshot *SchedulerSnapshotService,
) *ProxyFailoverService {
	return &ProxyFailoverService{
		settingService:    settingService,
		accountRepo:       accountRepo,
		proxyRepo:         proxyRepo,
		proxyProber:       proxyProber,
		proxyLatencyCache: proxyLatencyCache,
		tempUnschedCache:  tempUnschedCache,
		schedulerSnapshot: schedulerSnapshot,
		stopCh:            make(chan struct{}),
		doneCh:            make(chan struct{}),
		states:            make(map[int64]*proxyFailureState),
	}
}

func (s *ProxyFailoverService) Start() {
	if s == nil {
		return
	}
	go s.run()
}

func (s *ProxyFailoverService) Stop() {
	if s == nil {
		return
	}
	select {
	case <-s.stopCh:
		return
	default:
		close(s.stopCh)
	}
	select {
	case <-s.doneCh:
	case <-time.After(5 * time.Second):
	}
}

func (s *ProxyFailoverService) run() {
	defer close(s.doneCh)
	ticker := time.NewTicker(proxyFailoverLoopTick)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.maybeRunAutoProbe(context.Background())
		case <-s.stopCh:
			return
		}
	}
}

func (s *ProxyFailoverService) maybeRunAutoProbe(ctx context.Context) {
	settings, err := s.currentSettings(ctx)
	if err != nil {
		slog.Warn("proxy_failover.load_settings_failed", "error", err)
		return
	}
	if !settings.ProxyFailover.Enabled || !settings.ProxyFailover.AutoTestEnabled {
		return
	}

	s.mu.Lock()
	nextAllowed := s.lastProbeRun.Add(time.Duration(settings.ProxyFailover.ProbeIntervalMinutes) * time.Minute)
	if !s.lastProbeRun.IsZero() && time.Now().Before(nextAllowed) {
		s.mu.Unlock()
		return
	}
	s.lastProbeRun = time.Now()
	s.mu.Unlock()

	if err := s.RunProbeCycle(ctx); err != nil {
		slog.Warn("proxy_failover.auto_probe_failed", "error", err)
	}
}

func (s *ProxyFailoverService) RunProbeCycle(ctx context.Context) error {
	if s == nil || s.proxyRepo == nil || s.proxyProber == nil {
		return nil
	}
	settings, err := s.currentSettings(ctx)
	if err != nil {
		return err
	}
	if !settings.ProxyFailover.Enabled || !settings.ProxyFailover.AutoTestEnabled {
		return nil
	}

	proxies, err := s.proxyRepo.ListActiveWithAccountCount(ctx)
	if err != nil {
		return fmt.Errorf("list active proxies: %w", err)
	}
	for i := range proxies {
		proxy := proxies[i]
		if proxy.ID <= 0 {
			continue
		}
		s.runSingleProxyProbe(ctx, &proxy, settings.ProxyFailover)
	}
	return nil
}

func (s *ProxyFailoverService) RecordUpstreamFailure(ctx context.Context, account *Account, statusCode int, message string) {
	if s == nil || account == nil || account.ProxyID == nil || *account.ProxyID <= 0 {
		return
	}
	settings, err := s.currentSettings(ctx)
	if err != nil || !settings.ProxyFailover.Enabled {
		return
	}
	if settings.ProxyFailover.OnlyOpenAIOAuth && !(account.Platform == PlatformOpenAI && account.Type == AccountTypeOAuth) {
		return
	}
	if !shouldCountProxyUpstreamFailure(statusCode) {
		return
	}

	proxyID := *account.ProxyID
	shouldIsolate := s.recordFailure(proxyID, account.ID, statusCode, message, settings.ProxyFailover, false)
	if shouldIsolate {
		go s.isolateProxy(context.Background(), proxyID, settings.ProxyFailover, fmt.Sprintf("upstream_http_%d", statusCode))
	}
}

func shouldCountProxyUpstreamFailure(statusCode int) bool {
	switch statusCode {
	case 500, 502, 503, 504, 520, 521, 522, 523, 524:
		return true
	default:
		return false
	}
}

func (s *ProxyFailoverService) runSingleProxyProbe(ctx context.Context, proxy *ProxyWithAccountCount, settings ProxyFailoverSettings) {
	if proxy == nil || proxy.ID <= 0 || s.proxyProber == nil {
		return
	}
	proxyURL := proxy.URL()
	exitInfo, latencyMs, err := s.proxyProber.ProbeProxy(ctx, proxyURL)
	if err != nil {
		s.storeProbeLatency(ctx, proxy.ID, &ProxyLatencyInfo{
			Success:   false,
			Message:   err.Error(),
			UpdatedAt: time.Now(),
		})
		if s.recordFailure(proxy.ID, 0, 0, err.Error(), settings, true) {
			go s.isolateProxy(context.Background(), proxy.ID, settings, "scheduled_probe_failed")
		}
		return
	}

	s.markProxyHealthy(proxy.ID)
	latency := latencyMs
	s.storeProbeLatency(ctx, proxy.ID, &ProxyLatencyInfo{
		Success:     true,
		LatencyMs:   &latency,
		Message:     "Proxy is accessible",
		IPAddress:   exitInfo.IP,
		Country:     exitInfo.Country,
		CountryCode: exitInfo.CountryCode,
		Region:      exitInfo.Region,
		City:        exitInfo.City,
		UpdatedAt:   time.Now(),
	})
}

func (s *ProxyFailoverService) currentSettings(ctx context.Context) (*SchedulingMechanismSettings, error) {
	if s == nil || s.settingService == nil {
		return DefaultSchedulingMechanismSettings(), nil
	}
	return s.settingService.GetSchedulingMechanismSettings(ctx)
}

func (s *ProxyFailoverService) ensureState(proxyID int64) *proxyFailureState {
	state, ok := s.states[proxyID]
	if ok && state != nil {
		return state
	}
	state = &proxyFailureState{
		distinctAccounts: make(map[int64]time.Time),
	}
	s.states[proxyID] = state
	return state
}

func (s *ProxyFailoverService) pruneDistinctAccounts(state *proxyFailureState, now time.Time, window time.Duration) {
	if state == nil || len(state.distinctAccounts) == 0 {
		return
	}
	cutoff := now.Add(-window)
	for accountID, seenAt := range state.distinctAccounts {
		if seenAt.Before(cutoff) {
			delete(state.distinctAccounts, accountID)
		}
	}
}

func (s *ProxyFailoverService) recordFailure(
	proxyID int64,
	accountID int64,
	statusCode int,
	message string,
	settings ProxyFailoverSettings,
	fromProbe bool,
) bool {
	now := time.Now()
	window := time.Duration(settings.FailureWindowMinutes) * time.Minute

	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.ensureState(proxyID)
	if state.windowStartedAt.IsZero() || now.Sub(state.windowStartedAt) > window {
		state.failCount = 0
		state.windowStartedAt = now
		state.distinctAccounts = make(map[int64]time.Time)
	}
	state.failCount++
	state.lastFailureAt = now
	state.lastMessage = strings.TrimSpace(message)
	state.lastStatusCode = statusCode
	state.lastProbeFailed = fromProbe
	s.pruneDistinctAccounts(state, now, window)
	if accountID > 0 {
		state.distinctAccounts[accountID] = now
	}
	if state.migrationRunning || now.Before(state.unhealthyUntil) {
		return false
	}
	if state.failCount < settings.FailureThreshold {
		return false
	}
	if fromProbe || len(state.distinctAccounts) >= 2 {
		state.migrationRunning = true
		return true
	}
	return false
}

func (s *ProxyFailoverService) markProxyHealthy(proxyID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.ensureState(proxyID)
	state.failCount = 0
	state.windowStartedAt = time.Time{}
	state.lastSuccessAt = time.Now()
	state.lastProbeFailed = false
	state.distinctAccounts = make(map[int64]time.Time)
	if time.Now().After(state.unhealthyUntil) {
		state.unhealthyUntil = time.Time{}
	}
}

func (s *ProxyFailoverService) storeProbeLatency(ctx context.Context, proxyID int64, update *ProxyLatencyInfo) {
	if s == nil || s.proxyLatencyCache == nil || update == nil {
		return
	}
	merged := *update
	if existing, err := s.proxyLatencyCache.GetProxyLatencies(ctx, []int64{proxyID}); err == nil {
		if current := existing[proxyID]; current != nil {
			if merged.QualityStatus == "" {
				merged.QualityStatus = current.QualityStatus
				merged.QualityScore = current.QualityScore
				merged.QualityGrade = current.QualityGrade
				merged.QualitySummary = current.QualitySummary
				merged.QualityCheckedAt = current.QualityCheckedAt
				merged.QualityCFRay = current.QualityCFRay
			}
		}
	}
	if err := s.proxyLatencyCache.SetProxyLatency(ctx, proxyID, &merged); err != nil {
		slog.Warn("proxy_failover.store_latency_failed", "proxy_id", proxyID, "error", err)
	}
}

func (s *ProxyFailoverService) isolateProxy(ctx context.Context, proxyID int64, settings ProxyFailoverSettings, reason string) {
	defer func() {
		s.mu.Lock()
		if state := s.ensureState(proxyID); state != nil {
			state.migrationRunning = false
			state.unhealthyUntil = time.Now().Add(time.Duration(settings.CooldownMinutes) * time.Minute)
		}
		s.mu.Unlock()
	}()

	if s.proxyRepo == nil || s.accountRepo == nil {
		return
	}
	proxy, err := s.proxyRepo.GetByID(ctx, proxyID)
	if err != nil || proxy == nil {
		return
	}

	accountIDs, err := s.listProxyAccountIDs(ctx, proxyID)
	if err != nil || len(accountIDs) == 0 {
		return
	}
	accounts, err := s.accountRepo.GetByIDs(ctx, accountIDs)
	if err != nil || len(accounts) == 0 {
		return
	}

	eligible := make([]*Account, 0, len(accounts))
	for _, account := range accounts {
		if account == nil {
			continue
		}
		if settings.OnlyOpenAIOAuth && !(account.Platform == PlatformOpenAI && account.Type == AccountTypeOAuth) {
			continue
		}
		if !account.IsActive() {
			continue
		}
		eligible = append(eligible, account)
	}
	if len(eligible) == 0 {
		return
	}

	sourceCountry := s.lookupProxyCountry(ctx, proxyID)
	targets, err := s.listHealthyTargetProxies(ctx, proxyID, sourceCountry, settings)
	if err != nil {
		slog.Warn("proxy_failover.list_targets_failed", "proxy_id", proxyID, "error", err)
		s.tempUnscheduleAccounts(ctx, eligible, proxyID, reason, settings.TempUnschedMinutes)
		return
	}
	if len(targets) == 0 {
		s.tempUnscheduleAccounts(ctx, eligible, proxyID, reason, settings.TempUnschedMinutes)
		return
	}

	projectedLoad := make(map[int64]int64, len(targets))
	for _, target := range targets {
		projectedLoad[target.ID] = target.AccountCount
	}

	migrated := 0
	failedAccounts := make([]*Account, 0, len(eligible))
	for i, account := range eligible {
		if settings.MaxMigrationsPerCycle > 0 && migrated >= settings.MaxMigrationsPerCycle {
			failedAccounts = append(failedAccounts, eligible[i:]...)
			break
		}
		target := selectProxyFailoverTarget(targets, projectedLoad, settings)
		if target == nil {
			failedAccounts = append(failedAccounts, account)
			continue
		}
		if err := s.migrateAccountToProxy(ctx, account, proxy, target, reason); err != nil {
			slog.Warn("proxy_failover.migrate_account_failed", "account_id", account.ID, "source_proxy_id", proxy.ID, "target_proxy_id", target.ID, "error", err)
			failedAccounts = append(failedAccounts, account)
			continue
		}
		projectedLoad[target.ID]++
		migrated++
	}

	if len(failedAccounts) > 0 {
		s.tempUnscheduleAccounts(ctx, failedAccounts, proxyID, reason, settings.TempUnschedMinutes)
	}
}

func (s *ProxyFailoverService) listProxyAccountIDs(ctx context.Context, proxyID int64) ([]int64, error) {
	summaries, err := s.proxyRepo.ListAccountSummariesByProxyID(ctx, proxyID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(summaries))
	for _, summary := range summaries {
		if summary.ID > 0 {
			ids = append(ids, summary.ID)
		}
	}
	return ids, nil
}

func (s *ProxyFailoverService) lookupProxyCountry(ctx context.Context, proxyID int64) string {
	if s == nil || s.proxyLatencyCache == nil {
		return ""
	}
	latencies, err := s.proxyLatencyCache.GetProxyLatencies(ctx, []int64{proxyID})
	if err != nil {
		return ""
	}
	if latency := latencies[proxyID]; latency != nil {
		return strings.TrimSpace(latency.CountryCode)
	}
	return ""
}

func (s *ProxyFailoverService) listHealthyTargetProxies(
	ctx context.Context,
	sourceProxyID int64,
	sourceCountry string,
	settings ProxyFailoverSettings,
) ([]ProxyWithAccountCount, error) {
	proxies, err := s.proxyRepo.ListActiveWithAccountCount(ctx)
	if err != nil {
		return nil, err
	}

	candidates := make([]proxyFailoverCandidate, 0, len(proxies))
	for _, proxy := range proxies {
		if proxy.ID <= 0 || proxy.ID == sourceProxyID || !proxy.IsActive() {
			continue
		}
		if s.isProxyCoolingDown(proxy.ID) {
			continue
		}
		if settings.MaxAccountsPerProxy > 0 && proxy.AccountCount >= int64(settings.MaxAccountsPerProxy) {
			continue
		}
		healthy, updatedProxy := s.ensureProxyHealthyForTarget(ctx, proxy)
		if !healthy {
			continue
		}
		candidate := proxyFailoverCandidate{
			proxy:     updatedProxy,
			score:     0,
			latencyMs: int64(^uint64(0) >> 1),
		}
		if settings.PreferSameCountry && sourceCountry != "" && strings.EqualFold(updatedProxy.CountryCode, sourceCountry) {
			candidate.sameRegion = true
			candidate.score -= 1000
		}
		if updatedProxy.LatencyMs != nil {
			candidate.latencyMs = *updatedProxy.LatencyMs
		}
		candidates = append(candidates, candidate)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score < candidates[j].score
		}
		if candidates[i].proxy.AccountCount != candidates[j].proxy.AccountCount {
			return candidates[i].proxy.AccountCount < candidates[j].proxy.AccountCount
		}
		if candidates[i].latencyMs != candidates[j].latencyMs {
			return candidates[i].latencyMs < candidates[j].latencyMs
		}
		return candidates[i].proxy.ID < candidates[j].proxy.ID
	})

	result := make([]ProxyWithAccountCount, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, candidate.proxy)
	}

	return result, nil
}

func (s *ProxyFailoverService) ensureProxyHealthyForTarget(ctx context.Context, proxy ProxyWithAccountCount) (bool, ProxyWithAccountCount) {
	if s.proxyLatencyCache != nil {
		if latencies, err := s.proxyLatencyCache.GetProxyLatencies(ctx, []int64{proxy.ID}); err == nil {
			if info := latencies[proxy.ID]; info != nil {
				if info.Success {
					proxy.LatencyStatus = "success"
					proxy.LatencyMs = info.LatencyMs
					proxy.CountryCode = info.CountryCode
					proxy.Country = info.Country
					proxy.Region = info.Region
					proxy.City = info.City
					proxy.QualityStatus = info.QualityStatus
					if info.QualityStatus == "failed" || info.QualityStatus == "challenge" {
						return false, proxy
					}
					return true, proxy
				}
				return false, proxy
			}
		}
	}
	if s.proxyProber == nil {
		return false, proxy
	}
	exitInfo, latencyMs, err := s.proxyProber.ProbeProxy(ctx, proxy.URL())
	if err != nil {
		s.storeProbeLatency(ctx, proxy.ID, &ProxyLatencyInfo{
			Success:   false,
			Message:   err.Error(),
			UpdatedAt: time.Now(),
		})
		return false, proxy
	}
	latency := latencyMs
	proxy.LatencyStatus = "success"
	proxy.LatencyMs = &latency
	proxy.CountryCode = exitInfo.CountryCode
	proxy.Country = exitInfo.Country
	proxy.Region = exitInfo.Region
	proxy.City = exitInfo.City
	s.storeProbeLatency(ctx, proxy.ID, &ProxyLatencyInfo{
		Success:     true,
		LatencyMs:   &latency,
		Message:     "Proxy is accessible",
		IPAddress:   exitInfo.IP,
		Country:     exitInfo.Country,
		CountryCode: exitInfo.CountryCode,
		Region:      exitInfo.Region,
		City:        exitInfo.City,
		UpdatedAt:   time.Now(),
	})
	return true, proxy
}

func (s *ProxyFailoverService) isProxyCoolingDown(proxyID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.ensureState(proxyID)
	return time.Now().Before(state.unhealthyUntil)
}

func selectProxyFailoverTarget(
	targets []ProxyWithAccountCount,
	projectedLoad map[int64]int64,
	settings ProxyFailoverSettings,
) *ProxyWithAccountCount {
	for i := range targets {
		target := &targets[i]
		if settings.MaxAccountsPerProxy > 0 && projectedLoad[target.ID] >= int64(settings.MaxAccountsPerProxy) {
			continue
		}
		return target
	}
	return nil
}

func (s *ProxyFailoverService) migrateAccountToProxy(
	ctx context.Context,
	account *Account,
	sourceProxy *Proxy,
	targetProxy *ProxyWithAccountCount,
	reason string,
) error {
	if account == nil || sourceProxy == nil || targetProxy == nil {
		return nil
	}
	targetProxyID := targetProxy.ID
	account.ProxyID = &targetProxyID
	account.Proxy = &targetProxy.Proxy
	if account.Extra == nil {
		account.Extra = make(map[string]any)
	}
	account.Extra["proxy_failover_original_proxy_id"] = sourceProxy.ID
	account.Extra["proxy_failover_last_source_proxy_id"] = sourceProxy.ID
	account.Extra["proxy_failover_last_target_proxy_id"] = targetProxy.ID
	account.Extra["proxy_failover_last_reason"] = reason
	account.Extra["proxy_failover_last_migrated_at"] = time.Now().UTC().Format(time.RFC3339)
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return err
	}
	_ = s.accountRepo.ClearTempUnschedulable(ctx, account.ID)
	if s.tempUnschedCache != nil {
		_ = s.tempUnschedCache.DeleteTempUnsched(ctx, account.ID)
	}
	account.TempUnschedulableUntil = nil
	account.TempUnschedulableReason = ""
	if s.schedulerSnapshot != nil {
		_ = s.schedulerSnapshot.UpdateAccountInCache(ctx, account)
	}
	slog.Info("proxy_failover.account_migrated",
		"account_id", account.ID,
		"source_proxy_id", sourceProxy.ID,
		"target_proxy_id", targetProxy.ID,
		"reason", reason,
	)
	return nil
}

func (s *ProxyFailoverService) tempUnscheduleAccounts(
	ctx context.Context,
	accounts []*Account,
	sourceProxyID int64,
	reason string,
	minutes int,
) {
	if len(accounts) == 0 || minutes <= 0 {
		return
	}
	until := time.Now().Add(time.Duration(minutes) * time.Minute)
	for _, account := range accounts {
		if account == nil {
			continue
		}
		msg := fmt.Sprintf("proxy %d unhealthy (%s), waiting for healthy failover proxy", sourceProxyID, reason)
		if err := s.accountRepo.SetTempUnschedulable(ctx, account.ID, until, msg); err != nil {
			slog.Warn("proxy_failover.set_temp_unsched_failed", "account_id", account.ID, "proxy_id", sourceProxyID, "error", err)
			continue
		}
		if s.tempUnschedCache != nil {
			_ = s.tempUnschedCache.SetTempUnsched(ctx, account.ID, &TempUnschedState{
				UntilUnix:       until.Unix(),
				TriggeredAtUnix: time.Now().Unix(),
				ErrorMessage:    msg,
			})
		}
		account.TempUnschedulableUntil = &until
		account.TempUnschedulableReason = msg
		if s.schedulerSnapshot != nil {
			_ = s.schedulerSnapshot.UpdateAccountInCache(ctx, account)
		}
	}
}
