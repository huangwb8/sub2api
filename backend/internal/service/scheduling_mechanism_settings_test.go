//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGetSchedulingMechanismSettings_DefaultsWhenMissing(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetSchedulingMechanismSettings(context.Background())
	require.NoError(t, err)
	require.NotNil(t, settings)
	require.Empty(t, settings.Mechanisms)
	require.Equal(t, DefaultProxyFailoverSettings(), settings.ProxyFailover)
}

func TestSetSchedulingMechanismSettings_NormalizesPayload(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewSettingService(repo, &config.Config{})

	err := svc.SetSchedulingMechanismSettings(context.Background(), &SchedulingMechanismSettings{
		Mechanisms: []SchedulingMechanism{
			{
				Name:                     "  OpenAI 502 Gateway  ",
				Platform:                 "OpenAI",
				AccountType:              "OAuth",
				Enabled:                  true,
				TempUnschedulableEnabled: true,
				TempUnschedulableRules: []TempUnschedulableRule{
					{ErrorCode: 502, Keywords: []string{" bad gateway ", "", "upstream"}, DurationMinutes: 3},
					{ErrorCode: 0, Keywords: []string{"ignored"}, DurationMinutes: 0},
				},
			},
			{
				Name:                     "invalid-without-rules",
				Enabled:                  true,
				TempUnschedulableEnabled: true,
			},
		},
		ProxyFailover: ProxyFailoverSettings{
			Enabled:               true,
			AutoTestEnabled:       true,
			ProbeIntervalMinutes:  0,
			FailureThreshold:      99,
			FailureWindowMinutes:  0,
			CooldownMinutes:       999,
			MaxAccountsPerProxy:   0,
			MaxMigrationsPerCycle: 999,
			PreferSameCountry:     true,
			OnlyOpenAIOAuth:       true,
			TempUnschedMinutes:    0,
		},
	})
	require.NoError(t, err)

	settings, err := svc.GetSchedulingMechanismSettings(context.Background())
	require.NoError(t, err)
	require.Len(t, settings.Mechanisms, 1)

	mechanism := settings.Mechanisms[0]
	require.Equal(t, "OpenAI 502 Gateway", mechanism.Name)
	require.Equal(t, PlatformOpenAI, mechanism.Platform)
	require.Equal(t, AccountTypeOAuth, mechanism.AccountType)
	require.Len(t, mechanism.TempUnschedulableRules, 1)
	require.Equal(t, []string{"bad gateway", "upstream"}, mechanism.TempUnschedulableRules[0].Keywords)

	require.Equal(t, defaultProxyFailoverProbeIntervalMin, settings.ProxyFailover.ProbeIntervalMinutes)
	require.Equal(t, 10, settings.ProxyFailover.FailureThreshold)
	require.Equal(t, defaultProxyFailoverFailureWindowMin, settings.ProxyFailover.FailureWindowMinutes)
	require.Equal(t, 240, settings.ProxyFailover.CooldownMinutes)
	require.Equal(t, defaultProxyFailoverMaxPerProxy, settings.ProxyFailover.MaxAccountsPerProxy)
	require.Equal(t, 200, settings.ProxyFailover.MaxMigrationsPerCycle)
	require.Equal(t, defaultProxyFailoverTempUnschedMin, settings.ProxyFailover.TempUnschedMinutes)
}

func TestSetSchedulingMechanisms_PreservesProxyFailoverSettings(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewSettingService(repo, &config.Config{})

	customFailover := DefaultProxyFailoverSettings()
	customFailover.ProbeIntervalMinutes = 17
	customFailover.MaxAccountsPerProxy = 9
	require.NoError(t, svc.SetSchedulingMechanismSettings(context.Background(), &SchedulingMechanismSettings{
		Mechanisms: []SchedulingMechanism{
			{Name: "old", Enabled: true},
		},
		ProxyFailover: customFailover,
	}))

	require.NoError(t, svc.SetSchedulingMechanisms(context.Background(), []SchedulingMechanism{
		{
			Name:                     "new",
			Enabled:                  true,
			TempUnschedulableEnabled: true,
			TempUnschedulableRules: []TempUnschedulableRule{
				{ErrorCode: 502, Keywords: []string{"upstream"}, DurationMinutes: 5},
			},
		},
	}))

	settings, err := svc.GetSchedulingMechanismSettings(context.Background())
	require.NoError(t, err)
	require.Len(t, settings.Mechanisms, 1)
	require.Equal(t, "new", settings.Mechanisms[0].Name)
	require.Equal(t, customFailover, settings.ProxyFailover)
}

func TestSetProxyFailoverSettings_PreservesSchedulingMechanisms(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewSettingService(repo, &config.Config{})
	mechanisms := []SchedulingMechanism{
		{
			Name:                     "OpenAI 502",
			Platform:                 PlatformOpenAI,
			AccountType:              AccountTypeOAuth,
			Enabled:                  true,
			TempUnschedulableEnabled: true,
			TempUnschedulableRules: []TempUnschedulableRule{
				{ErrorCode: 502, Keywords: []string{"bad gateway"}, DurationMinutes: 3},
			},
		},
	}
	require.NoError(t, svc.SetSchedulingMechanismSettings(context.Background(), &SchedulingMechanismSettings{
		Mechanisms:    mechanisms,
		ProxyFailover: DefaultProxyFailoverSettings(),
	}))

	nextFailover := DefaultProxyFailoverSettings()
	nextFailover.FailureThreshold = 4
	updated, err := svc.SetProxyFailoverSettings(context.Background(), nextFailover)
	require.NoError(t, err)
	require.Equal(t, 4, updated.FailureThreshold)

	settings, err := svc.GetSchedulingMechanismSettings(context.Background())
	require.NoError(t, err)
	require.Len(t, settings.Mechanisms, 1)
	require.Equal(t, "OpenAI 502", settings.Mechanisms[0].Name)
	require.Equal(t, nextFailover, settings.ProxyFailover)
}
