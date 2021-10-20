package lsifstore

import (
	"testing"

	"github.com/hexops/autogold"

	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol"
)

// Note: You can `go test ./pkg -update` to update the expected `want` values in these tests.
// See https://github.com/hexops/autogold for more information.

func TestSerializeDocumentationPageData(t *testing.T) {
	testCases := []struct {
		page *precise.DocumentationPageData
		want autogold.Value
	}{
		{
			page: &precise.DocumentationPageData{
				Tree: &precise.DocumentationNode{
					PathID: "/",
					Children: []precise.DocumentationNodeChild{
						{PathID: "/somelinkedpage"},
						{Node: &precise.DocumentationNode{
							PathID: "/#main",
						}},
						{PathID: "/somelinkedpage2"},
						{Node: &precise.DocumentationNode{
							PathID: "/subpkg",
							Children: []precise.DocumentationNodeChild{
								{Node: &precise.DocumentationNode{
									PathID: "/subpkg#Router",
								}},
							},
						}},
					},
				},
			},
			want: autogold.Want("basic", nil),
		},
		{
			page: &precise.DocumentationPageData{
				Tree: &precise.DocumentationNode{
					PathID: "/",
					Children: nil,
				},
			},
			want: autogold.Want("nil children would be encoded as JSON array not null", nil),
		},
		{
			page: &precise.DocumentationPageData{
				Tree: &precise.DocumentationNode{
					PathID: "/",
					Documentation: protocol.Documentation{},
				},
			},
			want: autogold.Want("nil Documentation.Tags would be encoded as JSON array not null", nil),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.want.Name(), func(t *testing.T) {
			serializer := NewSerializer()
			encoded, err := serializer.MarshalDocumentationPageData(tc.page)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := serializer.UnmarshalDocumentationPageData(encoded)
			if err != nil {
				t.Fatal(err)
			}
			got := decoded
			tc.want.Equal(t, got)
		})
	}
}

// 	expected := precise.DocumentData{
// 		Ranges: map[precise.ID]precise.RangeData{
// 			precise.ID("7864"): {
// 				StartLine:             541,
// 				StartCharacter:        10,
// 				EndLine:               541,
// 				EndCharacter:          12,
// 				DefinitionResultID:    precise.ID("1266"),
// 				ReferenceResultID:     precise.ID("15871"),
// 				HoverResultID:         precise.ID("1269"),
// 				DocumentationResultID: precise.ID("3372"),
// 				MonikerIDs:            nil,
// 			},
// 			precise.ID("8265"): {
// 				StartLine:             266,
// 				StartCharacter:        10,
// 				EndLine:               266,
// 				EndCharacter:          16,
// 				DefinitionResultID:    precise.ID("311"),
// 				ReferenceResultID:     precise.ID("15500"),
// 				HoverResultID:         precise.ID("317"),
// 				DocumentationResultID: precise.ID("3372"),
// 				MonikerIDs:            []precise.ID{precise.ID("314")},
// 			},
// 		},
// 		HoverResults: map[precise.ID]string{
// 			precise.ID("1269"): "```go\nvar id string\n```",
// 			precise.ID("317"):  "```go\ntype Vertex struct\n```\n\n---\n\nVertex contains information of a vertex in the graph.\n\n---\n\n```go\nstruct {\n    Element\n    Label VertexLabel \"json:\\\"label\\\"\"\n}\n```",
// 		},
// 		Monikers: map[precise.ID]precise.MonikerData{
// 			precise.ID("314"): {
// 				Kind:                 "export",
// 				Scheme:               "gomod",
// 				Identifier:           "github.com/sourcegraph/lsif-go/protocol:Vertex",
// 				PackageInformationID: precise.ID("213"),
// 			},
// 			precise.ID("2494"): {
// 				Kind:                 "export",
// 				Scheme:               "gomod",
// 				Identifier:           "github.com/sourcegraph/lsif-go/protocol:VertexLabel",
// 				PackageInformationID: precise.ID("213"),
// 			},
// 		},
// 		PackageInformation: map[precise.ID]precise.PackageInformationData{
// 			precise.ID("213"): {
// 				Name:    "github.com/sourcegraph/lsif-go",
// 				Version: "v0.0.0-ad3507cbeb18",
// 			},
// 		},
// 	}

// 	t.Run("current", func(t *testing.T) {
// 		serializer := NewSerializer()

// 		recompressed, err := serializer.MarshalDocumentData(expected)
// 		if err != nil {
// 			t.Fatalf("unexpected error marshalling document data: %s", err)
// 		}

// 		roundtripActual, err := serializer.UnmarshalDocumentData(recompressed)
// 		if err != nil {
// 			t.Fatalf("unexpected error unmarshalling document data: %s", err)
// 		}

// 		if diff := cmp.Diff(expected, roundtripActual); diff != "" {
// 			t.Errorf("unexpected document data (-want +got):\n%s", diff)
// 		}
// 	})

// 	t.Run("legacy", func(t *testing.T) {
// 		serializer := NewSerializer()

// 		recompressed, err := serializer.MarshalLegacyDocumentData(expected)
// 		if err != nil {
// 			t.Fatalf("unexpected error marshalling document data: %s", err)
// 		}

// 		roundtripActual, err := serializer.UnmarshalLegacyDocumentData(recompressed)
// 		if err != nil {
// 			t.Fatalf("unexpected error unmarshalling document data: %s", err)
// 		}

// 		if diff := cmp.Diff(expected, roundtripActual); diff != "" {
// 			t.Errorf("unexpected document data (-want +got):\n%s", diff)
// 		}
// 	})
// }

// func TestResultChunkData(t *testing.T) {
// 	expected := precise.ResultChunkData{
// 		DocumentPaths: map[precise.ID]string{
// 			precise.ID("4"):   "internal/gomod/module.go",
// 			precise.ID("302"): "protocol/protocol.go",
// 			precise.ID("305"): "protocol/writer.go",
// 		},
// 		DocumentIDRangeIDs: map[precise.ID][]precise.DocumentIDRangeID{
// 			precise.ID("34"): {
// 				{DocumentID: precise.ID("4"), RangeID: precise.ID("31")},
// 			},
// 			precise.ID("14040"): {
// 				{DocumentID: precise.ID("3978"), RangeID: precise.ID("4544")},
// 			},
// 			precise.ID("14051"): {
// 				{DocumentID: precise.ID("3978"), RangeID: precise.ID("4568")},
// 				{DocumentID: precise.ID("3978"), RangeID: precise.ID("9224")},
// 				{DocumentID: precise.ID("3978"), RangeID: precise.ID("9935")},
// 				{DocumentID: precise.ID("3978"), RangeID: precise.ID("9996")},
// 			},
// 		},
// 	}

// 	serializer := NewSerializer()

// 	recompressed, err := serializer.MarshalResultChunkData(expected)
// 	if err != nil {
// 		t.Fatalf("unexpected error marshalling result chunk data: %s", err)
// 	}

// 	roundtripActual, err := serializer.UnmarshalResultChunkData(recompressed)
// 	if err != nil {
// 		t.Fatalf("unexpected error unmarshalling result chunk data: %s", err)
// 	}

// 	if diff := cmp.Diff(expected, roundtripActual); diff != "" {
// 		t.Errorf("unexpected document data (-want +got):\n%s", diff)
// 	}
// }

// func TestLocations(t *testing.T) {
// 	expected := []precise.LocationData{
// 		{
// 			URI:            "internal/index/indexer.go",
// 			StartLine:      36,
// 			StartCharacter: 26,
// 			EndLine:        36,
// 			EndCharacter:   32,
// 		},
// 		{
// 			URI:            "protocol/writer.go",
// 			StartLine:      100,
// 			StartCharacter: 9,
// 			EndLine:        100,
// 			EndCharacter:   15,
// 		},
// 		{
// 			URI:            "protocol/writer.go",
// 			StartLine:      115,
// 			StartCharacter: 9,
// 			EndLine:        115,
// 			EndCharacter:   15,
// 		},
// 		{
// 			URI:            "protocol/writer.go",
// 			StartLine:      95,
// 			StartCharacter: 9,
// 			EndLine:        95,
// 			EndCharacter:   15,
// 		},
// 		{
// 			URI:            "protocol/writer.go",
// 			StartLine:      130,
// 			StartCharacter: 9,
// 			EndLine:        130,
// 			EndCharacter:   15,
// 		},
// 		{
// 			URI:            "protocol/writer.go",
// 			StartLine:      155,
// 			StartCharacter: 9,
// 			EndLine:        155,
// 			EndCharacter:   15,
// 		},
// 		{
// 			URI:            "protocol/writer.go",
// 			StartLine:      80,
// 			StartCharacter: 9,
// 			EndLine:        80,
// 			EndCharacter:   15,
// 		},
// 		{
// 			URI:            "protocol/writer.go",
// 			StartLine:      36,
// 			StartCharacter: 9,
// 			EndLine:        36,
// 			EndCharacter:   15,
// 		},
// 		{
// 			URI:            "protocol/writer.go",
// 			StartLine:      135,
// 			StartCharacter: 9,
// 			EndLine:        135,
// 			EndCharacter:   15,
// 		},
// 		{
// 			URI:            "protocol/writer.go",
// 			StartLine:      12,
// 			StartCharacter: 5,
// 			EndLine:        12,
// 			EndCharacter:   11,
// 		},
// 	}

// 	serializer := NewSerializer()

// 	recompressed, err := serializer.MarshalLocations(expected)
// 	if err != nil {
// 		t.Fatalf("unexpected error marshalling locations: %s", err)
// 	}

// 	roundtripActual, err := serializer.UnmarshalLocations(recompressed)
// 	if err != nil {
// 		t.Fatalf("unexpected error unmarshalling locations: %s", err)
// 	}

// 	if diff := cmp.Diff(expected, roundtripActual); diff != "" {
// 		t.Errorf("unexpected locations (-want +got):\n%s", diff)
// 	}
// }
