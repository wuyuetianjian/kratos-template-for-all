package server

import (
	"context"
	"strings"
	v1 "temperate/api/temperate/v1"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/selector"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/golang-jwt/jwt/v5"
)

const (
	authorizationKey = "Authorization"
	bearerWord       = "Bearer"
	authReason       = "UNAUTHORIZED"
)

var (
	errMissingJWTToken        = errors.Unauthorized(authReason, "JWT token is missing")
	errTokenInvalid           = errors.Unauthorized(authReason, "Token is invalid")
	errTokenExpired           = errors.Unauthorized(authReason, "JWT token has expired")
	errTokenParseFail         = errors.Unauthorized(authReason, "Fail to parse JWT token")
	errUnsupportedSigning     = errors.Unauthorized(authReason, "Wrong signing method")
	errWrongContext           = errors.Unauthorized(authReason, "Wrong context for middleware")
	errUnsupportedSigningConf = errors.Unauthorized(authReason, "Unsupported JWT signing method")
	errMissingJWTKey          = errors.Unauthorized(authReason, "JWT key is missing")
)

var authAllowlist = newAuthAllowlist(
	[]string{
		v1.OperationTemperateServiceHealth,
	},
	nil,
)

type authOperationAllowlist struct {
	operations map[string]struct{}
	prefixes   []string
}

func newAuthAllowlist(operations []string, prefixes []string) authOperationAllowlist {
	allowlist := authOperationAllowlist{
		operations: make(map[string]struct{}, len(operations)),
		prefixes:   prefixes,
	}
	for _, operation := range operations {
		allowlist.operations[operation] = struct{}{}
	}
	return allowlist
}

func (a authOperationAllowlist) Contains(operation string) bool {
	if _, ok := a.operations[operation]; ok {
		return true
	}
	for _, prefix := range a.prefixes {
		if strings.HasPrefix(operation, prefix) {
			return true
		}
	}
	return false
}

func selectedAuthMiddleware(signingMethod, key string) middleware.Middleware {
	return selector.Server(authMiddleware(signingMethod, key)).
		Match(func(_ context.Context, operation string) bool {
			return !authAllowlist.Contains(operation)
		}).
		Build()
}

func authMiddleware(signingMethod, key string) middleware.Middleware {
	method, ok := jwtSigningMethod(signingMethod)
	if !ok {
		return func(middleware.Handler) middleware.Handler {
			return func(context.Context, any) (any, error) {
				return nil, errUnsupportedSigningConf
			}
		}
	}
	if key == "" {
		return func(middleware.Handler) middleware.Handler {
			return func(context.Context, any) (any, error) {
				return nil, errMissingJWTKey
			}
		}
	}
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			header, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, errWrongContext
			}

			auths := strings.SplitN(header.RequestHeader().Get(authorizationKey), " ", 2)
			if len(auths) != 2 || !strings.EqualFold(auths[0], bearerWord) {
				return nil, errMissingJWTToken
			}

			token, err := jwt.Parse(auths[1], func(token *jwt.Token) (any, error) {
				return []byte(key), nil
			})
			if err != nil {
				if errors.Is(err, jwt.ErrTokenMalformed) || errors.Is(err, jwt.ErrTokenUnverifiable) {
					return nil, errTokenInvalid
				}
				if errors.Is(err, jwt.ErrTokenNotValidYet) || errors.Is(err, jwt.ErrTokenExpired) {
					return nil, errTokenExpired
				}
				return nil, errTokenParseFail
			}
			if !token.Valid {
				return nil, errTokenInvalid
			}
			if token.Method != method {
				return nil, errUnsupportedSigning
			}

			return handler(ctx, req)
		}
	}
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
