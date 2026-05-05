//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type accountRepoStubForAdminCreate struct {
	accountRepoStub
	created       *Account
	bindAccountID int64
	bindGroupIDs  []int64
}

func (s *accountRepoStubForAdminCreate) Create(_ context.Context, account *Account) error {
	account.ID = 101
	s.created = account
	return nil
}

func (s *accountRepoStubForAdminCreate) BindGroups(_ context.Context, accountID int64, groupIDs []int64) error {
	s.bindAccountID = accountID
	s.bindGroupIDs = append([]int64{}, groupIDs...)
	return nil
}

func TestAdminService_CreateAccount_OpenAIOAuthDefaults(t *testing.T) {
	accountRepo := &accountRepoStubForAdminCreate{}
	groupRepo := &groupRepoStubForAdmin{
		listActiveByPlatformGroups: []Group{
			{ID: 10, Name: "openai-default", Platform: PlatformOpenAI, Status: StatusActive},
			{ID: 11, Name: "openai-oauth-only", Platform: PlatformOpenAI, Status: StatusActive, RequireOAuthOnly: true},
			{ID: 12, Name: "openai-disabled", Platform: PlatformOpenAI, Status: StatusDisabled},
			{ID: 13, Name: "anthropic-default", Platform: PlatformAnthropic, Status: StatusActive},
		},
	}
	svc := &adminServiceImpl{accountRepo: accountRepo, groupRepo: groupRepo}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                  "openai-auth",
		Platform:              PlatformOpenAI,
		Type:                  AccountTypeOAuth,
		Credentials:           map[string]any{"refresh_token": "rt"},
		SkipMixedChannelCheck: true,
	})

	require.NoError(t, err)
	require.Equal(t, int64(101), account.ID)
	require.Equal(t, PlatformOpenAI, groupRepo.listActiveByPlatform)
	require.NotNil(t, accountRepo.created)
	require.Equal(t, 3, accountRepo.created.Concurrency)
	require.NotNil(t, accountRepo.created.LoadFactor)
	require.Equal(t, 3, *accountRepo.created.LoadFactor)
	require.Equal(t, int64(101), accountRepo.bindAccountID)
	require.Equal(t, []int64{10, 11}, accountRepo.bindGroupIDs)
}

func TestAdminService_CreateAccount_OpenAIOAuthKeepsExplicitScheduling(t *testing.T) {
	loadFactor := 7
	accountRepo := &accountRepoStubForAdminCreate{}
	groupRepo := &groupRepoStubForAdmin{}
	svc := &adminServiceImpl{accountRepo: accountRepo, groupRepo: groupRepo}

	_, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                  "openai-auth",
		Platform:              PlatformOpenAI,
		Type:                  AccountTypeOAuth,
		Credentials:           map[string]any{"refresh_token": "rt"},
		Concurrency:           9,
		LoadFactor:            &loadFactor,
		GroupIDs:              []int64{20},
		SkipMixedChannelCheck: true,
	})

	require.NoError(t, err)
	require.NotNil(t, accountRepo.created)
	require.Equal(t, 9, accountRepo.created.Concurrency)
	require.NotNil(t, accountRepo.created.LoadFactor)
	require.Equal(t, 7, *accountRepo.created.LoadFactor)
	require.Equal(t, []int64{20}, accountRepo.bindGroupIDs)
	require.Empty(t, groupRepo.listActiveByPlatform)
}
