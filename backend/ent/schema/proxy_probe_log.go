package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ProxyProbeLog holds short-lived proxy probe history for operational analysis.
type ProxyProbeLog struct {
	ent.Schema
}

func (ProxyProbeLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "proxy_probe_logs"},
	}
}

func (ProxyProbeLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("proxy_id"),
		field.String("source").MaxLen(64).Default("scheduled_probe"),
		field.String("target").MaxLen(64).Default("probe_chain"),
		field.Bool("success").Default(false),
		field.Int64("latency_ms").Optional().Nillable(),
		field.String("error_message").MaxLen(1024).Optional().Nillable(),
		field.String("ip_address").MaxLen(45).Optional().Nillable(),
		field.String("country_code").MaxLen(16).Optional().Nillable(),
		field.String("country").MaxLen(100).Optional().Nillable(),
		field.String("region").MaxLen(100).Optional().Nillable(),
		field.String("city").MaxLen(100).Optional().Nillable(),
		field.Time("checked_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (ProxyProbeLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("proxy_id", "checked_at"),
		index.Fields("success", "checked_at"),
		index.Fields("source", "checked_at"),
	}
}
