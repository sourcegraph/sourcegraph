package debugproxies

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

// Represents an endpoint
type Endpoint struct {
	// Service to which the endpoint belongs
	Service string
	// Addr:port, so hostname part of a URL (ip address ok)
	Addr string
	// Hostname of the endpoint, if set. Only use this for display purposes,
	// it doesn't include the port nor is it gaurenteed to be resolvable.
	Hostname string
}

// ScanConsumer is the callback to consume scan results.
type ScanConsumer func([]Endpoint)

// clusterScanner scans the cluster for endpoints belonging to services that have annotation sourcegraph.prometheus/scrape=true.
// It runs an event loop that reacts to changes to the endpoints set. Everytime there is a change it calls the ScanConsumer.
type clusterScanner struct {
	client    v1.CoreV1Interface
	namespace string
	consume   ScanConsumer
}

// Starts a cluster scanner with the specified client and consumer. Does not block.
func startClusterScannerWithClient(client *kubernetes.Clientset, ns string, consumer ScanConsumer) error {

	cs := &clusterScanner{
		client:    client.CoreV1(),
		namespace: ns,
		consume:   consumer,
	}

	go cs.runEventLoop()
	return nil
}

// Starts a cluster scanner with the specified consumer. Does not block.
func StartClusterScanner(consumer ScanConsumer) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	ns := namespace()
	// access to K8s clients
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	return startClusterScannerWithClient(clientset, ns, consumer)
}

// Runs the k8s.Watch endpoints event loop, and triggers a rescan of cluster when something changes with endpoints.
// Before spinning in the loop does an initial scan.
func (cs *clusterScanner) runEventLoop() {
	cs.scanCluster()
	for {
		ok, err := cs.watchEndpointEvents()
		if ok {
			log15.Debug("ephemeral kubernetes endpoint watch error. Will start loop again in 5s", "endpoint", "/-/debug", "error", err)
		} else {
			log15.Warn("failed to connect to kubernetes endpoint watcher. Will retry in 5s.", "endpoint", "/-/debug", "error", err)
		}
		time.Sleep(time.Second * 5)
	}
}

// watchEndpointEvents uses the k8s watch API operation to watch for endpoint events. Spins forever unless an error
// occurs that would necessitate creating a new watcher. The caller will then call again creating the new watcher.
func (cs *clusterScanner) watchEndpointEvents() (bool, error) {

	// TODO(Dax): Rewrite this to used NewSharedInformerFactory from k8s/client-go
	watcher, err := cs.client.Endpoints(cs.namespace).Watch(metav1.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("k8s client.Watch error: %w", err)
	}
	defer watcher.Stop()

	for {
		event := <-watcher.ResultChan()
		if err != nil {
			// we need a new watcher
			return true, fmt.Errorf("k8s watcher.Next error: %w", err)
		}

		if event.Type == watch.Error {
			// we need a new watcher
			return true, errors.New("error event")
		}

		cs.scanCluster()
	}
}

// scanCluster looks for endpoints belonging to services that have annotation sourcegraph.prometheus/scrape=true.
// It derives the appropriate port from the prometheus.io/port annotation.
func (cs *clusterScanner) scanCluster() {

	// Get services from the current namespace
	services, err := cs.client.Services(cs.namespace).List(metav1.ListOptions{})
	if err != nil {
		log15.Error("k8s failed to list services", "error", err)
	}

	var scanResults []Endpoint

	for _, svc := range services.Items {
		svcName := svc.Name

		// TODO(uwedeportivo): pgsql doesn't work, figure out why
		if svcName == "pgsql" {
			continue
		}

		if svc.Annotations["sourcegraph.prometheus/scrape"] != "true" {
			continue
		}

		var port int
		if portStr := svc.Annotations["prometheus.io/port"]; portStr != "" {
			port, err = strconv.Atoi(portStr)
			if err != nil {
				log15.Debug("k8s prometheus.io/port annotation for service is not an integer", "service", svcName, "port", portStr)
				continue
			}
		}

		endpoints, err := cs.client.Endpoints(cs.namespace).Get(svcName, metav1.GetOptions{})
		if err != nil {
			log15.Error("k8s failed to get endpoints", "error", err)
			return
		}
		for _, subset := range endpoints.Subsets {
			var ports []int
			if port != 0 {
				ports = []int{port}
			} else {
				for _, port := range subset.Ports {
					ports = append(ports, int(port.Port))
				}
			}

			for _, addr := range subset.Addresses {
				for _, port := range ports {
					addrStr := addr.IP
					if addrStr == "" {
						addrStr = addr.Hostname
					}

					if addrStr != "" {
						scanResults = append(scanResults, Endpoint{
							Service:  svcName,
							Addr:     fmt.Sprintf("%s:%d", addrStr, port),
							Hostname: addr.Hostname,
						})
					}
				}
			}
		}
	}

	cs.consume(scanResults)
}

// namespace returns the namespace the pod is currently running in
// this is done because the k8s client we previously used set the namespace
// when the client was created, the official k8s client does not
func namespace() string {
	const filename = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log15.Warn("scanner: falling back to kubernetes default namespace", "filename", filename, "error", err)
		return "default"
	}

	ns := strings.TrimSpace(string(data))
	if ns == "" {
		log15.Warn("file: ", filename, " empty using \"default\" ns")
		return "default"
	}
	return ns
}
