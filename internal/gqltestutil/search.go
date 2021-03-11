package gqltestutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/pkg/errors"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming/api"
	streamhttp "github.com/sourcegraph/sourcegraph/internal/search/streaming/http"
)

type SearchRepositoryResult struct {
	Name string `json:"name"`
}

type SearchRepositoryResults []*SearchRepositoryResult

// Exists returns the list of missing repositories from given names that do not exist
// in search results. If all of given names are found, it returns empty list.
func (rs SearchRepositoryResults) Exists(names ...string) []string {
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}
	for _, r := range rs {
		delete(set, r.Name)
	}

	missing := make([]string, 0, len(set))
	for name := range set {
		missing = append(missing, name)
	}
	return missing
}

func (rs SearchRepositoryResults) String() string {
	var names []string
	for _, r := range rs {
		names = append(names, r.Name)
	}
	sort.Strings(names)
	return fmt.Sprintf("%q", names)
}

// SearchRepositories search repositories with given query.
func (c *Client) SearchRepositories(query string) (SearchRepositoryResults, error) {
	const gqlQuery = `
query Search($query: String!) {
	search(query: $query, version: V2) {
		results {
			results {
				... on Repository {
					name
				}
			}
		}
	}
}
`
	variables := map[string]interface{}{
		"query": query,
	}
	var resp struct {
		Data struct {
			Search struct {
				Results struct {
					Results []*SearchRepositoryResult `json:"results"`
				} `json:"results"`
			} `json:"search"`
		} `json:"data"`
	}
	err := c.GraphQL("", "", gqlQuery, variables, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "request GraphQL")
	}

	return resp.Data.Search.Results.Results, nil
}

type SearchFileResults struct {
	MatchCount int64               `json:"matchCount"`
	Alert      *SearchAlert        `json:"alert"`
	Results    []*SearchFileResult `json:"results"`
}

type SearchFileResult struct {
	File struct {
		Name string `json:"name"`
	} `json:"file"`
	Repository struct {
		Name string `json:"name"`
	} `json:"repository"`
	RevSpec struct {
		Expr string `json:"expr"`
	} `json:"revSpec"`
}

type ProposedQuery struct {
	Description string `json:"description"`
	Query       string `json:"query"`
}

// SearchAlert is an alert specific to searches (i.e. not site alert).
type SearchAlert struct {
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	ProposedQueries []ProposedQuery `json:"proposedQueries"`
}

// SearchFiles searches files with given query. It returns the match count and
// corresponding file matches. Search alert is also included if any.
func (c *Client) SearchFiles(query string) (*SearchFileResults, error) {
	const gqlQuery = `
query Search($query: String!) {
	search(query: $query, version: V2) {
		results {
			matchCount
			alert {
				title
				description
				proposedQueries {
					description
					query
				}
			}
			results {
				... on FileMatch {
					file {
						name
					}
					symbols {
						name
						containerName
						kind
						language
						url
					}
					repository {
						name
					}
					revSpec {
						... on GitRevSpecExpr {
							expr
						}
					}
				}
			}
		}
	}
}
`
	variables := map[string]interface{}{
		"query": query,
	}
	var resp struct {
		Data struct {
			Search struct {
				Results struct {
					*SearchFileResults
				} `json:"results"`
			} `json:"search"`
		} `json:"data"`
	}
	err := c.GraphQL("", "", gqlQuery, variables, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "request GraphQL")
	}

	return resp.Data.Search.Results.SearchFileResults, nil
}

type SearchCommitResults struct {
	MatchCount int64 `json:"matchCount"`
	Results    []*struct {
		URL string `json:"url"`
	} `json:"results"`
}

// SearchCommits searches commits with given query. It returns the match count and
// corresponding file matches.
func (c *Client) SearchCommits(query string) (*SearchCommitResults, error) {
	const gqlQuery = `
query Search($query: String!) {
	search(query: $query, version: V2) {
		results {
			matchCount
			results {
				... on CommitSearchResult {
					url
				}
			}
		}
	}
}
`
	variables := map[string]interface{}{
		"query": query,
	}
	var resp struct {
		Data struct {
			Search struct {
				Results struct {
					*SearchCommitResults
				} `json:"results"`
			} `json:"search"`
		} `json:"data"`
	}
	err := c.GraphQL("", "", gqlQuery, variables, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "request GraphQL")
	}

	return resp.Data.Search.Results.SearchCommitResults, nil
}

type AnyResult struct {
	Inner interface{}
}

func (r *AnyResult) UnmarshalJSON(b []byte) error {
	var typeUnmarshaller struct {
		TypeName string `json:"__typename"`
	}

	if err := json.Unmarshal(b, &typeUnmarshaller); err != nil {
		return err
	}

	switch typeUnmarshaller.TypeName {
	case "FileMatch":
		var f FileResult
		if err := json.Unmarshal(b, &f); err != nil {
			return err
		}
		r.Inner = f
	case "CommitSearchResult":
		var c CommitResult
		if err := json.Unmarshal(b, &c); err != nil {
			return err
		}
		r.Inner = c
	case "Repository":
		var rr RepositoryResult
		if err := json.Unmarshal(b, &rr); err != nil {
			return err
		}
		r.Inner = rr
	default:
		return fmt.Errorf("Unknown type %s", typeUnmarshaller.TypeName)
	}
	return nil
}

type FileResult struct {
	File struct {
		Path string
	} `json:"file"`
	Repository  RepositoryResult
	LineMatches []struct {
		OffsetAndLengths [][2]int32 `json:"offsetAndLengths"`
	} `json:"lineMatches"`
	Symbols []interface{} `json:"symbols"`
}

type CommitResult struct {
	URL string
}

type RepositoryResult struct {
	Name string
}

// SearchAll searches for all matches with a given query
// corresponding file matches.
func (c *Client) SearchAll(query string) ([]*AnyResult, error) {
	const gqlQuery = `
query Search($query: String!) {
	search(query: $query, version: V2) {
		results {
			results {
				__typename
				... on CommitSearchResult {
					url
				}
				... on FileMatch {
					file {
						path
					}
					repository {
						name
					}
					lineMatches {
						offsetAndLengths
					}
					symbols {
						name
					}
				}
				... on Repository {
					name
				}
			}
		}
	}
}
`
	variables := map[string]interface{}{
		"query": query,
	}
	var resp struct {
		Data struct {
			Search struct {
				Results struct {
					Results []*AnyResult `json:"results"`
				} `json:"results"`
			} `json:"search"`
		} `json:"data"`
	}
	err := c.GraphQL("", "", gqlQuery, variables, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "request GraphQL")
	}

	return resp.Data.Search.Results.Results, nil
}

type SearchStatsResult struct {
	Languages []struct {
		Name       string `json:"name"`
		TotalLines int    `json:"totalLines"`
	} `json:"languages"`
}

// SearchStats returns statistics of given query.
func (c *Client) SearchStats(query string) (*SearchStatsResult, error) {
	const gqlQuery = `
query SearchResultsStats($query: String!) {
	search(query: $query, version: V2) {
		stats {
			languages {
				name
				totalLines
			}
		}
	}
}
`
	variables := map[string]interface{}{
		"query": query,
	}
	var resp struct {
		Data struct {
			Search struct {
				Stats *SearchStatsResult `json:"stats"`
			} `json:"search"`
		} `json:"data"`
	}
	err := c.GraphQL("", "", gqlQuery, variables, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "request GraphQL")
	}

	return resp.Data.Search.Stats, nil
}

type SearchSuggestionsResult struct {
	inner interface{}
}

func (srr *SearchSuggestionsResult) UnmarshalJSON(data []byte) error {
	var typeDecoder struct {
		TypeName string `json:"__typename"`
	}
	if err := json.Unmarshal(data, &typeDecoder); err != nil {
		return err
	}

	switch typeDecoder.TypeName {
	case "File":
		var v FileSuggestionResult
		err := json.Unmarshal(data, &v)
		if err != nil {
			return err
		}
		srr.inner = v
	case "Repository":
		var v RepositorySuggestionResult
		err := json.Unmarshal(data, &v)
		if err != nil {
			return err
		}
		srr.inner = v
	case "Symbol":
		var v SymbolSuggestionResult
		err := json.Unmarshal(data, &v)
		if err != nil {
			return err
		}
		srr.inner = v
	case "Language":
		var v LanguageSuggestionResult
		err := json.Unmarshal(data, &v)
		if err != nil {
			return err
		}
		srr.inner = v
	default:
		return fmt.Errorf("unknown typename %s", typeDecoder.TypeName)
	}

	return nil
}

type RepositorySuggestionResult struct {
	Name string
}

type FileSuggestionResult struct {
	Path        string
	Name        string
	IsDirectory bool   `json:"isDirectory"`
	URL         string `json:"url"`
	Repository  struct {
		Name string
	}
}

type SymbolSuggestionResult struct {
	Name          string
	ContainerName string `json:"containerName"`
	URL           string `json:"url"`
	Kind          string
	Location      struct {
		Resource struct {
			Path       string
			Repository struct {
				Name string
			}
		}
	}
}

type LanguageSuggestionResult struct {
	Name string
}

func (c *Client) SearchSuggestions(query string) ([]SearchSuggestionsResult, error) {
	const gqlQuery = `
query SearchSuggestions($query: String!) {
	search(query: $query, version: V2) {
		suggestions {
			__typename
			... on Repository {
				name
			}
			... on File {
				path
				name
				isDirectory
				url
				repository {
					name
				}
			}
			... on Symbol {
				name
				containerName
				url
				kind
				location {
					resource {
						path
						repository {
							name
						}
					}
				}
			}
			... on Language {
				name
			}
		}
	}
}`

	variables := map[string]interface{}{
		"query": query,
	}

	var resp struct {
		Data struct {
			Search struct {
				Suggestions []SearchSuggestionsResult
			} `json:"search"`
		} `json:"data"`
	}
	err := c.GraphQL("", "", gqlQuery, variables, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "request GraphQL")
	}

	return resp.Data.Search.Suggestions, nil
}

type SearchStreamClient struct {
	Client *Client
}

func (s *SearchStreamClient) SearchRepositories(query string) (SearchRepositoryResults, error) {
	var results SearchRepositoryResults
	err := s.search(query, streamhttp.Decoder{
		OnMatches: func(matches []streamhttp.EventMatch) {
			for _, m := range matches {
				r, ok := m.(*streamhttp.EventRepoMatch)
				if !ok {
					continue
				}
				results = append(results, &SearchRepositoryResult{
					Name: r.Repository,
				})
			}
		},
	})
	return results, err
}

func (s *SearchStreamClient) SearchFiles(query string) (*SearchFileResults, error) {
	var results SearchFileResults
	err := s.search(query, streamhttp.Decoder{
		OnProgress: func(p *api.Progress) {
			results.MatchCount = int64(p.MatchCount)
		},
		OnMatches: func(matches []streamhttp.EventMatch) {
			for _, m := range matches {
				switch v := m.(type) {
				case *streamhttp.EventRepoMatch:
					results.Results = append(results.Results, &SearchFileResult{})

				case *streamhttp.EventFileMatch:
					var r SearchFileResult
					r.File.Name = v.Path
					r.Repository.Name = v.Repository
					r.RevSpec.Expr = v.Branches[0]
					results.Results = append(results.Results, &r)

				case *streamhttp.EventSymbolMatch:
					var r SearchFileResult
					r.File.Name = v.Path
					r.Repository.Name = v.Repository
					r.RevSpec.Expr = v.Branches[0]
					results.Results = append(results.Results, &r)

				case *streamhttp.EventCommitMatch:
					// The tests don't actually look at the value. We need to
					// update this client to be more generic, but this will do
					// for now.
					results.Results = append(results.Results, &SearchFileResult{})
				}
			}
		},
		OnAlert: func(alert *streamhttp.EventAlert) {
			results.Alert = &SearchAlert{
				Title:       alert.Title,
				Description: alert.Description,
			}
			for _, pq := range alert.ProposedQueries {
				results.Alert.ProposedQueries = append(results.Alert.ProposedQueries, ProposedQuery{
					Description: pq.Description,
					Query:       pq.Query,
				})
			}
		},
	})
	return &results, err
}
func (s *SearchStreamClient) SearchAll(query string) ([]*AnyResult, error) {
	var results []interface{}
	err := s.search(query, streamhttp.Decoder{
		OnMatches: func(matches []streamhttp.EventMatch) {
			for _, m := range matches {
				switch v := m.(type) {
				case *streamhttp.EventRepoMatch:
					results = append(results, RepositoryResult{
						Name: v.Repository,
					})

				case *streamhttp.EventFileMatch:
					lms := make([]struct {
						OffsetAndLengths [][2]int32 `json:"offsetAndLengths"`
					}, len(v.LineMatches))
					for i := range v.LineMatches {
						lms[i].OffsetAndLengths = v.LineMatches[i].OffsetAndLengths
					}
					results = append(results, FileResult{
						File:        struct{ Path string }{Path: v.Path},
						Repository:  RepositoryResult{Name: v.Repository},
						LineMatches: lms,
					})

				case *streamhttp.EventSymbolMatch:
					var r FileResult
					r.File.Path = v.Path
					r.Repository.Name = v.Repository
					r.Symbols = make([]interface{}, len(v.Symbols))
					results = append(results, &r)

				case *streamhttp.EventCommitMatch:
					// The tests don't actually look at the value. We need to
					// update this client to be more generic, but this will do
					// for now.
					results = append(results, CommitResult{URL: v.URL})
				}
			}
		},
	})
	if err != nil {
		return nil, err
	}

	var ar []*AnyResult
	for _, r := range results {
		ar = append(ar, &AnyResult{Inner: r})
	}
	return ar, nil
}

func (s *SearchStreamClient) OverwriteSettings(subjectID, contents string) error {
	return s.Client.OverwriteSettings(subjectID, contents)
}
func (s *SearchStreamClient) AuthenticatedUserID() string {
	return s.Client.AuthenticatedUserID()
}

func (s *SearchStreamClient) search(query string, dec streamhttp.Decoder) error {
	req, err := streamhttp.NewRequest(s.Client.baseURL, query)
	if err != nil {
		return err
	}
	s.Client.addCookies(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return dec.ReadAll(resp.Body)
}
