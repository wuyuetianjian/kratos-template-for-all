//go:build !polaris

package main

import (
	"fmt"

	kconfig "github.com/go-kratos/kratos/v3/config"

	"temperate/internal/conf"
)

func newPolarisConfigSource(*conf.Config_Remote) (kconfig.Source, error) {
	return nil, fmt.Errorf("polaris config driver requires building with -tags polaris")
}
