package backend

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/zoekt"
	"github.com/sourcegraph/zoekt/query"
	"github.com/sourcegraph/zoekt/stream"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var (
	metricIgnoredError = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_zoekt_ignored_error_total",
		Help: "Total number of errors ignored from Zoekt.",
	}, []string{"reason"})
)

// HorizontalSearcher is a Streamer which aggregates searches over
// Map. It manages the connections to Map as the endpoints come and go.
type HorizontalSearcher struct {
	// Map is a subset of EndpointMap only using the Endpoints function. We
	// use this to find the endpoints to dial over time.
	Map interface {
		Endpoints() ([]string, error)
	}
	Dial func(endpoint string) zoekt.Streamer

	mu      sync.RWMutex
	clients map[string]zoekt.Streamer // addr -> client
}

func (s *HorizontalSearcher) StreamSearch(ctx context.Context, q query.Q, opts *zoekt.SearchOptions, streamer zoekt.Sender) error {
	clients, err := s.searchers()
	if err != nil {
		return err
	}

	endpoints := make([]string, 0, len(clients))
	for endpoint := range clients {
		endpoints = append(endpoints, endpoint) //nolint:staticcheck
	}

	flushSender := newFlushCollectSender(opts, endpoints, conf.RankingMaxQueueSizeBytes(), streamer)
	defer flushSender.Flush()

	// During re-balancing a repository can appear on more than one replica.
	var mu sync.Mutex
	dedupper := dedupper{}

	pl := pool.New().WithErrors()
	for endpoint, client := range clients {
		pl.Go(func() error {
			err := client.StreamSearch(ctx, q, opts, stream.SenderFunc(func(sr *zoekt.SearchResult) {
				// This shouldn't happen, but skip event if sr is nil.
				if sr == nil {
					return
				}

				mu.Lock()
				sr.Files = dedupper.Dedup(endpoint, sr.Files)
				mu.Unlock()

				flushSender.Send(endpoint, sr)
			}))

			if isZoektRolloutError(ctx, err) {
				flushSender.Send(endpoint, crashEvent())
				return nil
			}

			flushSender.SendDone(endpoint)
			return err
		})
	}

	return pl.Wait()
}

type queueSearchResult struct {
	*zoekt.SearchResult

	// optimization: It can be expensive to calculate sizeBytes, hence we cache it
	// in the queue.
	sizeBytes uint64
}

// Search aggregates search over every endpoint in Map.
func (s *HorizontalSearcher) Search(ctx context.Context, q query.Q, opts *zoekt.SearchOptions) (*zoekt.SearchResult, error) {
	return AggregateStreamSearch(ctx, s.StreamSearch, q, opts)
}

// AggregateStreamSearch aggregates the stream events into a single batch
// result.
func AggregateStreamSearch(ctx context.Context, streamSearch func(context.Context, query.Q, *zoekt.SearchOptions, zoekt.Sender) error, q query.Q, opts *zoekt.SearchOptions) (*zoekt.SearchResult, error) {
	start := time.Now()

	var mu sync.Mutex
	aggregate := &zoekt.SearchResult{}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err := streamSearch(ctx, q, opts, ZoektStreamFunc(func(event *zoekt.SearchResult) {
		mu.Lock()
		defer mu.Unlock()
		aggregate.Files = append(aggregate.Files, event.Files...)
		aggregate.Stats.Add(event.Stats)
	}))
	if err != nil {
		return nil, err
	}

	aggregate.Duration = time.Since(start)

	return aggregate, nil
}

// List aggregates list over every endpoint in Map.
func (s *HorizontalSearcher) List(ctx context.Context, q query.Q, opts *zoekt.ListOptions) (*zoekt.RepoList, error) {
	clients, err := s.searchers()
	if err != nil {
		return nil, err
	}

	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	type result struct {
		rl  *zoekt.RepoList
		err error
	}
	results := make(chan result, len(clients))
	for _, c := range clients {
		go func() {
			rl, err := c.List(ctx, q, opts)
			results <- result{rl: rl, err: err}
		}()
	}

	// PERF: We don't deduplicate Repos since the only user of List already
	// does deduplication.

	aggregate := zoekt.RepoList{
		ReposMap: make(zoekt.ReposMap),
	}
	for range clients {
		r := <-results
		if r.err != nil {
			if isZoektRolloutError(ctx, r.err) {
				aggregate.Crashes++
				continue
			}

			return nil, r.err
		}

		aggregate.Repos = append(aggregate.Repos, r.rl.Repos...)
		aggregate.Crashes += r.rl.Crashes
		aggregate.Stats.Add(&r.rl.Stats)

		for k, v := range r.rl.ReposMap {
			aggregate.ReposMap[k] = v
		}
	}

	// Only one of these fields is populated and in all cases the size of that
	// field is the number of Repos. We may overcount in the case of asking
	// for Repos since we don't deduplicate, but this should be very rare
	// (only happens in the case of rebalancing)
	aggregate.Stats.Repos = len(aggregate.Repos) + len(aggregate.ReposMap)

	return &aggregate, nil
}

// Close will close all connections in Map.
func (s *HorizontalSearcher) Close() {
	s.mu.Lock()
	clients := s.clients
	s.clients = nil
	s.mu.Unlock()
	for _, c := range clients {
		c.Close()
	}
}

func (s *HorizontalSearcher) String() string {
	s.mu.RLock()
	clients := s.clients
	s.mu.RUnlock()
	addrs := make([]string, 0, len(clients))
	for addr := range clients {
		addrs = append(addrs, addr)
	}
	sort.Strings(addrs)
	return fmt.Sprintf("HorizontalSearcher{%v}", addrs)
}

// searchers returns the list of clients to aggregate over.
func (s *HorizontalSearcher) searchers() (map[string]zoekt.Streamer, error) {
	eps, err := s.Map.Endpoints()
	if err != nil {
		return nil, err
	}

	// Fast-path, check if Endpoints matches addrs. If it does we can use
	// s.clients.
	//
	// We structure our state to optimize for the fast-path.
	s.mu.RLock()
	clients := s.clients
	s.mu.RUnlock()
	if equalKeys(clients, eps) {
		return clients, nil
	}

	// Slow-path, need to remove/connect.
	return s.syncSearchers()
}

// syncSearchers syncs the set of clients with the set of endpoints. It is the
// slow-path of "searchers" since it obtains an write lock on the state before
// proceeding.
func (s *HorizontalSearcher) syncSearchers() (map[string]zoekt.Streamer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double check someone didn't beat us to the update
	eps, err := s.Map.Endpoints()
	if err != nil {
		return nil, err
	}

	if equalKeys(s.clients, eps) {
		return s.clients, nil
	}

	set := make(map[string]struct{}, len(eps))
	for _, ep := range eps {
		set[ep] = struct{}{}
	}

	// Disconnect first
	for addr, client := range s.clients {
		if _, ok := set[addr]; !ok {
			client.Close()
		}
	}

	// Use new map to avoid read conflicts
	clients := make(map[string]zoekt.Streamer, len(eps))
	for _, addr := range eps {
		// Try re-use
		client, ok := s.clients[addr]
		if !ok {
			client = s.Dial(addr)
		}
		clients[addr] = client
	}
	s.clients = clients

	return s.clients, nil
}

func equalKeys(a map[string]zoekt.Streamer, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, k := range b {
		if _, ok := a[k]; !ok {
			return false
		}
	}
	return true
}

type dedupper map[string]string // repoName -> endpoint

// Dedup will in-place filter out matches on Repositories we have already
// seen. A Repository has been seen if a previous call to Dedup had a match in
// it with a different endpoint.
func (repoEndpoint dedupper) Dedup(endpoint string, fms []zoekt.FileMatch) []zoekt.FileMatch {
	if len(fms) == 0 { // handles fms being nil
		return fms
	}

	// PERF: Normally fms is sorted by Repository. So we can avoid the map
	// lookup if we just did it for the previous entry.
	lastRepo := ""
	lastSeen := false

	// Remove entries for repos we have already seen.
	dedup := fms[:0]
	for _, fm := range fms {
		if lastRepo == fm.Repository {
			if lastSeen {
				continue
			}
		} else if ep, ok := repoEndpoint[fm.Repository]; ok && ep != endpoint {
			lastRepo = fm.Repository
			lastSeen = true
			continue
		}

		lastRepo = fm.Repository
		lastSeen = false
		dedup = append(dedup, fm)
	}

	// Update seenRepo now, so the next call of dedup will contain the
	// repos.
	lastRepo = ""
	for _, fm := range dedup {
		if lastRepo != fm.Repository {
			lastRepo = fm.Repository
			repoEndpoint[fm.Repository] = endpoint
		}
	}

	return dedup
}

// isZoektRolloutError returns true if the error we received from zoekt can be
// ignored.
//
// Note: ctx is passed in so we can log to the trace when we ignore an
// error. This is a convenience over logging at the call sites.
//
// Currently the only error we ignore is DNS lookup failures. This is since
// during rollouts of Zoekt, we may still have endpoints of zoekt which are
// not available in our endpoint map. In particular, this happens when using
// Kubernetes and the (default) stateful set watcher.
func isZoektRolloutError(ctx context.Context, err error) bool {
	reason := zoektRolloutReason(err)
	if reason == "" {
		return false
	}

	metricIgnoredError.WithLabelValues(reason).Inc()
	trace.FromContext(ctx).AddEvent("rollout",
		attribute.String("rollout.reason", reason),
		attribute.String("rollout.error", err.Error()))

	return true
}

func zoektRolloutReason(err error) string {
	// Please only add very specific error checks here. An error can be added
	// here if we see it correlated with rollouts on sourcegraph.com.

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
		return "dns-not-found"
	}

	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		return ""
	}

	if opErr.Op == "dial" {
		if opErr.Timeout() {
			return "dial-timeout"
		}
		// ugly to do this, but is the most robust way. go's net tests do the
		// same check. example:
		//
		//   dial tcp 10.164.51.47:6070: connect: connection refused
		if strings.Contains(opErr.Err.Error(), "connection refused") {
			return "dial-refused"
		}
	}

	// Zoekt does not have a proper graceful shutdown for net/rpc since those
	// connections are multi-plexed over a single HTTP connection. This means
	// we often run into this during rollout for List calls (Search calls use
	// streaming RPC).
	if opErr.Op == "read" {
		return "read-failed"
	}

	return ""
}

// crashEvent indicates a shard or backend failed to be searched due to a
// panic or being unreachable. The most common reason for this is during zoekt
// rollout.
func crashEvent() *zoekt.SearchResult {
	return &zoekt.SearchResult{Stats: zoekt.Stats{
		Crashes: 1,
	}}
}
