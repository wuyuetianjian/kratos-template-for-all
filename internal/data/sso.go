package data

import (
	"context"
	"github.com/wuyuetianjian/kratos-template-for-all/internal/biz"
	"github.com/wuyuetianjian/kratos-template-for-all/internal/data/ent"
	entsso "github.com/wuyuetianjian/kratos-template-for-all/internal/data/ent/ssoprovider"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v3/errors"
)

type ssoProviderRepo struct {
	data *Data
}

func NewSSOProviderRepo(data *Data) biz.SSOProviderRepo {
	return &ssoProviderRepo{data: data}
}

func (r *ssoProviderRepo) ListSSOProviders(ctx context.Context, includeDisabled bool) ([]biz.SSOProvider, error) {
	q := r.data.ReadEnt.SSOProvider.Query().Order(entsso.BySortOrder(sql.OrderAsc()), entsso.ByID(sql.OrderAsc()))
	if !includeDisabled {
		q = q.Where(entsso.EnabledEQ(true))
	}
	rows, err := q.All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]biz.SSOProvider, 0, len(rows))
	for _, row := range rows {
		result = append(result, toBizSSO(row))
	}
	return result, nil
}

func (r *ssoProviderRepo) GetSSOProvider(ctx context.Context, id int64) (*biz.SSOProvider, error) {
	row, err := r.data.ReadEnt.SSOProvider.Get(ctx, int(id))
	if ent.IsNotFound(err) {
		return nil, errors.NotFound(bizReasonNotFound, "sso provider not found")
	}
	if err != nil {
		return nil, err
	}
	p := toBizSSO(row)
	return &p, nil
}

func (r *ssoProviderRepo) CreateSSOProvider(ctx context.Context, in *biz.CreateSSOProvider) (*biz.SSOProvider, error) {
	row, err := r.data.WriteEnt.SSOProvider.Create().
		SetName(in.Name).
		SetType(in.Type).
		SetEnabled(in.Enabled).
		SetIcon(in.Icon).
		SetSortOrder(int(in.SortOrder)).
		SetConfig(in.Config).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	p := toBizSSO(row)
	return &p, nil
}

func (r *ssoProviderRepo) UpdateSSOProvider(ctx context.Context, in *biz.UpdateSSOProvider) (*biz.SSOProvider, error) {
	row, err := r.data.WriteEnt.SSOProvider.UpdateOneID(int(in.ID)).
		SetName(in.Name).
		SetEnabled(in.Enabled).
		SetIcon(in.Icon).
		SetSortOrder(int(in.SortOrder)).
		SetConfig(in.Config).
		Save(ctx)
	if ent.IsNotFound(err) {
		return nil, errors.NotFound(bizReasonNotFound, "sso provider not found")
	}
	if err != nil {
		return nil, err
	}
	p := toBizSSO(row)
	return &p, nil
}

func (r *ssoProviderRepo) DeleteSSOProvider(ctx context.Context, id int64) error {
	err := r.data.WriteEnt.SSOProvider.DeleteOneID(int(id)).Exec(ctx)
	if ent.IsNotFound(err) {
		return errors.NotFound(bizReasonNotFound, "sso provider not found")
	}
	return err
}

func toBizSSO(row *ent.SSOProvider) biz.SSOProvider {
	cfg := row.Config
	if cfg == nil {
		cfg = map[string]string{}
	}
	return biz.SSOProvider{
		ID:        int64(row.ID),
		Name:      row.Name,
		Type:      row.Type,
		Enabled:   row.Enabled,
		Icon:      row.Icon,
		SortOrder: int32(row.SortOrder),
		Config:    cfg,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
