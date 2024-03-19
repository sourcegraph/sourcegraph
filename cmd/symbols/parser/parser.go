package parser

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log
	"github.com/sourcegraph/go-ctags"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/log/std"

	"github.com/sourcegraph/sourcegraph/cmd/symbols/fetcher"
	"github.com/sourcegraph/sourcegraph/cmd/symbols/types"
	"github.com/sourcegraph/sourcegraph/internal/ctags_config"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/languages"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Parser interface {
	Parse(ctx context.Context, args search.SymbolsParameters, paths []string) (<-chan SymbolOrError, error)
}

type SymbolOrError struct {
	Symbol result.Symbol
	Err    error
}

type parser struct {
	parserPool         *parserPool
	repositoryFetcher  fetcher.RepositoryFetcher
	requestBufferSize  int
	numParserProcesses int
	operations         *operations
}

func NewParser(
	observationCtx *observation.Context,
	parserPool *parserPool,
	repositoryFetcher fetcher.RepositoryFetcher,
	requestBufferSize int,
	numParserProcesses int,
) Parser {
	return &parser{
		parserPool:         parserPool,
		repositoryFetcher:  repositoryFetcher,
		requestBufferSize:  requestBufferSize,
		numParserProcesses: numParserProcesses,
		operations:         newOperations(observationCtx),
	}
}

func (p *parser) Parse(ctx context.Context, args search.SymbolsParameters, paths []string) (_ <-chan SymbolOrError, err error) {
	ctx, _, endObservation := p.operations.parse.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		args.Repo.Attr(),
		args.CommitID.Attr(),
		attribute.Int("paths", len(paths)),
		attribute.StringSlice("paths", paths),
	}})
	// NOTE: We call endObservation synchronously within this function when we
	// return an error. Once we get on the success-only path, we install it to
	// run on defer of a background routine, which indicates when the returned
	// symbols channel is closed.

	parseRequestOrErrors := p.repositoryFetcher.FetchRepositoryArchive(ctx, args.Repo, args.CommitID, paths)
	if err != nil {
		endObservation(1, observation.Args{})
		return nil, errors.Wrap(err, "repositoryFetcher.FetchRepositoryArchive")
	}
	defer func() {
		if err != nil {
			go func() {
				// Drain channel on early exit
				for range parseRequestOrErrors {
				}
			}()
		}
	}()

	var (
		wg                          sync.WaitGroup                                         // concurrency control
		parseRequests               = make(chan fetcher.ParseRequest, p.requestBufferSize) // buffered requests
		symbolOrErrors              = make(chan SymbolOrError)                             // parsed responses
		totalRequests, totalSymbols uint32                                                 // stats
	)

	defer func() {
		close(parseRequests)

		go func() {
			defer func() {
				endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
					attribute.Int("numRequests", int(totalRequests)),
					attribute.Int("numSymbols", int(totalSymbols)),
				}})
			}()

			wg.Wait()
			close(symbolOrErrors)
		}()
	}()

	for range p.numParserProcesses {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for parseRequestOrError := range parseRequestOrErrors {
				if parseRequestOrError.Err != nil {
					symbolOrErrors <- SymbolOrError{Err: parseRequestOrError.Err}
					break
				}

				atomic.AddUint32(&totalRequests, 1)
				if err := p.handleParseRequest(ctx, symbolOrErrors, parseRequestOrError.ParseRequest, &totalSymbols); err != nil {
					log15.Error("error handling parse request", "error", err, "path", parseRequestOrError.ParseRequest.Path)
				}
			}
		}()
	}

	return symbolOrErrors, nil
}

func (p *parser) handleParseRequest(
	ctx context.Context,
	symbolOrErrors chan<- SymbolOrError,
	parseRequest fetcher.ParseRequest,
	totalSymbols *uint32,
) (err error) {
	ctx, trace, endObservation := p.operations.handleParseRequest.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("path", parseRequest.Path),
		attribute.Int("fileSize", len(parseRequest.Data)),
	}})
	defer endObservation(1, observation.Args{})

	language, found := languages.GetMostLikelyLanguage(parseRequest.Path, string(parseRequest.Data))
	if !found {
		return nil
	}

	source := GetParserType(language)
	if ctags_config.ParserIsNoop(source) {
		return nil
	}

	parser, err := p.parserFromPool(ctx, source)
	if err != nil {
		return err
	}
	trace.AddEvent("parser", attribute.String("event", "acquired parser from pool"))

	defer func() {
		if err == nil {
			if e := recover(); e != nil {
				err = errors.Errorf("panic: %s", e)
			}
		}

		if err == nil {
			p.parserPool.Done(parser, source)
		} else {
			// Close parser and return nil to pool, indicating that the next receiver should create a new parser
			log15.Error("Closing failed parser", "error", err)
			parser.Close()
			p.parserPool.Done(nil, source)
			p.operations.parseFailed.Inc()
		}
	}()

	p.operations.parsing.Inc()
	defer p.operations.parsing.Dec()

	entries, err := parser.Parse(parseRequest.Path, parseRequest.Data)
	if err != nil {
		return errors.Wrap(err, "parser.Parse")
	}
	trace.AddEvent("parser.Parse", attribute.Int("numEntries", len(entries)))

	lines := strings.Split(string(parseRequest.Data), "\n")

	for _, e := range entries {
		if !shouldPersistEntry(e) {
			continue
		}

		// ⚠️ Careful, ctags lines are 1-indexed!
		line := e.Line - 1
		if line < 0 || line >= len(lines) {
			log15.Warn("ctags returned an invalid line number", "path", parseRequest.Path, "line", e.Line, "len(lines)", len(lines), "symbol", e.Name)
			continue
		}

		character := strings.Index(lines[line], e.Name)
		if character == -1 {
			// Could not find the symbol in the line. ctags doesn't always return the right line.
			character = 0
		}

		symbol := result.Symbol{
			Name:        e.Name,
			Path:        e.Path,
			Line:        line,
			Character:   character,
			Kind:        e.Kind,
			Language:    e.Language,
			Parent:      e.Parent,
			ParentKind:  e.ParentKind,
			Signature:   e.Signature,
			FileLimited: e.FileLimited,
		}

		select {
		case symbolOrErrors <- SymbolOrError{Symbol: symbol}:
			atomic.AddUint32(totalSymbols, 1)

		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func (p *parser) parserFromPool(ctx context.Context, source ctags_config.ParserType) (ctags.Parser, error) {
	if ctags_config.ParserIsNoop(source) {
		return nil, errors.New("Should not pass Noop ParserType to this function")
	}

	p.operations.parseQueueSize.Inc()
	defer p.operations.parseQueueSize.Dec()

	parser, err := p.parserPool.Get(ctx, source)
	if err != nil {
		if err == context.DeadlineExceeded {
			p.operations.parseQueueTimeouts.Inc()
		}
		if err != ctx.Err() {
			err = errors.Wrap(err, "failed to create parser")
		}
	}

	return parser, err
}

func shouldPersistEntry(e *ctags.Entry) bool {
	if e.Name == "" {
		return false
	}

	for _, value := range []string{"__anon", "AnonymousFunction"} {
		if strings.HasPrefix(e.Name, value) || strings.HasPrefix(e.Parent, value) {
			return false
		}
	}

	return true
}

func SpawnCtags(logger log.Logger, ctagsConfig types.CtagsConfig, source ctags_config.ParserType) (ctags.Parser, error) {
	logger = logger.Scoped("ctags")

	var options ctags.Options
	if source == ctags_config.UniversalCtags {
		options = ctags.Options{
			Bin:                ctagsConfig.UniversalCommand,
			PatternLengthLimit: ctagsConfig.PatternLengthLimit,
		}
	} else {
		options = ctags.Options{
			Bin:                ctagsConfig.ScipCommand,
			PatternLengthLimit: ctagsConfig.PatternLengthLimit,
		}
	}
	if ctagsConfig.LogErrors {
		options.Info = std.NewLogger(logger, log.LevelInfo)
	}
	if ctagsConfig.DebugLogs {
		options.Debug = std.NewLogger(logger, log.LevelDebug)
	}

	parser, err := ctags.New(options)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create new ctags parser %q using bin path %q ", ctags_config.ParserTypeToName(source), options.Bin)
	}

	return NewFilteringParser(parser, ctagsConfig.MaxFileSize, ctagsConfig.MaxSymbols), nil
}
