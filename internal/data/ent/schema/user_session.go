package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// UserSession tracks an authenticated browser session.
type UserSession struct {
	ent.Schema
}

func (UserSession) Fields() []ent.Field {
	return []ent.Field{
		field.String("token_hash").NotEmpty().Unique(),
		field.String("ip").Optional(),
		field.String("browser").Optional(),
		field.String("os").Optional(),
		field.String("status").Default("active"),
		field.String("kicked_by").Optional(),
		field.Time("login_at").Default(time.Now).Immutable(),
		field.Time("last_access_at").Default(time.Now),
	}
}

func (UserSession) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("sessions").Unique().Required(),
	}
}

func (UserSession) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Table("auth_user_sessions"),
	}
}
