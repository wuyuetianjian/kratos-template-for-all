package biz

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/pquerna/otp/totp"
)

const reasonTOTPInvalid = "TOTP_INVALID"

type Setup2FAResult struct {
	Secret string
	QRURL  string
}

func (uc *UseCase) Setup2FA(ctx context.Context, userID int64) (*Setup2FAResult, error) {
	user, err := uc.authRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Temperate",
		AccountName: user.Username,
	})
	if err != nil {
		return nil, fmt.Errorf("generate totp key: %w", err)
	}
	if err := uc.authRepo.SetTOTPSecret(ctx, userID, key.Secret()); err != nil {
		return nil, err
	}
	return &Setup2FAResult{Secret: key.Secret(), QRURL: key.URL()}, nil
}

func (uc *UseCase) Enable2FA(ctx context.Context, userID int64, totpCode string) error {
	user, err := uc.authRepo.FindUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.TOTPSecret == "" {
		return errors.BadRequest(reasonInvalidArgument, "2FA not set up, call Setup2FA first")
	}
	if !totp.Validate(totpCode, user.TOTPSecret) {
		return errors.Unauthorized(reasonTOTPInvalid, "invalid TOTP code")
	}
	return uc.authRepo.EnableTOTP(ctx, userID)
}

func (uc *UseCase) Disable2FA(ctx context.Context, userID int64, totpCode string) error {
	user, err := uc.authRepo.FindUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if !user.TOTPEnabled {
		return errors.BadRequest(reasonInvalidArgument, "2FA is not enabled")
	}
	if !totp.Validate(totpCode, user.TOTPSecret) {
		return errors.Unauthorized(reasonTOTPInvalid, "invalid TOTP code")
	}
	return uc.authRepo.DisableTOTP(ctx, userID)
}

func (uc *UseCase) AdminDisableUser2FA(ctx context.Context, targetUserID int64) error {
	user, err := uc.authRepo.FindUserByID(ctx, targetUserID)
	if err != nil {
		return err
	}
	if !user.TOTPEnabled {
		return errors.BadRequest(reasonInvalidArgument, "2FA is not enabled for this user")
	}
	return uc.authRepo.DisableTOTP(ctx, targetUserID)
}

func (uc *UseCase) VerifyTOTPLogin(ctx context.Context, preAuthToken, totpCode string) (*LoginResult, error) {
	userID, err := uc.parsePreAuthToken(preAuthToken)
	if err != nil {
		return nil, err
	}
	user, err := uc.authRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Disabled {
		return nil, errors.Forbidden(reasonForbidden, "user is disabled")
	}
	if !user.TOTPEnabled || user.TOTPSecret == "" {
		return nil, errors.Unauthorized(reasonTOTPInvalid, "2FA not configured")
	}
	if !totp.Validate(totpCode, user.TOTPSecret) {
		return nil, errors.Unauthorized(reasonTOTPInvalid, "invalid TOTP code")
	}
	_, roles, err := uc.authRepo.EffectivePermissions(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	token, err := uc.signToken(user)
	if err != nil {
		return nil, err
	}
	user.Roles = roles
	return &LoginResult{Token: token, User: user}, nil
}
