package symbols

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp/syntax"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/api"

	"github.com/inconshreveable/log15"
	"github.com/jmoiron/sqlx"
	"github.com/keegancsmith/sqlf"
	"github.com/opentracing/opentracing-go/ext"
	otlog "github.com/opentracing/opentracing-go/log"
	nettrace "golang.org/x/net/trace"

	"github.com/sourcegraph/sourcegraph/cmd/symbols/internal/protocol"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"

	"github.com/sourcegraph/sourcegraph/internal/search/result"
)

// maxFileSize is the limit on file size in bytes. Only files smaller than this are processed.
const maxFileSize = 1 << 19 // 512KB

func (s *Service) handleSearch(w http.ResponseWriter, r *http.Request) {
	var args protocol.SearchArgs
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := s.search(r.Context(), args)
	if err != nil {
		if err == context.Canceled && r.Context().Err() == context.Canceled {
			return // client went away
		}
		log15.Error("Symbol search failed", "args", args, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Service) search(ctx context.Context, args protocol.SearchArgs) (*result.Symbols, error) {
	var err error
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	log15.Debug("Symbol search", "repo", args.Repo, "query", args.Query)
	span, ctx := ot.StartSpanFromContext(ctx, "search")
	span.SetTag("repo", args.Repo)
	span.SetTag("commitID", args.CommitID)
	span.SetTag("query", args.Query)
	span.SetTag("first", args.First)
	defer func() {
		if err != nil {
			ext.Error.Set(span, true)
			span.LogFields(otlog.Error(err))
		}
		span.Finish()
	}()

	tr := nettrace.New("symbols.search", fmt.Sprintf("args:%+v", args))
	defer func() {
		if err != nil {
			tr.LazyPrintf("error: %v", err)
			tr.SetError()
		}
		tr.Finish()
	}()

	dbFile, err := s.getDBFile(ctx, args)
	if err != nil {
		return nil, err
	}
	db, err := sqlx.Open("sqlite3_with_pcre", dbFile)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	result, err := filterSymbols(ctx, db, args)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// getDBFile returns the path to the sqlite3 database for the repo@commit
// specified in `args`. If the database doesn't already exist in the disk cache,
// it will create a new one and write all the symbols into it.
func (s *Service) getDBFile(ctx context.Context, args protocol.SearchArgs) (string, error) {
	diskcacheFile, err := s.cache.OpenWithPath(ctx, fmt.Sprintf("%d-%s@%s", symbolsDBVersion, args.Repo, args.CommitID), func(fetcherCtx context.Context, tempDBFile string) error {
		err := s.writeAllSymbolsToNewDB(fetcherCtx, tempDBFile, args.Repo, args.CommitID)
		if err != nil {
			if err == context.Canceled {
				log15.Error("Unable to parse repository symbols within the context", "repo", args.Repo, "commit", args.CommitID, "query", args.Query)
			}
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	defer diskcacheFile.File.Close()

	return diskcacheFile.File.Name(), err
}

// isLiteralEquality checks if the given regex matches literal strings exactly.
// Returns whether or not the regex is exact, along with the literal string if
// so.
func isLiteralEquality(expr string) (ok bool, lit string, err error) {
	r, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		return false, "", err
	}
	// Want a Concat of size 3 which is [Begin, Literal, End]
	if r.Op != syntax.OpConcat || len(r.Sub) != 3 || // size 3 concat
		!(r.Sub[0].Op == syntax.OpBeginLine || r.Sub[0].Op == syntax.OpBeginText) || // Starts with ^
		!(r.Sub[2].Op == syntax.OpEndLine || r.Sub[2].Op == syntax.OpEndText) || // Ends with $
		r.Sub[1].Op != syntax.OpLiteral { // is a literal
		return false, "", nil
	}
	return true, string(r.Sub[1].Rune), nil
}

func filterSymbols(ctx context.Context, db *sqlx.DB, args protocol.SearchArgs) (res []result.Symbol, err error) {
	span, _ := ot.StartSpanFromContext(ctx, "filterSymbols")
	defer func() {
		if err != nil {
			ext.Error.Set(span, true)
			span.LogFields(otlog.Error(err))
		}
		span.Finish()
	}()

	const maxFirst = 500
	if args.First < 0 || args.First > maxFirst {
		args.First = maxFirst
	}

	makeCondition := func(column string, regex string) []*sqlf.Query {
		conditions := []*sqlf.Query{}

		if regex == "" {
			return conditions
		}

		if isExact, symbolName, err := isLiteralEquality(regex); isExact && err == nil {
			// It looks like the user is asking for exact matches, so use `=` to
			// get the speed boost from the index on the column.
			if args.IsCaseSensitive {
				conditions = append(conditions, sqlf.Sprintf(column+" = %s", symbolName))
			} else {
				conditions = append(conditions, sqlf.Sprintf(column+"lowercase = %s", strings.ToLower(symbolName)))
			}
		} else {
			if !args.IsCaseSensitive {
				regex = "(?i:" + regex + ")"
			}
			conditions = append(conditions, sqlf.Sprintf(column+" REGEXP %s", regex))
		}

		return conditions
	}

	negateAll := func(oldConditions []*sqlf.Query) []*sqlf.Query {
		newConditions := []*sqlf.Query{}

		for _, oldCondition := range oldConditions {
			newConditions = append(newConditions, sqlf.Sprintf("NOT %s", oldCondition))
		}

		return newConditions
	}

	var conditions []*sqlf.Query
	conditions = append(conditions, makeCondition("name", args.Query)...)
	for _, includePattern := range args.IncludePatterns {
		conditions = append(conditions, makeCondition("path", includePattern)...)
	}
	conditions = append(conditions, negateAll(makeCondition("path", args.ExcludePattern))...)

	var sqlQuery *sqlf.Query
	if len(conditions) == 0 {
		sqlQuery = sqlf.Sprintf("SELECT * FROM symbols LIMIT %s", args.First)
	} else {
		sqlQuery = sqlf.Sprintf("SELECT * FROM symbols WHERE %s LIMIT %s", sqlf.Join(conditions, "AND"), args.First)
	}

	var symbolsInDB []symbolInDB
	err = db.Select(&symbolsInDB, sqlQuery.Query(sqlf.PostgresBindVar), sqlQuery.Args()...)
	if err != nil {
		return nil, err
	}

	for _, symbolInDB := range symbolsInDB {
		res = append(res, symbolInDBToSymbol(symbolInDB))
	}

	span.SetTag("hits", len(res))
	return res, nil
}

// The version of the symbols database schema. This is included in the database
// filenames to prevent a newer version of the symbols service from attempting
// to read from a database created by an older (and likely incompatible) symbols
// service. Increment this when you change the database schema.
const symbolsDBVersion = 3

// symbolInDB is the same as `protocol.Symbol`, but with two additional columns:
// namelowercase and pathlowercase, which enable indexed case insensitive
// queries.
type symbolInDB struct {
	Name          string
	NameLowercase string // derived from `Name`
	Path          string
	PathLowercase string // derived from `Path`
	Line          int
	Kind          string
	Language      string
	Parent        string
	ParentKind    string
	Signature     string
	Pattern       string

	FileLimited bool
}

func symbolToSymbolInDB(symbol result.Symbol) symbolInDB {
	return symbolInDB{
		Name:          symbol.Name,
		NameLowercase: strings.ToLower(symbol.Name),
		Path:          symbol.Path,
		PathLowercase: strings.ToLower(symbol.Path),
		Line:          symbol.Line,
		Kind:          symbol.Kind,
		Language:      symbol.Language,
		Parent:        symbol.Parent,
		ParentKind:    symbol.ParentKind,
		Signature:     symbol.Signature,
		Pattern:       symbol.Pattern,

		FileLimited: symbol.FileLimited,
	}
}

func symbolInDBToSymbol(symbolInDB symbolInDB) result.Symbol {
	return result.Symbol{
		Name:       symbolInDB.Name,
		Path:       symbolInDB.Path,
		Line:       symbolInDB.Line,
		Kind:       symbolInDB.Kind,
		Language:   symbolInDB.Language,
		Parent:     symbolInDB.Parent,
		ParentKind: symbolInDB.ParentKind,
		Signature:  symbolInDB.Signature,
		Pattern:    symbolInDB.Pattern,

		FileLimited: symbolInDB.FileLimited,
	}
}

// writeAllSymbolsToNewDB fetches the repo@commit from gitserver, parses all the
// symbols, and writes them to the blank database file `dbFile`.
func (s *Service) writeAllSymbolsToNewDB(ctx context.Context, dbFile string, repoName api.RepoName, commitID api.CommitID) error {
	db, err := sqlx.Open("sqlite3_with_pcre", dbFile)
	if err != nil {
		return err
	}
	defer db.Close()

	// Writing a bunch of rows into sqlite3 is much faster in a transaction.
	tx, err := db.Beginx()
	if err != nil {
		return err
	}

	// The column names are the lowercase version of fields in `symbolInDB`
	// because sqlx lowercases struct fields by default. See
	// http://jmoiron.github.io/sqlx/#query
	_, err = tx.Exec(
		`CREATE TABLE IF NOT EXISTS symbols (
			name VARCHAR(256) NOT NULL,
			namelowercase VARCHAR(256) NOT NULL,
			path VARCHAR(4096) NOT NULL,
			pathlowercase VARCHAR(4096) NOT NULL,
			line INT NOT NULL,
			kind VARCHAR(255) NOT NULL,
			language VARCHAR(255) NOT NULL,
			parent VARCHAR(255) NOT NULL,
			parentkind VARCHAR(255) NOT NULL,
			signature VARCHAR(255) NOT NULL,
			pattern VARCHAR(255) NOT NULL,
			filelimited BOOLEAN NOT NULL
		)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX name_index ON symbols(name);`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX path_index ON symbols(path);`)
	if err != nil {
		return err
	}

	// `*lowercase_index` enables indexed case insensitive queries.
	_, err = tx.Exec(`CREATE INDEX namelowercase_index ON symbols(namelowercase);`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX pathlowercase_index ON symbols(pathlowercase);`)
	if err != nil {
		return err
	}

	insertStatement, err := tx.PrepareNamed(
		fmt.Sprintf(
			"INSERT INTO symbols %s VALUES %s",
			"( name,  namelowercase,  path,  pathlowercase,  line,  kind,  language,  parent,  parentkind,  signature,  pattern,  filelimited)",
			"(:name, :namelowercase, :path, :pathlowercase, :line, :kind, :language, :parent, :parentkind, :signature, :pattern, :filelimited)"))
	if err != nil {
		return err
	}

	err = s.parseUncached(ctx, repoName, commitID, func(symbol result.Symbol) error {
		symbolInDBValue := symbolToSymbolInDB(symbol)
		_, err := insertStatement.Exec(&symbolInDBValue)
		return err
	})
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
