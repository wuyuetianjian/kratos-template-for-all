package service

import (
	"context"
	"log/slog"

	v1 "temperate/api/temperate/v1"
	"temperate/internal/biz"
	"temperate/internal/conf"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/google/wire"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(NewIncidentService)

type IncidentService struct {
	v1.UnimplementedTemperateServiceServer

	cnf *conf.Data

	useCase *biz.UseCase
	log     *slog.Logger
}

// NewIncidentService new a IncidentService.
func NewIncidentService(cnf *conf.Data, useCase *biz.UseCase, logger *slog.Logger) *IncidentService {
	if logger == nil {
		logger = log.Default()
	}
	return &IncidentService{
		cnf:     cnf,
		useCase: useCase,
		log:     logger.With("module", "service/incident"),
	}
}

func (s *IncidentService) Health(context.Context, *emptypb.Empty) (*v1.GetMessageResponse, error) {
	return &v1.GetMessageResponse{Message: "ok"}, nil
}

func (s *IncidentService) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginReply, error) {
	result, err := s.useCase.Login(ctx, req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, err
	}
	return &v1.LoginReply{
		Token:              result.Token,
		User:               convertUser(result.User),
		MustChangePassword: result.MustChangePassword,
		InitialPassword:    result.InitialPassword,
	}, nil
}

func (s *IncidentService) GetInitialPassword(ctx context.Context, _ *emptypb.Empty) (*v1.InitialPasswordReply, error) {
	result, err := s.useCase.InitialPassword(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.InitialPasswordReply{
		Available:       result.Available,
		Username:        result.Username,
		InitialPassword: result.InitialPassword,
	}, nil
}

func (s *IncidentService) ChangePassword(ctx context.Context, req *v1.ChangePasswordRequest) (*emptypb.Empty, error) {
	auth, ok := biz.AuthFromContext(ctx)
	if !ok {
		return nil, biz.ErrUnauthorized()
	}
	if err := s.useCase.ChangePassword(ctx, auth.UserID, req.GetOldPassword(), req.GetNewPassword()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *IncidentService) GetCurrentUser(ctx context.Context, _ *emptypb.Empty) (*v1.User, error) {
	auth, ok := biz.AuthFromContext(ctx)
	if !ok {
		return nil, biz.ErrUnauthorized()
	}
	user, err := s.useCase.CurrentUser(ctx, auth.UserID)
	if err != nil {
		return nil, err
	}
	return convertUser(user), nil
}

func (s *IncidentService) CreateUser(ctx context.Context, req *v1.CreateUserRequest) (*v1.User, error) {
	user, err := s.useCase.CreateUser(ctx, &biz.CreateUser{
		Username:    req.GetUsername(),
		Password:    req.GetPassword(),
		DisplayName: req.GetDisplayName(),
		Disabled:    req.GetDisabled(),
		RoleIDs:     req.GetRoleIds(),
	})
	if err != nil {
		return nil, err
	}
	return convertUser(user), nil
}

func (s *IncidentService) ListUsers(ctx context.Context, req *v1.ListUsersRequest) (*v1.ListUsersReply, error) {
	users, total, err := s.useCase.ListUsers(ctx, biz.Page{Size: int(req.GetPageSize()), Token: int(req.GetPageToken())})
	if err != nil {
		return nil, err
	}
	return &v1.ListUsersReply{Users: convertUsers(users), Total: int32(total)}, nil
}

func (s *IncidentService) GetUser(ctx context.Context, req *v1.GetUserRequest) (*v1.User, error) {
	user, err := s.useCase.CurrentUser(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return convertUser(user), nil
}

func (s *IncidentService) UpdateUser(ctx context.Context, req *v1.UpdateUserRequest) (*v1.User, error) {
	user, err := s.useCase.UpdateUser(ctx, &biz.UpdateUser{
		ID:          req.GetId(),
		DisplayName: req.GetDisplayName(),
		Disabled:    req.GetDisabled(),
		RoleIDs:     req.GetRoleIds(),
	})
	if err != nil {
		return nil, err
	}
	return convertUser(user), nil
}

func (s *IncidentService) DeleteUser(ctx context.Context, req *v1.DeleteUserRequest) (*emptypb.Empty, error) {
	if err := s.useCase.DeleteUser(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *IncidentService) AssignUserRoles(ctx context.Context, req *v1.AssignUserRolesRequest) (*v1.User, error) {
	user, err := s.useCase.AssignUserRoles(ctx, req.GetUserId(), req.GetRoleIds())
	if err != nil {
		return nil, err
	}
	return convertUser(user), nil
}

func (s *IncidentService) CreateRole(ctx context.Context, req *v1.CreateRoleRequest) (*v1.Role, error) {
	role, err := s.useCase.CreateRole(ctx, &biz.CreateRole{
		Name:             req.GetName(),
		Description:      req.GetDescription(),
		PermissionIDs:    req.GetPermissionIds(),
		InheritedRoleIDs: req.GetInheritedRoleIds(),
	})
	if err != nil {
		return nil, err
	}
	return convertRole(role), nil
}

func (s *IncidentService) ListRoles(ctx context.Context, req *v1.ListRolesRequest) (*v1.ListRolesReply, error) {
	roles, total, err := s.useCase.ListRoles(ctx, biz.Page{Size: int(req.GetPageSize()), Token: int(req.GetPageToken())})
	if err != nil {
		return nil, err
	}
	return &v1.ListRolesReply{Roles: convertRoles(roles), Total: int32(total)}, nil
}

func (s *IncidentService) GetRole(ctx context.Context, req *v1.GetRoleRequest) (*v1.Role, error) {
	role, err := s.useCase.GetRole(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return convertRole(role), nil
}

func (s *IncidentService) UpdateRole(ctx context.Context, req *v1.UpdateRoleRequest) (*v1.Role, error) {
	role, err := s.useCase.UpdateRole(ctx, &biz.UpdateRole{ID: req.GetId(), Description: req.GetDescription()})
	if err != nil {
		return nil, err
	}
	return convertRole(role), nil
}

func (s *IncidentService) DeleteRole(ctx context.Context, req *v1.DeleteRoleRequest) (*emptypb.Empty, error) {
	if err := s.useCase.DeleteRole(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *IncidentService) AssignRolePermissions(ctx context.Context, req *v1.AssignRolePermissionsRequest) (*v1.Role, error) {
	role, err := s.useCase.AssignRolePermissions(ctx, req.GetRoleId(), req.GetPermissionIds())
	if err != nil {
		return nil, err
	}
	return convertRole(role), nil
}

func (s *IncidentService) SetRoleInheritances(ctx context.Context, req *v1.SetRoleInheritancesRequest) (*v1.Role, error) {
	role, err := s.useCase.SetRoleInheritances(ctx, req.GetRoleId(), req.GetInheritedRoleIds())
	if err != nil {
		return nil, err
	}
	return convertRole(role), nil
}

func (s *IncidentService) CreatePermission(ctx context.Context, req *v1.CreatePermissionRequest) (*v1.Permission, error) {
	permission, err := s.useCase.CreatePermission(ctx, &biz.CreatePermission{
		Module:      req.GetModule(),
		Action:      req.GetAction(),
		Operation:   req.GetOperation(),
		Description: req.GetDescription(),
	})
	if err != nil {
		return nil, err
	}
	return convertPermission(permission), nil
}

func (s *IncidentService) ListPermissions(ctx context.Context, req *v1.ListPermissionsRequest) (*v1.ListPermissionsReply, error) {
	permissions, total, err := s.useCase.ListPermissions(ctx, biz.Page{Size: int(req.GetPageSize()), Token: int(req.GetPageToken())})
	if err != nil {
		return nil, err
	}
	return &v1.ListPermissionsReply{Permissions: convertPermissions(permissions), Total: int32(total)}, nil
}

func (s *IncidentService) UpdatePermission(ctx context.Context, req *v1.UpdatePermissionRequest) (*v1.Permission, error) {
	permission, err := s.useCase.UpdatePermission(ctx, &biz.UpdatePermission{
		ID:          req.GetId(),
		Operation:   req.GetOperation(),
		Description: req.GetDescription(),
	})
	if err != nil {
		return nil, err
	}
	return convertPermission(permission), nil
}

func (s *IncidentService) DeletePermission(ctx context.Context, req *v1.DeletePermissionRequest) (*emptypb.Empty, error) {
	if err := s.useCase.DeletePermission(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func convertUsers(users []biz.User) []*v1.User {
	result := make([]*v1.User, 0, len(users))
	for i := range users {
		result = append(result, convertUser(&users[i]))
	}
	return result
}

func convertUser(user *biz.User) *v1.User {
	if user == nil {
		return nil
	}
	return &v1.User{
		Id:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Disabled:    user.Disabled,
		System:      user.System,
		Roles:       convertRoles(user.Roles),
		CreatedAt:   timestamppb.New(user.CreatedAt),
		UpdatedAt:   timestamppb.New(user.UpdatedAt),
	}
}

func convertRoles(roles []biz.Role) []*v1.Role {
	result := make([]*v1.Role, 0, len(roles))
	for i := range roles {
		result = append(result, convertRole(&roles[i]))
	}
	return result
}

func convertRole(role *biz.Role) *v1.Role {
	if role == nil {
		return nil
	}
	return &v1.Role{
		Id:             role.ID,
		Name:           role.Name,
		Description:    role.Description,
		System:         role.System,
		Permissions:    convertPermissions(role.Permissions),
		InheritedRoles: convertRoles(role.InheritedRoles),
		CreatedAt:      timestamppb.New(role.CreatedAt),
		UpdatedAt:      timestamppb.New(role.UpdatedAt),
	}
}

func convertPermissions(permissions []biz.Permission) []*v1.Permission {
	result := make([]*v1.Permission, 0, len(permissions))
	for i := range permissions {
		result = append(result, convertPermission(&permissions[i]))
	}
	return result
}

func convertPermission(permission *biz.Permission) *v1.Permission {
	if permission == nil {
		return nil
	}
	return &v1.Permission{
		Id:          permission.ID,
		Module:      permission.Module,
		Action:      permission.Action,
		Operation:   permission.Operation,
		Description: permission.Description,
		System:      permission.System,
		CreatedAt:   timestamppb.New(permission.CreatedAt),
		UpdatedAt:   timestamppb.New(permission.UpdatedAt),
	}
}
