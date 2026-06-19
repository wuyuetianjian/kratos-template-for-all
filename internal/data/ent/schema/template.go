package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// Template is a minimal schema that keeps Ent generation active in this
// service template. Replace it with real resource schemas when adding data.
type Template struct {
	ent.Schema
}

// Fields of the Template.
func (Template) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Default("template"),
	}
}

// Edges of the Template.
func (Template) Edges() []ent.Edge {
	return nil
}
