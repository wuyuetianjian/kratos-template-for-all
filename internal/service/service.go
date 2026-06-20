package service

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"

	v1 "temperate/api/temperate/v1"
	"temperate/internal/biz"
	"temperate/internal/conf"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/transport"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
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
	ip, ua := requestMeta(ctx)
	browser, os := biz.ParseUserAgent(ua)
	tokenHash := biz.TokenHash(result.Token)
	_ = s.useCase.CreateSession(ctx, tokenHash, ip, browser, os, result.User.ID, result.User.Username)
	s.useCase.LogAuditEvent(ctx, "login", "session", result.User.Username, ip, "")
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
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "create", "user", req.GetUsername(), ip, "")
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
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "update", "user", user.Username, ip, "")
	return convertUser(user), nil
}

func (s *IncidentService) DeleteUser(ctx context.Context, req *v1.DeleteUserRequest) (*emptypb.Empty, error) {
	if err := s.useCase.DeleteUser(ctx, req.GetId()); err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "delete", "user", resourceID(req.GetId()), ip, "")
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
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "create", "role", req.GetName(), ip, "")
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
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "update", "role", role.Name, ip, "")
	return convertRole(role), nil
}

func (s *IncidentService) DeleteRole(ctx context.Context, req *v1.DeleteRoleRequest) (*emptypb.Empty, error) {
	if err := s.useCase.DeleteRole(ctx, req.GetId()); err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "delete", "role", resourceID(req.GetId()), ip, "")
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

func (s *IncidentService) ListPermissionActions(ctx context.Context, _ *emptypb.Empty) (*v1.ListPermissionActionsReply, error) {
	actions := s.useCase.PermissionActions(ctx)
	result := make([]*v1.PermissionAction, 0, len(actions))
	for _, action := range actions {
		result = append(result, &v1.PermissionAction{
			Action:      action.Action,
			Name:        action.Name,
			Description: action.Description,
		})
	}
	return &v1.ListPermissionActionsReply{Actions: result}, nil
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
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "delete", "permission", resourceID(req.GetId()), ip, "")
	return &emptypb.Empty{}, nil
}

// ── SSO providers ──────────────────────────────────────────────────────────

func (s *IncidentService) ListSSOProvidersPublic(ctx context.Context, _ *emptypb.Empty) (*v1.ListSSOProvidersPublicReply, error) {
	providers, err := s.useCase.ListSSOProviders(ctx, false)
	if err != nil {
		return nil, err
	}
	briefs := make([]*v1.SSOProviderBrief, 0, len(providers))
	for _, p := range providers {
		briefs = append(briefs, &v1.SSOProviderBrief{Id: p.ID, Name: p.Name, Type: p.Type, Icon: p.Icon})
	}
	return &v1.ListSSOProvidersPublicReply{Providers: briefs}, nil
}

func (s *IncidentService) ListSSOProviders(ctx context.Context, req *v1.ListSSOProvidersRequest) (*v1.ListSSOProvidersReply, error) {
	providers, err := s.useCase.ListSSOProviders(ctx, req.GetIncludeDisabled())
	if err != nil {
		return nil, err
	}
	return &v1.ListSSOProvidersReply{Providers: convertSSOProviders(providers)}, nil
}

func (s *IncidentService) GetSSOProvider(ctx context.Context, req *v1.GetSSOProviderRequest) (*v1.SSOProvider, error) {
	p, err := s.useCase.GetSSOProvider(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return convertSSOProvider(p), nil
}

func (s *IncidentService) CreateSSOProvider(ctx context.Context, req *v1.CreateSSOProviderRequest) (*v1.SSOProvider, error) {
	p, err := s.useCase.CreateSSOProvider(ctx, &biz.CreateSSOProvider{
		Name:      req.GetName(),
		Type:      req.GetType(),
		Enabled:   req.GetEnabled(),
		Icon:      req.GetIcon(),
		SortOrder: req.GetSortOrder(),
		Config:    req.GetConfig(),
	})
	if err != nil {
		return nil, err
	}
	return convertSSOProvider(p), nil
}

func (s *IncidentService) UpdateSSOProvider(ctx context.Context, req *v1.UpdateSSOProviderRequest) (*v1.SSOProvider, error) {
	p, err := s.useCase.UpdateSSOProvider(ctx, &biz.UpdateSSOProvider{
		ID:        req.GetId(),
		Name:      req.GetName(),
		Enabled:   req.GetEnabled(),
		Icon:      req.GetIcon(),
		SortOrder: req.GetSortOrder(),
		Config:    req.GetConfig(),
	})
	if err != nil {
		return nil, err
	}
	return convertSSOProvider(p), nil
}

func (s *IncidentService) DeleteSSOProvider(ctx context.Context, req *v1.DeleteSSOProviderRequest) (*emptypb.Empty, error) {
	if err := s.useCase.DeleteSSOProvider(ctx, req.GetId()); err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "delete", "sso_provider", resourceID(req.GetId()), ip, "")
	return &emptypb.Empty{}, nil
}

// ── Sessions ──────────────────────────────────────────────────────────────────

func (s *IncidentService) ListSessions(ctx context.Context, req *v1.ListSessionsRequest) (*v1.ListSessionsReply, error) {
	sessions, total, err := s.useCase.ListSessions(ctx, biz.Page{Size: int(req.GetPageSize()), Token: int(req.GetPageToken())})
	if err != nil {
		return nil, err
	}
	result := make([]*v1.UserSession, 0, len(sessions))
	for i := range sessions {
		sess := &sessions[i]
		result = append(result, &v1.UserSession{
			Id:           sess.ID,
			UserId:       sess.UserID,
			Username:     sess.Username,
			Ip:           sess.IP,
			Browser:      sess.Browser,
			Os:           sess.OS,
			Status:       sess.Status,
			KickedBy:     sess.KickedBy,
			LoginAt:      timestamppb.New(sess.LoginAt),
			LastAccessAt: timestamppb.New(sess.LastAccessAt),
		})
	}
	return &v1.ListSessionsReply{Sessions: result, Total: int32(total)}, nil
}

func (s *IncidentService) KickSession(ctx context.Context, req *v1.KickSessionRequest) (*emptypb.Empty, error) {
	if err := s.useCase.KickSession(ctx, req.GetId()); err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "kick", "session", resourceID(req.GetId()), ip, "")
	return &emptypb.Empty{}, nil
}

// ── Audit logs ────────────────────────────────────────────────────────────────

func (s *IncidentService) ListAuditLogs(ctx context.Context, req *v1.ListAuditLogsRequest) (*v1.ListAuditLogsReply, error) {
	logs, total, err := s.useCase.ListAuditLogs(ctx, req.GetAction(), biz.Page{Size: int(req.GetPageSize()), Token: int(req.GetPageToken())})
	if err != nil {
		return nil, err
	}
	result := make([]*v1.AuditLog, 0, len(logs))
	for i := range logs {
		l := &logs[i]
		result = append(result, &v1.AuditLog{
			Id:           l.ID,
			UserId:       l.UserID,
			Username:     l.Username,
			Action:       l.Action,
			ResourceType: l.ResourceType,
			ResourceName: l.ResourceName,
			Ip:           l.IP,
			Detail:       l.Detail,
			CreatedAt:    timestamppb.New(l.CreatedAt),
		})
	}
	return &v1.ListAuditLogsReply{Logs: result, Total: int32(total)}, nil
}

// ── System settings ───────────────────────────────────────────────────────────

func (s *IncidentService) GetSystemSettings(ctx context.Context, _ *emptypb.Empty) (*v1.SystemSettingsReply, error) {
	settings, err := s.useCase.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.SystemSettingsReply{
		AuditLogRetentionDays:   settings.AuditLogRetentionDays,
		SessionLogRetentionDays: settings.SessionLogRetentionDays,
	}, nil
}

func (s *IncidentService) UpdateSystemSettings(ctx context.Context, req *v1.UpdateSystemSettingsRequest) (*v1.SystemSettingsReply, error) {
	settings := &biz.SystemSettings{
		AuditLogRetentionDays:   req.GetAuditLogRetentionDays(),
		SessionLogRetentionDays: req.GetSessionLogRetentionDays(),
	}
	if err := s.useCase.UpdateSettings(ctx, settings); err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "update", "settings", "system", ip, "")
	return &v1.SystemSettingsReply{
		AuditLogRetentionDays:   settings.AuditLogRetentionDays,
		SessionLogRetentionDays: settings.SessionLogRetentionDays,
	}, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func requestMeta(ctx context.Context) (ip, userAgent string) {
	header, ok := transport.FromServerContext(ctx)
	if !ok {
		return
	}
	reqHeader := header.RequestHeader()
	ip = clientIPFromValues(
		reqHeader.Get("X-Forwarded-For"),
		reqHeader.Get("X-Real-IP"),
		reqHeader.Get("X-Client-IP"),
		reqHeader.Get("Remote-Addr"),
	)
	if ip == "" {
		if req, ok := khttp.RequestFromServerContext(ctx); ok && req != nil {
			ip = normalizeClientIP(req.RemoteAddr)
		}
	}
	userAgent = reqHeader.Get("User-Agent")
	return
}

func clientIPFromValues(values ...string) string {
	for _, value := range values {
		if ip := normalizeClientIP(value); ip != "" {
			return ip
		}
	}
	return ""
}

func normalizeClientIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if before, _, found := strings.Cut(value, ","); found {
		value = strings.TrimSpace(before)
	}
	if before, _, found := strings.Cut(value, "（"); found {
		value = strings.TrimSpace(before)
	}
	if before, _, found := strings.Cut(value, "("); found {
		value = strings.TrimSpace(before)
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		return strings.Trim(host, "[]")
	}
	if strings.HasPrefix(value, "[") {
		if end := strings.Index(value, "]"); end > 0 {
			return value[1:end]
		}
	}
	return value
}

func resourceID(id int64) string {
	return fmt.Sprintf("%d", id)
}

func convertSSOProviders(providers []biz.SSOProvider) []*v1.SSOProvider {
	result := make([]*v1.SSOProvider, 0, len(providers))
	for i := range providers {
		result = append(result, convertSSOProvider(&providers[i]))
	}
	return result
}

func convertSSOProvider(p *biz.SSOProvider) *v1.SSOProvider {
	if p == nil {
		return nil
	}
	return &v1.SSOProvider{
		Id:        p.ID,
		Name:      p.Name,
		Type:      p.Type,
		Enabled:   p.Enabled,
		Icon:      p.Icon,
		SortOrder: p.SortOrder,
		Config:    p.Config,
		CreatedAt: timestamppb.New(p.CreatedAt),
		UpdatedAt: timestamppb.New(p.UpdatedAt),
	}
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
