// Package endpoint provides a consistent hash map over service endpoints.
package endpoint

import (
	"fmt"
	"strings"
	"sync"

	"github.com/cespare/xxhash/v2"

	"github.com/sourcegraph/go-rendezvous"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
)

// EmptyError is returned when looking up an endpoint on an empty map.
type EmptyError struct {
	URLSpec string
}

func (e *EmptyError) Error() string {
	return fmt.Sprintf("endpoint.Map(%s) is empty", e.URLSpec)
}

// Map is a consistent hash map to URLs. It uses the kubernetes API to
// watch the endpoints for a service and update the map when they change. It
// can also fallback to static URLs if not configured for kubernetes.
type Map struct {
	urlspec string

	mu  sync.RWMutex
	hm  *rendezvous.Rendezvous
	err error

	init      sync.Once
	discofunk func(chan endpoints) // I like to know who is in my party!
}

// endpoints represents a list of a service's endpoints as discovered through
// the chosen service discovery mechanism.
type endpoints struct {
	Service   string
	Endpoints []string
	Error     error
}

// New creates a new Map for the URL specifier.
//
// If the scheme is prefixed with "k8s+", one URL is expected and the format is
// expected to match e.g. k8s+http://service.namespace:port/path. Namespace,
// port and path are optional. URLs of this form will consistently hash among
// the endpoints for the Kubernetes service. The values returned by Get will
// look like http://endpoint:port/path.
//
// If the scheme is not prefixed with "k8s+", a space separated list of URLs is
// expected. The map will consistently hash against these URLs in this case.
// This is useful for specifying non-Kubernetes endpoints.
//
// Examples URL specifiers:
//
//	"k8s+http://searcher"
//	"k8s+rpc://indexed-searcher?kind=sts"
//	"http://searcher-0 http://searcher-1 http://searcher-2"
//
// Note: this function does not take a logger because discovery is done in the
// in the background and does not connect to higher order functions.
func New(urlspec string) *Map {
	logger := log.Scoped("newmap")
	if !strings.HasPrefix(urlspec, "k8s+") {
		return Static(strings.Fields(urlspec)...)
	}
	return K8S(logger, urlspec)
}

// Static returns an Endpoint map which consistently hashes over endpoints.
//
// There are no requirements on endpoints, it can be any arbitrary
// string. Unlike static endpoints created via New.
//
// Static Maps are guaranteed to never return an error.
func Static(endpoints ...string) *Map {
	return &Map{
		urlspec: fmt.Sprintf("%v", endpoints),
		hm:      newConsistentHash(endpoints),
	}
}

// Empty returns an Endpoint map which always fails with err.
func Empty(err error) *Map {
	return &Map{
		urlspec: "error: " + err.Error(),
		err:     err,
	}
}

func (m *Map) String() string {
	return fmt.Sprintf("endpoint.Map(%s)", m.urlspec)
}

// Get the closest URL in the hash to the provided key.
//
// Note: For k8s URLs we return URLs based on the registered endpoints. The
// endpoint may not actually be available yet / at the moment. So users of the
// URL should implement a retry strategy.
func (m *Map) Get(key string) (string, error) {
	m.init.Do(m.discover)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.err != nil {
		return "", m.err
	}

	v := m.hm.Lookup(key)
	if v == "" {
		return "", &EmptyError{URLSpec: m.urlspec}
	}

	return v, nil
}

// GetN gets the n closest URLs in the hash to the provided key.
func (m *Map) GetN(key string, n int) ([]string, error) {
	m.init.Do(m.discover)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.err != nil {
		return nil, m.err
	}

	// LookupN can fail if n > len(nodes), but the client code will have a
	// race. So double check while we hold the lock.
	nodes := len(m.hm.Nodes())
	if nodes == 0 {
		return nil, &EmptyError{URLSpec: m.urlspec}
	}
	if n > nodes {
		n = nodes
	}

	return m.hm.LookupN(key, n), nil
}

// GetMany is the same as calling Get on each item of keys. It will only
// acquire the underlying endpoint map once, so is preferred to calling Get
// for each key which will acquire the endpoint map for each call. The benefit
// is it is faster (O(1) mutex acquires vs O(n)) and consistent (endpoint map
// is immutable vs may change between Get calls).
func (m *Map) GetMany(keys ...string) ([]string, error) {
	m.init.Do(m.discover)

	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.err != nil {
		return nil, m.err
	}

	// If we are doing a lookup ensure we are not empty.
	if len(keys) > 0 && len(m.hm.Nodes()) == 0 {
		return nil, &EmptyError{URLSpec: m.urlspec}
	}

	vals := make([]string, len(keys))
	for i := range keys {
		vals[i] = m.hm.Lookup(keys[i])
	}

	return vals, nil
}

// Endpoints returns a list of all addresses. Do not modify the returned value.
func (m *Map) Endpoints() ([]string, error) {
	m.init.Do(m.discover)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.err != nil {
		return nil, m.err
	}

	return m.hm.Nodes(), nil
}

// discover updates the Map with discovered endpoints
func (m *Map) discover() {
	if m.discofunk == nil {
		return
	}

	ch := make(chan endpoints)
	ready := make(chan struct{})

	go m.sync(ch, ready)
	go m.discofunk(ch)

	<-ready
}

func (m *Map) sync(ch chan endpoints, ready chan struct{}) {
	logger := log.Scoped("endpoint")
	for eps := range ch {

		logger.Info(
			"endpoints k8s discovered",
			log.String("urlspec", m.urlspec),
			log.String("service", eps.Service),
			log.Int("count", len(eps.Endpoints)),
			log.Error(eps.Error),
		)

		metricEndpointSize.WithLabelValues(eps.Service).Set(float64(len(eps.Endpoints)))

		var hm *rendezvous.Rendezvous
		if eps.Error == nil {
			hm = newConsistentHash(eps.Endpoints)
		}

		m.mu.Lock()
		m.hm = hm
		m.err = eps.Error
		m.mu.Unlock()

		select {
		case <-ready:
		default:
			close(ready)
		}
	}
}

// ConfBasedGetter is called each time the configuration is updated and
// returns the endpoints for the host.
type ConfBasedGetter func(conns conftypes.ServiceConnections) []string

// ConfBased returns a Map that watches the global conf and calls the provided
// getter to extract endpoints.
func ConfBased(getter ConfBasedGetter) *Map {
	return &Map{
		urlspec: "conf-based",
		discofunk: func(disco chan endpoints) {
			conf.Watch(func() {
				serviceConnections := conf.Get().ServiceConnections()

				eps := getter(serviceConnections)
				disco <- endpoints{Endpoints: eps}
			})
		},
	}
}

func newConsistentHash(nodes []string) *rendezvous.Rendezvous {
	return rendezvous.New(nodes, xxhash.Sum64String)
}
