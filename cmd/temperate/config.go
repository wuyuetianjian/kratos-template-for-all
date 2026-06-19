package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	apolloconfig "github.com/go-kratos/kratos/contrib/config/apollo/v3"
	consulconfig "github.com/go-kratos/kratos/contrib/config/consul/v3"
	kubeconfig "github.com/go-kratos/kratos/contrib/config/kubernetes/v3"
	nacosconfig "github.com/go-kratos/kratos/contrib/config/nacos/v3"
	polarisconfigsource "github.com/go-kratos/kratos/contrib/config/polaris/v3"
	kconfig "github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/env"
	"github.com/go-kratos/kratos/v3/config/file"
	consulapi "github.com/hashicorp/consul/api"
	nacosclients "github.com/nacos-group/nacos-sdk-go/clients"
	nacosconfigclient "github.com/nacos-group/nacos-sdk-go/clients/config_client"
	nacosconstant "github.com/nacos-group/nacos-sdk-go/common/constant"
	nacosvo "github.com/nacos-group/nacos-sdk-go/vo"
	polaris "github.com/polarismesh/polaris-go"
	polarisconfig "github.com/polarismesh/polaris-go/pkg/config"
	clientv3 "go.etcd.io/etcd/client/v3"

	"temperate/internal/conf"
	"temperate/internal/configsource"
)

func loadBootstrap(path string, logger *slog.Logger) (*conf.Bootstrap, kconfig.Config, error) {
	local := kconfig.New(kconfig.WithSource(file.NewSource(path)))
	defer local.Close()
	if err := local.Load(); err != nil {
		return nil, nil, err
	}

	var bootstrap conf.Bootstrap
	if err := local.Scan(&bootstrap); err != nil {
		return nil, nil, err
	}

	sources := []kconfig.Source{file.NewSource(path)}
	if envConfig := bootstrap.GetConfig().GetEnv(); envConfig.GetEnabled() {
		sources = append(sources, env.NewSource(envConfig.GetPrefix()))
	}
	if remote := bootstrap.GetConfig().GetRemote(); remote.GetEnabled() {
		source, err := newRemoteConfigSource(remote)
		if err != nil {
			return nil, nil, err
		}
		sources = append(sources, source)
	}

	cfg := kconfig.New(kconfig.WithSource(sources...))
	if err := cfg.Load(); err != nil {
		cfg.Close()
		return nil, nil, err
	}
	if err := cfg.Scan(&bootstrap); err != nil {
		cfg.Close()
		return nil, nil, err
	}
	registerConfigWatchers(cfg, bootstrap.GetConfig().GetWatch(), logger)
	return &bootstrap, cfg, nil
}

func registerConfigWatchers(cfg kconfig.Config, watch *conf.Config_Watch, logger *slog.Logger) {
	if !watch.GetEnabled() {
		return
	}
	for _, key := range watch.GetKeys() {
		key := key
		if err := cfg.Watch(key, func(key string, value kconfig.Value) {
			if logger != nil {
				logger.Info("config changed", slog.String("key", key), slog.Any("value", value.Load()))
			}
		}); err != nil && logger != nil {
			logger.Error("watch config key failed", slog.String("key", key), slog.Any("error", err))
		}
	}
}

func newRemoteConfigSource(c *conf.Config_Remote) (kconfig.Source, error) {
	switch strings.ToLower(c.GetDriver()) {
	case "apollo":
		return apolloconfig.NewSource(
			apolloconfig.WithAppID(c.GetAppId()),
			apolloconfig.WithCluster(c.GetCluster()),
			apolloconfig.WithEndpoint(firstEndpoint(c.GetEndpoints())),
			apolloconfig.WithNamespace(c.GetNamespace()),
			apolloconfig.WithSecret(c.GetSecret()),
			apolloconfig.WithOriginalConfig(),
		), nil
	case "consul":
		cfg := consulapi.DefaultConfig()
		if endpoint := firstEndpoint(c.GetEndpoints()); endpoint != "" {
			cfg.Address = endpoint
		}
		if c.GetToken() != "" {
			cfg.Token = c.GetToken()
		}
		client, err := consulapi.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		return consulconfig.New(client, consulconfig.WithPath(c.GetPath()))
	case "etcd":
		client, err := clientv3.New(clientv3.Config{
			Endpoints:   c.GetEndpoints(),
			Username:    c.GetUsername(),
			Password:    c.GetPassword(),
			DialTimeout: remoteTimeout(c),
		})
		if err != nil {
			return nil, err
		}
		return configsource.NewEtcdSource(client, c.GetPath()), nil
	case "kubernetes":
		return kubeconfig.NewSource(
			kubeconfig.Namespace(c.GetNamespace()),
			kubeconfig.LabelSelector(c.GetLabelSelector()),
			kubeconfig.FieldSelector(c.GetFieldSelector()),
			kubeconfig.KubeConfig(c.GetKubeconfig()),
			kubeconfig.Master(c.GetMaster()),
		), nil
	case "nacos":
		client, err := newNacosConfigClient(c)
		if err != nil {
			return nil, err
		}
		dataID := c.GetDataId()
		if dataID == "" {
			dataID = c.GetPath()
		}
		return nacosconfig.NewConfigSource(
			client,
			nacosconfig.WithDataID(dataID),
			nacosconfig.WithGroup(c.GetGroup()),
		), nil
	case "polaris":
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
	default:
		return nil, fmt.Errorf("unsupported config driver %q", c.GetDriver())
	}
}

func newNacosConfigClient(c *conf.Config_Remote) (nacosconfigclient.IConfigClient, error) {
	serverConfigs, err := nacosServerConfigs(c)
	if err != nil {
		return nil, err
	}
	clientConfig := nacosconstant.NewClientConfig(
		nacosconstant.WithNamespaceId(c.GetNamespace()),
		nacosconstant.WithUsername(c.GetUsername()),
		nacosconstant.WithPassword(c.GetPassword()),
		nacosconstant.WithTimeoutMs(uint64(remoteTimeout(c).Milliseconds())),
	)
	return nacosclients.NewConfigClient(nacosvo.NacosClientParam{
		ClientConfig:  clientConfig,
		ServerConfigs: serverConfigs,
	})
}

func nacosServerConfigs(c *conf.Config_Remote) ([]nacosconstant.ServerConfig, error) {
	endpoints := c.GetEndpoints()
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("nacos config requires at least one endpoint")
	}
	result := make([]nacosconstant.ServerConfig, 0, len(endpoints))
	for _, endpoint := range endpoints {
		host, port, err := splitHostPort(endpoint, 8848)
		if err != nil {
			return nil, err
		}
		opts := []nacosconstant.ServerOption{}
		if c.GetContextPath() != "" {
			opts = append(opts, nacosconstant.WithContextPath(c.GetContextPath()))
		}
		result = append(result, *nacosconstant.NewServerConfig(host, uint64(port), opts...))
	}
	return result, nil
}

func firstEndpoint(endpoints []string) string {
	if len(endpoints) > 0 {
		return endpoints[0]
	}
	return ""
}

func remoteTimeout(c *conf.Config_Remote) time.Duration {
	if c.GetTimeoutSeconds() > 0 {
		return time.Duration(c.GetTimeoutSeconds()) * time.Second
	}
	return 10 * time.Second
}

func splitHostPort(endpoint string, defaultPort int) (string, int, error) {
	if u, err := url.Parse(endpoint); err == nil && u.Host != "" {
		endpoint = u.Host
	}
	host, portText, err := net.SplitHostPort(endpoint)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			return endpoint, defaultPort, nil
		}
		return "", 0, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}
