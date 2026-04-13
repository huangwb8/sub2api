package admin

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

func newAdminPaymentHandlerTestService(t *testing.T) (*service.PaymentConfigService, *dbent.Client) {
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

func TestPaymentHandler_ListPlans_IncludesZeroPriceField(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	configSvc, client := newAdminPaymentHandlerTestService(t)
	ctx := context.Background()

	group, err := client.Group.Create().
		SetName("zero-price-group").
		SetPlatform(service.PlatformOpenAI).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeSubscription).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.SubscriptionPlan.Create().
		SetGroupID(group.ID).
		SetName("free-plan").
		SetDescription("free plan").
		SetPrice(0).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetForSale(true).
		Save(ctx)
	require.NoError(t, err)

	handler := NewPaymentHandler(nil, configSvc)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/payment/plans", nil)

	handler.ListPlans(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Code int               `json:"code"`
		Data []json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data, 1)

	var plan map[string]any
	require.NoError(t, json.Unmarshal(resp.Data[0], &plan))

	price, ok := plan["price"]
	require.True(t, ok, "zero-price plans must still include the price field")
	require.Equal(t, float64(0), price)
}
