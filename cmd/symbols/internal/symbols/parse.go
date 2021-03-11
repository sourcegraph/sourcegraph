package symbols

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/opentracing/opentracing-go/ext"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	nettrace "golang.org/x/net/trace"

	ctags "github.com/sourcegraph/go-ctags"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
)

// startParsers starts the parser process pool.
func (s *Service) startParsers() error {
	n := s.NumParserProcesses
	if n == 0 {
		n = runtime.GOMAXPROCS(0)
	}

	s.parsers = make(chan ctags.Parser, n)
	for i := 0; i < n; i++ {
		parser, err := s.NewParser()
		if err != nil {
			return errors.Wrap(err, "NewParser")
		}
		s.parsers <- parser
	}
	return nil
}

func (s *Service) parseUncached(ctx context.Context, repo api.RepoName, commitID api.CommitID, callback func(symbol result.Symbol) error) (err error) {
	span, ctx := ot.StartSpanFromContext(ctx, "parseUncached")
	defer func() {
		if err != nil {
			ext.Error.Set(span, true)
			span.LogFields(otlog.Error(err))
		}
		span.Finish()
	}()
	span.SetTag("repo", string(repo))
	span.SetTag("commit", string(commitID))

	tr := nettrace.New("parseUncached", string(repo))
	tr.LazyPrintf("commitID: %s", commitID)

	totalSymbols := 0
	defer func() {
		tr.LazyPrintf("symbols=%d", totalSymbols)
		if err != nil {
			tr.LazyPrintf("error: %s", err)
			tr.SetError()
		}
		tr.Finish()
	}()

	tr.LazyPrintf("fetch")
	parseRequests, errChan, err := s.fetchRepositoryArchive(ctx, repo, commitID)
	tr.LazyPrintf("fetch (returned chans)")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		mu  sync.Mutex // protects symbols and err
		wg  sync.WaitGroup
		sem = make(chan struct{}, runtime.GOMAXPROCS(0))
	)
	tr.LazyPrintf("parse")
	totalParseRequests := 0
	for req := range parseRequests {
		totalParseRequests++
		if ctx.Err() != nil {
			// Drain parseRequests
			go func() {
				for range parseRequests {
				}
			}()
			return ctx.Err()
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(req parseRequest) {
			defer func() {
				wg.Done()
				<-sem
			}()
			entries, parseErr := s.parse(ctx, req)
			if parseErr != nil && parseErr != context.Canceled && parseErr != context.DeadlineExceeded {
				log15.Error("Error parsing symbols.", "repo", repo, "commitID", commitID, "path", req.path, "dataSize", len(req.data), "error", parseErr)
			}
			if len(entries) > 0 {
				mu.Lock()
				defer mu.Unlock()
				for _, e := range entries {
					if e.Name == "" || strings.HasPrefix(e.Name, "__anon") || strings.HasPrefix(e.Parent, "__anon") || strings.HasPrefix(e.Name, "AnonymousFunction") || strings.HasPrefix(e.Parent, "AnonymousFunction") {
						continue
					}
					totalSymbols++
					err = callback(entryToSymbol(e))
					if err != nil {
						log15.Error("Failed to add symbol", "symbol", e, "error", err)
						return
					}
				}
			}
		}(req)
	}
	wg.Wait()
	tr.LazyPrintf("parse (done) totalParseRequests=%d symbols=%d", totalParseRequests, totalSymbols)

	return <-errChan
}

// parse gets a parser from the pool and uses it to satisfy the parse request.
func (s *Service) parse(ctx context.Context, req parseRequest) (entries []*ctags.Entry, err error) {
	parseQueueSize.Inc()

	select {
	case <-ctx.Done():
		parseQueueSize.Dec()
		if ctx.Err() == context.DeadlineExceeded {
			parseQueueTimeouts.Inc()
		}
		return nil, ctx.Err()
	case parser, ok := <-s.parsers:
		parseQueueSize.Dec()

		if !ok {
			return nil, nil
		}

		if parser == nil {
			// The parser failed for some previous receiver (who returned a nil parser to the channel). Try
			// creating a parser.
			var err error
			parser, err = s.NewParser()
			if err != nil {
				return nil, err
			}
		}

		defer func() {
			if err == nil {
				if e := recover(); e != nil {
					err = fmt.Errorf("panic: %s", e)
				}
			}
			if err == nil {
				// Return parser to pool.
				s.parsers <- parser
			} else {
				// Close parser and return nil to pool, indicating that the next receiver should create a new
				// parser.
				log15.Error("Closing failed parser and creating a new one.", "path", req.path, "error", err)
				parseFailed.Inc()
				parser.Close()
				s.parsers <- nil
			}
		}()
		parsing.Inc()
		defer parsing.Dec()
		return parser.Parse(req.path, req.data)
	}
}

func entryToSymbol(e *ctags.Entry) result.Symbol {
	return result.Symbol{
		Name:        e.Name,
		Path:        e.Path,
		Line:        e.Line,
		Kind:        e.Kind,
		Language:    e.Language,
		Parent:      e.Parent,
		ParentKind:  e.ParentKind,
		Signature:   e.Signature,
		Pattern:     e.Pattern,
		FileLimited: e.FileLimited,
	}
}

var (
	parsing = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "symbols_parse_parsing",
		Help: "The number of parse jobs currently running.",
	})
	parseQueueSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "symbols_parse_parse_queue_size",
		Help: "The number of parse jobs enqueued.",
	})
	parseQueueTimeouts = promauto.NewCounter(prometheus.CounterOpts{
		Name: "symbols_parse_parse_queue_timeouts",
		Help: "The total number of parse jobs that timed out while enqueued.",
	})
	parseFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "symbols_parse_parse_failed",
		Help: "The total number of parse jobs that failed.",
	})
)
