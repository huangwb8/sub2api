//go:build unit

package service

import (
	"errors"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestWrapPaymentProviderInitError_PreservesStructuredErrors(t *testing.T) {
	t.Parallel()

	err := infraerrors.BadRequest("WXPAY_CONFIG_MISSING_KEY", "missing_required_key").
		WithMetadata(map[string]string{"key": "publicKeyId"})

	wrapped := wrapPaymentProviderInitError(err, "wxpay", "42")
	appErr := infraerrors.FromError(wrapped)

	require.Equal(t, "WXPAY_CONFIG_MISSING_KEY", appErr.Reason)
	require.Equal(t, map[string]string{
		"key":         "publicKeyId",
		"provider":    "wxpay",
		"instance_id": "42",
	}, appErr.Metadata)
}

func TestWrapPaymentProviderInitError_WrapsPlainErrors(t *testing.T) {
	t.Parallel()

	wrapped := wrapPaymentProviderInitError(errors.New("boom"), "wxpay", "42")
	appErr := infraerrors.FromError(wrapped)

	require.Equal(t, "PAYMENT_PROVIDER_MISCONFIGURED", appErr.Reason)
	require.Equal(t, map[string]string{
		"provider":    "wxpay",
		"instance_id": "42",
	}, appErr.Metadata)
}

func TestWrapPaymentProviderRuntimeError_PreservesStructuredErrors(t *testing.T) {
	t.Parallel()

	err := infraerrors.TooManyRequests("TOO_MANY_PENDING", "too_many_pending").
		WithMetadata(map[string]string{"max": "3"})

	wrapped := wrapPaymentProviderRuntimeError(err, "wxpay", "42")
	appErr := infraerrors.FromError(wrapped)

	require.Equal(t, "TOO_MANY_PENDING", appErr.Reason)
	require.Equal(t, map[string]string{
		"max":         "3",
		"provider":    "wxpay",
		"instance_id": "42",
	}, appErr.Metadata)
}

func TestWrapPaymentProviderRuntimeError_WrapsPlainErrors(t *testing.T) {
	t.Parallel()

	wrapped := wrapPaymentProviderRuntimeError(errors.New("boom"), "wxpay", "42")
	appErr := infraerrors.FromError(wrapped)

	require.Equal(t, "PAYMENT_GATEWAY_ERROR", appErr.Reason)
	require.Equal(t, map[string]string{
		"provider":    "wxpay",
		"instance_id": "42",
	}, appErr.Metadata)
}
