package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// SSOProvider stores external identity provider configuration.
type SSOProvider struct {
	ent.Schema
}

func (SSOProvider) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty(),
		// type: oauth1 | oauth2 | oidc | saml1 | saml2 | ldap
		field.String("type").NotEmpty(),
		field.Bool("enabled").Default(false),
		field.String("icon").Optional().Default(""),
		field.Int("sort_order").Default(0),
		// config holds all protocol-specific key-value settings
		field.JSON("config", map[string]string{}).Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (SSOProvider) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Table("auth_sso_providers"),
	}
}
