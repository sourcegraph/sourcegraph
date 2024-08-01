package shared

import (
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/releaseregistry"
	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Config struct {
	env.BaseConfig

	k8sConfig              *rest.Config
	metrics                metricsConfig
	pprofAddr              string
	grpc                   grpcConfig
	http                   httpConfig
	namespace              string
	relregEndpoint         string
	applianceVersion       string
	selfDeploymentName     string
	noResourceRestrictions string
}

func (c *Config) Load() {
	var kubeConfig string
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		if home := homedir.HomeDir(); home != "" {
			kubeConfig = c.Get("KUBECONFIG", filepath.Join(home, ".kube", "config"), "Absolute path to the kubeconfig file.")
		}

		k8sConfig, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			c.AddError(errors.New("could not create kubernetes client config"))
		}
	}

	c.k8sConfig = k8sConfig
	c.metrics.addr = c.Get("APPLIANCE_METRICS_ADDR", ":8734", "Appliance metrics server address.")
	c.pprofAddr = c.Get("APPLIANCE_PPROF_ADDR", ":6060", "Server on which to expose pprof")
	c.metrics.secure = c.GetBool("APPLIANCE_METRICS_SECURE", "false", "Appliance metrics server uses https.")
	c.grpc.addr = c.Get("APPLIANCE_GRPC_ADDR", ":9000", "Appliance gRPC address.")
	c.http.addr = c.Get("APPLIANCE_HTTP_ADDR", ":8888", "Appliance http address.")
	c.namespace = c.Get("APPLIANCE_NAMESPACE", "default", "Namespace to monitor.")
	c.applianceVersion = c.Get("APPLIANCE_VERSION", version.Version(), "Version tag for the running appliance.")
	c.selfDeploymentName = c.Get("APPLIANCE_DEPLOYMENT_NAME", "", "Own deployment name for self-update. Default is to disable self-update.")
	c.relregEndpoint = c.Get("RELEASE_REGISTRY_ENDPOINT", releaseregistry.Endpoint, "Release registry endpoint.")
	c.noResourceRestrictions = c.Get("APPLIANCE_NO_RESOURCE_RESTRICTIONS", "false", "Remove all resource requests and limits from deployed resources. Only recommended for local development.")
}

func (c *Config) Validate() error {
	var errs error
	return errs
}

type metricsConfig struct {
	addr   string
	secure bool
}

type grpcConfig struct {
	addr string
}

type httpConfig struct {
	addr string
}
