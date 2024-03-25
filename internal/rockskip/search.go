package rockskip

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"github.com/amit7itz/goset"
	"github.com/grafana/regexp"
	"github.com/grafana/regexp/syntax"
	"github.com/keegancsmith/sqlf"
	pg "github.com/lib/pq"
	"github.com/segmentio/fasthash/fnv1"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (s *Service) Search(ctx context.Context, args search.SymbolsParameters) (_ result.Symbols, err error) {
	repo := string(args.Repo)
	commitHash := string(args.CommitID)

	threadStatus := s.status.NewThreadStatus(fmt.Sprintf("searching %+v", args))
	if s.logQueries {
		defer threadStatus.Tasklog.Print()
	}
	defer threadStatus.End()

	if args.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, args.Timeout)
		defer cancel()
		defer func() {
			if !errors.Is(ctx.Err(), context.DeadlineExceeded) &&
				!errors.Is(err, context.DeadlineExceeded) {
				return
			}

			err = errors.Newf("Processing symbols is taking a while, try again later ([more details](https://sourcegraph.com/docs/code_navigation/explanations/rockskip)).")
			for _, status := range s.status.threadIdToThreadStatus {
				if strings.HasPrefix(status.Name, fmt.Sprintf("indexing %s", args.Repo)) {
					err = errors.Newf("Still processing symbols ([more details](https://sourcegraph.com/docs/code_navigation/explanations/rockskip)). Estimated completion: %s.", status.Remaining())
				}
			}
		}()
	}

	// Acquire a read lock on the repo.
	locked, releaseRLock, err := tryRLock(ctx, s.db, threadStatus, repo)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.CombineErrors(err, releaseRLock()) }()
	if !locked {
		return nil, errors.Newf("deletion in progress", repo)
	}

	// Insert or set the last_accessed_at column for this repo to now() in the rockskip_repos table.
	threadStatus.Tasklog.Start("update last_accessed_at")
	repoId, err := updateLastAccessedAt(ctx, s.db, repo)
	if err != nil {
		return nil, err
	}

	// Non-blocking send on repoUpdates to notify the background deletion goroutine.
	select {
	case s.repoUpdates <- struct{}{}:
	default:
	}

	// Check if the commit has already been indexed, and if not then index it.
	threadStatus.Tasklog.Start("check commit presence")
	commit, _, present, err := GetCommitByHash(ctx, s.db, repoId, commitHash)
	if err != nil {
		return nil, err
	} else if !present {
		// Try to send an index request.
		done, err := s.emitIndexRequest(repoCommit{repo: repo, commit: commitHash})
		if err != nil {
			return nil, err
		}

		if s.searchLastIndexedCommit {
			found := false
			threadStatus.Tasklog.Start("RevList")
			err = s.git.RevList(ctx, repo, commitHash, func(commitHash string) (shouldContinue bool, err error) {
				defer threadStatus.Tasklog.Continue("RevList")

				threadStatus.Tasklog.Start("GetCommitByHash")
				id, _, present, err := GetCommitByHash(ctx, s.db, repoId, commitHash)
				if err != nil {
					return false, err
				} else if present {
					found = true
					commit = id
					args.CommitID = api.CommitID(commitHash)
					return false, nil
				}
				return true, nil
			})
			if err != nil {
				return nil, errors.Wrap(err, "RevList")
			}
			if !found {
				return nil, context.DeadlineExceeded
			}
		} else {
			// Wait for indexing to complete or the request to be canceled.
			threadStatus.Tasklog.Start("awaiting indexing completion")
			select {
			case <-done:
				threadStatus.Tasklog.Start("recheck commit presence")
				commit, _, present, err = GetCommitByHash(ctx, s.db, repoId, commitHash)
				if err != nil {
					return nil, err
				}
				if !present {
					return nil, errors.Newf("indexing failed, check server logs")
				}
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	// Finally search.
	symbols, err := s.querySymbols(ctx, args, repoId, commit, threadStatus)
	if err != nil {
		return nil, errors.Wrap(err, "querySymbols")
	}

	return symbols, nil
}

func mkIsMatch(args search.SymbolsParameters) (func(string) bool, error) {
	if !args.IsRegExp {
		if args.IsCaseSensitive {
			return func(symbol string) bool { return strings.Contains(symbol, args.Query) }, nil
		} else {
			return func(symbol string) bool {
				return strings.Contains(strings.ToLower(symbol), strings.ToLower(args.Query))
			}, nil
		}
	}

	expr := args.Query
	if !args.IsCaseSensitive {
		expr = "(?i)" + expr
	}

	regex, err := regexp.Compile(expr)
	if err != nil {
		return nil, err
	}

	if args.IsCaseSensitive {
		return func(symbol string) bool { return regex.MatchString(symbol) }, nil
	} else {
		return func(symbol string) bool { return regex.MatchString(strings.ToLower(symbol)) }, nil
	}
}

func (s *Service) emitIndexRequest(rc repoCommit) (chan struct{}, error) {
	key := fmt.Sprintf("%s@%s", rc.repo, rc.commit)

	s.repoCommitToDoneMu.Lock()

	if done, ok := s.repoCommitToDone[key]; ok {
		s.repoCommitToDoneMu.Unlock()
		return done, nil
	}

	done := make(chan struct{})

	s.repoCommitToDone[key] = done
	s.repoCommitToDoneMu.Unlock()
	go func() {
		<-done
		s.repoCommitToDoneMu.Lock()
		delete(s.repoCommitToDone, key)
		s.repoCommitToDoneMu.Unlock()
	}()

	request := indexRequest{
		repoCommit: repoCommit{
			repo:   rc.repo,
			commit: rc.commit,
		},
		done: done}

	// Route the index request to the indexer associated with the repo.
	ix := int(fnv1.HashString32(rc.repo)) % len(s.indexRequestQueues)

	select {
	case s.indexRequestQueues[ix] <- request:
	default:
		return nil, errors.Newf("the indexing queue is full")
	}

	return done, nil
}

const DEFAULT_LIMIT = 100

func (s *Service) querySymbols(ctx context.Context, args search.SymbolsParameters, repoId int, commit int, threadStatus *ThreadStatus) (result.Symbols, error) {
	db := database.NewDB(s.logger, s.db)
	hops, err := getHops(ctx, db, commit, threadStatus.Tasklog)
	if err != nil {
		return nil, err
	}
	// Drop the null commit.
	hops = hops[:len(hops)-1]

	limit := DEFAULT_LIMIT
	if args.First > 0 {
		limit = args.First
	}

	threadStatus.Tasklog.Start("run query")
	q := sqlf.Sprintf(`
		SELECT path
		FROM rockskip_symbols
		WHERE
			%s && singleton_integer(repo_id)
			AND     %s && added
			AND NOT %s && deleted
			AND %s
		LIMIT %s;`,
		pg.Array([]int{repoId}),
		pg.Array(hops),
		pg.Array(hops),
		convertSearchArgsToSqlQuery(args),
		limit,
	)

	start := time.Now()
	var rows *sql.Rows
	rows, err = s.db.QueryContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
	duration := time.Since(start)
	if err != nil {
		return nil, errors.Wrap(err, "Search")
	}
	defer rows.Close()

	isMatch, err := mkIsMatch(args)
	if err != nil {
		return nil, err
	}

	paths := goset.NewSet[string]()
	for rows.Next() {
		var path string
		err = rows.Scan(&path)
		if err != nil {
			return nil, errors.Wrap(err, "Search: Scan")
		}
		paths.Add(path)
	}

	stopErr := errors.New("stop iterating")

	symbols := []result.Symbol{}

	parser, err := s.createParser()
	if err != nil {
		return nil, errors.Wrap(err, "create parser")
	}
	defer parser.Close()

	threadStatus.Tasklog.Start("ArchiveEach")
	err = archiveEach(ctx, s.fetcher, string(args.Repo), string(args.CommitID), paths.Items(), func(path string, contents []byte) error {
		defer threadStatus.Tasklog.Continue("ArchiveEach")

		threadStatus.Tasklog.Start("parse")
		allSymbols, err := parser.Parse(path, contents)
		if err != nil {
			return err
		}

		lines := strings.Split(string(contents), "\n")

		for _, symbol := range allSymbols {
			if isMatch(symbol.Name) {
				if symbol.Line < 1 || symbol.Line > len(lines) {
					s.logger.Warn("ctags returned an invalid line number",
						log.String("path", path),
						log.Int("line", symbol.Line),
						log.Int("len(lines)", len(lines)),
						log.String("symbol", symbol.Name),
					)
					continue
				}

				character := strings.Index(lines[symbol.Line-1], symbol.Name)
				if character == -1 {
					// Could not find the symbol in the line. ctags doesn't always return the right line.
					character = 0
				}

				symbols = append(symbols, result.Symbol{
					Name:      symbol.Name,
					Path:      path,
					Line:      symbol.Line - 1,
					Character: character,
					Kind:      symbol.Kind,
					Parent:    symbol.Parent,
				})

				if len(symbols) >= limit {
					return stopErr
				}
			}
		}

		return nil
	})

	if err != nil && err != stopErr {
		return nil, err
	}

	if s.logQueries {
		err = logQuery(ctx, db, args, q, duration, len(symbols))
		if err != nil {
			return nil, errors.Wrap(err, "logQuery")
		}
	}

	return symbols, nil
}

func logQuery(ctx context.Context, db database.DB, args search.SymbolsParameters, q *sqlf.Query, duration time.Duration, symbols int) error {
	sb := &strings.Builder{}

	fmt.Fprintf(sb, "Search args: %+v\n", args)

	fmt.Fprintln(sb, "Query:")
	query, err := sqlfToString(q)
	if err != nil {
		return errors.Wrap(err, "sqlfToString")
	}
	fmt.Fprintln(sb, query)

	fmt.Fprintln(sb, "EXPLAIN:")
	explain, err := db.QueryContext(ctx, sqlf.Sprintf("EXPLAIN %s", q).Query(sqlf.PostgresBindVar), q.Args()...)
	if err != nil {
		return errors.Wrap(err, "EXPLAIN")
	}
	defer explain.Close()
	for explain.Next() {
		var plan string
		err = explain.Scan(&plan)
		if err != nil {
			return errors.Wrap(err, "EXPLAIN Scan")
		}
		fmt.Fprintln(sb, plan)
	}

	fmt.Fprintf(sb, "%.2fms, %d symbols", float64(duration.Microseconds())/1000, symbols)

	fmt.Println(" ")
	fmt.Println(bracket(sb.String()))
	fmt.Println(" ")

	return nil
}

func bracket(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i, line := range lines {
		if i == 0 {
			lines[i] = "┌ " + line
		} else if i == len(lines)-1 {
			lines[i] = "└ " + line
		} else {
			lines[i] = "│ " + line
		}
	}
	return strings.Join(lines, "\n")
}

func sqlfToString(q *sqlf.Query) (string, error) {
	s := q.Query(sqlf.PostgresBindVar)
	for i, arg := range q.Args() {
		argString, err := argToString(arg)
		if err != nil {
			return "", err
		}
		s = strings.ReplaceAll(s, fmt.Sprintf("$%d", i+1), argString)
	}
	return s, nil
}

func argToString(arg any) (string, error) {
	switch arg := arg.(type) {
	case string:
		return fmt.Sprintf("'%s'", sqlEscapeQuotes(arg)), nil
	case driver.Valuer:
		value, err := arg.Value()
		if err != nil {
			return "", err
		}
		switch value := value.(type) {
		case string:
			return fmt.Sprintf("'%s'", sqlEscapeQuotes(value)), nil
		case int:
			return fmt.Sprintf("'%d'", value), nil
		default:
			return "", errors.Newf("unrecognized array type %T", value)
		}
	case int:
		return fmt.Sprintf("%d", arg), nil
	default:
		return "", errors.Newf("unrecognized type %T", arg)
	}
}

func sqlEscapeQuotes(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func convertSearchArgsToSqlQuery(args search.SymbolsParameters) *sqlf.Query {
	// TODO support non regexp queries once the frontend supports it.

	conjunctOrNils := []*sqlf.Query{}

	// Query
	conjunctOrNils = append(conjunctOrNils, regexMatch(nameConditions, args.Query, args.IsCaseSensitive))

	// IncludePaths
	for _, includePattern := range args.IncludePatterns {
		conjunctOrNils = append(conjunctOrNils, regexMatch(pathConditions, includePattern, args.IsCaseSensitive))
	}

	// ExcludePaths
	conjunctOrNils = append(conjunctOrNils, negate(regexMatch(pathConditions, args.ExcludePattern, args.IsCaseSensitive)))

	// Rockskip doesn't store the file's language, so we convert the language filters into path filters as a
	// best effort approximation. We ignore the search's case-sensitivity, since it doesn't apply to these filters.
	for _, includeLang := range args.IncludeLangs {
		conjunctOrNils = append(conjunctOrNils, regexMatch(pathConditions, query.LangToFileRegexp(includeLang), false))
	}

	for _, excludeLang := range args.ExcludeLangs {
		conjunctOrNils = append(conjunctOrNils, negate(regexMatch(pathConditions, query.LangToFileRegexp(excludeLang), false)))
	}

	// Drop nils
	conjuncts := []*sqlf.Query{}
	for _, condition := range conjunctOrNils {
		if condition != nil {
			conjuncts = append(conjuncts, condition)
		}
	}

	if len(conjuncts) == 0 {
		return sqlf.Sprintf("TRUE")
	}

	return sqlf.Join(conjuncts, "AND")
}

// Conditions specifies how to construct query clauses depending on the regex kind.
type Conditions struct {
	regex    QueryFunc
	regexI   QueryFunc
	exact    QueryFunc
	exactI   QueryFunc
	prefix   QueryFunc
	prefixI  QueryFunc
	fileExt  QueryNFunc
	fileExtI QueryNFunc
}

// Returns a SQL query for the given value.
type QueryFunc func(value string) *sqlf.Query

// Returns a SQL query for the given values.
type QueryNFunc func(values []string) *sqlf.Query

var nameConditions = Conditions{
	regex:  func(v string) *sqlf.Query { return sqlf.Sprintf("name ~ %s", v) },
	regexI: func(v string) *sqlf.Query { return sqlf.Sprintf("name ~* %s", v) },
	exact:  func(v string) *sqlf.Query { return sqlf.Sprintf("ARRAY[%s] && singleton(name)", v) },
	exactI: func(v string) *sqlf.Query {
		return sqlf.Sprintf("ARRAY[%s] && singleton(lower(name))", strings.ToLower(v))
	},
	prefix:   nil,
	prefixI:  nil,
	fileExt:  nil,
	fileExtI: nil,
}

var pathConditions = Conditions{
	regex:  func(v string) *sqlf.Query { return sqlf.Sprintf("path ~ %s", v) },
	regexI: func(v string) *sqlf.Query { return sqlf.Sprintf("path ~* %s", v) },
	exact:  func(v string) *sqlf.Query { return sqlf.Sprintf("ARRAY[%s] && singleton(path)", v) },
	exactI: func(v string) *sqlf.Query {
		return sqlf.Sprintf("ARRAY[%s] && singleton(lower(path))", strings.ToLower(v))
	},
	prefix: func(v string) *sqlf.Query { return sqlf.Sprintf("ARRAY[%s] && path_prefixes(path)", v) },
	prefixI: func(v string) *sqlf.Query {
		return sqlf.Sprintf("ARRAY[%s] && path_prefixes(lower(path))", strings.ToLower(v))
	},
	fileExt: func(vs []string) *sqlf.Query {
		return sqlf.Sprintf("%s && singleton(get_file_extension(path))", pg.Array(vs))
	},
	fileExtI: func(vs []string) *sqlf.Query {
		return sqlf.Sprintf("%s && singleton(get_file_extension(lower(path)))", pg.Array(lowerAll(vs)))
	},
}

func lowerAll(strs []string) []string {
	lowers := []string{}
	for _, s := range strs {
		lowers = append(lowers, strings.ToLower(s))
	}
	return lowers
}

func regexMatch(conditions Conditions, regex string, isCaseSensitive bool) *sqlf.Query {
	if regex == "" || regex == "^" {
		return nil
	}

	// Exact match optimization
	if literal, ok, err := isLiteralEquality(regex); err == nil && ok {
		if isCaseSensitive && conditions.exact != nil {
			return conditions.exact(literal)
		}
		if !isCaseSensitive && conditions.exactI != nil {
			return conditions.exactI(literal)
		}
	}

	// Prefix match optimization
	if literal, ok, err := isLiteralPrefix(regex); err == nil && ok {
		if isCaseSensitive && conditions.prefix != nil {
			return conditions.prefix(literal)
		}
		if !isCaseSensitive && conditions.prefixI != nil {
			return conditions.prefixI(literal)
		}
	}

	// File extension match optimization
	if exts := isFileExtensionMatch(regex); exts != nil {
		if isCaseSensitive && conditions.fileExt != nil {
			return conditions.fileExt(exts)
		}
		if !isCaseSensitive && conditions.fileExtI != nil {
			return conditions.fileExtI(exts)
		}
	}

	// Regex match
	if isCaseSensitive && conditions.regex != nil {
		return conditions.regex(regex)
	}
	if !isCaseSensitive && conditions.regexI != nil {
		return conditions.regexI(regex)
	}

	return nil
}

// isLiteralEquality returns true if the given regex matches literal strings exactly.
// If so, this function returns true along with the literal search query. If not, this
// function returns false.
func isLiteralEquality(expr string) (string, bool, error) {
	regex, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		return "", false, errors.Wrap(err, "regexp/syntax.Parse")
	}

	// want a concat of size 3 which is [begin, literal, end]
	if regex.Op == syntax.OpConcat && len(regex.Sub) == 3 {
		// starts with ^
		if regex.Sub[0].Op == syntax.OpBeginLine || regex.Sub[0].Op == syntax.OpBeginText {
			// is a literal
			if regex.Sub[1].Op == syntax.OpLiteral {
				// ends with $
				if regex.Sub[2].Op == syntax.OpEndLine || regex.Sub[2].Op == syntax.OpEndText {
					return string(regex.Sub[1].Rune), true, nil
				}
			}
		}
	}

	return "", false, nil
}

// isLiteralPrefix returns true if the given regex matches literal strings by prefix.
// If so, this function returns true along with the literal search query. If not, this
// function returns false.
func isLiteralPrefix(expr string) (string, bool, error) {
	regex, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		return "", false, errors.Wrap(err, "regexp/syntax.Parse")
	}

	// want a concat of size 2 which is [begin, literal]
	if regex.Op == syntax.OpConcat && len(regex.Sub) == 2 {
		// starts with ^
		if regex.Sub[0].Op == syntax.OpBeginLine || regex.Sub[0].Op == syntax.OpBeginText {
			// is a literal
			if regex.Sub[1].Op == syntax.OpLiteral {
				return string(regex.Sub[1].Rune), true, nil
			}
		}
	}

	return "", false, nil
}

// isFileExtensionMatch returns true if the given regex matches file extensions. If so, this function
// returns true along with the extensions. If not, this function returns false.
func isFileExtensionMatch(expr string) []string {
	if !strings.HasPrefix(expr, `\.(`) {
		return nil
	}

	expr = strings.TrimPrefix(expr, `\.(`)

	if !strings.HasSuffix(expr, `)$`) {
		return nil
	}

	expr = strings.TrimSuffix(expr, `)$`)

	exts := strings.Split(expr, `|`)

	return exts
}

func negate(query *sqlf.Query) *sqlf.Query {
	if query == nil {
		return nil
	}

	return sqlf.Sprintf("NOT %s", query)
}
