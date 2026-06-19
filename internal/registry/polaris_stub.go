//go:build !polaris

package registry

import (
	"fmt"

	"github.com/go-kratos/kratos/v3/registry"

	"temperate/internal/conf"
)

func newPolaris(*conf.Registry) (registry.Registrar, func(), error) {
	return nil, nil, fmt.Errorf("polaris registry driver requires building with -tags polaris")
}
