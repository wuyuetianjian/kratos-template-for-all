package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// SystemSetting stores runtime configuration as key-value pairs.
type SystemSetting struct {
	ent.Schema
}

func (SystemSetting) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").NotEmpty().Unique(),
		field.String("value").Default(""),
	}
}

func (SystemSetting) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Table("system_settings"),
	}
}
