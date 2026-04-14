//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteSuccessResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name            string
		providerKey     string
		wantCode        int
		wantContentType string
		wantBody        string
		checkJSON       bool
		wantJSONCode    string
		wantJSONMessage string
	}{
		{
			name:            "wxpay returns JSON with code SUCCESS",
			providerKey:     "wxpay",
			wantCode:        http.StatusOK,
			wantContentType: "application/json",
			checkJSON:       true,
			wantJSONCode:    "SUCCESS",
			wantJSONMessage: "成功",
		},
		{
			name:            "stripe returns empty 200",
			providerKey:     "stripe",
			wantCode:        http.StatusOK,
			wantContentType: "text/plain",
			wantBody:        "",
		},
		{
			name:            "easypay returns plain text success",
			providerKey:     "easypay",
			wantCode:        http.StatusOK,
			wantContentType: "text/plain",
			wantBody:        "success",
		},
		{
			name:            "alipay returns plain text success",
			providerKey:     "alipay",
			wantCode:        http.StatusOK,
			wantContentType: "text/plain",
			wantBody:        "success",
		},
		{
			name:            "unknown provider returns plain text success",
			providerKey:     "unknown_provider",
			wantCode:        http.StatusOK,
			wantContentType: "text/plain",
			wantBody:        "success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			writeSuccessResponse(c, tt.providerKey)

			assert.Equal(t, tt.wantCode, w.Code)
			assert.Contains(t, w.Header().Get("Content-Type"), tt.wantContentType)

			if tt.checkJSON {
				var resp wxpaySuccessResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err, "response body should be valid JSON")
				assert.Equal(t, tt.wantJSONCode, resp.Code)
				assert.Equal(t, tt.wantJSONMessage, resp.Message)
			} else {
				assert.Equal(t, tt.wantBody, w.Body.String())
			}
		})
	}
}

func TestWebhookConstants(t *testing.T) {
	t.Run("maxWebhookBodySize is 1MB", func(t *testing.T) {
		assert.Equal(t, int64(1<<20), int64(maxWebhookBodySize))
	})

	t.Run("webhookLogTruncateLen is 200", func(t *testing.T) {
		assert.Equal(t, 200, webhookLogTruncateLen)
	})
}

type stubWebhookProvider struct {
	verifyFn func(context.Context, string, map[string]string) (*payment.PaymentNotification, error)
}

func (s stubWebhookProvider) Name() string        { return "stub" }
func (s stubWebhookProvider) ProviderKey() string { return payment.TypeStripe }
func (s stubWebhookProvider) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeStripe}
}
func (s stubWebhookProvider) CreatePayment(context.Context, payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	return nil, nil
}
func (s stubWebhookProvider) QueryOrder(context.Context, string) (*payment.QueryOrderResponse, error) {
	return nil, nil
}
func (s stubWebhookProvider) VerifyNotification(ctx context.Context, rawBody string, headers map[string]string) (*payment.PaymentNotification, error) {
	return s.verifyFn(ctx, rawBody, headers)
}
func (s stubWebhookProvider) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, nil
}

func TestExtractOutTradeNo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		provider  string
		rawBody   string
		wantOrder string
	}{
		{
			name:      "extracts easypay out_trade_no from query body",
			provider:  payment.TypeEasyPay,
			rawBody:   "trade_status=TRADE_SUCCESS&out_trade_no=sub2_abc123&sign=xxx",
			wantOrder: "sub2_abc123",
		},
		{
			name:      "extracts alipay out_trade_no from form body",
			provider:  payment.TypeAlipay,
			rawBody:   "gmt_create=2026-04-14+12%3A00%3A00&out_trade_no=sub2_ali123&trade_no=20260001",
			wantOrder: "sub2_ali123",
		},
		{
			name:      "extracts stripe order id from metadata",
			provider:  payment.TypeStripe,
			rawBody:   `{"type":"payment_intent.succeeded","data":{"object":{"metadata":{"orderId":"sub2_stripe123"}}}}`,
			wantOrder: "sub2_stripe123",
		},
		{
			name:      "wxpay encrypted body cannot be pre-extracted",
			provider:  payment.TypeWxpay,
			rawBody:   `{"resource":{"ciphertext":"encrypted"}}`,
			wantOrder: "",
		},
		{
			name:      "invalid stripe payload returns empty",
			provider:  payment.TypeStripe,
			rawBody:   `{not-json`,
			wantOrder: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantOrder, extractOutTradeNo(tt.rawBody, tt.provider))
		})
	}
}

func TestVerifyNotificationWithProviders(t *testing.T) {
	t.Parallel()

	t.Run("uses first provider that verifies successfully", func(t *testing.T) {
		t.Parallel()
		want := &payment.PaymentNotification{OrderID: "sub2_ok", Status: payment.ProviderStatusSuccess}
		providers := []payment.Provider{
			stubWebhookProvider{verifyFn: func(context.Context, string, map[string]string) (*payment.PaymentNotification, error) {
				return nil, assert.AnError
			}},
			stubWebhookProvider{verifyFn: func(context.Context, string, map[string]string) (*payment.PaymentNotification, error) {
				return want, nil
			}},
		}

		got, err := verifyNotificationWithProviders(context.Background(), providers, "raw", map[string]string{"stripe-signature": "sig"})
		require.NoError(t, err)
		require.Equal(t, want, got)
	})

	t.Run("returns nil without error for irrelevant verified event", func(t *testing.T) {
		t.Parallel()
		providers := []payment.Provider{
			stubWebhookProvider{verifyFn: func(context.Context, string, map[string]string) (*payment.PaymentNotification, error) {
				return nil, assert.AnError
			}},
			stubWebhookProvider{verifyFn: func(context.Context, string, map[string]string) (*payment.PaymentNotification, error) {
				return nil, nil
			}},
		}

		got, err := verifyNotificationWithProviders(context.Background(), providers, "raw", nil)
		require.NoError(t, err)
		require.Nil(t, got)
	})
}
