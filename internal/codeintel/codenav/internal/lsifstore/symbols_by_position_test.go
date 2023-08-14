package lsifstore

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
)

func TestDatabaseMonikersByPosition(t *testing.T) {
	testCases := []struct {
		name      string
		uploadID  int
		path      string
		line      int
		character int
		expected  [][]precise.MonikerData
	}{
		{
			name:     "scip",
			uploadID: testSCIPUploadID1,
			// `    const enabled = sourcegraph.configuration.get().get('codeIntel.lsif') ?? true`
			//                                  ^^^^^^^^^^^^^
			path: "template/src/lsif/providers.ts",
			line: 25, character: 35,
			expected: [][]precise.MonikerData{
				{
					{
						Kind:                 "import",
						Scheme:               "scip-typescript",
						Identifier:           "scip-typescript npm sourcegraph 25.5.0 src/`sourcegraph.d.ts`/`'sourcegraph'`/configuration.",
						PackageInformationID: "scip:bnBt:c291cmNlZ3JhcGg:MjUuNS4w",
					},
				},
			},
		},
	}

	store := populateTestStore(t)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if actual, err := store.GetMonikersByPosition(context.Background(), testCase.uploadID, testCase.path, testCase.line, testCase.character); err != nil {
				t.Fatalf("unexpected error %s", err)
			} else {
				if diff := cmp.Diff(testCase.expected, actual); diff != "" {
					t.Errorf("unexpected moniker result (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestGetPackageInformation(t *testing.T) {
	testCases := []struct {
		name                 string
		uploadID             int
		path                 string
		packageInformationID string
		expectedData         precise.PackageInformationData
	}{
		{
			name:                 "scip",
			uploadID:             testSCIPUploadID1,
			path:                 "protocol/protocol.go",
			packageInformationID: "scip:dGVzdC1tYW5hZ2Vy:dGVzdC1uYW1l:dGVzdC12ZXJzaW9u",
			expectedData: precise.PackageInformationData{
				Manager: "test-manager",
				Name:    "test-name",
				Version: "test-version",
			},
		},
	}

	store := populateTestStore(t)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if actual, exists, err := store.GetPackageInformation(context.Background(), testCase.uploadID, testCase.path, testCase.packageInformationID); err != nil {
				t.Fatalf("unexpected error %s", err)
			} else if !exists {
				t.Errorf("no package information")
			} else {
				if diff := cmp.Diff(testCase.expectedData, actual); diff != "" {
					t.Errorf("unexpected package information (-want +got):\n%s", diff)
				}
			}
		})
	}
}
