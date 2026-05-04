//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type affiliateRepoStub struct {
	invitee        *AffiliateSummary
	inviter        *AffiliateSummary
	accruedOrderID *int64
	overview       *AffiliateUserOverview
}

func (s *affiliateRepoStub) EnsureUserAffiliate(_ context.Context, userID int64) (*AffiliateSummary, error) {
	switch userID {
	case s.invitee.UserID:
		return s.invitee, nil
	case s.inviter.UserID:
		return s.inviter, nil
	default:
		return nil, ErrUserNotFound
	}
}

func (s *affiliateRepoStub) GetAffiliateByCode(context.Context, string) (*AffiliateSummary, error) {
	return nil, ErrAffiliateProfileNotFound
}

func (s *affiliateRepoStub) BindInviter(context.Context, int64, int64) (bool, error) {
	return false, nil
}

func (s *affiliateRepoStub) AccrueQuota(_ context.Context, _ int64, _ int64, _ float64, _ int, sourceOrderID *int64) (bool, error) {
	s.accruedOrderID = sourceOrderID
	return true, nil
}

func (s *affiliateRepoStub) GetAccruedRebateFromInvitee(context.Context, int64, int64) (float64, error) {
	return 0, nil
}

func (s *affiliateRepoStub) ThawFrozenQuota(context.Context, int64) (float64, error) {
	return 0, nil
}

func (s *affiliateRepoStub) TransferQuotaToBalance(context.Context, int64) (float64, float64, error) {
	return 0, 0, nil
}

func (s *affiliateRepoStub) ListInvitees(context.Context, int64, int) ([]AffiliateInvitee, error) {
	return nil, nil
}

func (s *affiliateRepoStub) UpdateUserAffCode(context.Context, int64, string) error {
	return nil
}

func (s *affiliateRepoStub) ResetUserAffCode(context.Context, int64) (string, error) {
	return "", nil
}

func (s *affiliateRepoStub) SetUserRebateRate(context.Context, int64, *float64) error {
	return nil
}

func (s *affiliateRepoStub) BatchSetUserRebateRate(context.Context, []int64, *float64) error {
	return nil
}

func (s *affiliateRepoStub) ListUsersWithCustomSettings(context.Context, AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	return nil, 0, nil
}

func (s *affiliateRepoStub) ListAffiliateInviteRecords(context.Context, AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error) {
	return nil, 0, nil
}

func (s *affiliateRepoStub) ListAffiliateRebateRecords(context.Context, AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error) {
	return nil, 0, nil
}

func (s *affiliateRepoStub) ListAffiliateTransferRecords(context.Context, AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error) {
	return nil, 0, nil
}

func (s *affiliateRepoStub) GetAffiliateUserOverview(context.Context, int64) (*AffiliateUserOverview, error) {
	return s.overview, nil
}

func TestAffiliateService_AccrueInviteRebateForOrder_PropagatesSourceOrderID(t *testing.T) {
	inviterID := int64(7)
	orderID := int64(99)
	repo := &affiliateRepoStub{
		invitee: &AffiliateSummary{UserID: 42, InviterID: &inviterID, CreatedAt: time.Now()},
		inviter: &AffiliateSummary{UserID: inviterID},
	}
	settings := NewSettingService(&settingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:           "true",
		SettingKeyAffiliateRebateRate:        "10",
		SettingKeyAffiliateRebateFreezeHours: "0",
	}}, &config.Config{})
	svc := NewAffiliateService(repo, settings, nil, nil)

	rebate, err := svc.AccrueInviteRebateForOrder(context.Background(), 42, 100, &orderID)

	require.NoError(t, err)
	require.Equal(t, 10.0, rebate)
	require.NotNil(t, repo.accruedOrderID)
	require.Equal(t, orderID, *repo.accruedOrderID)
}

func TestAffiliateService_AccrueInviteRebate_RemainsCompatibleWithoutSourceOrder(t *testing.T) {
	inviterID := int64(7)
	repo := &affiliateRepoStub{
		invitee: &AffiliateSummary{UserID: 42, InviterID: &inviterID, CreatedAt: time.Now()},
		inviter: &AffiliateSummary{UserID: inviterID},
	}
	settings := NewSettingService(&settingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:    "true",
		SettingKeyAffiliateRebateRate: "10",
	}}, &config.Config{})
	svc := NewAffiliateService(repo, settings, nil, nil)

	rebate, err := svc.AccrueInviteRebate(context.Background(), 42, 100)

	require.NoError(t, err)
	require.Equal(t, 10.0, rebate)
	require.Nil(t, repo.accruedOrderID)
}

func TestAffiliateService_AdminGetUserOverview_UsesGlobalRateWhenUserHasNoCustomRate(t *testing.T) {
	repo := &affiliateRepoStub{
		invitee: &AffiliateSummary{UserID: 1},
		inviter: &AffiliateSummary{UserID: 2},
		overview: &AffiliateUserOverview{
			UserID:           7,
			RebateRateCustom: false,
		},
	}
	settings := NewSettingService(&settingRepoStub{values: map[string]string{
		SettingKeyAffiliateRebateRate: "12.5",
	}}, &config.Config{})
	svc := NewAffiliateService(repo, settings, nil, nil)

	overview, err := svc.AdminGetUserOverview(context.Background(), 7)

	require.NoError(t, err)
	require.Equal(t, 12.5, overview.RebateRatePercent)
}
