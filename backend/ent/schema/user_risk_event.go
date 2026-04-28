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

type UserRiskEvent struct {
	ent.Schema
}

func (UserRiskEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_risk_events"},
	}
}

func (UserRiskEvent) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (UserRiskEvent) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("event_type").MaxLen(64),
		field.String("severity").MaxLen(32).Default("info"),
		field.Float("score_delta").
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}).
			Default(0),
		field.Float("score_after").
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}).
			Default(0),
		field.String("summary").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.JSON("metadata", map[string]any{}).
			Optional(),
		field.Time("window_start").Optional().Nillable(),
		field.Time("window_end").Optional().Nillable(),
	}
}

func (UserRiskEvent) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("risk_events").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (UserRiskEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
		index.Fields("event_type", "created_at"),
		index.Fields("severity", "created_at"),
	}
}
