package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"reflect"
	"strings"
	"time"

	v1 "temperate/api/temperate/v1"
	"temperate/internal/biz"
	"temperate/internal/conf"

	"github.com/go-kratos/kratos/v3/errors"
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
	if result.Requires2FA {
		return &v1.LoginReply{
			Requires_2Fa: true,
			PreAuthToken: result.PreAuthToken,
			User:         convertUser(result.User),
		}, nil
	}
	ip, ua := requestMeta(ctx)
	browser, os := biz.ParseUserAgent(ua)
	tokenHash := biz.TokenHash(result.Token)
	_ = s.useCase.CreateSession(ctx, tokenHash, ip, browser, os, result.User.ID, result.User.Username)
	s.useCase.LogAuditEvent(ctx, "login", "session", result.User.Username, ip,
		auditFieldsDetail("登录 "+userLabel(result.User), loginAuditFields(result.User, browser, os, false)))
	return &v1.LoginReply{
		Token:              result.Token,
		User:               convertUser(result.User),
		MustChangePassword: result.MustChangePassword,
		InitialPassword:    result.InitialPassword,
	}, nil
}

func (s *IncidentService) VerifyTOTPLogin(ctx context.Context, req *v1.VerifyTOTPLoginRequest) (*v1.LoginReply, error) {
	result, err := s.useCase.VerifyTOTPLogin(ctx, req.GetPreAuthToken(), req.GetTotpCode())
	if err != nil {
		return nil, err
	}
	ip, ua := requestMeta(ctx)
	browser, os := biz.ParseUserAgent(ua)
	tokenHash := biz.TokenHash(result.Token)
	_ = s.useCase.CreateSession(ctx, tokenHash, ip, browser, os, result.User.ID, result.User.Username)
	s.useCase.LogAuditEvent(ctx, "login", "session", result.User.Username, ip,
		auditFieldsDetail("完成 2FA 登录 "+userLabel(result.User), loginAuditFields(result.User, browser, os, true)))
	return &v1.LoginReply{
		Token: result.Token,
		User:  convertUser(result.User),
	}, nil
}

func (s *IncidentService) Setup2FA(ctx context.Context, _ *emptypb.Empty) (*v1.Setup2FAReply, error) {
	auth, ok := biz.AuthFromContext(ctx)
	if !ok {
		return nil, biz.ErrUnauthorized()
	}
	result, err := s.useCase.Setup2FA(ctx, auth.UserID)
	if err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	user, _ := s.useCase.CurrentUser(ctx, auth.UserID)
	label := userLabel(user)
	if label == "" {
		label = auth.Username
	}
	s.useCase.LogAuditEvent(ctx, "setup_2fa", "user", label, ip,
		auditFieldsDetail("创建 2FA 绑定 "+label, twoFactorSetupAuditFields(user, label)))
	return &v1.Setup2FAReply{Secret: result.Secret, QrUrl: result.QRURL}, nil
}

func (s *IncidentService) Enable2FA(ctx context.Context, req *v1.Enable2FARequest) (*emptypb.Empty, error) {
	auth, ok := biz.AuthFromContext(ctx)
	if !ok {
		return nil, biz.ErrUnauthorized()
	}
	before, _ := s.useCase.CurrentUser(ctx, auth.UserID)
	if err := s.useCase.Enable2FA(ctx, auth.UserID, req.GetTotpCode()); err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	after, _ := s.useCase.CurrentUser(ctx, auth.UserID)
	label := userLabel(after)
	if label == "" {
		label = userLabel(before)
	}
	if label == "" {
		label = auth.Username
	}
	s.useCase.LogAuditEvent(ctx, "enable_2fa", "user", label, ip,
		auditDiffDetail("启用 2FA "+label, twoFactorAuditMap(before), twoFactorAuditMap(after)))
	return &emptypb.Empty{}, nil
}

func (s *IncidentService) Disable2FA(ctx context.Context, req *v1.Disable2FARequest) (*emptypb.Empty, error) {
	auth, ok := biz.AuthFromContext(ctx)
	if !ok {
		return nil, biz.ErrUnauthorized()
	}
	before, _ := s.useCase.CurrentUser(ctx, auth.UserID)
	if err := s.useCase.Disable2FA(ctx, auth.UserID, req.GetTotpCode()); err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	after, _ := s.useCase.CurrentUser(ctx, auth.UserID)
	label := userLabel(after)
	if label == "" {
		label = userLabel(before)
	}
	if label == "" {
		label = auth.Username
	}
	s.useCase.LogAuditEvent(ctx, "disable_2fa", "user", label, ip,
		auditDiffDetail("关闭 2FA "+label, twoFactorAuditMap(before), twoFactorAuditMap(after)))
	return &emptypb.Empty{}, nil
}

func (s *IncidentService) AdminResetUser2FA(ctx context.Context, req *v1.AdminResetUser2FARequest) (*emptypb.Empty, error) {
	before, _ := s.useCase.CurrentUser(ctx, req.GetUserId())
	if err := s.useCase.AdminDisableUser2FA(ctx, req.GetUserId()); err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	after, _ := s.useCase.CurrentUser(ctx, req.GetUserId())
	label := userLabel(after)
	if label == "" {
		label = userLabel(before)
	}
	s.useCase.LogAuditEvent(ctx, "admin_reset_2fa", "user", label, ip,
		auditDiffDetail("管理员重置 2FA "+label, twoFactorAuditMap(before), twoFactorAuditMap(after)))
	return &emptypb.Empty{}, nil
}

func (s *IncidentService) Logout(ctx context.Context, req *v1.LogoutRequest) (*emptypb.Empty, error) {
	token := bearerTokenFromContext(ctx)
	_ = s.useCase.LogoutSession(ctx, token)
	ip, _ := requestMeta(ctx)
	detail := req.GetDetail()
	if detail == "" {
		if auth, ok := biz.AuthFromContext(ctx); ok {
			detail = fmt.Sprintf("logout for %q", auth.Username)
		}
	}
	s.useCase.LogAuditEvent(ctx, "logout", "session", "", ip, detail)
	return &emptypb.Empty{}, nil
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
	before, _ := s.useCase.CurrentUser(ctx, req.GetId())
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
	s.useCase.LogAuditEvent(ctx, "update", "user", userLabel(user), ip, auditDiffDetail("更新用户 "+userLabel(user), userAuditMap(before), userAuditMap(user)))
	return convertUser(user), nil
}

func (s *IncidentService) DeleteUser(ctx context.Context, req *v1.DeleteUserRequest) (*emptypb.Empty, error) {
	user, _ := s.useCase.CurrentUser(ctx, req.GetId())
	if err := s.useCase.DeleteUser(ctx, req.GetId()); err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	label := userLabel(user)
	if label == "" {
		label = resourceID(req.GetId())
	}
	s.useCase.LogAuditEvent(ctx, "delete", "user", label, ip, auditFieldsDetail("删除用户 "+label, userAuditMap(user)))
	return &emptypb.Empty{}, nil
}

func (s *IncidentService) AssignUserRoles(ctx context.Context, req *v1.AssignUserRolesRequest) (*v1.User, error) {
	before, _ := s.useCase.CurrentUser(ctx, req.GetUserId())
	user, err := s.useCase.AssignUserRoles(ctx, req.GetUserId(), req.GetRoleIds())
	if err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "update", "user", userLabel(user), ip, auditDiffDetail("更新用户角色 "+userLabel(user), userAuditMap(before), userAuditMap(user)))
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
	before, _ := s.useCase.GetRole(ctx, req.GetId())
	role, err := s.useCase.UpdateRole(ctx, &biz.UpdateRole{ID: req.GetId(), Description: req.GetDescription()})
	if err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "update", "role", roleLabel(role), ip, auditDiffDetail("更新角色 "+roleLabel(role), roleAuditMap(before), roleAuditMap(role)))
	return convertRole(role), nil
}

func (s *IncidentService) DeleteRole(ctx context.Context, req *v1.DeleteRoleRequest) (*emptypb.Empty, error) {
	role, _ := s.useCase.GetRole(ctx, req.GetId())
	if err := s.useCase.DeleteRole(ctx, req.GetId()); err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	label := roleLabel(role)
	if label == "" {
		label = resourceID(req.GetId())
	}
	s.useCase.LogAuditEvent(ctx, "delete", "role", label, ip, auditFieldsDetail("删除角色 "+label, roleAuditMap(role)))
	return &emptypb.Empty{}, nil
}

func (s *IncidentService) AssignRolePermissions(ctx context.Context, req *v1.AssignRolePermissionsRequest) (*v1.Role, error) {
	before, _ := s.useCase.GetRole(ctx, req.GetRoleId())
	role, err := s.useCase.AssignRolePermissions(ctx, req.GetRoleId(), req.GetPermissionIds())
	if err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "update", "role", roleLabel(role), ip, auditDiffDetail("更新角色权限 "+roleLabel(role), roleAuditMap(before), roleAuditMap(role)))
	return convertRole(role), nil
}

func (s *IncidentService) SetRoleInheritances(ctx context.Context, req *v1.SetRoleInheritancesRequest) (*v1.Role, error) {
	before, _ := s.useCase.GetRole(ctx, req.GetRoleId())
	role, err := s.useCase.SetRoleInheritances(ctx, req.GetRoleId(), req.GetInheritedRoleIds())
	if err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "update", "role", roleLabel(role), ip, auditDiffDetail("更新角色继承 "+roleLabel(role), roleAuditMap(before), roleAuditMap(role)))
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
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "create", "permission", permissionLabel(permission), ip, auditFieldsDetail("创建权限 "+permissionLabel(permission), permissionAuditMap(permission)))
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
	before, _ := s.useCase.GetPermission(ctx, req.GetId())
	permission, err := s.useCase.UpdatePermission(ctx, &biz.UpdatePermission{
		ID:          req.GetId(),
		Operation:   req.GetOperation(),
		Description: req.GetDescription(),
	})
	if err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "update", "permission", permissionLabel(permission), ip, auditDiffDetail("更新权限 "+permissionLabel(permission), permissionAuditMap(before), permissionAuditMap(permission)))
	return convertPermission(permission), nil
}

func (s *IncidentService) DeletePermission(ctx context.Context, req *v1.DeletePermissionRequest) (*emptypb.Empty, error) {
	permission, _ := s.useCase.GetPermission(ctx, req.GetId())
	if err := s.useCase.DeletePermission(ctx, req.GetId()); err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	label := permissionLabel(permission)
	if label == "" {
		label = resourceID(req.GetId())
	}
	s.useCase.LogAuditEvent(ctx, "delete", "permission", label, ip, auditFieldsDetail("删除权限 "+label, permissionAuditMap(permission)))
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
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "create", "sso_provider", ssoProviderLabel(p), ip, auditFieldsDetail("创建单点登录配置 "+ssoProviderLabel(p), ssoProviderAuditMap(p)))
	return convertSSOProvider(p), nil
}

func (s *IncidentService) UpdateSSOProvider(ctx context.Context, req *v1.UpdateSSOProviderRequest) (*v1.SSOProvider, error) {
	before, _ := s.useCase.GetSSOProvider(ctx, req.GetId())
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
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "update", "sso_provider", ssoProviderLabel(p), ip, auditDiffDetail("更新单点登录配置 "+ssoProviderLabel(p), ssoProviderAuditMap(before), ssoProviderAuditMap(p)))
	return convertSSOProvider(p), nil
}

func (s *IncidentService) DeleteSSOProvider(ctx context.Context, req *v1.DeleteSSOProviderRequest) (*emptypb.Empty, error) {
	provider, _ := s.useCase.GetSSOProvider(ctx, req.GetId())
	if err := s.useCase.DeleteSSOProvider(ctx, req.GetId()); err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	label := ssoProviderLabel(provider)
	if label == "" {
		label = resourceID(req.GetId())
	}
	s.useCase.LogAuditEvent(ctx, "delete", "sso_provider", label, ip, auditFieldsDetail("删除单点登录配置 "+label, ssoProviderAuditMap(provider)))
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
	session, _ := s.useCase.GetSession(ctx, req.GetId())
	if err := s.useCase.KickSession(ctx, req.GetId()); err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	kickedBy := ""
	if auth, ok := biz.AuthFromContext(ctx); ok {
		kickedBy = auth.Username
	}
	label := sessionLabel(session)
	if label == "" {
		label = resourceID(req.GetId())
	}
	s.useCase.LogAuditEvent(ctx, "kick", "session", label, ip, auditFieldsDetail("踢出会话 "+label, sessionAuditFields(session, kickedBy)))
	return &emptypb.Empty{}, nil
}

// ── Audit logs ────────────────────────────────────────────────────────────────

func (s *IncidentService) ListAuditLogs(ctx context.Context, req *v1.ListAuditLogsRequest) (*v1.ListAuditLogsReply, error) {
	startTime, err := parseAuditLogTime(req.GetStartTime())
	if err != nil {
		return nil, err
	}
	endTime, err := parseAuditLogTime(req.GetEndTime())
	if err != nil {
		return nil, err
	}
	logs, total, err := s.useCase.ListAuditLogs(ctx, biz.AuditLogFilter{
		Action:       req.GetAction(),
		Username:     req.GetUsername(),
		ResourceType: req.GetResourceType(),
		StartTime:    startTime,
		EndTime:      endTime,
	}, biz.Page{Size: int(req.GetPageSize()), Token: int(req.GetPageToken())})
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

func parseAuditLogTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, errors.BadRequest("INVALID_ARGUMENT", "invalid audit log time")
	}
	return t, nil
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
		ServiceName:             settings.ServiceName,
		SiteIcon:                settings.SiteIcon,
		CornerIcon:              settings.CornerIcon,
		TotpEnabled:             settings.TOTPEnabled,
	}, nil
}

func (s *IncidentService) UpdateSystemSettings(ctx context.Context, req *v1.UpdateSystemSettingsRequest) (*v1.SystemSettingsReply, error) {
	before, _ := s.useCase.GetSettings(ctx)
	settings := &biz.SystemSettings{
		AuditLogRetentionDays:   req.GetAuditLogRetentionDays(),
		SessionLogRetentionDays: req.GetSessionLogRetentionDays(),
		ServiceName:             req.GetServiceName(),
		SiteIcon:                req.GetSiteIcon(),
		CornerIcon:              req.GetCornerIcon(),
		TOTPEnabled:             req.GetTotpEnabled(),
	}
	if err := s.useCase.UpdateSettings(ctx, settings); err != nil {
		return nil, err
	}
	ip, _ := requestMeta(ctx)
	s.useCase.LogAuditEvent(ctx, "update", "settings", "system", ip, auditDiffDetail("更新系统设置", settingsAuditMap(before), settingsAuditMap(settings)))
	return &v1.SystemSettingsReply{
		AuditLogRetentionDays:   settings.AuditLogRetentionDays,
		SessionLogRetentionDays: settings.SessionLogRetentionDays,
		ServiceName:             settings.ServiceName,
		SiteIcon:                settings.SiteIcon,
		CornerIcon:              settings.CornerIcon,
		TotpEnabled:             settings.TOTPEnabled,
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

func bearerTokenFromContext(ctx context.Context) string {
	header, ok := transport.FromServerContext(ctx)
	if !ok {
		return ""
	}
	auths := strings.SplitN(header.RequestHeader().Get("Authorization"), " ", 2)
	if len(auths) != 2 || !strings.EqualFold(auths[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(auths[1])
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

type auditDetailPayload struct {
	Summary string         `json:"summary,omitempty"`
	Fields  map[string]any `json:"fields,omitempty"`
	Before  map[string]any `json:"before,omitempty"`
	After   map[string]any `json:"after,omitempty"`
}

func auditFieldsDetail(summary string, fields map[string]any) string {
	return marshalAuditDetail(auditDetailPayload{Summary: summary, Fields: fields})
}

func auditDiffDetail(summary string, before, after map[string]any) string {
	beforeChanges, afterChanges := changedFields(before, after)
	return marshalAuditDetail(auditDetailPayload{
		Summary: summary,
		Before:  beforeChanges,
		After:   afterChanges,
	})
}

func marshalAuditDetail(payload auditDetailPayload) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return payload.Summary
	}
	return string(data)
}

func changedFields(before, after map[string]any) (map[string]any, map[string]any) {
	if before == nil {
		before = map[string]any{}
	}
	if after == nil {
		after = map[string]any{}
	}
	beforeChanges := make(map[string]any)
	afterChanges := make(map[string]any)
	for key, beforeValue := range before {
		afterValue, ok := after[key]
		if !ok || !reflect.DeepEqual(beforeValue, afterValue) {
			beforeChanges[key] = beforeValue
			if ok {
				afterChanges[key] = afterValue
			} else {
				afterChanges[key] = nil
			}
		}
	}
	for key, afterValue := range after {
		if _, ok := before[key]; !ok {
			beforeChanges[key] = nil
			afterChanges[key] = afterValue
		}
	}
	return beforeChanges, afterChanges
}

func userLabel(user *biz.User) string {
	if user == nil {
		return ""
	}
	if user.DisplayName != "" && user.DisplayName != user.Username {
		return fmt.Sprintf("%s（%s）", user.DisplayName, user.Username)
	}
	if user.Username != "" {
		return user.Username
	}
	return resourceID(user.ID)
}

func roleLabel(role *biz.Role) string {
	if role == nil {
		return ""
	}
	if role.Name != "" {
		return role.Name
	}
	return resourceID(role.ID)
}

func permissionLabel(permission *biz.Permission) string {
	if permission == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	for _, part := range []string{permission.Module, permission.Operation, permission.Action} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, " / ")
	}
	return resourceID(permission.ID)
}

func ssoProviderLabel(provider *biz.SSOProvider) string {
	if provider == nil {
		return ""
	}
	if provider.Name != "" {
		return provider.Name
	}
	return resourceID(provider.ID)
}

func serviceAccountLabel(svc *biz.ServiceAccount) string {
	if svc == nil {
		return ""
	}
	if svc.Name != "" {
		return svc.Name
	}
	return resourceID(svc.ID)
}

func sessionLabel(session *biz.UserSession) string {
	if session == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	for _, part := range []string{session.Username, session.IP, session.Browser, session.OS} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	label := strings.Join(parts, " / ")
	if label == "" {
		return resourceID(session.ID)
	}
	return fmt.Sprintf("%s (#%d)", label, session.ID)
}

func userAuditMap(user *biz.User) map[string]any {
	if user == nil {
		return map[string]any{}
	}
	return map[string]any{
		"username":     user.Username,
		"display_name": user.DisplayName,
		"disabled":     user.Disabled,
		"roles":        roleNames(user.Roles),
	}
}

func loginAuditFields(user *biz.User, browser, os string, verified2FA bool) map[string]any {
	fields := twoFactorAuditMap(user)
	fields["browser"] = browser
	fields["os"] = os
	fields["totp_verified"] = verified2FA
	return fields
}

func twoFactorAuditMap(user *biz.User) map[string]any {
	if user == nil {
		return map[string]any{}
	}
	return map[string]any{
		"user_id":      user.ID,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"totp_enabled": user.TOTPEnabled,
	}
}

func twoFactorSetupAuditFields(user *biz.User, label string) map[string]any {
	fields := twoFactorAuditMap(user)
	if len(fields) == 0 && label != "" {
		fields["username"] = label
	}
	fields["setup_pending"] = true
	return fields
}

func roleAuditMap(role *biz.Role) map[string]any {
	if role == nil {
		return map[string]any{}
	}
	return map[string]any{
		"name":            role.Name,
		"description":     role.Description,
		"permissions":     permissionLabels(role.Permissions),
		"inherited_roles": roleNames(role.InheritedRoles),
	}
}

func permissionAuditMap(permission *biz.Permission) map[string]any {
	if permission == nil {
		return map[string]any{}
	}
	return map[string]any{
		"module":      permission.Module,
		"action":      permission.Action,
		"operation":   permission.Operation,
		"description": permission.Description,
	}
}

func ssoProviderAuditMap(provider *biz.SSOProvider) map[string]any {
	if provider == nil {
		return map[string]any{}
	}
	return map[string]any{
		"name":       provider.Name,
		"type":       provider.Type,
		"enabled":    provider.Enabled,
		"icon":       provider.Icon,
		"sort_order": provider.SortOrder,
		"config":     provider.Config,
	}
}

func serviceAccountAuditMap(svc *biz.ServiceAccount) map[string]any {
	if svc == nil {
		return map[string]any{}
	}
	return map[string]any{
		"name":         svc.Name,
		"description":  svc.Description,
		"disabled":     svc.Disabled,
		"roles":        roleNames(svc.Roles),
		"token_prefix": svc.TokenPrefix,
		"expires_at":   formatAuditOptionalTime(svc.ExpiresAt),
	}
}

func serviceAccountTokenAuditFields(svc *biz.ServiceAccount, expiresInDays int32) map[string]any {
	fields := serviceAccountAuditMap(svc)
	fields["token_regenerated"] = true
	fields["expires_in_days"] = expiresInDays
	return fields
}

func settingsAuditMap(settings *biz.SystemSettings) map[string]any {
	if settings == nil {
		return map[string]any{}
	}
	return map[string]any{
		"audit_log_retention_days":   settings.AuditLogRetentionDays,
		"session_log_retention_days": settings.SessionLogRetentionDays,
		"service_name":               settings.ServiceName,
		"site_icon":                  settings.SiteIcon,
		"corner_icon":                settings.CornerIcon,
		"totp_enabled":               settings.TOTPEnabled,
	}
}

func sessionAuditFields(session *biz.UserSession, kickedBy string) map[string]any {
	if session == nil {
		return map[string]any{"kicked_by": kickedBy}
	}
	return map[string]any{
		"session_id":     session.ID,
		"username":       session.Username,
		"ip":             session.IP,
		"browser":        session.Browser,
		"os":             session.OS,
		"status":         session.Status,
		"kicked_by":      kickedBy,
		"login_at":       formatAuditTime(session.LoginAt),
		"last_access_at": formatAuditTime(session.LastAccessAt),
	}
}

func roleNames(roles []biz.Role) []string {
	names := make([]string, 0, len(roles))
	for i := range roles {
		if label := roleLabel(&roles[i]); label != "" {
			names = append(names, label)
		}
	}
	return names
}

func permissionLabels(permissions []biz.Permission) []string {
	labels := make([]string, 0, len(permissions))
	for i := range permissions {
		if label := permissionLabel(&permissions[i]); label != "" {
			labels = append(labels, label)
		}
	}
	return labels
}

func formatAuditTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func formatAuditOptionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatAuditTime(*value)
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
		TotpEnabled: user.TOTPEnabled,
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

// ── Service account handlers ──────────────────────────────────────────────────

func (s *IncidentService) CreateServiceAccount(ctx context.Context, req *v1.CreateServiceAccountRequest) (*v1.ServiceAccountTokenReply, error) {
	ip, _ := requestMeta(ctx)
	result, err := s.useCase.CreateServiceAccount(ctx, &biz.CreateServiceAccount{
		Name:          req.GetName(),
		Description:   req.GetDescription(),
		ExpiresInDays: req.GetExpiresInDays(),
		RoleIDs:       req.GetRoleIds(),
	})
	if err != nil {
		return nil, err
	}
	label := serviceAccountLabel(result.ServiceAccount)
	s.useCase.LogAuditEvent(ctx, "create", "service_account", label, ip,
		auditFieldsDetail("创建服务账号 "+label, serviceAccountAuditMap(result.ServiceAccount)))
	return &v1.ServiceAccountTokenReply{
		ServiceAccount: convertServiceAccount(result.ServiceAccount),
		Token:          result.Token,
	}, nil
}

func (s *IncidentService) ListServiceAccounts(ctx context.Context, req *v1.ListServiceAccountsRequest) (*v1.ListServiceAccountsReply, error) {
	svcs, total, err := s.useCase.ListServiceAccounts(ctx, biz.Page{
		Size:  int(req.GetPageSize()),
		Token: int(req.GetPageToken()),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*v1.ServiceAccount, 0, len(svcs))
	for i := range svcs {
		result = append(result, convertServiceAccount(&svcs[i]))
	}
	return &v1.ListServiceAccountsReply{ServiceAccounts: result, Total: int32(total)}, nil
}

func (s *IncidentService) GetServiceAccount(ctx context.Context, req *v1.GetServiceAccountRequest) (*v1.ServiceAccount, error) {
	svc, err := s.useCase.GetServiceAccount(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return convertServiceAccount(svc), nil
}

func (s *IncidentService) UpdateServiceAccount(ctx context.Context, req *v1.UpdateServiceAccountRequest) (*v1.ServiceAccount, error) {
	ip, _ := requestMeta(ctx)
	before, _ := s.useCase.GetServiceAccount(ctx, req.GetId())
	svc, err := s.useCase.UpdateServiceAccount(ctx, &biz.UpdateServiceAccount{
		ID:          req.GetId(),
		Description: req.GetDescription(),
		Disabled:    req.GetDisabled(),
		RoleIDs:     req.GetRoleIds(),
	})
	if err != nil {
		return nil, err
	}
	label := serviceAccountLabel(svc)
	s.useCase.LogAuditEvent(ctx, "update", "service_account", label, ip,
		auditDiffDetail("更新服务账号 "+label, serviceAccountAuditMap(before), serviceAccountAuditMap(svc)))
	return convertServiceAccount(svc), nil
}

func (s *IncidentService) DeleteServiceAccount(ctx context.Context, req *v1.DeleteServiceAccountRequest) (*emptypb.Empty, error) {
	ip, _ := requestMeta(ctx)
	svc, err := s.useCase.GetServiceAccount(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if err := s.useCase.DeleteServiceAccount(ctx, req.GetId()); err != nil {
		return nil, err
	}
	label := serviceAccountLabel(svc)
	s.useCase.LogAuditEvent(ctx, "delete", "service_account", label, ip,
		auditFieldsDetail("删除服务账号 "+label, serviceAccountAuditMap(svc)))
	return &emptypb.Empty{}, nil
}

func (s *IncidentService) RegenerateServiceAccountToken(ctx context.Context, req *v1.RegenerateServiceAccountTokenRequest) (*v1.ServiceAccountTokenReply, error) {
	ip, _ := requestMeta(ctx)
	result, err := s.useCase.RegenerateServiceAccountToken(ctx, req.GetId(), req.GetExpiresInDays())
	if err != nil {
		return nil, err
	}
	label := serviceAccountLabel(result.ServiceAccount)
	s.useCase.LogAuditEvent(ctx, "regenerate_token", "service_account", label, ip,
		auditFieldsDetail("重置服务账号令牌 "+label, serviceAccountTokenAuditFields(result.ServiceAccount, req.GetExpiresInDays())))
	return &v1.ServiceAccountTokenReply{
		ServiceAccount: convertServiceAccount(result.ServiceAccount),
		Token:          result.Token,
	}, nil
}

func convertServiceAccount(svc *biz.ServiceAccount) *v1.ServiceAccount {
	if svc == nil {
		return nil
	}
	pb := &v1.ServiceAccount{
		Id:          svc.ID,
		Name:        svc.Name,
		Description: svc.Description,
		TokenPrefix: svc.TokenPrefix,
		Disabled:    svc.Disabled,
		Roles:       convertRoles(svc.Roles),
		CreatedAt:   timestamppb.New(svc.CreatedAt),
		UpdatedAt:   timestamppb.New(svc.UpdatedAt),
	}
	if svc.ExpiresAt != nil {
		pb.ExpiresAt = timestamppb.New(*svc.ExpiresAt)
	}
	return pb
}
