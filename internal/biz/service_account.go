package biz

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
)

// ServiceAccountRepo defines persistence operations for service accounts.
type ServiceAccountRepo interface {
	CreateServiceAccount(context.Context, *CreateServiceAccount, string, string) (*ServiceAccount, error)
	ListServiceAccounts(context.Context, Page) ([]ServiceAccount, int, error)
	GetServiceAccount(context.Context, int64) (*ServiceAccount, error)
	UpdateServiceAccount(context.Context, *UpdateServiceAccount) (*ServiceAccount, error)
	DeleteServiceAccount(context.Context, int64) error
	UpdateServiceAccountToken(context.Context, int64, string, string, *time.Time) (*ServiceAccount, error)
	FindServiceAccountByTokenHash(context.Context, string) (*ServiceAccount, error)
	EffectiveServiceAccountPermissions(context.Context, int64) ([]Permission, []Role, error)
}

type ServiceAccount struct {
	ID          int64
	Name        string
	Description string
	TokenHash   string
	TokenPrefix string
	ExpiresAt   *time.Time
	Disabled    bool
	Roles       []Role
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CreateServiceAccount struct {
	Name          string
	Description   string
	ExpiresInDays int32
	RoleIDs       []int64
}

type UpdateServiceAccount struct {
	ID          int64
	Description string
	Disabled    bool
	RoleIDs     []int64
}

type ServiceAccountTokenResult struct {
	ServiceAccount *ServiceAccount
	Token          string
}

// GenerateServiceToken creates a new svc_ token.
// Format: svc_ + base64url(8-byte-expiry-unix + 24-byte-random)
// A zero expiry (expiresInDays==0) means "never expires" and encodes 0.
func GenerateServiceToken(expiresInDays int32) (token string, tokenHash string, expiresAt *time.Time, err error) {
	payload := make([]byte, 32)
	var exp int64
	if expiresInDays > 0 {
		t := time.Now().Add(time.Duration(expiresInDays) * 24 * time.Hour)
		expiresAt = &t
		exp = t.Unix()
	}
	binary.BigEndian.PutUint64(payload[:8], uint64(exp))
	if _, err = rand.Read(payload[8:]); err != nil {
		return
	}
	token = "svc_" + base64.RawURLEncoding.EncodeToString(payload)
	h := sha256.Sum256([]byte(token))
	tokenHash = hex.EncodeToString(h[:])
	return
}

// TokenPrefix returns the display prefix of a service token (first 12 chars).
func TokenPrefix(token string) string {
	if len(token) >= 12 {
		return token[:12]
	}
	return token
}

// ValidateServiceTokenExpiry does a fast pre-check by decoding expiry from the
// token itself, avoiding a DB round-trip for already-expired tokens.
func ValidateServiceTokenExpiry(token string) error {
	const prefix = "svc_"
	if len(token) < len(prefix)+4 {
		return errors.Unauthorized(reasonUnauthorized, "invalid service token")
	}
	if token[:len(prefix)] != prefix {
		return errors.Unauthorized(reasonUnauthorized, "invalid service token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(token[len(prefix):])
	if err != nil || len(payload) < 8 {
		return errors.Unauthorized(reasonUnauthorized, "invalid service token")
	}
	exp := int64(binary.BigEndian.Uint64(payload[:8]))
	if exp != 0 && time.Now().Unix() > exp {
		return errors.Unauthorized(reasonUnauthorized, "service token has expired")
	}
	return nil
}

// ServiceTokenHash returns the SHA256 hex hash of a token for DB lookup.
func ServiceTokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// ── UseCase methods ───────────────────────────────────────────────────────────

func (uc *UseCase) CreateServiceAccount(ctx context.Context, in *CreateServiceAccount) (*ServiceAccountTokenResult, error) {
	token, tokenHash, expiresAt, err := GenerateServiceToken(in.ExpiresInDays)
	if err != nil {
		return nil, err
	}
	prefix := TokenPrefix(token)
	svc, err := uc.serviceAccountRepo.CreateServiceAccount(ctx, in, tokenHash, prefix)
	if err != nil {
		return nil, err
	}
	// Set expiry and roles via update
	svc, err = uc.serviceAccountRepo.UpdateServiceAccountToken(ctx, svc.ID, tokenHash, prefix, expiresAt)
	if err != nil {
		return nil, err
	}
	if len(in.RoleIDs) > 0 {
		svc, err = uc.serviceAccountRepo.UpdateServiceAccount(ctx, &UpdateServiceAccount{
			ID:          svc.ID,
			Description: svc.Description,
			Disabled:    svc.Disabled,
			RoleIDs:     in.RoleIDs,
		})
		if err != nil {
			return nil, err
		}
	}
	return &ServiceAccountTokenResult{ServiceAccount: svc, Token: token}, nil
}

func (uc *UseCase) ListServiceAccounts(ctx context.Context, page Page) ([]ServiceAccount, int, error) {
	return uc.serviceAccountRepo.ListServiceAccounts(ctx, page.normalize())
}

func (uc *UseCase) GetServiceAccount(ctx context.Context, id int64) (*ServiceAccount, error) {
	return uc.serviceAccountRepo.GetServiceAccount(ctx, id)
}

func (uc *UseCase) UpdateServiceAccount(ctx context.Context, in *UpdateServiceAccount) (*ServiceAccount, error) {
	return uc.serviceAccountRepo.UpdateServiceAccount(ctx, in)
}

func (uc *UseCase) DeleteServiceAccount(ctx context.Context, id int64) error {
	return uc.serviceAccountRepo.DeleteServiceAccount(ctx, id)
}

func (uc *UseCase) RegenerateServiceAccountToken(ctx context.Context, id int64, expiresInDays int32) (*ServiceAccountTokenResult, error) {
	token, tokenHash, expiresAt, err := GenerateServiceToken(expiresInDays)
	if err != nil {
		return nil, err
	}
	prefix := TokenPrefix(token)
	svc, err := uc.serviceAccountRepo.UpdateServiceAccountToken(ctx, id, tokenHash, prefix, expiresAt)
	if err != nil {
		return nil, err
	}
	return &ServiceAccountTokenResult{ServiceAccount: svc, Token: token}, nil
}

func (uc *UseCase) AuthorizeServiceAccount(ctx context.Context, token, operation string) (*AuthContext, error) {
	if err := ValidateServiceTokenExpiry(token); err != nil {
		return nil, err
	}
	hash := ServiceTokenHash(token)
	svc, err := uc.serviceAccountRepo.FindServiceAccountByTokenHash(ctx, hash)
	if err != nil {
		return nil, errors.Unauthorized(reasonUnauthorized, "invalid service token")
	}
	if svc.Disabled {
		return nil, errors.Forbidden(reasonForbidden, "service account is disabled")
	}
	if svc.ExpiresAt != nil && time.Now().After(*svc.ExpiresAt) {
		return nil, errors.Unauthorized(reasonUnauthorized, "service token has expired")
	}
	permissions, roles, err := uc.serviceAccountRepo.EffectiveServiceAccountPermissions(ctx, svc.ID)
	if err != nil {
		return nil, err
	}
	if !hasPermission(roles, permissions, operation) {
		return nil, errors.Forbidden(reasonForbidden, "permission denied")
	}
	return &AuthContext{
		UserID:           svc.ID,
		Username:         "svc:" + svc.Name,
		IsServiceAccount: true,
		Permissions:      permissions,
		Roles:            roles,
	}, nil
}
