package biz

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"slices"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	adminUsername = "admin"
	adminRoleName = "Admin"

	PermissionActionAll   = "*"
	PermissionActionRead  = "read"
	PermissionActionWrite = "write"
	PermissionActionGrant = "grant"

	reasonUnauthorized     = "UNAUTHORIZED"
	reasonForbidden        = "FORBIDDEN"
	reasonInvalidArgument  = "INVALID_ARGUMENT"
	reasonNotFound         = "NOT_FOUND"
	reasonAlreadyExists    = "ALREADY_EXISTS"
	reasonSystemProtected  = "SYSTEM_PROTECTED"
	reasonInvalidHierarchy = "INVALID_ROLE_INHERITANCE"
)

type AuthRepo interface {
	BootstrapAdmin(context.Context, string) (*User, error)
	AdminInitialPasswordUsed(context.Context) (bool, error)
	FindUserByUsername(context.Context, string) (*User, error)
	FindUserByID(context.Context, int64) (*User, error)
	MarkInitialPasswordUsed(context.Context, int64) error
	ChangePassword(context.Context, int64, string) error
	EffectivePermissions(context.Context, int64) ([]Permission, []Role, error)

	CreateUser(context.Context, *CreateUser) (*User, error)
	ListUsers(context.Context, Page) ([]User, int, error)
	UpdateUser(context.Context, *UpdateUser) (*User, error)
	DeleteUser(context.Context, int64, int64) error
	AssignUserRoles(context.Context, int64, []int64) (*User, error)

	CreateRole(context.Context, *CreateRole) (*Role, error)
	ListRoles(context.Context, Page) ([]Role, int, error)
	GetRole(context.Context, int64) (*Role, error)
	UpdateRole(context.Context, *UpdateRole) (*Role, error)
	DeleteRole(context.Context, int64) error
	AssignRolePermissions(context.Context, int64, []int64) (*Role, error)
	SetRoleInheritances(context.Context, int64, []int64) (*Role, error)

	CreatePermission(context.Context, *CreatePermission) (*Permission, error)
	ListPermissions(context.Context, Page) ([]Permission, int, error)
	UpdatePermission(context.Context, *UpdatePermission) (*Permission, error)
	DeletePermission(context.Context, int64) error
}

type Page struct {
	Size  int
	Token int
}

type User struct {
	ID                  int64
	Username            string
	PasswordHash        string
	DisplayName         string
	Disabled            bool
	System              bool
	InitialPasswordUsed bool
	Roles               []Role
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type Role struct {
	ID             int64
	Name           string
	Description    string
	System         bool
	Permissions    []Permission
	InheritedRoles []Role
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Permission struct {
	ID          int64
	Module      string
	Action      string
	Operation   string
	Description string
	System      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PermissionAction struct {
	Action      string
	Name        string
	Description string
}

type CreateUser struct {
	Username    string
	Password    string
	DisplayName string
	Disabled    bool
	RoleIDs     []int64
}

type UpdateUser struct {
	ID          int64
	DisplayName string
	Disabled    bool
	RoleIDs     []int64
}

type CreateRole struct {
	Name             string
	Description      string
	PermissionIDs    []int64
	InheritedRoleIDs []int64
}

type UpdateRole struct {
	ID          int64
	Description string
}

type CreatePermission struct {
	Module      string
	Action      string
	Operation   string
	Description string
}

type UpdatePermission struct {
	ID          int64
	Operation   string
	Description string
}

type LoginResult struct {
	Token              string
	User               *User
	MustChangePassword bool
	InitialPassword    string
}

type InitialPasswordResult struct {
	Available       bool
	Username        string
	InitialPassword string
}

type AuthContext struct {
	UserID      int64
	Username    string
	Permissions []Permission
	Roles       []Role
}

type authContextKey struct{}

func WithAuthContext(ctx context.Context, auth *AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey{}, auth)
}

func AuthFromContext(ctx context.Context) (*AuthContext, bool) {
	auth, ok := ctx.Value(authContextKey{}).(*AuthContext)
	return auth, ok
}

func ErrUnauthorized() error {
	return errors.Unauthorized(reasonUnauthorized, "unauthorized")
}

func ErrNotFound(message string) error {
	return errors.NotFound(reasonNotFound, message)
}

func (uc *UseCase) PermissionActions(context.Context) []PermissionAction {
	return []PermissionAction{
		{
			Action:      PermissionActionRead,
			Name:        "可看模块",
			Description: "允许查看指定模块的数据和页面",
		},
		{
			Action:      PermissionActionWrite,
			Name:        "可编辑模块",
			Description: "允许新增、修改、删除指定模块的数据",
		},
		{
			Action:      PermissionActionGrant,
			Name:        "可分配此模块读写",
			Description: "允许将指定模块的读写权限分配给其他角色",
		},
	}
}

func (uc *UseCase) BootstrapAdmin(ctx context.Context) error {
	password, err := randomPassword()
	if err != nil {
		return err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	user, err := uc.authRepo.BootstrapAdmin(ctx, hash)
	if err != nil {
		return err
	}
	if !user.InitialPasswordUsed {
		uc.initialAdminPassword = password
		uc.log.Warn("admin initialization password generated", "username", adminUsername, "password", password)
	}
	return nil
}

func (uc *UseCase) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	user, err := uc.authRepo.FindUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user.Disabled {
		return nil, errors.Forbidden(reasonForbidden, "user is disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.Unauthorized(reasonUnauthorized, "invalid username or password")
	}
	_, roles, err := uc.authRepo.EffectivePermissions(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	token, err := uc.signToken(user)
	if err != nil {
		return nil, err
	}
	result := &LoginResult{Token: token, User: user}
	if user.Username == adminUsername && !user.InitialPasswordUsed && uc.initialAdminPassword != "" {
		result.MustChangePassword = true
		result.InitialPassword = uc.initialAdminPassword
		if err := uc.authRepo.MarkInitialPasswordUsed(ctx, user.ID); err != nil {
			return nil, err
		}
		uc.initialAdminPassword = ""
	}
	result.User.Roles = roles
	return result, nil
}

func (uc *UseCase) InitialPassword(ctx context.Context) (*InitialPasswordResult, error) {
	used, err := uc.authRepo.AdminInitialPasswordUsed(ctx)
	if err != nil {
		return nil, err
	}
	if used || uc.initialAdminPassword == "" {
		return &InitialPasswordResult{Available: false, Username: adminUsername}, nil
	}
	return &InitialPasswordResult{
		Available:       true,
		Username:        adminUsername,
		InitialPassword: uc.initialAdminPassword,
	}, nil
}

func (uc *UseCase) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	user, err := uc.authRepo.FindUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return errors.Unauthorized(reasonUnauthorized, "invalid old password")
	}
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	return uc.authRepo.ChangePassword(ctx, userID, hash)
}

func (uc *UseCase) Authorize(ctx context.Context, userID int64, operation string) (*AuthContext, error) {
	user, err := uc.authRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Disabled {
		return nil, errors.Forbidden(reasonForbidden, "user is disabled")
	}
	permissions, roles, err := uc.authRepo.EffectivePermissions(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !hasPermission(roles, permissions, operation) {
		return nil, errors.Forbidden(reasonForbidden, "permission denied")
	}
	return &AuthContext{UserID: user.ID, Username: user.Username, Permissions: permissions, Roles: roles}, nil
}

func (uc *UseCase) CurrentUser(ctx context.Context, userID int64) (*User, error) {
	return uc.authRepo.FindUserByID(ctx, userID)
}

func (uc *UseCase) CreateUser(ctx context.Context, in *CreateUser) (*User, error) {
	hash, err := hashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	in.Password = hash
	return uc.authRepo.CreateUser(ctx, in)
}

func (uc *UseCase) ListUsers(ctx context.Context, page Page) ([]User, int, error) {
	return uc.authRepo.ListUsers(ctx, page.normalize())
}

func (uc *UseCase) UpdateUser(ctx context.Context, in *UpdateUser) (*User, error) {
	return uc.authRepo.UpdateUser(ctx, in)
}

func (uc *UseCase) DeleteUser(ctx context.Context, targetUserID int64) error {
	auth, ok := AuthFromContext(ctx)
	if !ok {
		return errors.Unauthorized(reasonUnauthorized, "auth context is missing")
	}
	return uc.authRepo.DeleteUser(ctx, targetUserID, auth.UserID)
}

func (uc *UseCase) AssignUserRoles(ctx context.Context, userID int64, roleIDs []int64) (*User, error) {
	return uc.authRepo.AssignUserRoles(ctx, userID, roleIDs)
}

func (uc *UseCase) CreateRole(ctx context.Context, in *CreateRole) (*Role, error) {
	return uc.authRepo.CreateRole(ctx, in)
}

func (uc *UseCase) ListRoles(ctx context.Context, page Page) ([]Role, int, error) {
	return uc.authRepo.ListRoles(ctx, page.normalize())
}

func (uc *UseCase) GetRole(ctx context.Context, roleID int64) (*Role, error) {
	return uc.authRepo.GetRole(ctx, roleID)
}

func (uc *UseCase) UpdateRole(ctx context.Context, in *UpdateRole) (*Role, error) {
	return uc.authRepo.UpdateRole(ctx, in)
}

func (uc *UseCase) DeleteRole(ctx context.Context, roleID int64) error {
	return uc.authRepo.DeleteRole(ctx, roleID)
}

func (uc *UseCase) AssignRolePermissions(ctx context.Context, roleID int64, permissionIDs []int64) (*Role, error) {
	return uc.authRepo.AssignRolePermissions(ctx, roleID, permissionIDs)
}

func (uc *UseCase) SetRoleInheritances(ctx context.Context, roleID int64, inheritedRoleIDs []int64) (*Role, error) {
	return uc.authRepo.SetRoleInheritances(ctx, roleID, inheritedRoleIDs)
}

func (uc *UseCase) CreatePermission(ctx context.Context, in *CreatePermission) (*Permission, error) {
	return uc.authRepo.CreatePermission(ctx, in)
}

func (uc *UseCase) ListPermissions(ctx context.Context, page Page) ([]Permission, int, error) {
	return uc.authRepo.ListPermissions(ctx, page.normalize())
}

func (uc *UseCase) UpdatePermission(ctx context.Context, in *UpdatePermission) (*Permission, error) {
	return uc.authRepo.UpdatePermission(ctx, in)
}

func (uc *UseCase) DeletePermission(ctx context.Context, permissionID int64) error {
	return uc.authRepo.DeletePermission(ctx, permissionID)
}

func (p Page) normalize() Page {
	if p.Size <= 0 || p.Size > 100 {
		p.Size = 50
	}
	if p.Token < 0 {
		p.Token = 0
	}
	return p
}

func hashPassword(password string) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", errors.BadRequest(reasonInvalidArgument, "password is required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func randomPassword() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hasPermission(roles []Role, permissions []Permission, operation string) bool {
	if slices.ContainsFunc(roles, func(role Role) bool { return role.Name == adminRoleName }) {
		return true
	}
	for _, permission := range permissions {
		if permission.Module == "system" && permission.Action == "*" {
			return true
		}
		if permission.Operation != "" && permission.Operation == operation {
			return true
		}
		if permission.Operation != "" && strings.HasSuffix(permission.Operation, "*") &&
			strings.HasPrefix(operation, strings.TrimSuffix(permission.Operation, "*")) {
			return true
		}
	}
	return false
}

func (uc *UseCase) signToken(user *User) (string, error) {
	method, ok := jwtSigningMethod(uc.confData.GetApi().GetSigningMethod())
	if !ok {
		return "", errors.Unauthorized(reasonUnauthorized, "unsupported jwt signing method")
	}
	key := uc.confData.GetApi().GetJwtKey()
	if key == "" {
		return "", errors.Unauthorized(reasonUnauthorized, "jwt key is missing")
	}
	token := jwt.NewWithClaims(method, jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"iat":      time.Now().Unix(),
	})
	return token.SignedString([]byte(key))
}

func jwtSigningMethod(method string) (jwt.SigningMethod, bool) {
	switch strings.ToUpper(method) {
	case "", "HS512":
		return jwt.SigningMethodHS512, true
	case "HS256":
		return jwt.SigningMethodHS256, true
	case "HS384":
		return jwt.SigningMethodHS384, true
	default:
		return nil, false
	}
}
