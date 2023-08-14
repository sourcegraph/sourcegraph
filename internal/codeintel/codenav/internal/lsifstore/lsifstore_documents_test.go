package lsifstore

import (
	"context"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/symbols"
)

func TestGetFullSCIPNameByDescriptor(t *testing.T) {
	ctx := context.Background()
	store := populateTestStore(t)
	uploadIDs := []int{testSCIPUploadID2}
	symbolNames := []string{
		"treesitter . . . jen/Statement#Add().",
		"treesitter . . . yamlPayload#Force.",
		"treesitter . . . internal/mockgen/generation/generateMockStructFromConstructorCommon().",
		"treesitter . . . testutil/require/CalledWith().",
		"treesitter . . . nomatch.",
		"malformed symbol",
	}

	explodedSymbols, err := store.GetFullSCIPNameByDescriptor(ctx, uploadIDs, symbolNames)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expected := []*symbols.ExplodedSymbol{
		{
			Scheme:              "scip-go",
			PackageManager:      "gomod",
			PackageName:         "github.com/dave/jennifer",
			PackageVersion:      "v1.5.0",
			DescriptorNamespace: "`github.com/dave/jennifer/jen`/",
			DescriptorSuffix:    "Statement#Add().",
		},
		{
			Scheme:              "scip-go",
			PackageManager:      "gomod",
			PackageName:         "github.com/derision-test/go-mockgen",
			PackageVersion:      "d061eb01e698",
			DescriptorNamespace: "`github.com/derision-test/go-mockgen/cmd/go-mockgen`/",
			DescriptorSuffix:    "yamlPayload#Force.",
		},
		{
			Scheme:              "scip-go",
			PackageManager:      "gomod",
			PackageName:         "github.com/derision-test/go-mockgen",
			PackageVersion:      "d061eb01e698",
			DescriptorNamespace: "`github.com/derision-test/go-mockgen/internal/mockgen/generation`/",
			DescriptorSuffix:    "generateMockStructFromConstructorCommon().",
		},
		{
			Scheme:              "scip-go",
			PackageManager:      "gomod",
			PackageName:         "github.com/derision-test/go-mockgen",
			PackageVersion:      "d061eb01e698",
			DescriptorNamespace: "`github.com/derision-test/go-mockgen/testutil/require`/",
			DescriptorSuffix:    "CalledWith().",
		},

		//
		// Over-selection:

		{
			Scheme:              "scip-go",
			PackageManager:      "gomod",
			PackageName:         "github.com/derision-test/go-mockgen",
			PackageVersion:      "d061eb01e698",
			DescriptorNamespace: "`github.com/derision-test/go-mockgen/testutil/require`/",
			DescriptorSuffix:    "NotCalledWith().",
		},
		{
			Scheme:              "scip-go",
			PackageManager:      "gomod",
			PackageName:         "github.com/derision-test/go-mockgen",
			PackageVersion:      "d061eb01e698",
			DescriptorNamespace: "`github.com/derision-test/go-mockgen/testutil/assert`/",
			DescriptorSuffix:    "CalledWith().",
		},
		{
			Scheme:              "scip-go",
			PackageManager:      "gomod",
			PackageName:         "github.com/derision-test/go-mockgen",
			PackageVersion:      "d061eb01e698",
			DescriptorNamespace: "`github.com/derision-test/go-mockgen/testutil/assert`/",
			DescriptorSuffix:    "NotCalledWith().",
		},
		{
			Scheme:              "scip-go",
			PackageManager:      "gomod",
			PackageName:         "github.com/derision-test/go-mockgen",
			PackageVersion:      "d061eb01e698",
			DescriptorNamespace: "`github.com/derision-test/go-mockgen/testutil/gomega`/",
			DescriptorSuffix:    "BeCalledWith().",
		},
	}

	sortExplodedSymbols(expected)
	sortExplodedSymbols(explodedSymbols)

	if diff := cmp.Diff(expected, explodedSymbols); diff != "" {
		t.Errorf("unexpected symbols (-want +got):\n%s", diff)
	}
}

func sortExplodedSymbols(symbols []*symbols.ExplodedSymbol) {
	sort.Slice(symbols, func(i, j int) bool {
		if symbols[i].DescriptorNamespace == symbols[j].DescriptorNamespace {
			return symbols[i].DescriptorSuffix < symbols[j].DescriptorSuffix
		}

		return symbols[i].DescriptorNamespace < symbols[j].DescriptorNamespace
	})
}
