package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newPaymentHandlerTestService(t *testing.T) (*service.PaymentConfigService, *dbent.Client) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	return service.NewPaymentConfigService(client, nil, nil), client
}

func TestPaymentHandler_GetPlans_EnrichesPlanDisplayFields(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	configSvc, client := newPaymentHandlerTestService(t)
	ctx := context.Background()

	group, err := client.Group.Create().
		SetName("research-openai").
		SetPlatform(service.PlatformOpenAI).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeSubscription).
		SetRateMultiplier(1.8).
		SetDailyLimitUsd(12.5).
		SetSupportedModelScopes([]string{"gpt-5", "gpt-4.1"}).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.SubscriptionPlan.Create().
		SetGroupID(group.ID).
		SetName("科研增强套餐").
		SetDescription("适合高频学术写作").
		SetPrice(199).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetFeatures("优先排队\n支持多模型").
		SetForSale(true).
		SetSortOrder(7).
		Save(ctx)
	require.NoError(t, err)

	handler := NewPaymentHandler(nil, configSvc, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/payment/plans", nil)

	handler.GetPlans(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Code int `json:"code"`
		Data []struct {
			GroupPlatform        string   `json:"group_platform"`
			GroupName            string   `json:"group_name"`
			RateMultiplier       float64  `json:"rate_multiplier"`
			DailyLimitUSD        *float64 `json:"daily_limit_usd"`
			SupportedModelScopes []string `json:"supported_model_scopes"`
			Features             []string `json:"features"`
			ForSale              bool     `json:"for_sale"`
			SortOrder            int      `json:"sort_order"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data, 1)

	plan := resp.Data[0]
	require.Equal(t, service.PlatformOpenAI, plan.GroupPlatform)
	require.Equal(t, "research-openai", plan.GroupName)
	require.Equal(t, 1.8, plan.RateMultiplier)
	require.NotNil(t, plan.DailyLimitUSD)
	require.Equal(t, 12.5, *plan.DailyLimitUSD)
	require.Equal(t, []string{"gpt-5", "gpt-4.1"}, plan.SupportedModelScopes)
	require.Equal(t, []string{"优先排队", "支持多模型"}, plan.Features)
	require.True(t, plan.ForSale)
	require.Equal(t, 7, plan.SortOrder)
}
