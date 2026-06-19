package biz

import (
	"log/slog"
	"temperate/internal/conf"

	"github.com/go-kratos/kratos/v3/log"

	"github.com/google/wire"
	"github.com/robfig/cron/v3"
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(NewUseCase)

type UseCase struct {
	log        *slog.Logger
	cron       *cron.Cron
	confServer *conf.Server
	confData   *conf.Data
}

// NewUseCase new a UseCase and return.
func NewUseCase(
	logger *slog.Logger,
	cron *cron.Cron,
	conf *conf.Server,
	confData *conf.Data,
) *UseCase {
	if logger == nil {
		logger = log.Default()
	}
	return &UseCase{
		log:        logger.With("module", "biz/biz"),
		cron:       cron,
		confServer: conf,
		confData:   confData,
	}
}
