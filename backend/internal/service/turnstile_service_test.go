//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func newTurnstileServiceForUnitTest(settings map[string]string, verifier TurnstileVerifier) *TurnstileService {
	return NewTurnstileService(NewSettingService(&settingRepoStub{values: settings}, &config.Config{}), verifier)
}

func TestTurnstileService_VerifyTokenFailureReturnsSentinelAndSanitizesRemoteIP(t *testing.T) {
	verifier := &turnstileVerifierSpy{
		result: &TurnstileVerifyResponse{
			Success:    false,
			ErrorCodes: []string{"invalid-input-response"},
			Hostname:   "api.example.com",
		},
	}
	svc := newTurnstileServiceForUnitTest(map[string]string{
		SettingKeyTurnstileEnabled:   "true",
		SettingKeyTurnstileSecretKey: "secret",
	}, verifier)

	err := svc.VerifyToken(context.Background(), "response-token", "172.18.0.2")

	require.ErrorIs(t, err, ErrTurnstileVerificationFailed)
	require.Equal(t, 1, verifier.called)
	require.Equal(t, "response-token", verifier.lastToken)
	require.Empty(t, verifier.lastRemoteIP)
}

func TestTurnstileService_VerifyTokenPassesPublicRemoteIP(t *testing.T) {
	verifier := &turnstileVerifierSpy{}
	svc := newTurnstileServiceForUnitTest(map[string]string{
		SettingKeyTurnstileEnabled:   "true",
		SettingKeyTurnstileSecretKey: "secret",
	}, verifier)

	err := svc.VerifyToken(context.Background(), "response-token", "1.2.3.4")

	require.NoError(t, err)
	require.Equal(t, "1.2.3.4", verifier.lastRemoteIP)
}

func TestRedactRemoteIPForLogDoesNotExposeFullPublicIPOrPrivateIP(t *testing.T) {
	require.Equal(t, "1.2.3.0", redactRemoteIPForLog("1.2.3.4"))
	require.Equal(t, "[redacted]", redactRemoteIPForLog("172.18.0.2"))
	require.Empty(t, redactRemoteIPForLog(""))
}
