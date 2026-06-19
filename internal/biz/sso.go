package biz

import (
	"context"
	"time"
)

// SSOProviderType enumerates supported identity provider protocols.
type SSOProviderType string

const (
	SSOTypeOAuth1 SSOProviderType = "oauth1"
	SSOTypeOAuth2 SSOProviderType = "oauth2"
	SSOTypeOIDC   SSOProviderType = "oidc"
	SSOTypeSAML1  SSOProviderType = "saml1"
	SSOTypeSAML2  SSOProviderType = "saml2"
	SSOTypeLDAP   SSOProviderType = "ldap"
)

type SSOProviderRepo interface {
	ListSSOProviders(context.Context, bool) ([]SSOProvider, error)
	GetSSOProvider(context.Context, int64) (*SSOProvider, error)
	CreateSSOProvider(context.Context, *CreateSSOProvider) (*SSOProvider, error)
	UpdateSSOProvider(context.Context, *UpdateSSOProvider) (*SSOProvider, error)
	DeleteSSOProvider(context.Context, int64) error
}

type SSOProvider struct {
	ID        int64
	Name      string
	Type      string
	Enabled   bool
	Icon      string
	SortOrder int32
	Config    map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateSSOProvider struct {
	Name      string
	Type      string
	Enabled   bool
	Icon      string
	SortOrder int32
	Config    map[string]string
}

type UpdateSSOProvider struct {
	ID        int64
	Name      string
	Enabled   bool
	Icon      string
	SortOrder int32
	Config    map[string]string
}

func (uc *UseCase) ListSSOProviders(ctx context.Context, includeDisabled bool) ([]SSOProvider, error) {
	return uc.ssoRepo.ListSSOProviders(ctx, includeDisabled)
}

func (uc *UseCase) GetSSOProvider(ctx context.Context, id int64) (*SSOProvider, error) {
	return uc.ssoRepo.GetSSOProvider(ctx, id)
}

func (uc *UseCase) CreateSSOProvider(ctx context.Context, in *CreateSSOProvider) (*SSOProvider, error) {
	return uc.ssoRepo.CreateSSOProvider(ctx, in)
}

func (uc *UseCase) UpdateSSOProvider(ctx context.Context, in *UpdateSSOProvider) (*SSOProvider, error) {
	return uc.ssoRepo.UpdateSSOProvider(ctx, in)
}

func (uc *UseCase) DeleteSSOProvider(ctx context.Context, id int64) error {
	return uc.ssoRepo.DeleteSSOProvider(ctx, id)
}
