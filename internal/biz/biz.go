package biz

import (
	"context"
	"log/slog"
	"temperate/internal/conf"

	"github.com/go-kratos/kratos/v3/log"

	"github.com/google/wire"
	"github.com/robfig/cron/v3"
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(NewUseCase)

type UseCase struct {
	log                  *slog.Logger
	cron                 *cron.Cron
	confServer           *conf.Server
	confData             *conf.Data
	authRepo             AuthRepo
	ssoRepo              SSOProviderRepo
	sessionRepo          SessionRepo
	auditRepo            AuditLogRepo
	settingsRepo         SettingsRepo
	initialAdminPassword string
}

// NewUseCase new a UseCase and return.
func NewUseCase(
	logger *slog.Logger,
	cron *cron.Cron,
	conf *conf.Server,
	confData *conf.Data,
	authRepo AuthRepo,
	ssoRepo SSOProviderRepo,
	sessionRepo SessionRepo,
	auditRepo AuditLogRepo,
	settingsRepo SettingsRepo,
) (*UseCase, error) {
	if logger == nil {
		logger = log.Default()
	}
	uc := &UseCase{
		log:          logger.With("module", "biz/biz"),
		cron:         cron,
		confServer:   conf,
		confData:     confData,
		authRepo:     authRepo,
		ssoRepo:      ssoRepo,
		sessionRepo:  sessionRepo,
		auditRepo:    auditRepo,
		settingsRepo: settingsRepo,
	}
	if err := uc.BootstrapAdmin(context.Background()); err != nil {
		return nil, err
	}
	uc.scheduleCleanup()
	return uc, nil
}
