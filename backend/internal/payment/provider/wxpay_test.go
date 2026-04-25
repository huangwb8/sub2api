//go:build unit

package provider

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
)

func TestMapWxState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "SUCCESS maps to paid",
			input: wxpayTradeStateSuccess,
			want:  payment.ProviderStatusPaid,
		},
		{
			name:  "REFUND maps to refunded",
			input: wxpayTradeStateRefund,
			want:  payment.ProviderStatusRefunded,
		},
		{
			name:  "CLOSED maps to failed",
			input: wxpayTradeStateClosed,
			want:  payment.ProviderStatusFailed,
		},
		{
			name:  "PAYERROR maps to failed",
			input: wxpayTradeStatePayError,
			want:  payment.ProviderStatusFailed,
		},
		{
			name:  "unknown state maps to pending",
			input: "NOTPAY",
			want:  payment.ProviderStatusPending,
		},
		{
			name:  "empty string maps to pending",
			input: "",
			want:  payment.ProviderStatusPending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mapWxState(tt.input)
			if got != tt.want {
				t.Errorf("mapWxState(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWxSV(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input *string
		want  string
	}{
		{
			name:  "nil pointer returns empty string",
			input: nil,
			want:  "",
		},
		{
			name:  "non-nil pointer returns value",
			input: strPtr("hello"),
			want:  "hello",
		},
		{
			name:  "pointer to empty string returns empty string",
			input: strPtr(""),
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := wxSV(tt.input)
			if got != tt.want {
				t.Errorf("wxSV() = %q, want %q", got, tt.want)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func generateTestKeyPair(t *testing.T) (string, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)

	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})

	return string(privatePEM), string(publicPEM)
}

func TestFormatPEM(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		keyType string
		want    string
	}{
		{
			name:    "raw key gets wrapped with headers",
			key:     "MIIBIjANBgkqhki...",
			keyType: "PUBLIC KEY",
			want:    "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhki...\n-----END PUBLIC KEY-----",
		},
		{
			name:    "already formatted key is returned as-is",
			key:     "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBg...\n-----END PRIVATE KEY-----",
			keyType: "PRIVATE KEY",
			want:    "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBg...\n-----END PRIVATE KEY-----",
		},
		{
			name:    "key with leading/trailing whitespace is trimmed before check",
			key:     "  \n MIIBIjANBgkqhki...  \n ",
			keyType: "PUBLIC KEY",
			want:    "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhki...\n-----END PUBLIC KEY-----",
		},
		{
			name:    "already formatted key with whitespace is trimmed and returned",
			key:     "  -----BEGIN RSA PRIVATE KEY-----\ndata\n-----END RSA PRIVATE KEY-----  ",
			keyType: "RSA PRIVATE KEY",
			want:    "-----BEGIN RSA PRIVATE KEY-----\ndata\n-----END RSA PRIVATE KEY-----",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatPEM(tt.key, tt.keyType)
			if got != tt.want {
				t.Errorf("formatPEM(%q, %q) =\n%s\nwant:\n%s", tt.key, tt.keyType, got, tt.want)
			}
		})
	}
}

func TestNewWxpay(t *testing.T) {
	t.Parallel()

	privateKey, publicKey := generateTestKeyPair(t)
	validConfig := map[string]string{
		"appId":       "wx1234567890",
		"mchId":       "1234567890",
		"privateKey":  privateKey,
		"apiV3Key":    "12345678901234567890123456789012",
		"publicKey":   publicKey,
		"publicKeyId": "key-id-001",
		"certSerial":  "SERIAL001",
	}

	// helper to clone and override config fields
	withOverride := func(overrides map[string]string) map[string]string {
		cfg := make(map[string]string, len(validConfig))
		for k, v := range validConfig {
			cfg[k] = v
		}
		for k, v := range overrides {
			cfg[k] = v
		}
		return cfg
	}

	tests := []struct {
		name         string
		config       map[string]string
		wantReason   string
		wantMetadata map[string]string
	}{
		{
			name:   "valid config succeeds",
			config: validConfig,
		},
		{
			name:       "missing appId",
			config:     withOverride(map[string]string{"appId": ""}),
			wantReason: "WXPAY_CONFIG_MISSING_KEY",
			wantMetadata: map[string]string{
				"key": "appId",
			},
		},
		{
			name:       "missing mchId",
			config:     withOverride(map[string]string{"mchId": ""}),
			wantReason: "WXPAY_CONFIG_MISSING_KEY",
			wantMetadata: map[string]string{
				"key": "mchId",
			},
		},
		{
			name:       "missing privateKey",
			config:     withOverride(map[string]string{"privateKey": ""}),
			wantReason: "WXPAY_CONFIG_MISSING_KEY",
			wantMetadata: map[string]string{
				"key": "privateKey",
			},
		},
		{
			name:       "missing apiV3Key",
			config:     withOverride(map[string]string{"apiV3Key": ""}),
			wantReason: "WXPAY_CONFIG_MISSING_KEY",
			wantMetadata: map[string]string{
				"key": "apiV3Key",
			},
		},
		{
			name:       "missing publicKey",
			config:     withOverride(map[string]string{"publicKey": ""}),
			wantReason: "WXPAY_CONFIG_MISSING_KEY",
			wantMetadata: map[string]string{
				"key": "publicKey",
			},
		},
		{
			name:       "missing publicKeyId",
			config:     withOverride(map[string]string{"publicKeyId": ""}),
			wantReason: "WXPAY_CONFIG_MISSING_KEY",
			wantMetadata: map[string]string{
				"key": "publicKeyId",
			},
		},
		{
			name:       "missing certSerial",
			config:     withOverride(map[string]string{"certSerial": ""}),
			wantReason: "WXPAY_CONFIG_MISSING_KEY",
			wantMetadata: map[string]string{
				"key": "certSerial",
			},
		},
		{
			name:       "apiV3Key too short",
			config:     withOverride(map[string]string{"apiV3Key": "short"}),
			wantReason: "WXPAY_CONFIG_INVALID_KEY_LENGTH",
			wantMetadata: map[string]string{
				"key":      "apiV3Key",
				"expected": "32",
				"actual":   "5",
			},
		},
		{
			name:       "apiV3Key too long",
			config:     withOverride(map[string]string{"apiV3Key": "123456789012345678901234567890123"}),
			wantReason: "WXPAY_CONFIG_INVALID_KEY_LENGTH",
			wantMetadata: map[string]string{
				"key":      "apiV3Key",
				"expected": "32",
				"actual":   "33",
			},
		},
		{
			name:       "invalid private key pem",
			config:     withOverride(map[string]string{"privateKey": "not-a-private-key"}),
			wantReason: "WXPAY_CONFIG_INVALID_KEY",
			wantMetadata: map[string]string{
				"key": "privateKey",
			},
		},
		{
			name:       "invalid public key pem",
			config:     withOverride(map[string]string{"publicKey": "not-a-public-key"}),
			wantReason: "WXPAY_CONFIG_INVALID_KEY",
			wantMetadata: map[string]string{
				"key": "publicKey",
			},
		},
		{
			name:       "public key pem with trailing garbage",
			config:     withOverride(map[string]string{"publicKey": publicKey + "broken"}),
			wantReason: "WXPAY_CONFIG_INVALID_KEY",
			wantMetadata: map[string]string{
				"key": "publicKey",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewWxpay("test-instance", tt.config)
			if tt.wantReason != "" {
				require.Error(t, err)
				appErr := infraerrors.FromError(err)
				require.Equal(t, tt.wantReason, appErr.Reason)
				require.Equal(t, tt.wantMetadata, appErr.Metadata)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, "test-instance", got.instanceID)
		})
	}
}

func TestWxpaySupportedTypes_ShouldRegisterWxpay(t *testing.T) {
	t.Parallel()

	privateKey, publicKey := generateTestKeyPair(t)
	p, err := NewWxpay("test-instance", map[string]string{
		"appId":       "wx123",
		"mchId":       "1900000000",
		"privateKey":  privateKey,
		"apiV3Key":    "12345678901234567890123456789012",
		"publicKey":   publicKey,
		"publicKeyId": "pub-key-id",
		"certSerial":  "SERIAL123",
	})
	require.NoError(t, err)

	got := p.SupportedTypes()
	require.Len(t, got, 1)
	require.Equal(t, payment.TypeWxpay, got[0])
}

func TestBuildWxpayTransactionMetadata(t *testing.T) {
	t.Parallel()

	appID := "wx-app"
	merchantID := "mch-1"
	tradeState := "SUCCESS"
	currency := "CNY"
	tx := &payments.Transaction{
		Appid:      &appID,
		Mchid:      &merchantID,
		TradeState: &tradeState,
		Amount:     &payments.TransactionAmount{Currency: &currency},
	}

	require.Equal(t, map[string]string{
		"appid":       appID,
		"mchid":       merchantID,
		"trade_state": tradeState,
		"currency":    currency,
	}, buildWxpayTransactionMetadata(tx))
	require.Nil(t, buildWxpayTransactionMetadata(nil))
}
