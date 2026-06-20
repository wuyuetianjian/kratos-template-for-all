package data

import (
	"context"
	"time"

	"temperate/internal/biz"
	"temperate/internal/data/ent"
	entserviceaccount "temperate/internal/data/ent/serviceaccount"

	"github.com/go-kratos/kratos/v3/errors"
)

type serviceAccountRepo struct {
	data *Data
}

func NewServiceAccountRepo(data *Data) biz.ServiceAccountRepo {
	return &serviceAccountRepo{data: data}
}

func withSvcRoleEdges(q *ent.RoleQuery) {
	q.WithPermissions(func(pq *ent.PermissionQuery) {
		pq.WithModule()
	})
}

func (r *serviceAccountRepo) CreateServiceAccount(ctx context.Context, in *biz.CreateServiceAccount, tokenHash, tokenPrefix string) (*biz.ServiceAccount, error) {
	c := r.data.WriteEnt.ServiceAccount.Create().
		SetName(in.Name).
		SetTokenHash(tokenHash).
		SetTokenPrefix(tokenPrefix)
	if in.Description != "" {
		c = c.SetDescription(in.Description)
	}
	s, err := c.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, errors.Conflict(bizReasonAlreadyExists, "service account name already exists")
		}
		return nil, err
	}
	return r.getServiceAccount(ctx, int64(s.ID))
}

func (r *serviceAccountRepo) ListServiceAccounts(ctx context.Context, page biz.Page) ([]biz.ServiceAccount, int, error) {
	q := r.data.ReadEnt.ServiceAccount.Query().
		WithRoles(withSvcRoleEdges).
		Order(entserviceaccount.ByID())
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := q.Offset(page.Token).Limit(page.Size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	svcs := make([]biz.ServiceAccount, 0, len(rows))
	for _, s := range rows {
		svcs = append(svcs, *toBizServiceAccount(s))
	}
	return svcs, total, nil
}

func (r *serviceAccountRepo) GetServiceAccount(ctx context.Context, id int64) (*biz.ServiceAccount, error) {
	return r.getServiceAccount(ctx, id)
}

func (r *serviceAccountRepo) UpdateServiceAccount(ctx context.Context, in *biz.UpdateServiceAccount) (*biz.ServiceAccount, error) {
	u := r.data.WriteEnt.ServiceAccount.UpdateOneID(int(in.ID)).
		SetDescription(in.Description).
		SetDisabled(in.Disabled)
	if in.RoleIDs != nil {
		intRoleIDs := intIDs(in.RoleIDs)
		existing, err := r.data.WriteEnt.ServiceAccount.Query().
			Where(entserviceaccount.ID(int(in.ID))).
			QueryRoles().IDs(ctx)
		if err != nil {
			return nil, err
		}
		u = u.RemoveRoleIDs(existing...).AddRoleIDs(intRoleIDs...)
	}
	if _, err := u.Save(ctx); err != nil {
		return nil, err
	}
	return r.getServiceAccount(ctx, in.ID)
}

func (r *serviceAccountRepo) DeleteServiceAccount(ctx context.Context, id int64) error {
	err := r.data.WriteEnt.ServiceAccount.DeleteOneID(int(id)).Exec(ctx)
	if ent.IsNotFound(err) {
		return errors.NotFound(bizReasonNotFound, "service account not found")
	}
	return err
}

func (r *serviceAccountRepo) UpdateServiceAccountToken(ctx context.Context, id int64, tokenHash, tokenPrefix string, expiresAt *time.Time) (*biz.ServiceAccount, error) {
	u := r.data.WriteEnt.ServiceAccount.UpdateOneID(int(id)).
		SetTokenHash(tokenHash).
		SetTokenPrefix(tokenPrefix)
	if expiresAt != nil {
		u = u.SetExpiresAt(*expiresAt)
	} else {
		u = u.ClearExpiresAt()
	}
	if _, err := u.Save(ctx); err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.NotFound(bizReasonNotFound, "service account not found")
		}
		return nil, err
	}
	return r.getServiceAccount(ctx, id)
}

func (r *serviceAccountRepo) FindServiceAccountByTokenHash(ctx context.Context, hash string) (*biz.ServiceAccount, error) {
	s, err := r.data.ReadEnt.ServiceAccount.Query().
		Where(entserviceaccount.TokenHash(hash)).
		WithRoles(withSvcRoleEdges).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, errors.Unauthorized(bizReasonUnauthorized, "invalid service token")
	}
	if err != nil {
		return nil, err
	}
	return toBizServiceAccount(s), nil
}

func (r *serviceAccountRepo) EffectiveServiceAccountPermissions(ctx context.Context, id int64) ([]biz.Permission, []biz.Role, error) {
	roles, err := r.data.ReadEnt.ServiceAccount.Query().
		Where(entserviceaccount.ID(int(id))).
		QueryRoles().
		WithPermissions(withPermissionEdges).
		WithParents(withRoleEdges).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	visitedRoles := map[int]struct{}{}
	permissionMap := map[int]biz.Permission{}
	resultRoles := make([]biz.Role, 0, len(roles))
	for _, role := range roles {
		if err := collectRolePermissions(ctx, r.data, role, visitedRoles, permissionMap, &resultRoles); err != nil {
			return nil, nil, err
		}
	}
	permissions := make([]biz.Permission, 0, len(permissionMap))
	for _, perm := range permissionMap {
		permissions = append(permissions, perm)
	}
	return permissions, resultRoles, nil
}

func (r *serviceAccountRepo) getServiceAccount(ctx context.Context, id int64) (*biz.ServiceAccount, error) {
	s, err := r.data.ReadEnt.ServiceAccount.Query().
		Where(entserviceaccount.ID(int(id))).
		WithRoles(withSvcRoleEdges).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, errors.NotFound(bizReasonNotFound, "service account not found")
	}
	if err != nil {
		return nil, err
	}
	return toBizServiceAccount(s), nil
}

func toBizServiceAccount(s *ent.ServiceAccount) *biz.ServiceAccount {
	roles := make([]biz.Role, 0, len(s.Edges.Roles))
	for _, r := range s.Edges.Roles {
		roles = append(roles, *toBizRole(r))
	}
	return &biz.ServiceAccount{
		ID:          int64(s.ID),
		Name:        s.Name,
		Description: s.Description,
		TokenHash:   s.TokenHash,
		TokenPrefix: s.TokenPrefix,
		ExpiresAt:   s.ExpiresAt,
		Disabled:    s.Disabled,
		Roles:       roles,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}
