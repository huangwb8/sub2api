//go:build unit

package service

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestExpectedNotificationProviderKeyForOrder(t *testing.T) {
	t.Parallel()

	order := &dbent.PaymentOrder{PaymentType: payment.TypeAlipay}
	registry := payment.NewRegistry()

	require.Equal(t, payment.TypeWxpay, expectedNotificationProviderKeyForOrder(registry, order, payment.TypeWxpay))

	order.ProviderKey = ptrValue("easypay")
	require.Equal(t, payment.TypeEasyPay, expectedNotificationProviderKeyForOrder(registry, order, ""))

	order.ProviderKey = nil
	require.Equal(t, payment.TypeAlipay, expectedNotificationProviderKeyForOrder(registry, order, ""))
}

func TestValidateProviderNotificationMetadata(t *testing.T) {
	t.Parallel()

	order := &dbent.PaymentOrder{
		ProviderSnapshot: map[string]any{
			"merchant_app_id": "wx-app-1",
			"merchant_id":     "mch-1",
			"currency":        "CNY",
		},
	}

	err := validateProviderNotificationMetadata(order, payment.TypeWxpay, map[string]string{
		"appid":       "wx-app-1",
		"mchid":       "mch-1",
		"currency":    "CNY",
		"trade_state": "SUCCESS",
	})
	require.NoError(t, err)

	err = validateProviderNotificationMetadata(order, payment.TypeWxpay, map[string]string{
		"appid":       "wrong-app",
		"mchid":       "mch-1",
		"currency":    "CNY",
		"trade_state": "SUCCESS",
	})
	require.ErrorContains(t, err, "appid mismatch")

	err = validateProviderNotificationMetadata(order, payment.TypeWxpay, map[string]string{
		"appid":       "wx-app-1",
		"mchid":       "mch-1",
		"currency":    "USD",
		"trade_state": "SUCCESS",
	})
	require.ErrorContains(t, err, "currency mismatch")

	err = validateProviderNotificationMetadata(order, payment.TypeAlipay, map[string]string{
		"appid": "wrong-app",
	})
	require.NoError(t, err)
}

func ptrValue[T any](v T) *T {
	return &v
}
