//go:build !polaris

package registry

import (
	"fmt"

	"github.com/go-kratos/kratos/v3/registry"

	"github.com/wuyuetianjian/kratos-template-for-all/internal/conf"
)

func newPolaris(*conf.Registry) (registry.Registrar, func(), error) {
	return nil, nil, fmt.Errorf("polaris registry driver requires building with -tags polaris")
}
