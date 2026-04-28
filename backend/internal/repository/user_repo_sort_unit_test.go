//go:build unit

package repository

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"

	entsql "entgo.io/ent/dialect/sql"
)

func TestUserListOrder_TotalRechargedDesc(t *testing.T) {
	selector := entsql.Select("id").From(entsql.Table("users"))
	for _, order := range userListOrder(pagination.PaginationParams{
		SortBy:    "total_recharged",
		SortOrder: "desc",
	}) {
		order(selector)
	}

	query, _ := selector.Query()
	require.Contains(t, query, "FROM redeem_codes AS rc")
	require.Contains(t, query, "rc.used_by = `users`.`id`")
	require.Contains(t, query, "ORDER BY COALESCE((SELECT SUM(rc.value)")
	require.True(t, strings.Contains(query, "DESC, `users`.`id` DESC"))
}

func TestUserListOrder_UsageAsc(t *testing.T) {
	selector := entsql.Select("id").From(entsql.Table("users"))
	for _, order := range userListOrder(pagination.PaginationParams{
		SortBy:    "usage",
		SortOrder: "asc",
	}) {
		order(selector)
	}

	query, _ := selector.Query()
	require.Contains(t, query, "FROM usage_logs AS ul")
	require.Contains(t, query, "ul.user_id = `users`.`id`")
	require.Contains(t, query, "ORDER BY COALESCE((SELECT SUM(ul.actual_cost)")
	require.True(t, strings.Contains(query, "ASC, `users`.`id` ASC"))
}
