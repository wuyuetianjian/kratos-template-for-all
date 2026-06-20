package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// AuditLog records user actions for security auditing.
type AuditLog struct {
	ent.Schema
}

func (AuditLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("username"),
		field.String("action"),
		field.String("resource_type").Optional(),
		field.String("resource_name").Optional(),
		field.String("ip").Optional(),
		field.String("detail").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (AuditLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Table("audit_logs"),
	}
}
