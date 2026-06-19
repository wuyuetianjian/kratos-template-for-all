package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Module groups permissions by functional area.
type Module struct {
	ent.Schema
}

// Fields of the Module.
func (Module) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty().Unique(),
		field.String("description").Optional(),
		field.Bool("system").Default(false),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the Module.
func (Module) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("permissions", Permission.Type),
	}
}

// Annotations of the Module.
func (Module) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Table("auth_modules"),
	}
}
