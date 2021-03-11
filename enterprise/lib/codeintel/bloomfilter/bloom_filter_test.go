package bloomfilter

import (
	"fmt"
	"testing"
)

func TestCreateFilter(t *testing.T) {
	testCases := []struct {
		includeFile  string
		excludeFiles []string
	}{
		{includeFile: "lorem-ipsum", excludeFiles: []string{"corporate-ipsum", "emojis"}},
		{includeFile: "corporate-ipsum", excludeFiles: []string{"lorem-ipsum", "emojis"}},
		{includeFile: "emojis", excludeFiles: []string{"lorem-ipsum", "corporate-ipsum"}},
	}

	for _, testCase := range testCases {
		name := fmt.Sprintf("includeFile=%s", testCase.includeFile)

		t.Run(name, func(t *testing.T) {
			filter, err := CreateFilter(readTestWords(t, testCase.includeFile))
			if err != nil {
				t.Fatalf("unexpected error creating filter: %s", filter)
			}

			test, err := Decode(filter)
			if err != nil {
				t.Fatalf("unexpected error decoding filter: %s", err)
			}

			for _, v := range readTestWords(t, testCase.includeFile) {
				if !test(v) {
					t.Errorf("expected %s to be in bloom filter", v)
				}
			}

			for _, excludeFile := range testCase.excludeFiles {
				for _, v := range readTestWords(t, excludeFile) {
					if test(v) {
						t.Errorf("expected %s not to be in bloom filter", v)
					}
				}
			}
		})
	}
}

func TestTestTypeScriptGeneratedBloomFilters(t *testing.T) {
	testCases := []struct {
		filterFile  string
		includeFile string
		excludeFile string
	}{
		{filterFile: "64kb-16", includeFile: "lorem-ipsum", excludeFile: "corporate-ipsum"},
		{filterFile: "64kb-08", includeFile: "lorem-ipsum", excludeFile: "corporate-ipsum"},
		{filterFile: "64kb-24", includeFile: "lorem-ipsum", excludeFile: "corporate-ipsum"},
		{filterFile: "64kb-32", includeFile: "lorem-ipsum", excludeFile: "corporate-ipsum"},
		{filterFile: "32kb-16", includeFile: "lorem-ipsum", excludeFile: "corporate-ipsum"},
		{filterFile: "32kb-08", includeFile: "lorem-ipsum", excludeFile: "corporate-ipsum"},
		{filterFile: "32kb-24", includeFile: "lorem-ipsum", excludeFile: "corporate-ipsum"},
		{filterFile: "32kb-32", includeFile: "lorem-ipsum", excludeFile: "corporate-ipsum"},
		{filterFile: "96kb-16", includeFile: "lorem-ipsum", excludeFile: "corporate-ipsum"},
		{filterFile: "96kb-08", includeFile: "lorem-ipsum", excludeFile: "corporate-ipsum"},
		{filterFile: "96kb-24", includeFile: "lorem-ipsum", excludeFile: "corporate-ipsum"},
		{filterFile: "96kb-32", includeFile: "lorem-ipsum", excludeFile: "corporate-ipsum"},
		{filterFile: "128kb-16", includeFile: "lorem-ipsum", excludeFile: "corporate-ipsum"},
		{filterFile: "128kb-08", includeFile: "lorem-ipsum", excludeFile: "corporate-ipsum"},
		{filterFile: "128kb-24", includeFile: "lorem-ipsum", excludeFile: "corporate-ipsum"},
		{filterFile: "128kb-32", includeFile: "lorem-ipsum", excludeFile: "corporate-ipsum"},
		{filterFile: "emojis", includeFile: "emojis", excludeFile: "lorem-ipsum"},
	}

	for _, testCase := range testCases {
		name := fmt.Sprintf("filter=%s", testCase.filterFile)

		t.Run(name, func(t *testing.T) {
			test, err := Decode(readTestFilter(t, "stress", testCase.filterFile))
			if err != nil {
				t.Fatalf("unexpected error decoding filter: %s", err)
			}

			for _, v := range readTestWords(t, testCase.includeFile) {
				if !test(v) {
					t.Errorf("expected %s to be in bloom filter", v)
				}
			}

			for _, v := range readTestWords(t, testCase.excludeFile) {
				if test(v) {
					t.Errorf("expected %s not to be in bloom filter", v)
				}
			}
		})
	}
}
