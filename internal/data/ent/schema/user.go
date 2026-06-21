package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// User holds application account data.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("username").NotEmpty().Unique(),
		field.String("password_hash").Sensitive(),
		field.String("display_name").Optional(),
		field.Bool("disabled").Default(false),
		field.Bool("system").Default(false),
		field.Bool("initial_password_used").Default(true),
		field.String("totp_secret").Optional().Sensitive(),
		field.Bool("totp_enabled").Default(false),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("roles", Role.Type).StorageKey(edge.Table("auth_user_roles"), edge.Columns("user_id", "role_id")),
		edge.To("sessions", UserSession.Type),
	}
}

// Annotations of the User.
func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Table("auth_users"),
	}
}
