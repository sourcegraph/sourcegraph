package backend

import (
	"context"
	"sync"
	"time"

	"github.com/google/zoekt"
	"github.com/google/zoekt/query"
	"github.com/inconshreveable/log15"
	"github.com/keegancsmith/rpc"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
)

var requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "src_zoekt_request_duration_seconds",
	Help:    "Time (in seconds) spent on request.",
	Buckets: prometheus.DefBuckets,
}, []string{"hostname", "category", "code"})

type meteredSearcher struct {
	zoekt.Streamer

	hostname string
}

func NewMeteredSearcher(hostname string, z zoekt.Streamer) zoekt.Streamer {
	return &meteredSearcher{
		Streamer: z,
		hostname: hostname,
	}
}

func (m *meteredSearcher) StreamSearch(ctx context.Context, q query.Q, opts *zoekt.SearchOptions, c zoekt.Sender) error {
	start := time.Now()

	// isLeaf is true if this is a zoekt.Searcher which does a network
	// call. False if we are an aggregator. We use this to decide if we need
	// to add RPC tracing and adjust how we record metrics.
	isLeaf := m.hostname != ""

	var cat string
	var tags []trace.Tag
	if !isLeaf {
		cat = "SearchAll"
	} else {
		cat = "Search"
		tags = []trace.Tag{
			{Key: "span.kind", Value: "client"},
			{Key: "peer.address", Value: m.hostname},
			{Key: "peer.service", Value: "zoekt"},
		}
	}

	tr, ctx := trace.New(ctx, "zoekt."+cat, queryString(q), tags...)
	if opts != nil {
		tr.LogFields(
			log.Bool("opts.estimate_doc_count", opts.EstimateDocCount),
			log.Bool("opts.whole", opts.Whole),
			log.Int("opts.shard_max_match_count", opts.ShardMaxMatchCount),
			log.Int("opts.total_max_match_count", opts.TotalMaxMatchCount),
			log.Int("opts.shard_max_important_match", opts.ShardMaxImportantMatch),
			log.Int("opts.total_max_important_match", opts.TotalMaxImportantMatch),
			log.Int64("opts.max_wall_time_ms", opts.MaxWallTime.Milliseconds()),
			log.Int("opts.max_doc_display_count", opts.MaxDocDisplayCount),
		)
	}
	defer tr.Finish()

	if isLeaf && opts != nil && ot.ShouldTrace(ctx) {
		// Replace any existing spanContext with a new one, given we've done additional tracing
		spanContext := make(map[string]string)
		if err := ot.GetTracer(ctx).Inject(opentracing.SpanFromContext(ctx).Context(), opentracing.TextMap, opentracing.TextMapCarrier(spanContext)); err == nil {
			newOpts := *opts
			newOpts.SpanContext = spanContext
			opts = &newOpts
		} else {
			log15.Warn("meteredSearcher: Error injecting new span context into map: %s", err)
		}
	}

	// Instrument the RPC layer
	var writeRequestStart, writeRequestDone time.Time
	if isLeaf {
		ctx = rpc.WithClientTrace(ctx, &rpc.ClientTrace{
			WriteRequestStart: func() {
				tr.LogFields(log.String("event", "rpc.write_request_start"))
				writeRequestStart = time.Now()
			},

			WriteRequestDone: func(err error) {
				fields := []log.Field{log.String("event", "rpc.write_request_done")}
				if err != nil {
					fields = append(fields, log.String("rpc.write_request.error", err.Error()))
				}
				tr.LogFields(fields...)
				writeRequestDone = time.Now()
			},
		})
	}

	var (
		code  = "200" // final code to record
		first sync.Once
	)

	mu := sync.Mutex{}
	statsAgg := &zoekt.Stats{}
	nFilesMatches := 0

	err := m.Streamer.StreamSearch(ctx, q, opts, ZoektStreamFunc(func(zsr *zoekt.SearchResult) {
		first.Do(func() {
			if isLeaf {
				tr.LogFields(
					log.Int64("rpc.queue_latency_ms", writeRequestStart.Sub(start).Milliseconds()),
					log.Int64("rpc.write_duration_ms", writeRequestDone.Sub(writeRequestStart).Milliseconds()),
				)
			}
		})

		if zsr != nil {
			mu.Lock()
			statsAgg.Add(zsr.Stats)
			nFilesMatches += len(zsr.Files)
			mu.Unlock()

			c.Send(zsr)
		}
	}))
	if err != nil {
		return err
	}
	tr.LogFields(
		log.Int("filematches", nFilesMatches),

		// Zoekt stats.
		log.Int64("stats.content_bytes_loaded", statsAgg.ContentBytesLoaded),
		log.Int64("stats.index_bytes_loaded", statsAgg.IndexBytesLoaded),
		log.Int("stats.crashes", statsAgg.Crashes),
		log.Int("stats.file_count", statsAgg.FileCount),
		log.Int("stats.files_considered", statsAgg.FilesConsidered),
		log.Int("stats.files_loaded", statsAgg.FilesLoaded),
		log.Int("stats.files_skipped", statsAgg.FilesSkipped),
		log.Int("stats.match_count", statsAgg.MatchCount),
		log.Int("stats.ngram_matches", statsAgg.NgramMatches),
		log.Int("stats.shard_files_considered", statsAgg.ShardFilesConsidered),
		log.Int("stats.shards_skipped", statsAgg.ShardsSkipped),
		log.Int64("stats.wait_ms", statsAgg.Wait.Milliseconds()),
	)

	// Record total duration of stream
	requestDuration.WithLabelValues(m.hostname, cat, code).Observe(time.Since(start).Seconds())
	return nil
}

func (m *meteredSearcher) Search(ctx context.Context, q query.Q, opts *zoekt.SearchOptions) (*zoekt.SearchResult, error) {
	return AggregateStreamSearch(ctx, m.StreamSearch, q, opts)
}

func (m *meteredSearcher) List(ctx context.Context, q query.Q) (*zoekt.RepoList, error) {
	start := time.Now()

	var cat string
	var tags []trace.Tag
	if m.hostname == "" {
		cat = "ListAll"
	} else {
		cat = "List"
		tags = []trace.Tag{
			{Key: "span.kind", Value: "client"},
			{Key: "peer.address", Value: m.hostname},
			{Key: "peer.service", Value: "zoekt"},
		}
	}

	tr, ctx := trace.New(ctx, "zoekt."+cat, queryString(q), tags...)

	zsl, err := m.Streamer.List(ctx, q)

	code := "200"
	if err != nil {
		code = "error"
	}

	requestDuration.WithLabelValues(m.hostname, cat, code).Observe(time.Since(start).Seconds())

	tr.SetError(err)
	if zsl != nil {
		tr.LogFields(log.Int("repos", len(zsl.Repos)))
	}
	tr.Finish()

	return zsl, err
}

func (m *meteredSearcher) String() string {
	return "MeteredSearcher{" + m.Streamer.String() + "}"
}

func queryString(q query.Q) string {
	if q == nil {
		return "<nil>"
	}
	return q.String()
}
