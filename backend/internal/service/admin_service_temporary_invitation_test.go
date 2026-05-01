//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type adminTempUserRepoStub struct {
	user        *User
	updatedUser *User
}

func (s *adminTempUserRepoStub) Create(context.Context, *User) error { panic("unexpected Create call") }
func (s *adminTempUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	if s.user == nil {
		return nil, ErrUserNotFound
	}
	clone := *s.user
	return &clone, nil
}
func (s *adminTempUserRepoStub) GetByEmail(context.Context, string) (*User, error) {
	panic("unexpected GetByEmail call")
}
func (s *adminTempUserRepoStub) GetFirstAdmin(context.Context) (*User, error) {
	panic("unexpected GetFirstAdmin call")
}
func (s *adminTempUserRepoStub) Update(_ context.Context, user *User) error {
	clone := *user
	s.updatedUser = &clone
	return nil
}
func (s *adminTempUserRepoStub) Delete(context.Context, int64) error { panic("unexpected Delete call") }
func (s *adminTempUserRepoStub) List(context.Context, pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (s *adminTempUserRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, UserListFilters) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}
func (s *adminTempUserRepoStub) UpdateBalance(context.Context, int64, float64) error {
	panic("unexpected UpdateBalance call")
}
func (s *adminTempUserRepoStub) DeductBalance(context.Context, int64, float64) error {
	panic("unexpected DeductBalance call")
}
func (s *adminTempUserRepoStub) UpdateConcurrency(context.Context, int64, int) error {
	panic("unexpected UpdateConcurrency call")
}
func (s *adminTempUserRepoStub) ExistsByEmail(context.Context, string) (bool, error) {
	return false, nil
}
func (s *adminTempUserRepoStub) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	panic("unexpected RemoveGroupFromAllowedGroups call")
}
func (s *adminTempUserRepoStub) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected AddGroupToAllowedGroups call")
}
func (s *adminTempUserRepoStub) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected RemoveGroupFromUserAllowedGroups call")
}
func (s *adminTempUserRepoStub) UpdateTotpSecret(context.Context, int64, *string) error {
	panic("unexpected UpdateTotpSecret call")
}
func (s *adminTempUserRepoStub) EnableTotp(context.Context, int64) error {
	panic("unexpected EnableTotp call")
}
func (s *adminTempUserRepoStub) DisableTotp(context.Context, int64) error {
	panic("unexpected DisableTotp call")
}

func TestAdminService_UpdateUser_ReenableTemporaryInvitationResetsWindow(t *testing.T) {
	disabledAt := time.Now().Add(-time.Hour)
	repo := &adminTempUserRepoStub{
		user: &User{
			ID:                            17,
			Email:                         "temp-user@test.com",
			Role:                          RoleUser,
			Status:                        StatusDisabled,
			Concurrency:                   3,
			TemporaryInvitation:           true,
			TemporaryInvitationDisabledAt: &disabledAt,
		},
	}
	svc := &adminServiceImpl{
		userRepo:       repo,
		redeemCodeRepo: &redeemRepoStub{},
	}

	before := time.Now()
	updated, err := svc.UpdateUser(context.Background(), 17, &UpdateUserInput{
		Status: StatusActive,
	})
	after := time.Now()

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.NotNil(t, repo.updatedUser)
	require.Equal(t, StatusActive, repo.updatedUser.Status)
	require.True(t, repo.updatedUser.TemporaryInvitation)
	require.NotNil(t, repo.updatedUser.TemporaryInvitationDeadlineAt)
	require.Nil(t, repo.updatedUser.TemporaryInvitationDisabledAt)
	require.Nil(t, repo.updatedUser.TemporaryInvitationDeleteAt)
	require.WithinDuration(t, before.Add(TemporaryInvitationSignupWindow), *repo.updatedUser.TemporaryInvitationDeadlineAt, 2*time.Second)
	require.WithinDuration(t, after.Add(TemporaryInvitationSignupWindow), *repo.updatedUser.TemporaryInvitationDeadlineAt, 2*time.Second)
}
