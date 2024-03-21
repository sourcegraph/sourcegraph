package api

import (
	"context"
	"time"

	"github.com/dustin/go-humanize"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/sourcegraph/sourcegraph/lib/errors"

	"github.com/sourcegraph/sourcegraph/cmd/symbols/internal/api/observability"
	"github.com/sourcegraph/sourcegraph/cmd/symbols/internal/database/store"
	"github.com/sourcegraph/sourcegraph/cmd/symbols/internal/database/writer"
	sharedobservability "github.com/sourcegraph/sourcegraph/cmd/symbols/observability"
	"github.com/sourcegraph/sourcegraph/cmd/symbols/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
)

const searchTimeout = 60 * time.Second

type limitHitError struct {
	description string
}

func (e *limitHitError) Error() string { return e.description }

func MakeSqliteSearchFunc(observationCtx *observation.Context, cachedDatabaseWriter writer.CachedDatabaseWriter, db database.DB) types.SearchFunc {
	operations := sharedobservability.NewOperations(observationCtx)

	return func(ctx context.Context, args search.SymbolsParameters) (results []result.Symbol, err error) {
		ctx, trace, endObservation := operations.Search.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
			args.Repo.Attr(),
			args.CommitID.Attr(),
			attribute.String("query", args.Query),
			attribute.Bool("isRegExp", args.IsRegExp),
			attribute.Bool("isCaseSensitive", args.IsCaseSensitive),
			attribute.Int("numIncludePatterns", len(args.IncludePatterns)),
			attribute.StringSlice("includePatterns", args.IncludePatterns),
			attribute.String("excludePattern", args.ExcludePattern),
			attribute.StringSlice("includeLangs", args.IncludeLangs),
			attribute.StringSlice("excludeLangs", args.ExcludeLangs),
			attribute.Int("first", args.First),
			attribute.Float64("timeoutSeconds", args.Timeout.Seconds()),
		}})
		defer func() {
			endObservation(1, observation.Args{
				MetricLabelValues: []string{observability.GetParseAmount(ctx)},
				Attrs:             []attribute.KeyValue{attribute.String("parseAmount", observability.GetParseAmount(ctx))},
			})
		}()
		ctx = observability.SeedParseAmount(ctx)

		timeout := searchTimeout
		if args.Timeout > 0 && args.Timeout < timeout {
			timeout = args.Timeout
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		defer func() {
			if ctx.Err() == nil || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			info, err2 := db.GitserverRepos().GetByName(ctx, args.Repo)
			if err2 != nil {
				err = errors.New("Processing symbols using the SQLite backend is taking a while. If this repository is ~1GB+, enable [Rockskip](https://sourcegraph.com/docs/code_navigation/explanations/rockskip).")
				return
			}
			size := info.RepoSizeBytes

			help := ""
			if size > 1_000_000_000 {
				help = "Enable [Rockskip](https://sourcegraph.com/docs/code_navigation/explanations/rockskip)."
			} else if size > 100_000_000 {
				help = "If this persists, enable [Rockskip](https://sourcegraph.com/docs/code_navigation/explanations/rockskip)."
			} else {
				help = "If this persists, make sure the symbols service has an SSD, a few GHz of CPU, and a few GB of RAM."
			}

			err = errors.Newf("Processing symbols using the SQLite backend is taking a while on this %s repository. %s", humanize.Bytes(uint64(size)), help)
		}()

		dbFile, err := cachedDatabaseWriter.GetOrCreateDatabaseFile(ctx, args)
		if err != nil {
			return nil, errors.Wrap(err, "databaseWriter.GetOrCreateDatabaseFile")
		}
		trace.AddEvent("databaseWriter", attribute.String("dbFile", dbFile))

		var res result.Symbols
		err = store.WithSQLiteStore(observationCtx, dbFile, func(db store.Store) (err error) {
			var limitHit bool
			if res, limitHit, err = db.Search(ctx, args); err != nil {
				return errors.Wrap(err, "store.Search")
			}
			if limitHit {
				p := message.NewPrinter(language.English)
				return &limitHitError{description: p.Sprintf("unindexed symbol search out of bounds. Expected args.First to be within [0, %d], got %d", store.MaxSymbolLimit, args.First)}
			}

			return nil
		})

		return res, err
	}
}
