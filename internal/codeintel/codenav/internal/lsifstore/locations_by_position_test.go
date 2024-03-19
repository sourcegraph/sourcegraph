package lsifstore

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"
	"github.com/sourcegraph/scip/bindings/go/scip"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
	codeintelshared "github.com/sourcegraph/sourcegraph/internal/codeintel/shared"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
)

const (
	testSCIPUploadID = 2408562
)

func TestExtractDefinitionLocationsFromPosition(t *testing.T) {
	store := populateTestStore(t)

	// `const lru = new LRU<string, V>(cacheOptions)`
	//        ^^^
	// -> `    if (lru.has(key)) {`
	//             ^^^
	// -> `        return lru.get(key)!`
	//                    ^^^
	// -> `    lru.set(key, value)`
	//         ^^^

	scipDefinitionLocations := []shared.Location{
		{
			UploadID: testSCIPUploadID,
			Path:     "template/src/lsif/util.ts",
			Range:    newRange(7, 10, 7, 13),
		},
	}

	testCases := []struct {
		key                 LocationKey
		expectedLocations   []shared.Location
		expectedSymbolNames []string
	}{
		{LocationKey{testSCIPUploadID, "template/src/lsif/util.ts", 7, 12}, scipDefinitionLocations, nil},
		{LocationKey{testSCIPUploadID, "template/src/lsif/util.ts", 10, 13}, scipDefinitionLocations, nil},
		{LocationKey{testSCIPUploadID, "template/src/lsif/util.ts", 12, 19}, scipDefinitionLocations, nil},
		{LocationKey{testSCIPUploadID, "template/src/lsif/util.ts", 15, 10}, scipDefinitionLocations, nil},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("i=%d", i), func(t *testing.T) {
			if locations, symbolNames, err := store.ExtractDefinitionLocationsFromPosition(context.Background(), testCase.key); err != nil {
				t.Fatalf("unexpected error %s", err)
			} else {
				if diff := cmp.Diff(testCase.expectedLocations, locations); diff != "" {
					t.Errorf("unexpected locations (-want +got):\n%s", diff)
				}

				if diff := cmp.Diff(testCase.expectedSymbolNames, symbolNames); diff != "" {
					t.Errorf("unexpected symbol names (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestExtractReferenceLocationsFromPosition(t *testing.T) {
	store := populateTestStore(t)

	// `const lru = new LRU<string, V>(cacheOptions)`
	//        ^^^
	// -> `    if (lru.has(key)) {`
	//             ^^^
	// -> `        return lru.get(key)!`
	//                    ^^^
	// -> `    lru.set(key, value)`
	//         ^^^

	scipExpected := []shared.Location{
		{UploadID: testSCIPUploadID, Path: "template/src/lsif/util.ts", Range: newRange(10, 12, 10, 15)},
		{UploadID: testSCIPUploadID, Path: "template/src/lsif/util.ts", Range: newRange(12, 19, 12, 22)},
		{UploadID: testSCIPUploadID, Path: "template/src/lsif/util.ts", Range: newRange(15, 8, 15, 11)},
	}

	testCases := []struct {
		key                 LocationKey
		expectedLocations   []shared.Location
		expectedSymbolNames []string
	}{
		{LocationKey{testSCIPUploadID, "template/src/lsif/util.ts", 12, 21}, scipExpected, nil},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("i=%d", i), func(t *testing.T) {
			if locations, symbolNames, err := store.ExtractReferenceLocationsFromPosition(context.Background(), testCase.key); err != nil {
				t.Fatalf("unexpected error %s", err)
			} else {
				if diff := cmp.Diff(testCase.expectedLocations, locations); diff != "" {
					t.Errorf("unexpected locations (-want +got):\n%s", diff)
				}

				if diff := cmp.Diff(testCase.expectedSymbolNames, symbolNames); diff != "" {
					t.Errorf("unexpected symbol names (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestGetMinimalBulkMonikerLocations(t *testing.T) {
	tableName := "references"
	uploadIDs := []int{testSCIPUploadID}
	skipPaths := map[int]string{}
	monikers := []precise.MonikerData{
		{
			Scheme:     "gomod",
			Identifier: "github.com/sourcegraph/lsif-go/protocol:DefinitionResult.Vertex",
		},
		{
			Scheme:     "scip-typescript",
			Identifier: "scip-typescript npm template 0.0.0-DEVELOPMENT src/util/`helpers.ts`/asArray().",
		},
	}

	store := populateTestStore(t)

	locations, totalCount, err := store.GetMinimalBulkMonikerLocations(context.Background(), tableName, uploadIDs, skipPaths, monikers, 100, 0)
	if err != nil {
		t.Fatalf("unexpected error querying bulk moniker locations: %s", err)
	}
	if expected := 9; totalCount != expected {
		t.Fatalf("unexpected total count: want=%d have=%d\n", expected, totalCount)
	}

	expectedLocations := []shared.Location{
		// SCIP results
		{UploadID: testSCIPUploadID, Path: "template/src/providers.ts", Range: newRange(10, 9, 10, 16)},
		{UploadID: testSCIPUploadID, Path: "template/src/providers.ts", Range: newRange(186, 43, 186, 50)},
		{UploadID: testSCIPUploadID, Path: "template/src/providers.ts", Range: newRange(296, 34, 296, 41)},
		{UploadID: testSCIPUploadID, Path: "template/src/providers.ts", Range: newRange(324, 38, 324, 45)},
		{UploadID: testSCIPUploadID, Path: "template/src/providers.ts", Range: newRange(384, 30, 384, 37)},
		{UploadID: testSCIPUploadID, Path: "template/src/providers.ts", Range: newRange(415, 8, 415, 15)},
		{UploadID: testSCIPUploadID, Path: "template/src/providers.ts", Range: newRange(420, 27, 420, 34)},
		{UploadID: testSCIPUploadID, Path: "template/src/search/providers.ts", Range: newRange(9, 9, 9, 16)},
		{UploadID: testSCIPUploadID, Path: "template/src/search/providers.ts", Range: newRange(225, 20, 225, 27)},
	}
	if diff := cmp.Diff(expectedLocations, locations); diff != "" {
		t.Errorf("unexpected locations (-want +got):\n%s", diff)
	}
}

func TestDatabaseDefinitions(t *testing.T) {
	store := populateTestStore(t)

	// `const lru = new LRU<string, V>(cacheOptions)`
	//        ^^^
	// -> `    if (lru.has(key)) {`
	//             ^^^
	// -> `        return lru.get(key)!`
	//                    ^^^
	// -> `    lru.set(key, value)`
	//         ^^^

	scipDefinitionLocations := []shared.Location{
		{
			UploadID: testSCIPUploadID,
			Path:     "template/src/lsif/util.ts",
			Range:    newRange(7, 10, 7, 13),
		},
	}

	// Symbol name search for
	//
	// `export interface HoverPayload {`
	//                   ^^^^^^^^^^^^

	scipNonLocalDefinitionLocations := []shared.Location{
		{
			UploadID: testSCIPUploadID,
			Path:     "template/src/lsif/definition-hover.ts",
			Range:    newRange(21, 17, 21, 29),
		},
	}

	testCases := []struct {
		uploadID        int
		path            string
		line, character int
		totalCount      int
		limit           int
		offset          int
		expected        []shared.Location
	}{
		// SCIP (local)
		{testSCIPUploadID, "template/src/lsif/util.ts", 7, 12, 1, 1, 0, scipDefinitionLocations},
		{testSCIPUploadID, "template/src/lsif/util.ts", 10, 13, 1, 1, 0, scipDefinitionLocations},
		{testSCIPUploadID, "template/src/lsif/util.ts", 12, 19, 1, 1, 0, scipDefinitionLocations},
		{testSCIPUploadID, "template/src/lsif/util.ts", 15, 10, 1, 1, 0, scipDefinitionLocations},

		// SCIP (non-local)
		{testSCIPUploadID, "template/src/lsif/ranges.ts", 6, 15, 1, 1, 0, scipNonLocalDefinitionLocations},
		{testSCIPUploadID, "template/src/lsif/ranges.ts", 38, 20, 1, 1, 0, scipNonLocalDefinitionLocations},
		{testSCIPUploadID, "template/src/lsif/ranges.ts", 385, 20, 1, 1, 0, scipNonLocalDefinitionLocations},
		{testSCIPUploadID, "template/src/lsif/definition-hover.ts", 18, 20, 1, 1, 0, scipNonLocalDefinitionLocations},
		{testSCIPUploadID, "template/src/lsif/definition-hover.ts", 123, 52, 1, 1, 0, scipNonLocalDefinitionLocations},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("i=%d", i), func(t *testing.T) {
			if actual, totalCount, err := store.GetDefinitionLocations(
				context.Background(),
				testCase.uploadID,
				testCase.path,
				testCase.line,
				testCase.character,
				testCase.limit,
				testCase.offset,
			); err != nil {
				t.Fatalf("unexpected error %s", err)
			} else {
				if totalCount != testCase.totalCount {
					t.Errorf("unexpected count. want=%d have=%d", testCase.totalCount, totalCount)
				}
				if diff := cmp.Diff(testCase.expected, actual); diff != "" {
					t.Errorf("unexpected reference locations (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestDatabaseReferences(t *testing.T) {
	store := populateTestStore(t)

	// `const lru = new LRU<string, V>(cacheOptions)`
	//        ^^^
	// -> `    if (lru.has(key)) {`
	//             ^^^
	// -> `        return lru.get(key)!`
	//                    ^^^
	// -> `    lru.set(key, value)`
	//         ^^^

	scipExpected := []shared.Location{
		{UploadID: testSCIPUploadID, Path: "template/src/lsif/util.ts", Range: newRange(10, 12, 10, 15)},
		{UploadID: testSCIPUploadID, Path: "template/src/lsif/util.ts", Range: newRange(12, 19, 12, 22)},
		{UploadID: testSCIPUploadID, Path: "template/src/lsif/util.ts", Range: newRange(15, 8, 15, 11)},
	}

	// Symbol name search for
	//
	// `export interface HoverPayload {`
	//                   ^^^^^^^^^^^^

	scipNonLocalExpected := []shared.Location{
		{UploadID: testSCIPUploadID, Path: "template/src/lsif/ranges.ts", Range: newRange(6, 9, 6, 21)},
		{UploadID: testSCIPUploadID, Path: "template/src/lsif/ranges.ts", Range: newRange(38, 12, 38, 24)},
		{UploadID: testSCIPUploadID, Path: "template/src/lsif/ranges.ts", Range: newRange(385, 12, 385, 24)},
		{UploadID: testSCIPUploadID, Path: "template/src/lsif/definition-hover.ts", Range: newRange(18, 12, 18, 24)},
		{UploadID: testSCIPUploadID, Path: "template/src/lsif/definition-hover.ts", Range: newRange(123, 45, 123, 57)},
	}

	testCases := []struct {
		uploadID        int
		path            string
		line, character int
		totalCount      int
		limit           int
		offset          int
		expected        []shared.Location
	}{
		// SCIP (local)
		{testSCIPUploadID, "template/src/lsif/util.ts", 12, 21, 3, 5, 0, scipExpected},
		{testSCIPUploadID, "template/src/lsif/util.ts", 12, 21, 3, 2, 0, scipExpected[:2]},
		{testSCIPUploadID, "template/src/lsif/util.ts", 12, 21, 3, 2, 1, scipExpected[1:3]},
		{testSCIPUploadID, "template/src/lsif/util.ts", 12, 21, 3, 5, 5, scipExpected[:0]},

		// SCIP (non-local)
		{testSCIPUploadID, "template/src/lsif/ranges.ts", 38, 15, 5, 5, 0, scipNonLocalExpected},
		{testSCIPUploadID, "template/src/lsif/ranges.ts", 38, 15, 5, 2, 0, scipNonLocalExpected[:2]},
		{testSCIPUploadID, "template/src/lsif/ranges.ts", 38, 15, 5, 2, 1, scipNonLocalExpected[1:3]},
		{testSCIPUploadID, "template/src/lsif/ranges.ts", 38, 15, 5, 5, 5, scipNonLocalExpected[:0]},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("i=%d", i), func(t *testing.T) {
			if actual, totalCount, err := store.GetReferenceLocations(
				context.Background(),
				testCase.uploadID,
				testCase.path,
				testCase.line,
				testCase.character,
				testCase.limit,
				testCase.offset,
			); err != nil {
				t.Fatalf("unexpected error %s", err)
			} else {
				if totalCount != testCase.totalCount {
					t.Errorf("unexpected count. want=%d have=%d", testCase.totalCount, totalCount)
				}
				if diff := cmp.Diff(testCase.expected, actual); diff != "" {
					t.Errorf("unexpected reference locations (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func populateTestStore(t testing.TB) LsifStore {
	logger := logtest.Scoped(t)
	codeIntelDB := codeintelshared.NewCodeIntelDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, codeIntelDB)

	loadTestFile(t, codeIntelDB, "./testdata/code-intel-extensions@7802976b.sql")
	return store
}

func loadTestFile(t testing.TB, codeIntelDB codeintelshared.CodeIntelDB, filepath string) {
	contents, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatalf("unexpected error reading testdata from %q: %s", filepath, err)
	}

	tx, err := codeIntelDB.Transact(context.Background())
	if err != nil {
		t.Fatalf("unexpected error starting transaction: %s", err)
	}
	defer func() {
		if err := tx.Done(nil); err != nil {
			t.Fatalf("unexpected error finishing transaction: %s", err)
		}
	}()

	// Remove comments from the lines.
	var withoutComments []byte
	for _, line := range bytes.Split(contents, []byte{'\n'}) {
		if string(line) == "" || bytes.HasPrefix(line, []byte("--")) {
			continue
		}
		withoutComments = append(withoutComments, line...)
		withoutComments = append(withoutComments, '\n')
	}

	// Execute each statement. Split on ";\n" because statements may have e.g. string literals that
	// span multiple lines.
	for _, statement := range strings.Split(string(withoutComments), ";\n") {
		if strings.Contains(statement, "_schema_versions") {
			// Statements which insert into lsif_data_*_schema_versions should not be executed, as
			// these are already inserted during regular DB up migrations.
			continue
		}
		if _, err := tx.ExecContext(context.Background(), statement); err != nil {
			t.Fatalf("unexpected error loading database data: %s", err)
		}
	}
}

func TestExtractOccurrenceData(t *testing.T) {
	t.Run("definitions", func(t *testing.T) {
		testCases := []struct {
			explanation    string
			document       *scip.Document
			occurrence     *scip.Occurrence
			expectedRanges []*scip.Range
		}{
			{
				explanation: "#1 happy path: symbol name matches and is definition",
				document: &scip.Document{
					Occurrences: []*scip.Occurrence{
						{
							Range:       []int32{1, 100, 1, 200},
							Symbol:      "react 17.1 main.go func1",
							SymbolRoles: 1, // is definition
						},
					},
					Symbols: []*scip.SymbolInformation{
						{
							Symbol: "react 17.1 main.go func3",
							Relationships: []*scip.Relationship{
								{IsReference: true},
							},
						},
					},
				},
				occurrence: &scip.Occurrence{
					Symbol:      "react 17.1 main.go func1",
					SymbolRoles: 1, // is definition
				},
				expectedRanges: []*scip.Range{
					scip.NewRange([]int32{1, 100, 1, 200}),
				},
			},
			{
				explanation: "#2 no ranges available: symbol name does not match",
				document: &scip.Document{
					Occurrences: []*scip.Occurrence{
						{
							Range:       []int32{1, 100, 1, 200},
							Symbol:      "react-test index.js func2",
							SymbolRoles: 1, // is definition
						},
					},
					Symbols: []*scip.SymbolInformation{
						{
							Symbol: "react-test index.js func2",
							Relationships: []*scip.Relationship{
								{IsReference: false},
							},
						},
					},
				},
				occurrence: &scip.Occurrence{
					Symbol:      "react-jest main.js func7",
					SymbolRoles: 1, // is definition
				},
				expectedRanges: []*scip.Range{},
			},
			{
				explanation: "#3 symbol name match but the SymbolRole is not a definition",
				document: &scip.Document{
					Occurrences: []*scip.Occurrence{
						{
							Range:       []int32{1, 100, 1, 200},
							Symbol:      "react-test index.js func2",
							SymbolRoles: 0, // not a definition
						},
					},
					Symbols: []*scip.SymbolInformation{
						{
							Symbol: "react-test index.js func2",
							Relationships: []*scip.Relationship{
								{IsReference: true},
							},
						},
					},
				},
				occurrence: &scip.Occurrence{
					Symbol:      "react-test index.js func2",
					SymbolRoles: 0, // not a definition
				},
				expectedRanges: []*scip.Range{},
			},
		}

		for _, testCase := range testCases {
			if diff := cmp.Diff(testCase.expectedRanges, extractOccurrenceData(testCase.document, testCase.occurrence).definitions); diff != "" {
				t.Errorf("unexpected ranges (-want +got):\n%s  -- %s", diff, testCase.explanation)
			}
		}
	})

	t.Run("references", func(t *testing.T) {
		testCases := []struct {
			explanation    string
			document       *scip.Document
			occurrence     *scip.Occurrence
			expectedRanges []*scip.Range
		}{
			{
				explanation: "#1 happy path: symbol name matches and it is a reference",
				document: &scip.Document{
					Occurrences: []*scip.Occurrence{
						{
							Range:       []int32{1, 100, 1, 200},
							Symbol:      "react 17.1 main.go func1",
							SymbolRoles: 0, // not a definition so its a reference
						},
					},
					Symbols: []*scip.SymbolInformation{
						{
							Symbol: "react 17.1 main.go func1",
							Relationships: []*scip.Relationship{
								{
									Symbol:      "react 17.1 main.go func1",
									IsReference: true,
								},
								{
									Symbol:       "react 17.1 main.go func2",
									IsDefinition: true,
								},
							},
						},
					},
				},
				occurrence: &scip.Occurrence{
					Symbol:      "react 17.1 main.go func1",
					SymbolRoles: 0, // not a definition so its a reference
				},
				expectedRanges: []*scip.Range{
					scip.NewRange([]int32{1, 100, 1, 200}),
				},
			},
			{
				explanation: "#2 no ranges available: symbol name does not match",
				document: &scip.Document{
					Occurrences: []*scip.Occurrence{
						{
							Range:       []int32{1, 100, 1, 200},
							Symbol:      "react-test index.js func2",
							SymbolRoles: 1, // is a definition
						},
					},
					Symbols: []*scip.SymbolInformation{
						{
							Symbol: "react-test index.js func2",
							Relationships: []*scip.Relationship{
								{IsReference: true},
							},
						},
					},
				},
				occurrence: &scip.Occurrence{
					Symbol:      "react-jest main.js func7",
					SymbolRoles: 0, // not a definition so its a reference
				},
				expectedRanges: []*scip.Range{},
			},
			{
				explanation: "#3 symbol name match but it is not a reference",
				document: &scip.Document{
					Occurrences: []*scip.Occurrence{
						{
							Range:       []int32{5, 500, 7, 700},
							Symbol:      "react-test index.js func2",
							SymbolRoles: 1, // is a definition
						},
					},
					Symbols: []*scip.SymbolInformation{
						{
							Symbol: "react-test index.js func2",
							Relationships: []*scip.Relationship{
								{IsReference: false},
							},
						},
					},
				},
				occurrence: &scip.Occurrence{
					Symbol:      "react-test index.js func2",
					SymbolRoles: 0, // not a definition so its a reference
				},
				expectedRanges: []*scip.Range{},
			},
		}

		for _, testCase := range testCases {
			if diff := cmp.Diff(testCase.expectedRanges, extractOccurrenceData(testCase.document, testCase.occurrence).references); diff != "" {
				t.Errorf("unexpected ranges (-want +got):\n%s  -- %s", diff, testCase.explanation)
			}
		}
	})

	t.Run("implementations", func(t *testing.T) {
		testCases := []struct {
			explanation    string
			document       *scip.Document
			occurrence     *scip.Occurrence
			expectedRanges []*scip.Range
		}{
			{
				explanation: "#1 happy path: we have implementation",
				document: &scip.Document{
					Occurrences: []*scip.Occurrence{
						{
							Range:       []int32{3, 300, 4, 400},
							Symbol:      "react 17.1 main.go func1A",
							SymbolRoles: 1, // a definition
						},
					},
					Symbols: []*scip.SymbolInformation{
						{
							Symbol: "react 17.1 main.go func1A",
							Relationships: []*scip.Relationship{
								{
									Symbol:           "react 17.1 main.go func1",
									IsImplementation: true,
								},
							},
						},
					},
				},
				occurrence: &scip.Occurrence{
					Symbol:      "react 17.1 main.go func1",
					SymbolRoles: 1,
				},
				expectedRanges: []*scip.Range{
					scip.NewRange([]int32{3, 300, 4, 400}),
				},
			},
			{
				explanation: "#2 no ranges available: symbol name does not match",
				document: &scip.Document{
					Occurrences: []*scip.Occurrence{
						{
							Range:       []int32{1, 100, 1, 200},
							Symbol:      "react-test index.js func2",
							SymbolRoles: 1,
						},
					},
					Symbols: []*scip.SymbolInformation{
						{
							Symbol: "react-test index.js func2",
							Relationships: []*scip.Relationship{
								{IsImplementation: true},
							},
						},
					},
				},
				occurrence: &scip.Occurrence{
					Symbol:      "react-jest main.js func7",
					SymbolRoles: 1,
				},
				expectedRanges: []*scip.Range{},
			},
		}

		for _, testCase := range testCases {
			if diff := cmp.Diff(testCase.expectedRanges, extractOccurrenceData(testCase.document, testCase.occurrence).implementations); diff != "" {
				t.Errorf("unexpected ranges (-want +got):\n%s -- %s", diff, testCase.explanation)
			}
		}
	})

	t.Run("prototypes", func(t *testing.T) {
		testCases := []struct {
			explanation    string
			document       *scip.Document
			occurrence     *scip.Occurrence
			expectedRanges []*scip.Range
		}{
			{
				explanation: "#1 happy path: we have prototype",
				document: &scip.Document{
					Occurrences: []*scip.Occurrence{
						{
							Range:       []int32{3, 300, 4, 400},
							Symbol:      "react 17.1 main.go func1",
							SymbolRoles: 1, // a definition
						},
					},
					Symbols: []*scip.SymbolInformation{
						{
							Symbol: "react 17.1 main.go func1A",
							Relationships: []*scip.Relationship{
								{
									Symbol:           "react 17.1 main.go func1",
									IsImplementation: true,
								},
							},
						},
					},
				},
				occurrence: &scip.Occurrence{
					Symbol:      "react 17.1 main.go func1A",
					SymbolRoles: 1,
				},
				expectedRanges: []*scip.Range{
					scip.NewRange([]int32{3, 300, 4, 400}),
				},
			},
		}

		for _, testCase := range testCases {
			if diff := cmp.Diff(testCase.expectedRanges, extractOccurrenceData(testCase.document, testCase.occurrence).prototypes); diff != "" {
				t.Errorf("unexpected ranges (-want +got):\n%s -- %s", diff, testCase.explanation)
			}
		}
	})
}

func TestGetBulkMonikerLocations(t *testing.T) {
	tableName := "references"
	uploadIDs := []int{testSCIPUploadID}
	monikers := []precise.MonikerData{
		{
			Scheme:     "gomod",
			Identifier: "github.com/sourcegraph/lsif-go/protocol:DefinitionResult.Vertex",
		},
		{
			Scheme:     "scip-typescript",
			Identifier: "scip-typescript npm template 0.0.0-DEVELOPMENT src/util/`helpers.ts`/asArray().",
		},
	}

	store := populateTestStore(t)

	locations, totalCount, err := store.GetBulkMonikerLocations(context.Background(), tableName, uploadIDs, monikers, 100, 0)
	if err != nil {
		t.Fatalf("unexpected error querying bulk moniker locations: %s", err)
	}
	if expected := 9; totalCount != expected {
		t.Fatalf("unexpected total count: want=%d have=%d\n", expected, totalCount)
	}

	expectedLocations := []shared.Location{
		// SCIP results
		{UploadID: testSCIPUploadID, Path: "template/src/providers.ts", Range: newRange(10, 9, 10, 16)},
		{UploadID: testSCIPUploadID, Path: "template/src/providers.ts", Range: newRange(186, 43, 186, 50)},
		{UploadID: testSCIPUploadID, Path: "template/src/providers.ts", Range: newRange(296, 34, 296, 41)},
		{UploadID: testSCIPUploadID, Path: "template/src/providers.ts", Range: newRange(324, 38, 324, 45)},
		{UploadID: testSCIPUploadID, Path: "template/src/providers.ts", Range: newRange(384, 30, 384, 37)},
		{UploadID: testSCIPUploadID, Path: "template/src/providers.ts", Range: newRange(415, 8, 415, 15)},
		{UploadID: testSCIPUploadID, Path: "template/src/providers.ts", Range: newRange(420, 27, 420, 34)},
		{UploadID: testSCIPUploadID, Path: "template/src/search/providers.ts", Range: newRange(9, 9, 9, 16)},
		{UploadID: testSCIPUploadID, Path: "template/src/search/providers.ts", Range: newRange(225, 20, 225, 27)},
	}
	if diff := cmp.Diff(expectedLocations, locations); diff != "" {
		t.Errorf("unexpected locations (-want +got):\n%s", diff)
	}
}
