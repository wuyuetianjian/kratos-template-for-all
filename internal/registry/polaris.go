//go:build polaris

package registry

import (
	polarisreg "github.com/go-kratos/kratos/contrib/registry/polaris/v3"
	"github.com/go-kratos/kratos/v3/registry"
	polarisapi "github.com/polarismesh/polaris-go/api"
	polarisconfig "github.com/polarismesh/polaris-go/pkg/config"

	"temperate/internal/conf"
)

func newPolaris(c *conf.Registry) (registry.Registrar, func(), error) {
	cfg := polarisconfig.NewDefaultConfiguration(c.GetEndpoints())
	provider, err := polarisapi.NewProviderAPIByConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	consumer, err := polarisapi.NewConsumerAPIByConfig(cfg)
	if err != nil {
		provider.Destroy()
		return nil, nil, err
	}
	opts := []polarisreg.Option{}
	if c.GetNamespace() != "" {
		opts = append(opts, polarisreg.WithNamespace(c.GetNamespace()))
	}
	if c.GetServiceToken() != "" {
		opts = append(opts, polarisreg.WithServiceToken(c.GetServiceToken()))
	}
	if c.GetProtocol() != "" {
		opts = append(opts, polarisreg.WithProtocol(c.GetProtocol()))
	}
	if c.GetWeight() > 0 {
		opts = append(opts, polarisreg.WithWeight(int(c.GetWeight())))
	}
	if c.GetTtlSeconds() > 0 {
		opts = append(opts, polarisreg.WithTTL(int(c.GetTtlSeconds())))
	}
	if c.GetTimeoutSeconds() > 0 {
		opts = append(opts, polarisreg.WithTimeout(timeout(c)))
	}
	if c.GetRetryCount() > 0 {
		opts = append(opts, polarisreg.WithRetryCount(int(c.GetRetryCount())))
	}
	if c.GetHeartbeat() {
		opts = append(opts, polarisreg.WithHeartbeat(true))
	}
	return polarisreg.NewRegistry(provider, consumer, opts...), func() {
		provider.Destroy()
		consumer.Destroy()
	}, nil
}
