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
