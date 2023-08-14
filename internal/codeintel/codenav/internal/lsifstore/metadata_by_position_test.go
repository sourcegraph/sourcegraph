package lsifstore

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
)

func TestDatabaseHover(t *testing.T) {
	testCases := []struct {
		name            string
		uploadID        int
		path            string
		line, character int
		expectedText    string
		expectedRange   shared.Range
	}{
		{
			// `export async function queryLSIF<P extends { query: string; uri: string }, R>(`
			//                        ^^^^^^^^^

			name:     "scip",
			uploadID: testSCIPUploadID1,
			path:     "template/src/lsif/api.ts",
			line:     14, character: 25,
			expectedText:  "```ts\nfunction queryLSIF<P extends { query: string; uri: string; }, R>({ query, uri, ...rest }: P, queryGraphQL: QueryGraphQLFn<GenericLSIFResponse<R>>): Promise<R | null>\n```\nPerform an LSIF request to the GraphQL API.",
			expectedRange: newRange(14, 22, 14, 31),
		},
		{
			// `    const { repo, commit, path } = parseGitURI(new URL(uri))`
			//                                     ^^^^^^^^^^^

			name:     "scip",
			uploadID: testSCIPUploadID1,
			path:     "template/src/lsif/api.ts",
			line:     25, character: 40,
			expectedText:  "```ts\nfunction parseGitURI({ hostname, pathname, search, hash }: URL): { repo: string; commit: string; path: string; }\n```\nExtracts the components of a text document URI.",
			expectedRange: newRange(25, 35, 25, 46),
		},
	}

	store := populateTestStore(t)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if actualText, actualRange, exists, err := store.GetHover(context.Background(), testCase.uploadID, testCase.path, testCase.line, testCase.character); err != nil {
				t.Fatalf("unexpected error %s", err)
			} else if !exists {
				t.Errorf("no hover found")
			} else {
				if diff := cmp.Diff(testCase.expectedText, actualText); diff != "" {
					t.Errorf("unexpected hover text (-want +got):\n%s", diff)
				}

				if diff := cmp.Diff(testCase.expectedRange, actualRange); diff != "" {
					t.Errorf("unexpected hover range (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestGetDiagnostics(t *testing.T) {
	// NOTE: No SCIP indexer currently emit diagnostics
	t.Skip()
}
