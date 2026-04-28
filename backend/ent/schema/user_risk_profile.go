package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type UserRiskProfile struct {
	ent.Schema
}

func (UserRiskProfile) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_risk_profiles"},
	}
}

func (UserRiskProfile) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (UserRiskProfile) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id").Unique(),
		field.Float("score").
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}).
			Default(5),
		field.String("status").MaxLen(32).Default("healthy"),
		field.Int("consecutive_bad_days").Default(0),
		field.Time("last_evaluated_at").Optional().Nillable(),
		field.Time("last_warned_at").Optional().Nillable(),
		field.Time("grace_period_started_at").Optional().Nillable(),
		field.Time("locked_at").Optional().Nillable(),
		field.String("lock_reason").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.String("last_evaluation_summary").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.Bool("exempted").Default(false),
		field.Time("exempted_at").Optional().Nillable(),
		field.Int64("exempted_by").Optional().Nillable(),
		field.String("exemption_reason").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.Time("unlocked_at").Optional().Nillable(),
		field.Int64("unlocked_by").Optional().Nillable(),
		field.String("unlock_reason").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
	}
}

func (UserRiskProfile) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("risk_profile").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (UserRiskProfile) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("exempted"),
		index.Fields("locked_at"),
	}
}
