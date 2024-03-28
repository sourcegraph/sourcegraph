package endpoint

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// K8S returns a Map for the given k8s urlspec (e.g. k8s+http://searcher), starting
// service discovery in the background.
func K8S(logger log.Logger, urlspec string) *Map {
	logger = logger.Scoped("k8s")
	return &Map{
		urlspec:   urlspec,
		discofunk: k8sDiscovery(logger, urlspec, Namespace(logger), LoadClient),
	}
}

// k8sDiscovery does service discovery of the given k8s urlspec (e.g. k8s+http://searcher),
// publishing endpoint changes to the given disco channel. It's started by endpoint.K8S as a
// go-routine.
func k8sDiscovery(logger log.Logger, urlspec, ns string, clientFactory func() (*kubernetes.Clientset, error)) func(chan endpoints) {
	return func(disco chan endpoints) {
		u, err := parseURL(urlspec)
		if err != nil {
			disco <- endpoints{Service: urlspec, Error: err}
			return
		}

		var cli *kubernetes.Clientset
		if cli, err = clientFactory(); err != nil {
			disco <- endpoints{Service: urlspec, Error: err}
			return
		}

		factory := informers.NewSharedInformerFactoryWithOptions(cli, 0,
			informers.WithNamespace(ns),
			informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
				opts.FieldSelector = "metadata.name=" + u.Service
			}),
		)

		var informer cache.SharedIndexInformer
		switch u.Kind {
		case "sts", "statefulset":
			informer = factory.Apps().V1().StatefulSets().Informer()
		default:
			informer = factory.Core().V1().Endpoints().Informer()
		}

		handle := func(obj any) {
			eps := k8sEndpoints(u, obj)
			logger.Info(
				"endpoints k8s discovered",
				log.String("urlspec", urlspec),
				log.String("service", u.Service),
				log.Int("count", len(eps)),
				log.Strings("endpoints", eps),
			)

			if len(eps) == 0 {
				err := errors.Errorf(
					"no %s endpoints could be found (this may indicate more %s replicas are needed, contact support@sourcegraph.com for assistance)",
					u.Service,
					u.Service,
				)
				disco <- endpoints{Service: u.Service, Error: err}
				return
			}

			disco <- endpoints{Service: u.Service, Endpoints: eps}
		}

		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    handle,
			UpdateFunc: func(_, obj any) { handle(obj) },
		})

		stop := make(chan struct{})
		defer close(stop)

		informer.Run(stop)
	}
}

// k8sEndpoints constructs a list of endpoint addresses for u based on the
// kubernetes resource object obj.
func k8sEndpoints(u *k8sURL, obj any) []string {
	var eps []string

	switch o := (obj).(type) {
	case *corev1.Endpoints:
		for _, s := range o.Subsets {
			for _, a := range s.Addresses {
				var ep string
				if a.Hostname != "" {
					ep = u.endpointURL(a.Hostname + "." + u.Service)
				} else if a.IP != "" {
					ep = u.endpointURL(a.IP)
				}
				eps = append(eps, ep)
			}
		}
	case *appsv1.StatefulSet:
		replicas := int32(1)
		if o.Spec.Replicas != nil {
			replicas = *o.Spec.Replicas
		}
		for i := int32(0); i < replicas; i++ {
			// Quoting k8s Reference: https://v1-21.docs.kubernetes.io/docs/concepts/workloads/controllers/statefulset/#stable-network-id
			//
			// Each Pod in a StatefulSet derives its hostname from the
			// name of the StatefulSet and the ordinal of the Pod. The
			// pattern for the constructed hostname is $(statefulset
			// name)-$(ordinal). ... A StatefulSet can use a Headless
			// Service to control the domain of its Pods. ... As each
			// Pod is created, it gets a matching DNS subdomain,
			// taking the form: $(podname).$(governing service
			// domain), where the governing service is defined by the
			// serviceName field on the StatefulSet.
			//
			// We set serviceName in our resources and ensure it is a
			// headless service.
			eps = append(eps, u.endpointURL(fmt.Sprintf("%s-%d.%s", o.Name, i, o.Spec.ServiceName)))
		}
	}

	return eps
}

type k8sURL struct {
	url.URL

	Service   string
	Namespace string
	Kind      string
}

func (u *k8sURL) endpointURL(endpoint string) string {
	uCopy := u.URL
	if port := u.Port(); port != "" {
		uCopy.Host = endpoint + ":" + port
	} else {
		uCopy.Host = endpoint
	}
	if uCopy.Scheme == "rpc" {
		return uCopy.Host
	}
	return uCopy.String()
}

func parseURL(rawurl string) (*k8sURL, error) {
	u, err := url.Parse(strings.TrimPrefix(rawurl, "k8s+"))
	if err != nil {
		return nil, err
	}

	parts := strings.Split(u.Hostname(), ".")
	var svc, ns string
	switch len(parts) {
	case 1:
		svc = parts[0]
	case 2:
		svc, ns = parts[0], parts[1]
	default:
		return nil, errors.Errorf("invalid k8s url. expected k8s+http://service.namespace:port/path?kind=$kind, got %s", rawurl)
	}

	return &k8sURL{
		URL:       *u,
		Service:   svc,
		Namespace: ns,
		Kind:      strings.ToLower(u.Query().Get("kind")),
	}, nil
}

// Namespace returns the Namespace the pod is currently running in
// this is done because the k8s client we previously used set the Namespace
// when the client was created, the official k8s client does not
func Namespace(logger log.Logger) string {
	logger = logger.Scoped("namespace")
	const filename = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	data, err := os.ReadFile(filename)
	if err != nil {
		logger.Warn("falling back to kubernetes default namespace", log.String("error", filename+" is empty"))
		return "default"
	}

	ns := strings.TrimSpace(string(data))
	if ns == "" {
		logger.Warn("empty namespace in file", log.String("filename", filename), log.String("namespaceInFile", ""), log.String("namespace", "default"))
		return "default"
	}
	return ns
}

func LoadClient() (client *kubernetes.Clientset, err error) {
	// Uncomment below to test against a real cluster. This is only important
	// when you are changing how we interact with the k8s API and you want to
	// test against the real thing.
	// Ensure you set your KUBECONFIG env var or your current kubeconfig will be used

	// InClusterConfig only works when running inside of a pod in a k8s
	// cluster.
	// From https://github.com/kubernetes/client-go/tree/master/examples/out-of-cluster-client-configuration
	/*
		c, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
		if err != nil {
			log15.Error("couldn't load kubeconfig")
			os.Exit(1)
		}
		clientConfig := clientcmd.NewDefaultClientConfig(*c, nil)
		config, err = clientConfig.ClientConfig()
		Namespace = "prod"
	*/

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return client, err
}

var metricEndpointSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "src_endpoints_size",
	Help: "The number of service endpoints discovered",
}, []string{"service"})
