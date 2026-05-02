package repository

import (
	"fmt"

	dbpredicate "github.com/Wei-Shaw/sub2api/ent/predicate"
	"github.com/Wei-Shaw/sub2api/internal/service"

	entsql "entgo.io/ent/dialect/sql"
)

const sqlSafeNumericPattern = `^[+-]?(?:\d+(?:\.\d+)?|\.\d+)(?:[eE][+-]?\d+)?$`

func trimmedJSONTextSQL(column, key string) string {
	return fmt.Sprintf("NULLIF(BTRIM((%s->>'%s')), '')", column, key)
}

func safeJSONNumericSQL(column, key string) string {
	trimmed := trimmedJSONTextSQL(column, key)
	return fmt.Sprintf(
		"CASE WHEN %s IS NULL THEN NULL::numeric WHEN %s ~ '%s' THEN (%s)::numeric ELSE NULL::numeric END",
		trimmed,
		trimmed,
		sqlSafeNumericPattern,
		trimmed,
	)
}

func codexWindowRateLimitedSQL(extraColumn, window, nowExpr string) string {
	usedExpr := safeJSONNumericSQL(extraColumn, fmt.Sprintf("codex_%s_used_percent", window))
	resetExpr := trimmedJSONTextSQL(extraColumn, fmt.Sprintf("codex_%s_reset_at", window))
	return fmt.Sprintf("(%s >= 100 AND %s IS NOT NULL AND (%s)::timestamptz > %s)", usedExpr, resetExpr, resetExpr, nowExpr)
}

func codexRateLimitedSQL(platformColumn, typeColumn, extraColumn, nowExpr string) string {
	return fmt.Sprintf(
		"(%s = '%s' AND %s = '%s' AND (%s OR %s))",
		platformColumn,
		service.PlatformOpenAI,
		typeColumn,
		service.AccountTypeOAuth,
		codexWindowRateLimitedSQL(extraColumn, "7d", nowExpr),
		codexWindowRateLimitedSQL(extraColumn, "5h", nowExpr),
	)
}

func notCodexRateLimitedPredicate(nowExpr string) dbpredicate.Account {
	return dbpredicate.Account(func(s *entsql.Selector) {
		s.Where(entsql.Not(entsql.ExprP(codexRateLimitedSQL(s.C("platform"), s.C("type"), s.C("extra"), nowExpr))))
	})
}
