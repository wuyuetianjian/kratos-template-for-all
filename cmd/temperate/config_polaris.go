//go:build polaris

package main

import (
	polarisconfigsource "github.com/go-kratos/kratos/contrib/config/polaris/v3"
	kconfig "github.com/go-kratos/kratos/v3/config"
	polaris "github.com/polarismesh/polaris-go"
	polarisconfig "github.com/polarismesh/polaris-go/pkg/config"

	"github.com/wuyuetianjian/kratos-template-for-all/internal/conf"
)

func newPolarisConfigSource(c *conf.Config_Remote) (kconfig.Source, error) {
	client, err := polaris.NewConfigAPIByConfig(polarisconfig.NewDefaultConfiguration(c.GetEndpoints()))
	if err != nil {
		return nil, err
	}
	fileName := c.GetFileName()
	if fileName == "" {
		fileName = c.GetPath()
	}
	return polarisconfigsource.New(
		client,
		polarisconfigsource.WithNamespace(c.GetNamespace()),
		polarisconfigsource.WithFileGroup(c.GetFileGroup()),
		polarisconfigsource.WithFileName(fileName),
	)
}
