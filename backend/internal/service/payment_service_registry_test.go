package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

type stubPaymentLoadBalancer struct {
	configByInstance map[int64]map[string]string
}

func (s stubPaymentLoadBalancer) GetInstanceConfig(_ context.Context, instanceID int64) (map[string]string, error) {
	return s.configByInstance[instanceID], nil
}

func (s stubPaymentLoadBalancer) SelectInstance(_ context.Context, _ string, _ payment.PaymentType, _ payment.Strategy, _ float64) (*payment.InstanceSelection, error) {
	return nil, nil
}

func TestPaymentService_RefreshProviders_RegistersCheckoutTypes(t *testing.T) {
	t.Parallel()

	_, client := newPaymentConfigServiceSQLite(t)
	ctx := context.Background()

	wxInst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeWxpay).
		SetName("wxpay-direct").
		SetConfig("ignored").
		SetSupportedTypes(payment.TypeWxpay).
		SetEnabled(true).
		Save(ctx)
	if err != nil {
		t.Fatalf("create wxpay instance: %v", err)
	}

	aliInst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-direct").
		SetConfig("ignored").
		SetSupportedTypes(payment.TypeAlipay).
		SetEnabled(true).
		Save(ctx)
	if err != nil {
		t.Fatalf("create alipay instance: %v", err)
	}

	stripeInst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeStripe).
		SetName("stripe").
		SetConfig("ignored").
		SetSupportedTypes("card,alipay,wxpay,link").
		SetEnabled(true).
		Save(ctx)
	if err != nil {
		t.Fatalf("create stripe instance: %v", err)
	}

	svc := &PaymentService{
		entClient: client,
		registry:  payment.NewRegistry(),
		loadBalancer: stubPaymentLoadBalancer{
			configByInstance: map[int64]map[string]string{
				int64(wxInst.ID): {
					"appId":       "wx123",
					"mchId":       "1900000000",
					"privateKey":  "fake-private-key",
					"apiV3Key":    "12345678901234567890123456789012",
					"publicKey":   "fake-public-key",
					"publicKeyId": "pub-key-id",
					"certSerial":  "SERIAL123",
				},
				int64(aliInst.ID): {
					"appId":      "2026000000000000",
					"privateKey": "dummy-private-key",
					"publicKey":  "dummy-public-key",
				},
				int64(stripeInst.ID): {
					"secretKey": "sk_test_123",
				},
			},
		},
	}

	svc.RefreshProviders(ctx)

	if key := svc.registry.GetProviderKey(payment.TypeWxpay); key != payment.TypeWxpay {
		t.Fatalf("GetProviderKey(%s) = %q, want %q", payment.TypeWxpay, key, payment.TypeWxpay)
	}
	if key := svc.registry.GetProviderKey(payment.TypeAlipay); key != payment.TypeAlipay {
		t.Fatalf("GetProviderKey(%s) = %q, want %q", payment.TypeAlipay, key, payment.TypeAlipay)
	}
	if key := svc.registry.GetProviderKey(payment.TypeStripe); key != payment.TypeStripe {
		t.Fatalf("GetProviderKey(%s) = %q, want %q", payment.TypeStripe, key, payment.TypeStripe)
	}
	if key := svc.registry.GetProviderKey(payment.TypeWxpayDirect); key != "" {
		t.Fatalf("GetProviderKey(%s) = %q, want empty", payment.TypeWxpayDirect, key)
	}
	if key := svc.registry.GetProviderKey(payment.TypeAlipayDirect); key != "" {
		t.Fatalf("GetProviderKey(%s) = %q, want empty", payment.TypeAlipayDirect, key)
	}
}
