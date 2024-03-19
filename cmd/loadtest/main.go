package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/sanitycheck"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var (
	FrontendHost     = env.Get("LOAD_TEST_FRONTEND_URL", "http://sourcegraph-frontend-internal", "URL to the Sourcegraph frontend host to load test")
	FrontendPort     = env.Get("loadTestFrontendPort", "80", "Port that the Sourcegraph frontend is listening on")
	SearchQueriesEnv = env.Get("loadTestSearches", "[]", "Search queries to use in load testing")
	QueryPeriodMSEnv = env.Get("loadTestSearchPeriod", "2000", "Period of search query issuance (milliseconds). E.g., a value of 200 corresponds to 200ms or 5 QPS")
)

type GQLSearchVars struct {
	Query string `json:"query"`
}

func main() {
	sanitycheck.Pass()

	log.Init(log.Resource{Name: "loadtest"})
	logger := log.Scoped("loadtest")

	if err := run(logger); err != nil {
		logger.Fatal("run failed", log.Error(err))
	}
}

func frontendURL(thePath string) string {
	return fmt.Sprintf("%s:%s%s", FrontendHost, FrontendPort, thePath)
}

func run(logger log.Logger) error {
	var searchQueries []GQLSearchVars
	if err := json.Unmarshal([]byte(SearchQueriesEnv), &searchQueries); err != nil {
		return err
	}

	qps, err := strconv.Atoi(QueryPeriodMSEnv)
	if err != nil {
		return err
	}

	if len(searchQueries) == 0 {
		logger.Warn("No search queries specified. Hanging indefinitely")
		select {}
	}

	ticker := time.NewTicker(time.Duration(qps) * time.Millisecond)
	for {
		for _, v := range searchQueries {
			<-ticker.C
			go func() {
				if count, err := search(v); err != nil {
					logger.Error("Error issuing search query", log.String("query", v.Query), log.Error(err))
				} else {
					logger.Info("Search results", log.String("query", v.Query), log.Int("matchCount", count))
				}
			}()
		}
	}
}

func search(v GQLSearchVars) (int, error) {
	gqlQuery := GraphQLQuery{Query: gqlSearch, Variables: v}
	b, err := json.Marshal(gqlQuery)
	if err != nil {
		return 0, errors.Errorf("failed to marshal query: %s", err)
	}
	resp, err := http.Post(frontendURL("/.api/graphql?Search"), "application/json", bytes.NewReader(b))
	if err != nil {
		return 0, errors.Errorf("response error: %s", err)
	}
	defer resp.Body.Close()
	var res GraphQLResponseSearch
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return 0, errors.Errorf("could not decode response body: %s", err)
	}
	return len(res.Data.Search.Results.Results), nil
}

type GraphQLResponseSearch struct {
	Data struct {
		Search struct {
			Results struct {
				Results []any `json:"results"`
			} `json:"results"`
		} `json:"search"`
	} `json:"data"`
}

type GraphQLQuery struct {
	Query     string `json:"query"`
	Variables any    `json:"variables"`
}

const gqlSearch = `query Search(
	$query: String!,
) {
	search(query: $query) {
		results {
			limitHit
			missing { uri }
			cloning { uri }
			timedout { uri }
			results {
				__typename
				... on FileMatch {
					resource
					limitHit
					lineMatches {
						preview
						lineNumber
						offsetAndLengths
					}
				}
				... on CommitSearchResult {
					refs {
						name
						displayName
						prefix
						repository { uri }
					}
					sourceRefs {
						name
						displayName
						prefix
						repository { uri }
					}
					messagePreview {
						value
						highlights {
							line
							character
							length
						}
					}
					diffPreview {
						value
						highlights {
							line
							character
							length
						}
					}
					commit {
						repository {
							uri
						}
						oid
						abbreviatedOID
						author {
							person {
								displayName
								avatarURL
							}
							date
						}
						message
					}
				}
			}
			alert {
				title
				description
				proposedQueries {
					description
					query {
						query
					}
				}
			}
		}
	}
}
`
