//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type proxyFailoverAccountRepoStub struct {
	accounts          map[int64]*Account
	updateErrByID     map[int64]error
	updatedAccountIDs []int64
	tempUnschedIDs    []int64
}

func (s *proxyFailoverAccountRepoStub) Create(ctx context.Context, account *Account) error {
	panic("unexpected Create call")
}

func (s *proxyFailoverAccountRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	if account, ok := s.accounts[id]; ok {
		return account, nil
	}
	return nil, nil
}

func (s *proxyFailoverAccountRepoStub) GetByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	accounts := make([]*Account, 0, len(ids))
	for _, id := range ids {
		if account, ok := s.accounts[id]; ok {
			accounts = append(accounts, account)
		}
	}
	return accounts, nil
}

func (s *proxyFailoverAccountRepoStub) ExistsByID(ctx context.Context, id int64) (bool, error) {
	_, ok := s.accounts[id]
	return ok, nil
}

func (s *proxyFailoverAccountRepoStub) GetByCRSAccountID(ctx context.Context, crsAccountID string) (*Account, error) {
	panic("unexpected GetByCRSAccountID call")
}

func (s *proxyFailoverAccountRepoStub) FindByExtraField(ctx context.Context, key string, value any) ([]Account, error) {
	panic("unexpected FindByExtraField call")
}

func (s *proxyFailoverAccountRepoStub) ListCRSAccountIDs(ctx context.Context) (map[string]int64, error) {
	panic("unexpected ListCRSAccountIDs call")
}

func (s *proxyFailoverAccountRepoStub) Update(ctx context.Context, account *Account) error {
	s.updatedAccountIDs = append(s.updatedAccountIDs, account.ID)
	if err := s.updateErrByID[account.ID]; err != nil {
		return err
	}
	s.accounts[account.ID] = account
	return nil
}

func (s *proxyFailoverAccountRepoStub) Delete(ctx context.Context, id int64) error {
	panic("unexpected Delete call")
}

func (s *proxyFailoverAccountRepoStub) List(ctx context.Context, params pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *proxyFailoverAccountRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (s *proxyFailoverAccountRepoStub) ListByGroup(ctx context.Context, groupID int64) ([]Account, error) {
	panic("unexpected ListByGroup call")
}

func (s *proxyFailoverAccountRepoStub) ListActive(ctx context.Context) ([]Account, error) {
	panic("unexpected ListActive call")
}

func (s *proxyFailoverAccountRepoStub) ListByPlatform(ctx context.Context, platform string) ([]Account, error) {
	panic("unexpected ListByPlatform call")
}

func (s *proxyFailoverAccountRepoStub) UpdateLastUsed(ctx context.Context, id int64) error {
	panic("unexpected UpdateLastUsed call")
}

func (s *proxyFailoverAccountRepoStub) BatchUpdateLastUsed(ctx context.Context, updates map[int64]time.Time) error {
	panic("unexpected BatchUpdateLastUsed call")
}

func (s *proxyFailoverAccountRepoStub) SetError(ctx context.Context, id int64, errorMsg string) error {
	panic("unexpected SetError call")
}

func (s *proxyFailoverAccountRepoStub) ClearError(ctx context.Context, id int64) error {
	panic("unexpected ClearError call")
}

func (s *proxyFailoverAccountRepoStub) SetSchedulable(ctx context.Context, id int64, schedulable bool) error {
	panic("unexpected SetSchedulable call")
}

func (s *proxyFailoverAccountRepoStub) AutoPauseExpiredAccounts(ctx context.Context, now time.Time) (int64, error) {
	panic("unexpected AutoPauseExpiredAccounts call")
}

func (s *proxyFailoverAccountRepoStub) BindGroups(ctx context.Context, accountID int64, groupIDs []int64) error {
	panic("unexpected BindGroups call")
}

func (s *proxyFailoverAccountRepoStub) ListSchedulable(ctx context.Context) ([]Account, error) {
	panic("unexpected ListSchedulable call")
}

func (s *proxyFailoverAccountRepoStub) ListSchedulableByGroupID(ctx context.Context, groupID int64) ([]Account, error) {
	panic("unexpected ListSchedulableByGroupID call")
}

func (s *proxyFailoverAccountRepoStub) ListSchedulableByPlatform(ctx context.Context, platform string) ([]Account, error) {
	panic("unexpected ListSchedulableByPlatform call")
}

func (s *proxyFailoverAccountRepoStub) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	panic("unexpected ListSchedulableByGroupIDAndPlatform call")
}

func (s *proxyFailoverAccountRepoStub) ListSchedulableByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	panic("unexpected ListSchedulableByPlatforms call")
}

func (s *proxyFailoverAccountRepoStub) ListSchedulableByGroupIDAndPlatforms(ctx context.Context, groupID int64, platforms []string) ([]Account, error) {
	panic("unexpected ListSchedulableByGroupIDAndPlatforms call")
}

func (s *proxyFailoverAccountRepoStub) ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]Account, error) {
	panic("unexpected ListSchedulableUngroupedByPlatform call")
}

func (s *proxyFailoverAccountRepoStub) ListSchedulableUngroupedByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	panic("unexpected ListSchedulableUngroupedByPlatforms call")
}

func (s *proxyFailoverAccountRepoStub) SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error {
	panic("unexpected SetRateLimited call")
}

func (s *proxyFailoverAccountRepoStub) SetModelRateLimit(ctx context.Context, id int64, scope string, resetAt time.Time) error {
	panic("unexpected SetModelRateLimit call")
}

func (s *proxyFailoverAccountRepoStub) SetOverloaded(ctx context.Context, id int64, until time.Time) error {
	panic("unexpected SetOverloaded call")
}

func (s *proxyFailoverAccountRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	s.tempUnschedIDs = append(s.tempUnschedIDs, id)
	account := s.accounts[id]
	account.TempUnschedulableUntil = &until
	account.TempUnschedulableReason = reason
	return nil
}

func (s *proxyFailoverAccountRepoStub) ClearTempUnschedulable(ctx context.Context, id int64) error {
	return nil
}

func (s *proxyFailoverAccountRepoStub) ClearRateLimit(ctx context.Context, id int64) error {
	panic("unexpected ClearRateLimit call")
}

func (s *proxyFailoverAccountRepoStub) ClearAntigravityQuotaScopes(ctx context.Context, id int64) error {
	panic("unexpected ClearAntigravityQuotaScopes call")
}

func (s *proxyFailoverAccountRepoStub) ClearModelRateLimits(ctx context.Context, id int64) error {
	panic("unexpected ClearModelRateLimits call")
}

func (s *proxyFailoverAccountRepoStub) UpdateSessionWindow(ctx context.Context, id int64, start, end *time.Time, status string) error {
	panic("unexpected UpdateSessionWindow call")
}

func (s *proxyFailoverAccountRepoStub) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	panic("unexpected UpdateExtra call")
}

func (s *proxyFailoverAccountRepoStub) BulkUpdate(ctx context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	panic("unexpected BulkUpdate call")
}

func (s *proxyFailoverAccountRepoStub) IncrementQuotaUsed(ctx context.Context, id int64, amount float64) error {
	panic("unexpected IncrementQuotaUsed call")
}

func (s *proxyFailoverAccountRepoStub) ResetQuotaUsed(ctx context.Context, id int64) error {
	panic("unexpected ResetQuotaUsed call")
}

type proxyFailoverProxyRepoStub struct {
	proxies           map[int64]*Proxy
	activeWithAccount []ProxyWithAccountCount
	accountIDsByProxy map[int64][]ProxyAccountSummary
}

func (s *proxyFailoverProxyRepoStub) Create(ctx context.Context, proxy *Proxy) error {
	panic("unexpected Create call")
}

func (s *proxyFailoverProxyRepoStub) GetByID(ctx context.Context, id int64) (*Proxy, error) {
	if proxy, ok := s.proxies[id]; ok {
		return proxy, nil
	}
	return nil, nil
}

func (s *proxyFailoverProxyRepoStub) ListByIDs(ctx context.Context, ids []int64) ([]Proxy, error) {
	panic("unexpected ListByIDs call")
}

func (s *proxyFailoverProxyRepoStub) Update(ctx context.Context, proxy *Proxy) error {
	panic("unexpected Update call")
}

func (s *proxyFailoverProxyRepoStub) Delete(ctx context.Context, id int64) error {
	panic("unexpected Delete call")
}

func (s *proxyFailoverProxyRepoStub) List(ctx context.Context, params pagination.PaginationParams) ([]Proxy, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *proxyFailoverProxyRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]Proxy, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (s *proxyFailoverProxyRepoStub) ListWithFiltersAndAccountCount(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]ProxyWithAccountCount, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFiltersAndAccountCount call")
}

func (s *proxyFailoverProxyRepoStub) ListActive(ctx context.Context) ([]Proxy, error) {
	panic("unexpected ListActive call")
}

func (s *proxyFailoverProxyRepoStub) ListActiveWithAccountCount(ctx context.Context) ([]ProxyWithAccountCount, error) {
	return s.activeWithAccount, nil
}

func (s *proxyFailoverProxyRepoStub) ExistsByHostPortAuth(ctx context.Context, host string, port int, username, password string) (bool, error) {
	panic("unexpected ExistsByHostPortAuth call")
}

func (s *proxyFailoverProxyRepoStub) CountAccountsByProxyID(ctx context.Context, proxyID int64) (int64, error) {
	panic("unexpected CountAccountsByProxyID call")
}

func (s *proxyFailoverProxyRepoStub) ListAccountSummariesByProxyID(ctx context.Context, proxyID int64) ([]ProxyAccountSummary, error) {
	return s.accountIDsByProxy[proxyID], nil
}

type proxyFailoverProberStub struct{}

func (s *proxyFailoverProberStub) ProbeProxy(ctx context.Context, proxyURL string) (*ProxyExitInfo, int64, error) {
	return &ProxyExitInfo{
		IP:          "203.0.113.10",
		Country:     "Japan",
		CountryCode: "JP",
		Region:      "Tokyo",
		City:        "Tokyo",
	}, 120, nil
}

func TestProxyFailoverService_IsolateProxyTempUnschedulesOnlyFailedAccounts(t *testing.T) {
	t.Parallel()

	sourceProxyID := int64(11)
	targetProxyID := int64(22)

	accountRepo := &proxyFailoverAccountRepoStub{
		accounts: map[int64]*Account{
			1: {ID: 1, Name: "acct-1", Platform: PlatformOpenAI, Type: AccountTypeOAuth, ProxyID: &sourceProxyID, Status: StatusActive, Schedulable: true},
			2: {ID: 2, Name: "acct-2", Platform: PlatformOpenAI, Type: AccountTypeOAuth, ProxyID: &sourceProxyID, Status: StatusActive, Schedulable: true},
			3: {ID: 3, Name: "acct-3", Platform: PlatformOpenAI, Type: AccountTypeOAuth, ProxyID: &sourceProxyID, Status: StatusActive, Schedulable: true},
		},
		updateErrByID: map[int64]error{
			1: errors.New("write failed"),
		},
	}

	proxyRepo := &proxyFailoverProxyRepoStub{
		proxies: map[int64]*Proxy{
			sourceProxyID: {ID: sourceProxyID, Name: "source", Protocol: "http", Host: "source.example.com", Port: 8080, Status: StatusActive},
		},
		activeWithAccount: []ProxyWithAccountCount{
			{Proxy: Proxy{ID: sourceProxyID, Name: "source", Protocol: "http", Host: "source.example.com", Port: 8080, Status: StatusActive}, AccountCount: 3},
			{Proxy: Proxy{ID: targetProxyID, Name: "target", Protocol: "http", Host: "target.example.com", Port: 8080, Status: StatusActive}, AccountCount: 1},
		},
		accountIDsByProxy: map[int64][]ProxyAccountSummary{
			sourceProxyID: {
				{ID: 1},
				{ID: 2},
				{ID: 3},
			},
		},
	}

	svc := NewProxyFailoverService(nil, accountRepo, proxyRepo, &proxyFailoverProberStub{}, nil, nil, nil)

	svc.isolateProxy(context.Background(), sourceProxyID, ProxyFailoverSettings{
		Enabled:               true,
		OnlyOpenAIOAuth:       true,
		MaxAccountsPerProxy:   10,
		MaxMigrationsPerCycle: 10,
		TempUnschedMinutes:    10,
		CooldownMinutes:       15,
	}, "upstream_http_502")

	require.Equal(t, []int64{1, 2, 3}, accountRepo.updatedAccountIDs)
	require.Equal(t, []int64{1}, accountRepo.tempUnschedIDs)
	require.Equal(t, targetProxyID, *accountRepo.accounts[2].ProxyID)
	require.Equal(t, targetProxyID, *accountRepo.accounts[3].ProxyID)
	require.NotNil(t, accountRepo.accounts[1].TempUnschedulableUntil)
	require.Nil(t, accountRepo.accounts[2].TempUnschedulableUntil)
	require.Nil(t, accountRepo.accounts[3].TempUnschedulableUntil)
}
