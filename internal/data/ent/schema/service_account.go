package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// ServiceAccount represents a machine/service identity that can call the API.
type ServiceAccount struct {
	ent.Schema
}

func (ServiceAccount) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty().Unique(),
		field.String("description").Optional(),
		field.String("token_hash").NotEmpty().Unique(),
		field.String("token_prefix").NotEmpty(),
		field.Time("expires_at").Optional().Nillable(),
		field.Bool("disabled").Default(false),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (ServiceAccount) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("roles", Role.Type).StorageKey(edge.Table("auth_service_account_roles"), edge.Columns("service_account_id", "role_id")),
	}
}

func (ServiceAccount) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Table("auth_service_accounts"),
	}
}
