package registry

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"temperate/internal/conf"

	consulreg "github.com/go-kratos/kratos/contrib/registry/consul/v3"
	discoveryreg "github.com/go-kratos/kratos/contrib/registry/discovery/v3"
	etcdreg "github.com/go-kratos/kratos/contrib/registry/etcd/v3"
	kubereg "github.com/go-kratos/kratos/contrib/registry/kubernetes/v3"
	nacosreg "github.com/go-kratos/kratos/contrib/registry/nacos/v3"
	polarisreg "github.com/go-kratos/kratos/contrib/registry/polaris/v3"
	zookeeperreg "github.com/go-kratos/kratos/contrib/registry/zookeeper/v3"
	"github.com/go-kratos/kratos/v3/registry"
	"github.com/go-zookeeper/zk"
	"github.com/google/wire"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	polarisapi "github.com/polarismesh/polaris-go/api"
	polarisconfig "github.com/polarismesh/polaris-go/pkg/config"
	clientv3 "go.etcd.io/etcd/client/v3"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ProviderSet is registry providers.
var ProviderSet = wire.NewSet(NewRegistrar)

func NewRegistrar(c *conf.Registry) (registry.Registrar, func(), error) {
	if c == nil || !c.GetEnabled() {
		return nil, func() {}, nil
	}
	switch strings.ToLower(c.GetDriver()) {
	case "consul":
		return newConsul(c)
	case "discovery":
		return newDiscovery(c)
	case "etcd":
		return newEtcd(c)
	case "kubernetes":
		return newKubernetes(c)
	case "nacos":
		return newNacos(c)
	case "polaris":
		return newPolaris(c)
	case "zookeeper":
		return newZookeeper(c)
	default:
		return nil, nil, fmt.Errorf("unsupported registry driver %q", c.GetDriver())
	}
}

func newConsul(c *conf.Registry) (registry.Registrar, func(), error) {
	cfg := consulapi.DefaultConfig()
	if endpoint := firstEndpoint(c); endpoint != "" {
		cfg.Address = endpoint
	}
	if c.GetToken() != "" {
		cfg.Token = c.GetToken()
	}
	if c.GetDatacenter() != "" {
		cfg.Datacenter = c.GetDatacenter()
	}
	client, err := consulapi.NewClient(cfg)
	if err != nil {
		return nil, nil, err
	}
	opts := []consulreg.Option{}
	if c.GetHealthCheck() {
		opts = append(opts, consulreg.WithHealthCheck(true))
	}
	if c.GetHeartbeat() {
		opts = append(opts, consulreg.WithHeartbeat(true))
	}
	return consulreg.New(client, opts...), func() {}, nil
}

func newDiscovery(c *conf.Registry) (registry.Registrar, func(), error) {
	reg := discoveryreg.New(&discoveryreg.Config{
		Nodes:  c.GetEndpoints(),
		Region: c.GetRegion(),
		Zone:   c.GetZone(),
		Env:    c.GetEnv(),
		Host:   c.GetHost(),
	})
	return reg, func() { _ = reg.Close() }, nil
}

func newEtcd(c *conf.Registry) (registry.Registrar, func(), error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   c.GetEndpoints(),
		Username:    c.GetUsername(),
		Password:    c.GetPassword(),
		DialTimeout: timeout(c),
	})
	if err != nil {
		return nil, nil, err
	}
	opts := []etcdreg.Option{}
	if c.GetNamespace() != "" {
		opts = append(opts, etcdreg.Namespace(c.GetNamespace()))
	}
	if c.GetTtlSeconds() > 0 {
		opts = append(opts, etcdreg.RegisterTTL(time.Duration(c.GetTtlSeconds())*time.Second))
	}
	return etcdreg.New(client, opts...), func() { _ = client.Close() }, nil
}

func newKubernetes(c *conf.Registry) (registry.Registrar, func(), error) {
	restConfig, err := rest.InClusterConfig()
	if c.GetKubeconfig() != "" {
		restConfig, err = clientcmd.BuildConfigFromFlags("", c.GetKubeconfig())
	}
	if err != nil {
		return nil, nil, err
	}
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, err
	}
	return kubereg.NewRegistry(clientSet, c.GetNamespace()), func() {}, nil
}

func newNacos(c *conf.Registry) (registry.Registrar, func(), error) {
	serverConfigs, err := nacosServerConfigs(c)
	if err != nil {
		return nil, nil, err
	}
	clientConfig := constant.NewClientConfig(
		constant.WithNamespaceId(c.GetNamespace()),
		constant.WithUsername(c.GetUsername()),
		constant.WithPassword(c.GetPassword()),
		constant.WithTimeoutMs(uint64(timeout(c).Milliseconds())),
	)
	client, err := clients.NewNamingClient(voClientParam(clientConfig, serverConfigs))
	if err != nil {
		return nil, nil, err
	}
	opts := []nacosreg.Option{}
	if c.GetGroup() != "" {
		opts = append(opts, nacosreg.WithGroup(c.GetGroup()))
	}
	if c.GetCluster() != "" {
		opts = append(opts, nacosreg.WithCluster(c.GetCluster()))
	}
	return nacosreg.New(client, opts...), func() {}, nil
}

func voClientParam(clientConfig *constant.ClientConfig, serverConfigs []constant.ServerConfig) vo.NacosClientParam {
	return vo.NacosClientParam{
		ClientConfig:  clientConfig,
		ServerConfigs: serverConfigs,
	}
}

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

func newZookeeper(c *conf.Registry) (registry.Registrar, func(), error) {
	conn, _, err := zk.Connect(c.GetEndpoints(), timeout(c))
	if err != nil {
		return nil, nil, err
	}
	opts := []zookeeperreg.Option{}
	if c.GetNamespace() != "" {
		opts = append(opts, zookeeperreg.WithRootPath(c.GetNamespace()))
	}
	if c.GetUsername() != "" && c.GetPassword() != "" {
		opts = append(opts, zookeeperreg.WithDigestACL(c.GetUsername(), c.GetPassword()))
	}
	return zookeeperreg.New(conn, opts...), func() { conn.Close() }, nil
}

func firstEndpoint(c *conf.Registry) string {
	if endpoints := c.GetEndpoints(); len(endpoints) > 0 {
		return endpoints[0]
	}
	return ""
}

func timeout(c *conf.Registry) time.Duration {
	if c.GetTimeoutSeconds() > 0 {
		return time.Duration(c.GetTimeoutSeconds()) * time.Second
	}
	return 10 * time.Second
}

func nacosServerConfigs(c *conf.Registry) ([]constant.ServerConfig, error) {
	endpoints := c.GetEndpoints()
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("nacos registry requires at least one endpoint")
	}
	result := make([]constant.ServerConfig, 0, len(endpoints))
	for _, endpoint := range endpoints {
		host, port, err := splitHostPort(endpoint, 8848)
		if err != nil {
			return nil, err
		}
		opts := []constant.ServerOption{}
		if c.GetContextPath() != "" {
			opts = append(opts, constant.WithContextPath(c.GetContextPath()))
		}
		result = append(result, *constant.NewServerConfig(host, uint64(port), opts...))
	}
	return result, nil
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
