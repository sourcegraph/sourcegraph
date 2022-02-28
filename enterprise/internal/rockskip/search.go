package rockskip

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"github.com/grafana/regexp"
	"github.com/grafana/regexp/syntax"
	"github.com/keegancsmith/sqlf"
	pg "github.com/lib/pq"
	"github.com/segmentio/fasthash/fnv1"

	"github.com/sourcegraph/sourcegraph/cmd/symbols/types"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (s *Server) Search(ctx context.Context, args types.SearchArgs) (symbols []result.Symbol, err error) {
	repo := string(args.Repo)
	commitHash := string(args.CommitID)

	threadStatus := s.status.NewThreadStatus(fmt.Sprintf("searching %+v", args))
	if s.logQueries {
		defer threadStatus.Tasklog.Print()
	}
	defer threadStatus.End()

	// Acquire a read lock on the repo.
	locked, releaseRLock, err := tryRLock(ctx, s.db, threadStatus, repo)
	if err != nil {
		return nil, err
	}
	defer func() { err = combineErrors(err, releaseRLock()) }()
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

	// Finally search.
	symbols, err = s.querySymbols(ctx, args, repoId, commit, threadStatus)
	if err != nil {
		return nil, err
	}

	return symbols, nil
}

func mkIsMatch(args types.SearchArgs) (func(string) bool, error) {
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

func (s *Server) emitIndexRequest(rc repoCommit) (chan struct{}, error) {
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

func (s *Server) querySymbols(ctx context.Context, args types.SearchArgs, repoId int, commit int, threadStatus *ThreadStatus) ([]result.Symbol, error) {
	hops, err := getHops(ctx, s.db, commit, threadStatus.Tasklog)
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
		SELECT DISTINCT path
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

	paths := []string{}
	for rows.Next() {
		var path string
		err = rows.Scan(&path)
		if err != nil {
			return nil, errors.Wrap(err, "Search: Scan")
		}
		paths = append(paths, path)
	}

	stopErr := errors.New("stop iterating")

	symbols := []result.Symbol{}

	parse := s.createParser()

	threadStatus.Tasklog.Start("ArchiveEach")
	err = s.git.ArchiveEach(string(args.Repo), string(args.CommitID), paths, func(path string, contents []byte) error {
		defer threadStatus.Tasklog.Continue("ArchiveEach")

		threadStatus.Tasklog.Start("parse")
		allSymbols, err := parse(path, contents)
		if err != nil {
			return err
		}

		for _, symbol := range allSymbols {
			if isMatch(symbol.Name) {
				symbols = append(symbols, result.Symbol{
					Name:   symbol.Name,
					Path:   path,
					Line:   symbol.Line,
					Kind:   symbol.Kind,
					Parent: symbol.Parent,
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
		err = logQuery(ctx, s.db, args, q, duration, len(symbols))
		if err != nil {
			return nil, errors.Wrap(err, "logQuery")
		}
	}

	return symbols, nil
}

func logQuery(ctx context.Context, db dbutil.DB, args types.SearchArgs, q *sqlf.Query, duration time.Duration, symbols int) error {
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

func argToString(arg interface{}) (string, error) {
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

func convertSearchArgsToSqlQuery(args types.SearchArgs) *sqlf.Query {
	// TODO support non regexp queries once the frontend supports it.

	conjunctOrNils := []*sqlf.Query{}

	// Query
	conjunctOrNils = append(conjunctOrNils, regexMatch("name", "", args.Query, args.IsCaseSensitive))

	// IncludePatterns
	for _, includePattern := range args.IncludePatterns {
		conjunctOrNils = append(conjunctOrNils, regexMatch("path", "path_prefixes(path)", includePattern, args.IsCaseSensitive))
	}

	// ExcludePattern
	conjunctOrNils = append(conjunctOrNils, negate(regexMatch("path", "path_prefixes(path)", args.ExcludePattern, args.IsCaseSensitive)))

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

func regexMatch(column, columnForLiteralPrefix, regex string, isCaseSensitive bool) *sqlf.Query {
	if regex == "" || regex == "^" {
		return nil
	}

	// Exact match optimization
	if literal, ok, err := isLiteralEquality(regex); err == nil && ok && isCaseSensitive {
		return sqlf.Sprintf(fmt.Sprintf("%%s = %s", column), literal)
	}

	// Prefix match optimization
	if literal, ok, err := isLiteralPrefix(regex); err == nil && ok && isCaseSensitive && columnForLiteralPrefix != "" {
		return sqlf.Sprintf(fmt.Sprintf("%%s && %s", columnForLiteralPrefix), pg.Array([]string{literal}))
	}

	// Regex match
	operator := "~"
	if !isCaseSensitive {
		operator = "~*"
	}

	return sqlf.Sprintf(fmt.Sprintf("%s %s %%s", column, operator), regex)
}

// isLiteralEquality returns true if the given regex matches literal strings exactly.
// If so, this function returns true along with the literal search query. If not, this
// function returns false.
func isLiteralEquality(expr string) (string, bool, error) {
	regexp, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		return "", false, errors.Wrap(err, "regexp/syntax.Parse")
	}

	// want a concat of size 3 which is [begin, literal, end]
	if regexp.Op == syntax.OpConcat && len(regexp.Sub) == 3 {
		// starts with ^
		if regexp.Sub[0].Op == syntax.OpBeginLine || regexp.Sub[0].Op == syntax.OpBeginText {
			// is a literal
			if regexp.Sub[1].Op == syntax.OpLiteral {
				// ends with $
				if regexp.Sub[2].Op == syntax.OpEndLine || regexp.Sub[2].Op == syntax.OpEndText {
					return string(regexp.Sub[1].Rune), true, nil
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
	regexp, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		return "", false, errors.Wrap(err, "regexp/syntax.Parse")
	}

	// want a concat of size 2 which is [begin, literal]
	if regexp.Op == syntax.OpConcat && len(regexp.Sub) == 2 {
		// starts with ^
		if regexp.Sub[0].Op == syntax.OpBeginLine || regexp.Sub[0].Op == syntax.OpBeginText {
			// is a literal
			if regexp.Sub[1].Op == syntax.OpLiteral {
				return string(regexp.Sub[1].Rune), true, nil
			}
		}
	}

	return "", false, nil
}

func negate(query *sqlf.Query) *sqlf.Query {
	if query == nil {
		return nil
	}

	return sqlf.Sprintf("NOT %s", query)
}
