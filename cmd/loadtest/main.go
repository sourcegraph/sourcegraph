package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/internal/env"
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
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func frontendURL(thePath string) string {
	return fmt.Sprintf("%s:%s%s", FrontendHost, FrontendPort, thePath)
}

func run() error {
	var searchQueries []GQLSearchVars
	if err := json.Unmarshal([]byte(SearchQueriesEnv), &searchQueries); err != nil {
		return err
	}

	qps, err := strconv.Atoi(QueryPeriodMSEnv)
	if err != nil {
		return err
	}

	if len(searchQueries) == 0 {
		log.Printf("No search queries specified. Hanging indefinitely")
		select {}
	}

	ticker := time.NewTicker(time.Duration(qps) * time.Millisecond)
	for {
		for _, v := range searchQueries {
			<-ticker.C
			go func(v GQLSearchVars) {
				if count, err := search(v); err != nil {
					log15.Error("Error issuing search query", "query", v.Query, "error", err)
				} else {
					log15.Info("Search results", "query", v.Query, "resultCount", count)
				}
			}(v)
		}
	}
}

func search(v GQLSearchVars) (int, error) {
	gqlQuery := GraphQLQuery{Query: gqlSearch, Variables: v}
	b, err := json.Marshal(gqlQuery)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal query: %s", err)
	}
	resp, err := http.Post(frontendURL("/.api/graphql?Search"), "application/json", bytes.NewReader(b))
	if err != nil {
		return 0, fmt.Errorf("response error: %s", err)
	}
	defer resp.Body.Close()
	var res GraphQLResponseSearch
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return 0, fmt.Errorf("could not decode response body: %s", err)
	}
	return len(res.Data.Search.Results.Results), nil
}

type GraphQLResponseSearch struct {
	Data struct {
		Search struct {
			Results struct {
				Results []interface{} `json:"results"`
			} `json:"results"`
		} `json:"search"`
	} `json:"data"`
}

type GraphQLQuery struct {
	Query     string      `json:"query"`
	Variables interface{} `json:"variables"`
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
