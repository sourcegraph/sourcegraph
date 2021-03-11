package conversion

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/datastructures"
)

func TestCanonicalizeDocuments(t *testing.T) {
	state := &State{
		DocumentData: map[int]string{
			1001: "main.go",
			1002: "foo.go",
			1003: "bar.go",
			1004: "main.go",
		},
		DefinitionData: map[int]*datastructures.DefaultIDSetMap{
			2001: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{1001: datastructures.IDSetWith(3005)}),
			2002: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{1002: datastructures.IDSetWith(3006), 1004: datastructures.IDSetWith(3007)}),
		},
		ReferenceData: map[int]*datastructures.DefaultIDSetMap{
			2003: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{1001: datastructures.IDSetWith(3008)}),
			2004: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{1003: datastructures.IDSetWith(3009), 1004: datastructures.IDSetWith(3010)}),
		},
		Contains: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
			1001: datastructures.IDSetWith(3001),
			1002: datastructures.IDSetWith(3002),
			1003: datastructures.IDSetWith(3003),
			1004: datastructures.IDSetWith(3004),
		}),
		Monikers:    datastructures.NewDefaultIDSetMap(),
		Diagnostics: datastructures.NewDefaultIDSetMap(),
	}
	canonicalizeDocuments(state)

	expectedState := &State{
		DocumentData: map[int]string{
			1001: "main.go",
			1002: "foo.go",
			1003: "bar.go",
		},
		DefinitionData: map[int]*datastructures.DefaultIDSetMap{
			2001: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{1001: datastructures.IDSetWith(3005)}),
			2002: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{1002: datastructures.IDSetWith(3006), 1001: datastructures.IDSetWith(3007)}),
		},
		ReferenceData: map[int]*datastructures.DefaultIDSetMap{
			2003: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{1001: datastructures.IDSetWith(3008)}),
			2004: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{1003: datastructures.IDSetWith(3009), 1001: datastructures.IDSetWith(3010)}),
		},
		Contains: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
			1001: datastructures.IDSetWith(3001, 3004),
			1002: datastructures.IDSetWith(3002),
			1003: datastructures.IDSetWith(3003),
		}),
		Monikers:    datastructures.NewDefaultIDSetMap(),
		Diagnostics: datastructures.NewDefaultIDSetMap(),
	}

	if diff := cmp.Diff(expectedState, state, datastructures.Comparers...); diff != "" {
		t.Errorf("unexpected state (-want +got):\n%s", diff)
	}
}

func TestCanonicalizeReferenceResults(t *testing.T) {
	linkedReferenceResults := datastructures.NewDisjointIDSet()
	linkedReferenceResults.Link(2001, 2003)

	state := &State{
		RangeData: map[int]Range{
			3001: {ReferenceResultID: 2002},
			3002: {ReferenceResultID: 2003},
		},
		ResultSetData: map[int]ResultSet{
			5003: {ReferenceResultID: 2003},
			5004: {ReferenceResultID: 2004},
		},
		ReferenceData: map[int]*datastructures.DefaultIDSetMap{
			2001: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
				1001: datastructures.IDSetWith(3005),
			}),
			2002: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
				1002: datastructures.IDSetWith(3006),
				1004: datastructures.IDSetWith(3007),
			}),
			2003: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
				1001: datastructures.IDSetWith(3008),
				1003: datastructures.IDSetWith(3009),
			}),
			2004: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
				1004: datastructures.IDSetWith(3010),
			}),
		},
		LinkedReferenceResults: linkedReferenceResults,
	}
	canonicalizeReferenceResults(state)

	expectedState := &State{
		RangeData: map[int]Range{
			3001: {ReferenceResultID: 2002},
			3002: {ReferenceResultID: 2001},
		},
		ResultSetData: map[int]ResultSet{
			5003: {ReferenceResultID: 2001},
			5004: {ReferenceResultID: 2004},
		},
		ReferenceData: map[int]*datastructures.DefaultIDSetMap{
			2001: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
				1001: datastructures.IDSetWith(3005, 3008),
				1003: datastructures.IDSetWith(3009),
			}),
			2002: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
				1002: datastructures.IDSetWith(3006),
				1004: datastructures.IDSetWith(3007),
			}),
			2004: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
				1004: datastructures.IDSetWith(3010),
			}),
		},

		LinkedReferenceResults: linkedReferenceResults,
	}

	if diff := cmp.Diff(expectedState, state, datastructures.Comparers...); diff != "" {
		t.Errorf("unexpected state (-want +got):\n%s", diff)
	}
}

func TestCanonicalizeResultSets(t *testing.T) {
	linkedMonikers := datastructures.NewDisjointIDSet()
	linkedMonikers.Link(4002, 4005)

	state := &State{
		ResultSetData: map[int]ResultSet{
			5001: {
				DefinitionResultID: 0,
				ReferenceResultID:  0,
				HoverResultID:      0,
			},
			5002: {
				DefinitionResultID: 2001,
				ReferenceResultID:  2002,
				HoverResultID:      2003,
			},
			5003: {
				DefinitionResultID: 2004,
				ReferenceResultID:  2005,
				HoverResultID:      0,
			},
			5004: {
				DefinitionResultID: 2006,
				ReferenceResultID:  2007,
				HoverResultID:      0,
			},
			5005: {
				DefinitionResultID: 0,
				ReferenceResultID:  2008,
				HoverResultID:      2008,
			},
		},
		NextData: map[int]int{
			5001: 5004,
			5003: 5005,
			5004: 5005,
		},
		LinkedMonikers: linkedMonikers,
		Monikers: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
			5001: datastructures.IDSetWith(4001),
			5002: datastructures.IDSetWith(4002),
			5003: datastructures.IDSetWith(4003),
			5004: datastructures.IDSetWith(4004),
			5005: datastructures.IDSetWith(4005),
		}),
	}
	canonicalizeResultSets(state)

	expectedState := &State{
		ResultSetData: map[int]ResultSet{
			5001: {
				DefinitionResultID: 2006,
				ReferenceResultID:  2007,
				HoverResultID:      2008,
			},
			5002: {
				DefinitionResultID: 2001,
				ReferenceResultID:  2002,
				HoverResultID:      2003,
			},
			5003: {
				DefinitionResultID: 2004,
				ReferenceResultID:  2005,
				HoverResultID:      2008,
			},
			5004: {
				DefinitionResultID: 2006,
				ReferenceResultID:  2007,
				HoverResultID:      2008,
			},
			5005: {
				DefinitionResultID: 0,
				ReferenceResultID:  2008,
				HoverResultID:      2008,
			},
		},
		NextData:       map[int]int{},
		LinkedMonikers: linkedMonikers,
		Monikers: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
			5001: datastructures.IDSetWith(4001, 4002, 4004, 4005),
			5002: datastructures.IDSetWith(4002, 4005),
			5003: datastructures.IDSetWith(4002, 4003, 4005),
			5004: datastructures.IDSetWith(4002, 4004, 4005),
			5005: datastructures.IDSetWith(4002, 4005),
		}),
	}

	if diff := cmp.Diff(expectedState, state, datastructures.Comparers...); diff != "" {
		t.Errorf("unexpected state (-want +got):\n%s", diff)
	}
}

func TestCanonicalizeRanges(t *testing.T) {
	linkedMonikers := datastructures.NewDisjointIDSet()
	linkedMonikers.Link(4002, 4005)

	state := &State{
		RangeData: map[int]Range{
			3001: {
				DefinitionResultID: 0,
				ReferenceResultID:  0,
				HoverResultID:      0,
			},
			3002: {
				DefinitionResultID: 2001,
				ReferenceResultID:  2002,
				HoverResultID:      2003,
			},
			3003: {
				DefinitionResultID: 2004,
				ReferenceResultID:  2005,
				HoverResultID:      0,
			},
		},
		ResultSetData: map[int]ResultSet{
			5001: {
				DefinitionResultID: 2006,
				ReferenceResultID:  2007,
				HoverResultID:      0,
			},
			5002: {
				DefinitionResultID: 0,
				ReferenceResultID:  2008,
				HoverResultID:      2008,
			},
		},
		NextData: map[int]int{
			3001: 5001,
			3003: 5002,
		},
		LinkedMonikers: linkedMonikers,
		Contains:       datastructures.NewDefaultIDSetMap(),
		Monikers: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
			3001: datastructures.IDSetWith(4001),
			3002: datastructures.IDSetWith(4002),
			3003: datastructures.IDSetWith(4003),
			5001: datastructures.IDSetWith(4004),
			5002: datastructures.IDSetWith(4005),
		}),
		Diagnostics: datastructures.NewDefaultIDSetMap(),
	}
	canonicalizeRanges(state)

	expectedState := &State{
		RangeData: map[int]Range{
			3001: {
				DefinitionResultID: 2006,
				ReferenceResultID:  2007,
				HoverResultID:      0,
			},
			3002: {
				DefinitionResultID: 2001,
				ReferenceResultID:  2002,
				HoverResultID:      2003,
			},
			3003: {
				DefinitionResultID: 2004,
				ReferenceResultID:  2005,
				HoverResultID:      2008,
			},
		},
		ResultSetData: map[int]ResultSet{
			5001: {
				DefinitionResultID: 2006,
				ReferenceResultID:  2007,
				HoverResultID:      0,
			},
			5002: {
				DefinitionResultID: 0,
				ReferenceResultID:  2008,
				HoverResultID:      2008,
			},
		},
		NextData:       map[int]int{},
		LinkedMonikers: linkedMonikers,
		Contains:       datastructures.NewDefaultIDSetMap(),
		Monikers: datastructures.DefaultIDSetMapWith(map[int]*datastructures.IDSet{
			3001: datastructures.IDSetWith(4001, 4004),
			3002: datastructures.IDSetWith(4002, 4005),
			3003: datastructures.IDSetWith(4002, 4003, 4005),
			5001: datastructures.IDSetWith(4004),
			5002: datastructures.IDSetWith(4005),
		}),
		Diagnostics: datastructures.NewDefaultIDSetMap(),
	}

	if diff := cmp.Diff(expectedState, state, datastructures.Comparers...); diff != "" {
		t.Errorf("unexpected state (-want +got):\n%s", diff)
	}
}
