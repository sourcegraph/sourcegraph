package graphqlbackend

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/hashicorp/go-multierror"

	searchrepos "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/search/repos"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestSearchPatternForSuggestion(t *testing.T) {
	db := new(dbtesting.MockDB)

	cases := []struct {
		Name  string
		Alert searchAlert
		Want  string
	}{
		{
			Name: "with_regex_suggestion",
			Alert: searchAlert{
				db:          db,
				title:       "An alert for regex",
				description: "An alert for regex",
				proposedQueries: []*searchQueryDescription{
					{
						description: "Some query description",
						query:       "repo:github.com/sourcegraph/sourcegraph",
						patternType: query.SearchTypeRegex,
					},
				},
			},
			Want: "repo:github.com/sourcegraph/sourcegraph patternType:regexp",
		},
		{
			Name: "with_structural_suggestion",
			Alert: searchAlert{
				db:          db,
				title:       "An alert for structural",
				description: "An alert for structural",
				proposedQueries: []*searchQueryDescription{
					{
						description: "Some query description",
						query:       "repo:github.com/sourcegraph/sourcegraph",
						patternType: query.SearchTypeStructural,
					},
				},
			},
			Want: "repo:github.com/sourcegraph/sourcegraph patternType:structural",
		},
	}

	for _, tt := range cases {
		t.Run(tt.Name, func(t *testing.T) {
			got := tt.Alert.ProposedQueries()
			if !reflect.DeepEqual((*got)[0].Query(), tt.Want) {
				t.Errorf("got: %s, want: %s", (*got)[0].query, tt.Want)
			}
		})
	}
}

func TestAddQueryRegexpField(t *testing.T) {
	tests := []struct {
		query      string
		addField   string
		addPattern string
		want       string
	}{
		{
			query:      "",
			addField:   "repo",
			addPattern: "p",
			want:       "repo:p",
		},
		{
			query:      "foo",
			addField:   "repo",
			addPattern: "p",
			want:       "repo:p foo",
		},
		{
			query:      "foo repo:p",
			addField:   "repo",
			addPattern: "p",
			want:       "repo:p foo",
		},
		{
			query:      "foo repo:q",
			addField:   "repo",
			addPattern: "p",
			want:       "repo:q repo:p foo",
		},
		{
			query:      "foo repo:p",
			addField:   "repo",
			addPattern: "pp",
			want:       "repo:pp foo",
		},
		{
			query:      "foo repo:p",
			addField:   "repo",
			addPattern: "^p",
			want:       "repo:^p foo",
		},
		{
			query:      "foo repo:p",
			addField:   "repo",
			addPattern: "p$",
			want:       "repo:p$ foo",
		},
		{
			query:      "foo repo:^p",
			addField:   "repo",
			addPattern: "^pq",
			want:       "repo:^pq foo",
		},
		{
			query:      "foo repo:p$",
			addField:   "repo",
			addPattern: "qp$",
			want:       "repo:qp$ foo",
		},
		{
			query:      "foo repo:^p",
			addField:   "repo",
			addPattern: "x$",
			want:       "repo:^p repo:x$ foo",
		},
		{
			query:      "foo repo:p|q",
			addField:   "repo",
			addPattern: "pq",
			want:       "repo:p|q repo:pq foo",
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%s, add %s:%s", test.query, test.addField, test.addPattern), func(t *testing.T) {
			q, err := query.ParseLiteral(test.query)
			if err != nil {
				t.Fatal(err)
			}
			got := query.AddRegexpField(q, test.addField, test.addPattern)
			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestAlertForDiffCommitSearchLimits(t *testing.T) {
	db := new(dbtesting.MockDB)

	cases := []struct {
		name                 string
		multiErr             *multierror.Error
		wantAlertDescription string
	}{
		{
			name:                 "diff_search_warns_on_repos_greater_than_search_limit",
			multiErr:             multierror.Append(&multierror.Error{}, &RepoLimitError{ResultType: "diff", Max: 50}),
			wantAlertDescription: `Diff search can currently only handle searching across 50 repositories at a time. Try using the "repo:" filter to narrow down which repositories to search, or using 'after:"1 week ago"'.`,
		},
		{
			name:                 "commit_search_warns_on_repos_greater_than_search_limit",
			multiErr:             multierror.Append(&multierror.Error{}, &RepoLimitError{ResultType: "commit", Max: 50}),
			wantAlertDescription: `Commit search can currently only handle searching across 50 repositories at a time. Try using the "repo:" filter to narrow down which repositories to search, or using 'after:"1 week ago"'.`,
		},
		{
			name:                 "commit_search_warns_on_repos_greater_than_search_limit_with_time_filter",
			multiErr:             multierror.Append(&multierror.Error{}, &TimeLimitError{ResultType: "commit", Max: 10000}),
			wantAlertDescription: `Commit search can currently only handle searching across 10000 repositories at a time. Try using the "repo:" filter to narrow down which repositories to search.`,
		},
	}

	for _, test := range cases {
		alert := alertForError(db, test.multiErr, &SearchInputs{})
		haveAlertDescription := alert.description
		if diff := cmp.Diff(test.wantAlertDescription, haveAlertDescription); diff != "" {
			t.Fatalf("test %s, mismatched alert (-want, +got):\n%s", test.name, diff)
		}
	}
}

func TestErrorToAlertStructuralSearch(t *testing.T) {
	db := new(dbtesting.MockDB)

	cases := []struct {
		name           string
		errors         []error
		wantAlertTitle string
	}{
		{
			name:           "multierr_is_unaffected",
			errors:         []error{errors.New("some error")},
			wantAlertTitle: "",
		},
		{
			name: "surface_friendly_alert_on_oom_err_message",
			errors: []error{
				errors.New("some error"),
				errors.New("Worker_oomed"),
				errors.New("some other error"),
			},
			wantAlertTitle: "Structural search needs more memory",
		},
	}
	for _, test := range cases {
		multiErr := &multierror.Error{
			Errors:      test.errors,
			ErrorFormat: multierror.ListFormatFunc,
		}
		haveAlert := alertForError(db, multiErr, &SearchInputs{})

		if haveAlert != nil && haveAlert.title != test.wantAlertTitle {
			t.Fatalf("test %s, have alert: %q, want: %q", test.name, haveAlert.title, test.wantAlertTitle)
		}

	}
}

func TestAlertForOverRepoLimit(t *testing.T) {
	db := new(dbtesting.MockDB)

	generateRepoRevs := func(numRepos int) []*search.RepositoryRevisions {
		repoRevs := make([]*search.RepositoryRevisions, numRepos)
		chars := []string{"a", "b", "c"} // create some parent names
		j := 0
		for i := range repoRevs {
			repoRevs[i] = &search.RepositoryRevisions{
				Repo: &types.RepoName{
					ID:   api.RepoID(i),
					Name: api.RepoName(chars[j] + "/repoName" + strconv.Itoa(i)),
				},
			}
			if j == 2 {
				j = 0
			} else {
				j++
			}
		}
		return repoRevs
	}

	setMockResolveRepositories := func(numRepos int) {
		mockResolveRepositories = func(effectiveRepoFieldValues []string) (resolved searchrepos.Resolved, err error) {
			return searchrepos.Resolved{
				RepoRevs:        generateRepoRevs(numRepos),
				MissingRepoRevs: make([]*search.RepositoryRevisions, 0),
				OverLimit:       true,
			}, nil
		}
	}
	defer func() { mockResolveRepositories = nil }()

	cases := []struct {
		name      string
		globbing  bool
		repoRevs  int
		query     string
		wantAlert *searchAlert

		// simulates a timeout in alertForOverRepoLimit if "true"
		cancelContext bool
	}{
		{
			name:          "should return default alert because of 0 resolved repos",
			cancelContext: false,
			repoRevs:      0,
			query:         "foo",
			wantAlert: &searchAlert{
				db:              db,
				prometheusType:  "over_repo_limit",
				title:           "Too many matching repositories",
				proposedQueries: nil,
				description:     "Use a 'repo:' or 'repogroup:' filter to narrow your search and see results.",
			},
		},
		{
			name:          "should return default alert because time limit is reached",
			cancelContext: true,
			repoRevs:      1,
			query:         "foo",
			wantAlert: &searchAlert{
				db:             db,
				prometheusType: "over_repo_limit",
				title:          "Too many matching repositories",
				proposedQueries: []*searchQueryDescription{
					{
						"in the repository a/repoName0",
						"repo:^a/repoName0$ foo",
						query.SearchType(0),
					},
				},
				description: "Use a 'repo:' or 'repogroup:' filter to narrow your search and see results.",
			},
		},
		{
			name:          "should return default alert because globbing is activated",
			globbing:      true,
			cancelContext: false,
			repoRevs:      1,
			query:         "foo",
			wantAlert: &searchAlert{
				db:              db,
				prometheusType:  "over_repo_limit",
				title:           "Too many matching repositories",
				proposedQueries: nil,
				description:     "Use a 'repo:' or 'repogroup:' filter to narrow your search and see results.",
			},
		},
		{
			name:          "this query is not basic, so return a default alert without suggestions",
			cancelContext: false,
			repoRevs:      1,
			query:         "a or (b and c)",
			wantAlert: &searchAlert{
				db:              db,
				prometheusType:  "over_repo_limit",
				title:           "Too many matching repositories",
				proposedQueries: nil,
				description:     "Use a 'repo:' or 'repogroup:' filter to narrow your search and see results.",
			},
		},
		{
			name:          "should return smart alert",
			cancelContext: false,
			repoRevs:      1,
			query:         "foo",
			wantAlert: &searchAlert{
				db:             db,
				prometheusType: "over_repo_limit",
				title:          "Too many matching repositories",
				proposedQueries: []*searchQueryDescription{
					{
						"in repositories under a (further filtering required)",
						"repo:^a/ foo",
						query.SearchType(0),
					},
				},
				description: "Use a 'repo:' or 'repogroup:' filter to narrow your search and see results.",
			},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			setMockResolveRepositories(test.repoRevs)
			q, err := query.Parse(test.query, query.ParserOptions{SearchType: query.SearchType(0), Globbing: test.globbing})
			if err != nil {
				t.Fatal(err)
			}
			sr := searchResolver{
				db: db,
				SearchInputs: &SearchInputs{
					OriginalQuery: test.query,
					Query:         q,
					UserSettings: &schema.Settings{
						SearchGlobbing: &test.globbing,
					}},
			}

			ctx, cancel := context.WithCancel(context.Background())
			if test.cancelContext {
				cancel()
			}
			alert := sr.alertForOverRepoLimit(ctx)

			wantAlert := test.wantAlert
			if !reflect.DeepEqual(alert, wantAlert) {
				t.Fatalf("test %s, have alert %+v, want: %+v", test.name, alert, test.wantAlert)
			}
		})
	}
}

func TestCapFirst(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "a", in: "a", want: "A"},
		{name: "ab", in: "ab", want: "Ab"},
		{name: "хлеб", in: "хлеб", want: "Хлеб"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := capFirst(tt.in); got != tt.want {
				t.Errorf("makeTitle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAlertForNoResolvedReposWithNonGlobalSearchContext(t *testing.T) {
	db := new(dbtesting.MockDB)

	mockResolveRepositories = func(effectiveRepoFieldValues []string) (resolved searchrepos.Resolved, err error) {
		return searchrepos.Resolved{
			RepoRevs:        []*search.RepositoryRevisions{},
			MissingRepoRevs: make([]*search.RepositoryRevisions, 0),
			OverLimit:       false,
		}, nil
	}
	defer func() {
		mockResolveRepositories = nil
	}()

	searchQuery := "context:@user repo:r1 foo"
	wantAlert := &searchAlert{
		db:             db,
		prometheusType: "no_resolved_repos__context_none_in_common",
		title:          "No repositories found for your query within the context @user",
		proposedQueries: []*searchQueryDescription{{
			description: "search in the global context",
			query:       "context:global repo:r1 foo",
			patternType: query.SearchTypeRegex,
		}},
	}

	q, err := query.ParseLiteral(searchQuery)
	if err != nil {
		t.Fatal(err)
	}
	sr := searchResolver{
		db: db,
		SearchInputs: &SearchInputs{
			OriginalQuery: searchQuery,
			Query:         q,
			UserSettings:  &schema.Settings{},
		},
	}

	alert := sr.alertForNoResolvedRepos(context.Background())
	if !reflect.DeepEqual(alert, wantAlert) {
		t.Fatalf("have alert %+v, want: %+v", alert, wantAlert)
	}
}
