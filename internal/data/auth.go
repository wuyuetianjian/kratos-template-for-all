package data

import (
	"context"
	"fmt"
	"slices"

	"temperate/internal/biz"
	"temperate/internal/data/ent"
	entmodule "temperate/internal/data/ent/module"
	entpermission "temperate/internal/data/ent/permission"
	entrole "temperate/internal/data/ent/role"
	entuser "temperate/internal/data/ent/user"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v3/errors"
)

type authRepo struct {
	data *Data
}

func NewAuthRepo(data *Data) biz.AuthRepo {
	return &authRepo{data: data}
}

func (r *authRepo) BootstrapAdmin(ctx context.Context, passwordHash string) (*biz.User, error) {
	module, err := r.ensureModule(ctx, "system", "System permissions", true)
	if err != nil {
		return nil, err
	}
	permission, err := r.ensurePermission(ctx, module, "*", "", "Full system access", true)
	if err != nil {
		return nil, err
	}
	for _, action := range []struct {
		name        string
		description string
	}{
		{biz.PermissionActionRead, "Read module"},
		{biz.PermissionActionWrite, "Write module"},
		{biz.PermissionActionGrant, "Grant module permissions"},
	} {
		if _, err := r.ensurePermission(ctx, module, action.name, "", action.description, true); err != nil {
			return nil, err
		}
	}
	role, err := r.ensureRole(ctx, "Admin", "System administrator", true)
	if err != nil {
		return nil, err
	}
	if _, err := role.QueryPermissions().Where(entpermission.ID(permission.ID)).Only(ctx); ent.IsNotFound(err) {
		if err := role.Update().AddPermissionIDs(permission.ID).Exec(ctx); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	u, err := r.data.WriteEnt.User.Query().
		Where(entuser.Username("admin")).
		WithRoles(withRoleEdges).
		Only(ctx)
	if ent.IsNotFound(err) {
		u, err = r.data.WriteEnt.User.Create().
			SetUsername("admin").
			SetDisplayName("System Administrator").
			SetPasswordHash(passwordHash).
			SetSystem(true).
			SetInitialPasswordUsed(false).
			AddRoleIDs(role.ID).
			Save(ctx)
		if err != nil {
			return nil, err
		}
		return r.findUserByID(ctx, r.data.WriteEnt, int64(u.ID))
	}
	if err != nil {
		return nil, err
	}
	if !u.InitialPasswordUsed {
		u, err = u.Update().SetPasswordHash(passwordHash).Save(ctx)
		if err != nil {
			return nil, err
		}
	}
	if _, err := u.QueryRoles().Where(entrole.ID(role.ID)).Only(ctx); ent.IsNotFound(err) {
		if err := u.Update().AddRoleIDs(role.ID).Exec(ctx); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return r.findUserByID(ctx, r.data.WriteEnt, int64(u.ID))
}

func (r *authRepo) FindUserByUsername(ctx context.Context, username string) (*biz.User, error) {
	u, err := r.data.ReadEnt.User.Query().
		Where(entuser.Username(username)).
		WithRoles(withRoleEdges).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, errors.NotFound(bizReasonNotFound, "user not found")
	}
	if err != nil {
		return nil, err
	}
	return toBizUser(u), nil
}

func (r *authRepo) AdminInitialPasswordUsed(ctx context.Context) (bool, error) {
	u, err := r.data.ReadEnt.User.Query().
		Where(entuser.Username("admin")).
		Only(ctx)
	if ent.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return u.InitialPasswordUsed, nil
}

func (r *authRepo) FindUserByID(ctx context.Context, id int64) (*biz.User, error) {
	return r.findUserByID(ctx, r.data.ReadEnt, id)
}

func (r *authRepo) findUserByID(ctx context.Context, client *ent.Client, id int64) (*biz.User, error) {
	u, err := client.User.Query().
		Where(entuser.ID(int(id))).
		WithRoles(withRoleEdges).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, errors.NotFound(bizReasonNotFound, "user not found")
	}
	if err != nil {
		return nil, err
	}
	return toBizUser(u), nil
}

func (r *authRepo) MarkInitialPasswordUsed(ctx context.Context, userID int64) error {
	return r.data.WriteEnt.User.UpdateOneID(int(userID)).SetInitialPasswordUsed(true).Exec(ctx)
}

func (r *authRepo) ChangePassword(ctx context.Context, userID int64, passwordHash string) error {
	return r.data.WriteEnt.User.UpdateOneID(int(userID)).
		SetPasswordHash(passwordHash).
		SetInitialPasswordUsed(true).
		Exec(ctx)
}

func (r *authRepo) EffectivePermissions(ctx context.Context, userID int64) ([]biz.Permission, []biz.Role, error) {
	roles, err := r.data.ReadEnt.User.Query().
		Where(entuser.ID(int(userID))).
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
		if err := r.collectRolePermissions(ctx, role, visitedRoles, permissionMap, &resultRoles); err != nil {
			return nil, nil, err
		}
	}
	permissions := make([]biz.Permission, 0, len(permissionMap))
	for _, permission := range permissionMap {
		permissions = append(permissions, permission)
	}
	return permissions, resultRoles, nil
}

func (r *authRepo) CreateUser(ctx context.Context, in *biz.CreateUser) (*biz.User, error) {
	u, err := r.data.WriteEnt.User.Create().
		SetUsername(in.Username).
		SetPasswordHash(in.Password).
		SetDisplayName(in.DisplayName).
		SetDisabled(in.Disabled).
		AddRoleIDs(intIDs(in.RoleIDs)...).
		Save(ctx)
	if ent.IsConstraintError(err) {
		return nil, errors.Conflict(bizReasonAlreadyExists, "user already exists")
	}
	if err != nil {
		return nil, err
	}
	return r.FindUserByID(ctx, int64(u.ID))
}

func (r *authRepo) ListUsers(ctx context.Context, page biz.Page) ([]biz.User, int, error) {
	query := r.data.ReadEnt.User.Query()
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	users, err := query.
		WithRoles(withRoleEdges).
		Order(entuser.ByID(sql.OrderAsc())).
		Limit(page.Size).
		Offset(page.Token).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	return toBizUsers(users), total, nil
}

func (r *authRepo) UpdateUser(ctx context.Context, in *biz.UpdateUser) (*biz.User, error) {
	updater := r.data.WriteEnt.User.UpdateOneID(int(in.ID)).
		SetDisplayName(in.DisplayName).
		SetDisabled(in.Disabled)
	if len(in.RoleIDs) > 0 {
		updater.ClearRoles().AddRoleIDs(intIDs(in.RoleIDs)...)
	}
	if err := updater.Exec(ctx); err != nil {
		return nil, err
	}
	return r.FindUserByID(ctx, in.ID)
}

func (r *authRepo) DeleteUser(ctx context.Context, targetUserID int64, currentUserID int64) error {
	if targetUserID == currentUserID {
		return errors.Forbidden(bizReasonSystemProtected, "cannot delete current user")
	}
	u, err := r.data.ReadEnt.User.Get(ctx, int(targetUserID))
	if ent.IsNotFound(err) {
		return errors.NotFound(bizReasonNotFound, "user not found")
	}
	if err != nil {
		return err
	}
	if u.System || u.Username == "admin" {
		return errors.Forbidden(bizReasonSystemProtected, "cannot delete system admin user")
	}
	return r.data.WriteEnt.User.DeleteOneID(int(targetUserID)).Exec(ctx)
}

func (r *authRepo) AssignUserRoles(ctx context.Context, userID int64, roleIDs []int64) (*biz.User, error) {
	u, err := r.data.ReadEnt.User.Query().Where(entuser.ID(int(userID))).WithRoles().Only(ctx)
	if ent.IsNotFound(err) {
		return nil, errors.NotFound(bizReasonNotFound, "user not found")
	}
	if err != nil {
		return nil, err
	}
	if u.Username == "admin" || u.System {
		adminRole, err := r.data.ReadEnt.Role.Query().Where(entrole.Name("Admin")).Only(ctx)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(roleIDs, int64(adminRole.ID)) {
			roleIDs = append(roleIDs, int64(adminRole.ID))
		}
	}
	if err := r.data.WriteEnt.User.UpdateOneID(int(userID)).
		ClearRoles().
		AddRoleIDs(intIDs(roleIDs)...).
		Exec(ctx); err != nil {
		return nil, err
	}
	return r.FindUserByID(ctx, userID)
}

func (r *authRepo) CreateRole(ctx context.Context, in *biz.CreateRole) (*biz.Role, error) {
	if err := r.validateRoleInheritance(ctx, 0, in.InheritedRoleIDs); err != nil {
		return nil, err
	}
	role, err := r.data.WriteEnt.Role.Create().
		SetName(in.Name).
		SetDescription(in.Description).
		AddPermissionIDs(intIDs(in.PermissionIDs)...).
		AddParentIDs(intIDs(in.InheritedRoleIDs)...).
		Save(ctx)
	if ent.IsConstraintError(err) {
		return nil, errors.Conflict(bizReasonAlreadyExists, "role already exists")
	}
	if err != nil {
		return nil, err
	}
	return r.getRole(ctx, int64(role.ID))
}

func (r *authRepo) ListRoles(ctx context.Context, page biz.Page) ([]biz.Role, int, error) {
	query := r.data.ReadEnt.Role.Query()
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	roles, err := query.
		WithPermissions(withPermissionEdges).
		WithParents(withRoleEdges).
		Order(entrole.ByID(sql.OrderAsc())).
		Limit(page.Size).
		Offset(page.Token).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	return toBizRoles(roles), total, nil
}

func (r *authRepo) GetRole(ctx context.Context, roleID int64) (*biz.Role, error) {
	return r.getRole(ctx, roleID)
}

func (r *authRepo) UpdateRole(ctx context.Context, in *biz.UpdateRole) (*biz.Role, error) {
	if err := r.data.WriteEnt.Role.UpdateOneID(int(in.ID)).SetDescription(in.Description).Exec(ctx); err != nil {
		return nil, err
	}
	return r.getRole(ctx, in.ID)
}

func (r *authRepo) DeleteRole(ctx context.Context, roleID int64) error {
	role, err := r.data.ReadEnt.Role.Get(ctx, int(roleID))
	if ent.IsNotFound(err) {
		return errors.NotFound(bizReasonNotFound, "role not found")
	}
	if err != nil {
		return err
	}
	if role.System || role.Name == "Admin" {
		return errors.Forbidden(bizReasonSystemProtected, "cannot delete system admin role")
	}
	return r.data.WriteEnt.Role.DeleteOneID(int(roleID)).Exec(ctx)
}

func (r *authRepo) AssignRolePermissions(ctx context.Context, roleID int64, permissionIDs []int64) (*biz.Role, error) {
	if err := r.data.WriteEnt.Role.UpdateOneID(int(roleID)).
		ClearPermissions().
		AddPermissionIDs(intIDs(permissionIDs)...).
		Exec(ctx); err != nil {
		return nil, err
	}
	return r.getRole(ctx, roleID)
}

func (r *authRepo) SetRoleInheritances(ctx context.Context, roleID int64, inheritedRoleIDs []int64) (*biz.Role, error) {
	if err := r.validateRoleInheritance(ctx, roleID, inheritedRoleIDs); err != nil {
		return nil, err
	}
	if err := r.data.WriteEnt.Role.UpdateOneID(int(roleID)).
		ClearParents().
		AddParentIDs(intIDs(inheritedRoleIDs)...).
		Exec(ctx); err != nil {
		return nil, err
	}
	return r.getRole(ctx, roleID)
}

func (r *authRepo) CreatePermission(ctx context.Context, in *biz.CreatePermission) (*biz.Permission, error) {
	module, err := r.ensureModule(ctx, in.Module, "", false)
	if err != nil {
		return nil, err
	}
	permission, err := r.data.WriteEnt.Permission.Create().
		SetAction(in.Action).
		SetOperation(in.Operation).
		SetDescription(in.Description).
		SetModule(module).
		Save(ctx)
	if ent.IsConstraintError(err) {
		return nil, errors.Conflict(bizReasonAlreadyExists, "permission already exists")
	}
	if err != nil {
		return nil, err
	}
	return toBizPermission(permission), nil
}

func (r *authRepo) ListPermissions(ctx context.Context, page biz.Page) ([]biz.Permission, int, error) {
	query := r.data.ReadEnt.Permission.Query()
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	permissions, err := query.
		WithModule().
		Order(entpermission.ByID(sql.OrderAsc())).
		Limit(page.Size).
		Offset(page.Token).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	return toBizPermissions(permissions), total, nil
}

func (r *authRepo) GetPermission(ctx context.Context, permissionID int64) (*biz.Permission, error) {
	permission, err := r.data.ReadEnt.Permission.Query().
		Where(entpermission.ID(int(permissionID))).
		WithModule().
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, errors.NotFound(bizReasonNotFound, "permission not found")
	}
	if err != nil {
		return nil, err
	}
	return toBizPermission(permission), nil
}

func (r *authRepo) UpdatePermission(ctx context.Context, in *biz.UpdatePermission) (*biz.Permission, error) {
	permission, err := r.data.WriteEnt.Permission.UpdateOneID(int(in.ID)).
		SetOperation(in.Operation).
		SetDescription(in.Description).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	permission, err = r.data.ReadEnt.Permission.Query().Where(entpermission.ID(permission.ID)).WithModule().Only(ctx)
	if err != nil {
		return nil, err
	}
	return toBizPermission(permission), nil
}

func (r *authRepo) DeletePermission(ctx context.Context, permissionID int64) error {
	permission, err := r.data.ReadEnt.Permission.Get(ctx, int(permissionID))
	if ent.IsNotFound(err) {
		return errors.NotFound(bizReasonNotFound, "permission not found")
	}
	if err != nil {
		return err
	}
	if permission.System {
		return errors.Forbidden(bizReasonSystemProtected, "cannot delete system permission")
	}
	return r.data.WriteEnt.Permission.DeleteOneID(int(permissionID)).Exec(ctx)
}

func (r *authRepo) ensureModule(ctx context.Context, name, description string, system bool) (*ent.Module, error) {
	module, err := r.data.WriteEnt.Module.Query().Where(entmodule.Name(name)).Only(ctx)
	if ent.IsNotFound(err) {
		return r.data.WriteEnt.Module.Create().
			SetName(name).
			SetDescription(description).
			SetSystem(system).
			Save(ctx)
	}
	return module, err
}

func (r *authRepo) ensurePermission(ctx context.Context, module *ent.Module, action, operation, description string, system bool) (*ent.Permission, error) {
	permission, err := r.data.WriteEnt.Permission.Query().
		Where(entpermission.Action(action), entpermission.HasModuleWith(entmodule.ID(module.ID))).
		WithModule().
		Only(ctx)
	if ent.IsNotFound(err) {
		return r.data.WriteEnt.Permission.Create().
			SetAction(action).
			SetOperation(operation).
			SetDescription(description).
			SetSystem(system).
			SetModule(module).
			Save(ctx)
	}
	return permission, err
}

func (r *authRepo) ensureRole(ctx context.Context, name, description string, system bool) (*ent.Role, error) {
	role, err := r.data.WriteEnt.Role.Query().Where(entrole.Name(name)).Only(ctx)
	if ent.IsNotFound(err) {
		return r.data.WriteEnt.Role.Create().
			SetName(name).
			SetDescription(description).
			SetSystem(system).
			Save(ctx)
	}
	return role, err
}

func (r *authRepo) getRole(ctx context.Context, roleID int64) (*biz.Role, error) {
	role, err := r.data.ReadEnt.Role.Query().
		Where(entrole.ID(int(roleID))).
		WithPermissions(withPermissionEdges).
		WithParents(withRoleEdges).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, errors.NotFound(bizReasonNotFound, "role not found")
	}
	if err != nil {
		return nil, err
	}
	return toBizRole(role), nil
}

func (r *authRepo) collectRolePermissions(ctx context.Context, role *ent.Role, visited map[int]struct{}, permissions map[int]biz.Permission, roles *[]biz.Role) error {
	return collectRolePermissions(ctx, r.data, role, visited, permissions, roles)
}

func collectRolePermissions(ctx context.Context, data *Data, role *ent.Role, visited map[int]struct{}, permissions map[int]biz.Permission, roles *[]biz.Role) error {
	if _, ok := visited[role.ID]; ok {
		return nil
	}
	visited[role.ID] = struct{}{}
	loaded, err := data.ReadEnt.Role.Query().
		Where(entrole.ID(role.ID)).
		WithPermissions(withPermissionEdges).
		WithParents(withRoleEdges).
		Only(ctx)
	if err != nil {
		return err
	}
	*roles = append(*roles, *toBizRole(loaded))
	for _, permission := range loaded.Edges.Permissions {
		permissions[permission.ID] = *toBizPermission(permission)
	}
	for _, parent := range loaded.Edges.Parents {
		if err := collectRolePermissions(ctx, data, parent, visited, permissions, roles); err != nil {
			return err
		}
	}
	return nil
}

func (r *authRepo) validateRoleInheritance(ctx context.Context, roleID int64, parentIDs []int64) error {
	for _, parentID := range parentIDs {
		if roleID != 0 && parentID == roleID {
			return errors.BadRequest(bizReasonInvalidHierarchy, "role cannot inherit itself")
		}
		if roleID != 0 {
			hasCycle, err := r.roleReaches(ctx, int(parentID), int(roleID), map[int]struct{}{})
			if err != nil {
				return err
			}
			if hasCycle {
				return errors.BadRequest(bizReasonInvalidHierarchy, "role inheritance cycle detected")
			}
		}
	}
	return nil
}

func (r *authRepo) roleReaches(ctx context.Context, fromID int, targetID int, visited map[int]struct{}) (bool, error) {
	if fromID == targetID {
		return true, nil
	}
	if _, ok := visited[fromID]; ok {
		return false, nil
	}
	visited[fromID] = struct{}{}
	parents, err := r.data.ReadEnt.Role.Query().Where(entrole.ID(fromID)).QueryParents().All(ctx)
	if err != nil {
		return false, err
	}
	for _, parent := range parents {
		reaches, err := r.roleReaches(ctx, parent.ID, targetID, visited)
		if err != nil || reaches {
			return reaches, err
		}
	}
	return false, nil
}

func withRoleEdges(query *ent.RoleQuery) {
	query.WithPermissions(withPermissionEdges).WithParents()
}

func withPermissionEdges(query *ent.PermissionQuery) {
	query.WithModule()
}

func intIDs(ids []int64) []int {
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		result = append(result, int(id))
	}
	return result
}

func toBizUsers(users []*ent.User) []biz.User {
	result := make([]biz.User, 0, len(users))
	for _, user := range users {
		result = append(result, *toBizUser(user))
	}
	return result
}

func toBizUser(user *ent.User) *biz.User {
	return &biz.User{
		ID:                  int64(user.ID),
		Username:            user.Username,
		PasswordHash:        user.PasswordHash,
		DisplayName:         user.DisplayName,
		Disabled:            user.Disabled,
		System:              user.System,
		InitialPasswordUsed: user.InitialPasswordUsed,
		Roles:               toBizRoles(user.Edges.Roles),
		CreatedAt:           user.CreatedAt,
		UpdatedAt:           user.UpdatedAt,
	}
}

func toBizRoles(roles []*ent.Role) []biz.Role {
	result := make([]biz.Role, 0, len(roles))
	for _, role := range roles {
		result = append(result, *toBizRole(role))
	}
	return result
}

func toBizRole(role *ent.Role) *biz.Role {
	return &biz.Role{
		ID:             int64(role.ID),
		Name:           role.Name,
		Description:    role.Description,
		System:         role.System,
		Permissions:    toBizPermissions(role.Edges.Permissions),
		InheritedRoles: toBizRoles(role.Edges.Parents),
		CreatedAt:      role.CreatedAt,
		UpdatedAt:      role.UpdatedAt,
	}
}

func toBizPermissions(permissions []*ent.Permission) []biz.Permission {
	result := make([]biz.Permission, 0, len(permissions))
	for _, permission := range permissions {
		result = append(result, *toBizPermission(permission))
	}
	return result
}

func toBizPermission(permission *ent.Permission) *biz.Permission {
	module := ""
	if permission.Edges.Module != nil {
		module = permission.Edges.Module.Name
	}
	return &biz.Permission{
		ID:          int64(permission.ID),
		Module:      module,
		Action:      permission.Action,
		Operation:   permission.Operation,
		Description: permission.Description,
		System:      permission.System,
		CreatedAt:   permission.CreatedAt,
		UpdatedAt:   permission.UpdatedAt,
	}
}

const (
	bizReasonNotFound         = "NOT_FOUND"
	bizReasonAlreadyExists    = "ALREADY_EXISTS"
	bizReasonSystemProtected  = "SYSTEM_PROTECTED"
	bizReasonInvalidHierarchy = "INVALID_ROLE_INHERITANCE"
	bizReasonUnauthorized     = "UNAUTHORIZED"
)

func unsupported(msg string) error {
	return fmt.Errorf("unsupported auth repo operation: %s", msg)
}
