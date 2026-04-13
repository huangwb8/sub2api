//go:build unit

package service

import (
	"context"
	"testing"
	"github.com/stretchr/testify/require"
)

type subscriptionEmailCall struct {
	email   string
	subject string
	body    string
}

type subscriptionEmailerStub struct {
	calls []subscriptionEmailCall
	err   error
}

func (s *subscriptionEmailerStub) EnqueueCustomEmail(email, subject, body string) error {
	s.calls = append(s.calls, subscriptionEmailCall{
		email:   email,
		subject: subject,
		body:    body,
	})
	return s.err
}

type subscriptionNotifyUserRepoStub struct {
	mockUserRepo
	users map[int64]*User
}

func (s *subscriptionNotifyUserRepoStub) GetByID(_ context.Context, id int64) (*User, error) {
	if user, ok := s.users[id]; ok {
		cp := *user
		return &cp, nil
	}
	return nil, ErrUserNotFound
}

func TestAssignSubscriptionSendsAdminNotificationForNewSubscription(t *testing.T) {
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{
			ID:                  1,
			Name:                "OpenAI Pro",
			Platform:            PlatformOpenAI,
			SubscriptionType:    SubscriptionTypeSubscription,
			RequireOAuthOnly:    true,
			AccountCount:        2,
			ActiveAccountCount:  1,
			DefaultValidityDays: 30,
		},
	}
	subRepo := newSubscriptionUserSubRepoStub()
	userRepo := &subscriptionNotifyUserRepoStub{
		users: map[int64]*User{
			1001: {
				ID:       1001,
				Email:    "buyer@example.com",
				Username: "buyer",
			},
		},
	}
	emailer := &subscriptionEmailerStub{}

	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
	svc.SetAdminNotificationDeps(&settingRepoStub{values: map[string]string{
		SettingKeySubscriptionNotificationEmail: "ops@example.com",
		SettingKeySiteName:                     "Sub2API",
	}}, userRepo, emailer)

	sub, assignErr := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       1001,
		GroupID:      1,
		ValidityDays: 30,
		Notes:        "payment order 123",
	})
	require.NoError(t, assignErr)
	require.NotNil(t, sub)
	require.Len(t, emailer.calls, 1)
	require.Equal(t, "ops@example.com", emailer.calls[0].email)
	require.Contains(t, emailer.calls[0].subject, "OpenAI Pro")
	require.Contains(t, emailer.calls[0].body, "buyer@example.com")
	require.Contains(t, emailer.calls[0].body, "建议补充 1 个")
	require.Contains(t, emailer.calls[0].body, "OAuth")
}

func TestAssignSubscriptionDoesNotSendAdminNotificationWhenReused(t *testing.T) {
	start := MaxExpiresAt.AddDate(-10, 0, 0)
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{
			ID:               1,
			Name:             "Anthropic Pro",
			Platform:         PlatformAnthropic,
			SubscriptionType: SubscriptionTypeSubscription,
		},
	}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        10,
		UserID:    1001,
		GroupID:   1,
		StartsAt:  start,
		ExpiresAt: start.AddDate(0, 0, 30),
		Notes:     "same-note",
	})
	emailer := &subscriptionEmailerStub{}

	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
	svc.SetAdminNotificationDeps(&settingRepoStub{values: map[string]string{
		SettingKeySubscriptionNotificationEmail: "ops@example.com",
	}}, &subscriptionNotifyUserRepoStub{}, emailer)

	sub, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       1001,
		GroupID:      1,
		ValidityDays: 30,
		Notes:        "same-note",
	})
	require.NoError(t, err)
	require.Equal(t, int64(10), sub.ID)
	require.Empty(t, emailer.calls)
}

func TestAssignSubscriptionDoesNotSendAdminNotificationWhenEmailNotConfigured(t *testing.T) {
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{
			ID:                 1,
			Name:               "Gemini Team",
			Platform:           PlatformGemini,
			SubscriptionType:   SubscriptionTypeSubscription,
			AccountCount:       1,
			ActiveAccountCount: 1,
		},
	}
	subRepo := newSubscriptionUserSubRepoStub()
	userRepo := &subscriptionNotifyUserRepoStub{
		users: map[int64]*User{
			2001: {
				ID:       2001,
				Email:    "new-user@example.com",
				Username: "new-user",
			},
		},
	}
	emailer := &subscriptionEmailerStub{}

	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
	svc.SetAdminNotificationDeps(&settingRepoStub{values: map[string]string{
		SettingKeySiteName: "Sub2API",
	}}, userRepo, emailer)

	sub, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       2001,
		GroupID:      1,
		ValidityDays: 30,
	})
	require.NoError(t, err)
	require.NotNil(t, sub)
	require.Empty(t, emailer.calls)
}
