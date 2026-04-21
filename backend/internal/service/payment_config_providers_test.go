//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateProviderRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		providerKey    string
		providerName   string
		supportedTypes string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "valid easypay with types",
			providerKey:    "easypay",
			providerName:   "MyProvider",
			supportedTypes: "alipay,wxpay",
			wantErr:        false,
		},
		{
			name:           "valid stripe with empty types",
			providerKey:    "stripe",
			providerName:   "Stripe Provider",
			supportedTypes: "",
			wantErr:        false,
		},
		{
			name:           "valid alipay provider",
			providerKey:    "alipay",
			providerName:   "Alipay Direct",
			supportedTypes: "alipay",
			wantErr:        false,
		},
		{
			name:           "valid wxpay provider",
			providerKey:    "wxpay",
			providerName:   "WeChat Pay",
			supportedTypes: "wxpay",
			wantErr:        false,
		},
		{
			name:           "invalid provider key",
			providerKey:    "invalid",
			providerName:   "Name",
			supportedTypes: "alipay",
			wantErr:        true,
			errContains:    "invalid provider key",
		},
		{
			name:           "empty name",
			providerKey:    "easypay",
			providerName:   "",
			supportedTypes: "alipay",
			wantErr:        true,
			errContains:    "provider name is required",
		},
		{
			name:           "whitespace-only name",
			providerKey:    "easypay",
			providerName:   "  ",
			supportedTypes: "alipay",
			wantErr:        true,
			errContains:    "provider name is required",
		},
		{
			name:           "tab-only name",
			providerKey:    "easypay",
			providerName:   "\t",
			supportedTypes: "alipay",
			wantErr:        true,
			errContains:    "provider name is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateProviderRequest(tc.providerKey, tc.providerName, tc.supportedTypes)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIsSensitiveProviderConfigField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		providerKey string
		field       string
		wantSen     bool
	}{
		{"stripe", "secretKey", true},
		{"stripe", "webhookSecret", true},
		{"stripe", "SecretKey", true},
		{"stripe", "publishableKey", false},
		{"stripe", "appId", false},
		{"alipay", "privateKey", true},
		{"alipay", "publicKey", true},
		{"alipay", "alipayPublicKey", true},
		{"alipay", "appId", false},
		{"alipay", "notifyUrl", false},
		{"wxpay", "privateKey", true},
		{"wxpay", "apiV3Key", true},
		{"wxpay", "publicKey", true},
		{"wxpay", "publicKeyId", false},
		{"wxpay", "certSerial", false},
		{"wxpay", "mchId", false},
		{"easypay", "pkey", true},
		{"easypay", "pid", false},
		{"easypay", "apiBase", false},
		{"unknown", "secretKey", false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.providerKey+"/"+tc.field, func(t *testing.T) {
			t.Parallel()

			got := isSensitiveProviderConfigField(tc.providerKey, tc.field)
			assert.Equal(t, tc.wantSen, got, "isSensitiveProviderConfigField(%q, %q)", tc.providerKey, tc.field)
		})
	}
}

func TestJoinTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{
			name:  "multiple types",
			input: []string{"alipay", "wxpay"},
			want:  "alipay,wxpay",
		},
		{
			name:  "single type",
			input: []string{"stripe"},
			want:  "stripe",
		},
		{
			name:  "empty slice",
			input: []string{},
			want:  "",
		},
		{
			name:  "nil slice",
			input: nil,
			want:  "",
		},
		{
			name:  "three types",
			input: []string{"alipay", "wxpay", "stripe"},
			want:  "alipay,wxpay,stripe",
		},
		{
			name:  "types with spaces are not trimmed",
			input: []string{" alipay ", " wxpay "},
			want:  " alipay , wxpay ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := joinTypes(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPaymentConfigEncryptionCompatibility(t *testing.T) {
	t.Parallel()

	svc := &PaymentConfigService{
		encryptionKey: []byte("12345678901234567890123456789012"),
	}
	cfg := map[string]string{
		"appId":      "app-1",
		"privateKey": "secret-value",
	}

	t.Run("new writes use plaintext json", func(t *testing.T) {
		stored, err := svc.encryptConfig(cfg)
		require.NoError(t, err)
		require.JSONEq(t, `{"appId":"app-1","privateKey":"secret-value"}`, stored)

		decoded, err := svc.decryptConfig(stored)
		require.NoError(t, err)
		require.Equal(t, cfg, decoded)
	})

	t.Run("legacy aes ciphertext still reads", func(t *testing.T) {
		raw, err := json.Marshal(cfg)
		require.NoError(t, err)

		//nolint:staticcheck // test covers legacy compatibility
		legacy, err := payment.Encrypt(string(raw), svc.encryptionKey)
		require.NoError(t, err)

		decoded, err := svc.decryptConfig(legacy)
		require.NoError(t, err)
		require.Equal(t, cfg, decoded)
	})

	t.Run("unreadable value falls back to empty config", func(t *testing.T) {
		decoded, err := svc.decryptConfig("definitely-not-json-or-ciphertext")
		require.NoError(t, err)
		require.Nil(t, decoded)
	})
}

func TestDecryptAndMaskConfig(t *testing.T) {
	t.Parallel()

	svc := &PaymentConfigService{}
	cfg := map[string]string{
		"appId":             "app-1",
		"privateKey":        "secret-value",
		"publicKey":         "public-secret",
		"notifyUrl":         "https://example.com/notify",
		"publishableKey":    "pk_live_visible",
		"webhookSecret":     "whsec-hidden",
		"alipayPublicKey":   "ali-public",
		"nonSensitiveField": "visible",
	}

	stored, err := json.Marshal(cfg)
	require.NoError(t, err)

	t.Run("alipay omits sensitive fields", func(t *testing.T) {
		masked, err := svc.decryptAndMaskConfig("alipay", string(stored))
		require.NoError(t, err)
		require.Equal(t, map[string]string{
			"appId":             "app-1",
			"notifyUrl":         "https://example.com/notify",
			"publishableKey":    "pk_live_visible",
			"webhookSecret":     "whsec-hidden",
			"nonSensitiveField": "visible",
		}, masked)
	})

	t.Run("stripe keeps publishable key but hides secrets", func(t *testing.T) {
		masked, err := svc.decryptAndMaskConfig("stripe", string(stored))
		require.NoError(t, err)
		require.Equal(t, map[string]string{
			"appId":             "app-1",
			"privateKey":        "secret-value",
			"publicKey":         "public-secret",
			"notifyUrl":         "https://example.com/notify",
			"publishableKey":    "pk_live_visible",
			"alipayPublicKey":   "ali-public",
			"nonSensitiveField": "visible",
		}, masked)
	})
}

func TestPaymentConfigService_CreateProviderInstance_ValidatesEnabledWxpayConfig(t *testing.T) {
	t.Parallel()

	svc, _ := newPaymentConfigServiceSQLite(t)
	ctx := context.Background()
	privateKey, publicKey := generateTestWxpayKeyPair(t)

	_, err := svc.CreateProviderInstance(ctx, CreateProviderInstanceRequest{
		ProviderKey:    payment.TypeWxpay,
		Name:           "wxpay-enabled-invalid",
		Enabled:        true,
		SupportedTypes: []string{payment.TypeWxpay},
		Config: map[string]string{
			"appId":       "wx123",
			"mchId":       "1900000000",
			"privateKey":  privateKey,
			"apiV3Key":    "12345678901234567890123456789012",
			"publicKey":   publicKey + "broken",
			"publicKeyId": "pub-key-id",
			"certSerial":  "SERIAL123",
		},
	})
	require.Error(t, err)

	appErr := infraerrors.FromError(err)
	require.Equal(t, "WXPAY_CONFIG_INVALID_KEY", appErr.Reason)
	require.Equal(t, map[string]string{"key": "publicKey"}, appErr.Metadata)
}

func TestPaymentConfigService_CreateProviderInstance_AllowsDisabledWxpayDraft(t *testing.T) {
	t.Parallel()

	svc, _ := newPaymentConfigServiceSQLite(t)
	ctx := context.Background()

	inst, err := svc.CreateProviderInstance(ctx, CreateProviderInstanceRequest{
		ProviderKey:    payment.TypeWxpay,
		Name:           "wxpay-disabled-draft",
		Enabled:        false,
		SupportedTypes: []string{payment.TypeWxpay},
		Config: map[string]string{
			"appId":    "wx123",
			"mchId":    "1900000000",
			"apiV3Key": "short",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, inst)
	require.False(t, inst.Enabled)
}

func TestPaymentConfigService_UpdateProviderInstance_ValidatesFinalEnabledWxpayConfig(t *testing.T) {
	t.Parallel()

	svc, _ := newPaymentConfigServiceSQLite(t)
	ctx := context.Background()

	inst, err := svc.CreateProviderInstance(ctx, CreateProviderInstanceRequest{
		ProviderKey:    payment.TypeWxpay,
		Name:           "wxpay-disabled-draft",
		Enabled:        false,
		SupportedTypes: []string{payment.TypeWxpay},
		Config: map[string]string{
			"appId":    "wx123",
			"mchId":    "1900000000",
			"apiV3Key": "short",
		},
	})
	require.NoError(t, err)

	enabled := true
	_, err = svc.UpdateProviderInstance(ctx, int64(inst.ID), UpdateProviderInstanceRequest{
		Enabled: &enabled,
	})
	require.Error(t, err)

	appErr := infraerrors.FromError(err)
	require.Equal(t, "WXPAY_CONFIG_MISSING_KEY", appErr.Reason)
	require.Equal(t, map[string]string{"key": "privateKey"}, appErr.Metadata)
}
