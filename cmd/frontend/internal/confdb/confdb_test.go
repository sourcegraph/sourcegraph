package confdb

import (
	"context"
	"strings"
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
)

func TestSiteGetLatestDefault(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	dbtesting.SetupGlobalTestDB(t)
	ctx := context.Background()

	latest, err := SiteGetLatest(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if latest == nil {
		t.Errorf("expected non-nil latest config since default config should be created, got: %+v", latest)
	}
}

func TestSiteCreate_RejectInvalidJSON(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	dbtesting.SetupGlobalTestDB(t)
	ctx := context.Background()

	malformedJSON := "[This is malformed.}"

	_, err := SiteCreateIfUpToDate(ctx, nil, malformedJSON)

	if err == nil || !strings.Contains(err.Error(), "invalid settings JSON") {
		t.Fatalf("expected parse error after creating configuration with malformed JSON, got: %+v", err)
	}
}

func TestSiteCreateIfUpToDate(t *testing.T) {
	type input struct {
		lastID   int32
		contents string
	}

	type output struct {
		ID       int32
		contents string
		err      error
	}

	type pair struct {
		input    input
		expected output
	}

	type test struct {
		name     string
		sequence []pair
	}

	for _, test := range []test{
		{
			name: "create_one",
			sequence: []pair{
				{
					input{
						lastID:   0,
						contents: `"This is a test."`,
					},
					output{
						ID:       2,
						contents: `"This is a test."`,
					},
				},
			},
		},
		{
			name: "create_two",
			sequence: []pair{
				{
					input{
						lastID:   0,
						contents: `"This is the first one."`,
					},
					output{
						ID:       2,
						contents: `"This is the first one."`,
					},
				},
				{
					input{
						lastID:   2,
						contents: `"This is the second one."`,
					},
					output{
						ID:       3,
						contents: `"This is the second one."`,
					},
				},
			},
		},
		{
			name: "do_not_update_if_outdated",
			sequence: []pair{
				{
					input{
						lastID:   0,
						contents: `"This is the first one."`,
					},
					output{
						ID:       2,
						contents: `"This is the first one."`,
					},
				},
				{
					input{
						lastID:   0,
						contents: `"This configuration is now behind the first one, so it shouldn't be saved."`,
					},
					output{
						ID:       2,
						contents: `"This is the first one."`,
						err:      ErrNewerEdit,
					},
				},
			},
		},
		{
			name: "maintain_commments_and_whitespace",
			sequence: []pair{
				{
					input{
						lastID: 0,
						contents: `{"fieldA": "valueA",

// This is a comment.
             "fieldB": "valueB",
						}`,
					},
					output{
						ID: 2,
						contents: `{"fieldA": "valueA",

// This is a comment.
             "fieldB": "valueB",
						}`,
					},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dbtesting.SetupGlobalTestDB(t)
			ctx := context.Background()
			for _, p := range test.sequence {
				output, err := SiteCreateIfUpToDate(ctx, &p.input.lastID, p.input.contents)
				if err != nil {
					if err == p.expected.err {
						continue
					}
					t.Fatal(err)
				}

				if output == nil {
					t.Fatal("got unexpected nil configuration after creation")
				}

				if output.Contents != p.expected.contents {
					t.Fatalf("returned configuration contents after creation - expected: %q, got:%q", p.expected.contents, output.Contents)
				}
				if output.ID != p.expected.ID {
					t.Fatalf("returned configuration ID after creation - expected: %v, got:%v", p.expected.ID, output.ID)
				}

				latest, err := SiteGetLatest(ctx)
				if err != nil {
					t.Fatal(err)
				}

				if latest == nil {
					t.Fatalf("got unexpected nil configuration after GetLatest")
				}

				if latest.Contents != p.expected.contents {
					t.Fatalf("returned configuration contents after GetLatest - expected: %q, got:%q", p.expected.contents, latest.Contents)
				}
				if latest.ID != p.expected.ID {
					t.Fatalf("returned configuration ID after GetLatest - expected: %v, got:%v", p.expected.ID, latest.ID)
				}
			}
		})
	}
}
