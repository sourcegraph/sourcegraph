package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/grafana/regexp"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/api/internalapi"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var MockCountGoImporters func(ctx context.Context, repo api.RepoName) (int, error)

var (
	goImportersCountCache = rcache.NewWithTTL("go-importers-count", 14400, false) // 4 hours
)

// CountGoImporters returns the number of Go importers for the repository's Go subpackages. This is
// a special case used only on Sourcegraph.com for repository badges.
func CountGoImporters(ctx context.Context, cli httpcli.Doer, repo api.RepoName) (count int, err error) {
	if MockCountGoImporters != nil {
		return MockCountGoImporters(ctx, repo)
	}

	if !dotcom.SourcegraphDotComMode() {
		// Avoid confusing users by exposing this on self-hosted instances, because it relies on the
		// public godoc.org API.
		return 0, errors.New("counting Go importers is not supported on self-hosted instances")
	}

	cacheKey := string(repo)
	b, ok := goImportersCountCache.Get(cacheKey)
	if ok {
		count, err = strconv.Atoi(string(b))
		if err == nil {
			return count, nil // cache hit
		}
		goImportersCountCache.Delete(cacheKey) // remove unexpectedly invalid cache value
	}

	defer func() {
		if err == nil {
			// Store in cache.
			goImportersCountCache.Set(cacheKey, []byte(strconv.Itoa(count)))
		}
	}()

	var q struct {
		Query     string
		Variables map[string]any
	}

	q.Query = countGoImportersGraphQLQuery
	q.Variables = map[string]any{
		"query": countGoImportersSearchQuery(repo),
	}

	body, err := json.Marshal(q)
	if err != nil {
		return 0, err
	}

	rawurl, err := gqlURL("CountGoImporters")
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", rawurl, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}

	resp, err := cli.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, errors.Wrap(err, "ReadBody")
	}

	var v struct {
		Data struct {
			Search struct{ Results struct{ MatchCount int } }
		}
		Errors []any
	}

	if err := json.Unmarshal(respBody, &v); err != nil {
		return 0, errors.Wrap(err, "Decode")
	}

	if len(v.Errors) > 0 {
		return 0, errors.Errorf("graphql: errors: %v", v.Errors)
	}

	return v.Data.Search.Results.MatchCount, nil
}

// gqlURL returns the frontend's internal GraphQL API URL, with the given ?queryName parameter
// which is used to keep track of the source and type of GraphQL queries.
func gqlURL(queryName string) (string, error) {
	u, err := url.Parse(internalapi.Client.URL)
	if err != nil {
		return "", err
	}
	u.Path = "/.internal/graphql"
	u.RawQuery = queryName
	return u.String(), nil
}

func countGoImportersSearchQuery(repo api.RepoName) string {
	//
	// Walk-through of the regular expression:
	// - ^\s* to not match the repo inside replace blocks which have a $repo => $replacement $version format.
	// - (/\S+)? to match sub-packages or packages at different versions (e.g. github.com/tsenart/vegeta/v12)
	// - \s+ to match spaces between repo name and version identifier
	// - v\d to match beginning of version identifier
	//
	// See: https://sourcegraph.com/search?q=context:global+type:file+f:%28%5E%7C/%29go%5C.mod%24+content:%5E%5Cs*github%5C.com/tsenart/vegeta%28/%5CS%2B%29%3F%5Cs%2Bv%5Cd+visibility:public+count:all&patternType=regexp
	return strings.Join([]string{
		`type:file`,
		`f:(^|/)go\.mod$`,
		`patterntype:regexp`,
		`content:^\s*` + regexp.QuoteMeta(string(repo)) + `(/\S+)?\s+v\d`,
		`count:all`,
		`visibility:public`,
		`timeout:20s`,
	}, " ")
}

const countGoImportersGraphQLQuery = `
query CountGoImporters($query: String!) {
  search(query: $query) { results { matchCount } }
}`
