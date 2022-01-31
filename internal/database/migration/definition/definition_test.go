package definition

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
)

func TestDefinitionGetByID(t *testing.T) {
	definitions := []Definition{
		{ID: 1, UpQuery: sqlf.Sprintf(`SELECT 1;`)},
		{ID: 2, UpQuery: sqlf.Sprintf(`SELECT 2;`), Parents: []int{1}},
		{ID: 3, UpQuery: sqlf.Sprintf(`SELECT 3;`), Parents: []int{2}},
		{ID: 4, UpQuery: sqlf.Sprintf(`SELECT 4;`), Parents: []int{3}},
		{ID: 5, UpQuery: sqlf.Sprintf(`SELECT 5;`), Parents: []int{4}},
	}

	definition, ok := newDefinitions(definitions).GetByID(3)
	if !ok {
		t.Fatalf("expected definition")
	}

	if diff := cmp.Diff(definitions[2], definition, queryComparer); diff != "" {
		t.Errorf("unexpected definition (-want, +got):\n%s", diff)
	}
}

func TestLeaves(t *testing.T) {
	definitions := []Definition{
		{ID: 1, UpQuery: sqlf.Sprintf(`SELECT 1;`)},
		{ID: 2, UpQuery: sqlf.Sprintf(`SELECT 2;`), Parents: []int{1}},
		{ID: 3, UpQuery: sqlf.Sprintf(`SELECT 3;`), Parents: []int{2}},
		{ID: 4, UpQuery: sqlf.Sprintf(`SELECT 4;`), Parents: []int{2}},
		{ID: 5, UpQuery: sqlf.Sprintf(`SELECT 5;`), Parents: []int{3, 4}},
		{ID: 6, UpQuery: sqlf.Sprintf(`SELECT 6;`), Parents: []int{5}},
		{ID: 7, UpQuery: sqlf.Sprintf(`SELECT 7;`), Parents: []int{5}},
		{ID: 8, UpQuery: sqlf.Sprintf(`SELECT 8;`), Parents: []int{5, 6}},
		{ID: 9, UpQuery: sqlf.Sprintf(`SELECT 9;`), Parents: []int{5, 8}},
	}

	expectedLeaves := []Definition{
		definitions[6],
		definitions[8],
	}
	if diff := cmp.Diff(expectedLeaves, newDefinitions(definitions).Leaves(), queryComparer); diff != "" {
		t.Errorf("unexpected leaves (-want, +got):\n%s", diff)
	}
}

func TestFilter(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		definitions := []Definition{
			{ID: 1, UpQuery: sqlf.Sprintf(`SELECT 1;`)},
			{ID: 2, UpQuery: sqlf.Sprintf(`SELECT 2;`), Parents: []int{1}},
			{ID: 3, UpQuery: sqlf.Sprintf(`SELECT 3;`), Parents: []int{2}},
			{ID: 4, UpQuery: sqlf.Sprintf(`SELECT 4;`), Parents: []int{2}},
			{ID: 5, UpQuery: sqlf.Sprintf(`SELECT 5;`), Parents: []int{3}},
			{ID: 6, UpQuery: sqlf.Sprintf(`SELECT 6;`), Parents: []int{4}},
		}

		filtered, err := newDefinitions(definitions).Filter([]int{})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if count := len(filtered.All()); count != 0 {
			t.Fatalf("unexpected count. want=%d have=%d", 0, count)
		}
	})

	t.Run("prefix", func(t *testing.T) {
		definitions := []Definition{
			{ID: 1, UpQuery: sqlf.Sprintf(`SELECT 1;`)},
			{ID: 2, UpQuery: sqlf.Sprintf(`SELECT 2;`), Parents: []int{1}},
			{ID: 3, UpQuery: sqlf.Sprintf(`SELECT 3;`), Parents: []int{2}},
			{ID: 4, UpQuery: sqlf.Sprintf(`SELECT 4;`), Parents: []int{2}},
			{ID: 5, UpQuery: sqlf.Sprintf(`SELECT 5;`), Parents: []int{3}},
			{ID: 6, UpQuery: sqlf.Sprintf(`SELECT 6;`), Parents: []int{4}},
		}

		filtered, err := newDefinitions(definitions).Filter([]int{1, 2, 4})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		expectedDefinitions := []Definition{
			definitions[0],
			definitions[1],
			definitions[3],
		}
		if diff := cmp.Diff(expectedDefinitions, filtered.All(), queryComparer); diff != "" {
			t.Errorf("unexpected definitions (-want, +got):\n%s", diff)
		}
	})

	t.Run("incomplete subtree", func(t *testing.T) {
		definitions := []Definition{
			{ID: 1, UpQuery: sqlf.Sprintf(`SELECT 1;`)},
			{ID: 2, UpQuery: sqlf.Sprintf(`SELECT 2;`), Parents: []int{1}},
			{ID: 3, UpQuery: sqlf.Sprintf(`SELECT 3;`), Parents: []int{2}},
			{ID: 4, UpQuery: sqlf.Sprintf(`SELECT 4;`), Parents: []int{2}},
			{ID: 5, UpQuery: sqlf.Sprintf(`SELECT 5;`), Parents: []int{3}},
			{ID: 6, UpQuery: sqlf.Sprintf(`SELECT 6;`), Parents: []int{4}},
		}

		expectedErrorMessage := "migration 5 (included) references parent migration 3 (excluded)"
		if _, err := newDefinitions(definitions).Filter([]int{1, 2, 5}); err == nil || !strings.Contains(err.Error(), expectedErrorMessage) {
			t.Fatalf("unexpected error: want=%q have=%q", expectedErrorMessage, err)
		}
	})
}

func TestLeafDominator(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if _, ok := newDefinitions([]Definition{}).LeafDominator(); ok {
			t.Fatalf("unexpected definition")
		}
	})

	t.Run("single leaf", func(t *testing.T) {
		definitions := []Definition{
			{ID: 1, UpQuery: sqlf.Sprintf(`SELECT 1;`)},
			{ID: 2, UpQuery: sqlf.Sprintf(`SELECT 2;`), Parents: []int{1}},
			{ID: 3, UpQuery: sqlf.Sprintf(`SELECT 3;`), Parents: []int{2}},
			{ID: 4, UpQuery: sqlf.Sprintf(`SELECT 4;`), Parents: []int{1}},
			{ID: 5, UpQuery: sqlf.Sprintf(`SELECT 5;`), Parents: []int{4}},
			{ID: 6, UpQuery: sqlf.Sprintf(`SELECT 6;`), Parents: []int{3, 5}},
		}

		definition, ok := newDefinitions(definitions).LeafDominator()
		if !ok {
			t.Fatalf("expected a definition")
		}

		if diff := cmp.Diff(definitions[5], definition, queryComparer); diff != "" {
			t.Errorf("unexpected leave dominataor (-want, +got):\n%s", diff)
		}
	})

	t.Run("multiple leaves (simple)", func(t *testing.T) {
		definitions := []Definition{
			{ID: 1, UpQuery: sqlf.Sprintf(`SELECT 1;`)},
			{ID: 2, UpQuery: sqlf.Sprintf(`SELECT 2;`), Parents: []int{1}},
			{ID: 3, UpQuery: sqlf.Sprintf(`SELECT 3;`), Parents: []int{2}},
			{ID: 4, UpQuery: sqlf.Sprintf(`SELECT 4;`), Parents: []int{3}},
			{ID: 5, UpQuery: sqlf.Sprintf(`SELECT 5;`), Parents: []int{3}},
		}

		definition, ok := newDefinitions(definitions).LeafDominator()
		if !ok {
			t.Fatalf("expected a definition")
		}

		if diff := cmp.Diff(definitions[2], definition, queryComparer); diff != "" {
			t.Errorf("unexpected leave dominataor (-want, +got):\n%s", diff)
		}
	})

	t.Run("multiple leaves (complex)", func(t *testing.T) {
		definitions := []Definition{
			{ID: 1, UpQuery: sqlf.Sprintf(`SELECT 1;`)},
			{ID: 2, UpQuery: sqlf.Sprintf(`SELECT 2;`), Parents: []int{1}},
			{ID: 3, UpQuery: sqlf.Sprintf(`SELECT 3;`), Parents: []int{1}},
			{ID: 4, UpQuery: sqlf.Sprintf(`SELECT 4;`), Parents: []int{2, 3}},
			{ID: 5, UpQuery: sqlf.Sprintf(`SELECT 5;`), Parents: []int{4}},
			{ID: 6, UpQuery: sqlf.Sprintf(`SELECT 6;`), Parents: []int{4}},
			{ID: 7, UpQuery: sqlf.Sprintf(`SELECT 7;`), Parents: []int{5}},
			{ID: 8, UpQuery: sqlf.Sprintf(`SELECT 8;`), Parents: []int{7}},
		}

		definition, ok := newDefinitions(definitions).LeafDominator()
		if !ok {
			t.Fatalf("expected a definition")
		}

		if diff := cmp.Diff(definitions[3], definition, queryComparer); diff != "" {
			t.Errorf("unexpected leave dominataor (-want, +got):\n%s", diff)
		}
	})
}

func TestUp(t *testing.T) {
	definitions := []Definition{
		{ID: 1, UpQuery: sqlf.Sprintf(`SELECT 1;`)},
		{ID: 2, UpQuery: sqlf.Sprintf(`SELECT 2;`), Parents: []int{1}},
		{ID: 3, UpQuery: sqlf.Sprintf(`SELECT 3;`), Parents: []int{2}},
		{ID: 4, UpQuery: sqlf.Sprintf(`SELECT 4;`), Parents: []int{2}},
		{ID: 5, UpQuery: sqlf.Sprintf(`SELECT 5;`), Parents: []int{3, 4}},
		{ID: 6, UpQuery: sqlf.Sprintf(`SELECT 6;`), Parents: []int{5}},
		{ID: 7, UpQuery: sqlf.Sprintf(`SELECT 7;`), Parents: []int{5}},
		{ID: 8, UpQuery: sqlf.Sprintf(`SELECT 8;`), Parents: []int{5, 6}},
		{ID: 9, UpQuery: sqlf.Sprintf(`SELECT 9;`), Parents: []int{5, 8}},
		{ID: 10, UpQuery: sqlf.Sprintf(`SELECT 10;`), Parents: []int{7, 9}},
	}

	for _, testCase := range []struct {
		name                string
		appliedIDs          []int
		targetIDs           []int
		expectedDefinitions []Definition
	}{
		{"empty", nil, nil, []Definition{}},
		{"empty to leaf", nil, []int{10}, definitions},
		{"empty to internal node", nil, []int{7}, append(append([]Definition(nil), definitions[0:5]...), definitions[6])},
		{"already applied", []int{1, 2, 3, 4, 5, 6, 8}, []int{8}, []Definition{}},
		{"partially applied", []int{1, 4, 5, 8}, []int{8}, append(append([]Definition(nil), definitions[1:3]...), definitions[5])},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			definitions, err := newDefinitions(definitions).Up(testCase.appliedIDs, testCase.targetIDs)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if diff := cmp.Diff(testCase.expectedDefinitions, definitions, queryComparer); diff != "" {
				t.Errorf("unexpected definitions (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestDown(t *testing.T) {
	definitions := []Definition{
		{ID: 1, UpQuery: sqlf.Sprintf(`SELECT 1;`)},
		{ID: 2, UpQuery: sqlf.Sprintf(`SELECT 2;`), Parents: []int{1}},
		{ID: 3, UpQuery: sqlf.Sprintf(`SELECT 3;`), Parents: []int{2}},
		{ID: 4, UpQuery: sqlf.Sprintf(`SELECT 4;`), Parents: []int{2}},
		{ID: 5, UpQuery: sqlf.Sprintf(`SELECT 5;`), Parents: []int{3, 4}},
		{ID: 6, UpQuery: sqlf.Sprintf(`SELECT 6;`), Parents: []int{5}},
		{ID: 7, UpQuery: sqlf.Sprintf(`SELECT 7;`), Parents: []int{5}},
		{ID: 8, UpQuery: sqlf.Sprintf(`SELECT 8;`), Parents: []int{5, 6}},
		{ID: 9, UpQuery: sqlf.Sprintf(`SELECT 9;`), Parents: []int{5, 8}},
		{ID: 10, UpQuery: sqlf.Sprintf(`SELECT 10;`), Parents: []int{7, 9}},
	}

	reverse := func(definitions []Definition) []Definition {
		reversed := make([]Definition, 0, len(definitions))
		for i := len(definitions) - 1; i >= 0; i-- {
			reversed = append(reversed, definitions[i])
		}

		return reversed
	}

	for _, testCase := range []struct {
		name                string
		appliedIDs          []int
		targetIDs           []int
		expectedDefinitions []Definition
	}{
		{"empty", nil, nil, []Definition{}},
		{"unapply dominator", []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, []int{5}, reverse(definitions[5:])},
		{"unapply non-dominator (1)", []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, []int{6}, reverse(definitions[7:])},
		{"unapply non-dominator (2)", []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, []int{7}, reverse(definitions[9:])},
		{"partial unapplied", []int{1, 2, 3, 4, 5, 6, 7, 10}, []int{5}, reverse(append(append([]Definition(nil), definitions[5:7]...), definitions[9]))},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			definitions, err := newDefinitions(definitions).Down(testCase.appliedIDs, testCase.targetIDs)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if diff := cmp.Diff(testCase.expectedDefinitions, definitions, queryComparer); diff != "" {
				t.Errorf("unexpected definitions (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestUpTo(t *testing.T) {
	definitions := newDefinitions([]Definition{
		{ID: 11, UpFilename: "11.up.sql"},
		{ID: 12, UpFilename: "12.up.sql"},
		{ID: 13, UpFilename: "13.up.sql"},
		{ID: 14, UpFilename: "14.up.sql"},
		{ID: 15, UpFilename: "15.up.sql"},
	})

	t.Run("zero", func(t *testing.T) {
		// middle of sequence
		ds, err := definitions.UpTo(12, 0)
		if err != nil {
			t.Fatalf("unexpected error")
		}

		var definitionIDs []int
		for _, definition := range ds {
			definitionIDs = append(definitionIDs, definition.ID)
		}

		expectedIDs := []int{13, 14, 15}
		if diff := cmp.Diff(expectedIDs, definitionIDs); diff != "" {
			t.Fatalf("unexpected ids (-want +got):\n%s", diff)
		}
	})

	t.Run("with limit", func(t *testing.T) {
		// directly before sequence
		ds, err := definitions.UpTo(10, 12)
		if err != nil {
			t.Fatalf("unexpected error")
		}

		var definitionIDs []int
		for _, definition := range ds {
			definitionIDs = append(definitionIDs, definition.ID)
		}

		expectedIDs := []int{11, 12}
		if diff := cmp.Diff(expectedIDs, definitionIDs); diff != "" {
			t.Fatalf("unexpected ids (-want +got):\n%s", diff)
		}
	})

	t.Run("missing migrations", func(t *testing.T) {
		// missing migration 10
		if _, err := definitions.UpTo(9, 12); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("wrong direction", func(t *testing.T) {
		if _, err := definitions.UpTo(14, 12); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestUpFrom(t *testing.T) {
	definitions := newDefinitions([]Definition{
		{ID: 11, UpFilename: "11.up.sql"},
		{ID: 12, UpFilename: "12.up.sql"},
		{ID: 13, UpFilename: "13.up.sql"},
		{ID: 14, UpFilename: "14.up.sql"},
		{ID: 15, UpFilename: "15.up.sql"},
	})

	t.Run("no limit", func(t *testing.T) {
		// middle of sequence
		ds, err := definitions.UpFrom(12, 0)
		if err != nil {
			t.Fatalf("unexpected error")
		}

		var definitionIDs []int
		for _, definition := range ds {
			definitionIDs = append(definitionIDs, definition.ID)
		}

		expectedIDs := []int{13, 14, 15}
		if diff := cmp.Diff(expectedIDs, definitionIDs); diff != "" {
			t.Fatalf("unexpected ids (-want +got):\n%s", diff)
		}
	})

	t.Run("empty", func(t *testing.T) {
		// after sequence
		ds, err := definitions.UpFrom(16, 0)
		if err != nil {
			t.Fatalf("unexpected error")
		}
		if len(ds) != 0 {
			t.Fatalf("expected no definitions")
		}
	})

	t.Run("with limit", func(t *testing.T) {
		// directly before sequence
		ds, err := definitions.UpFrom(10, 2)
		if err != nil {
			t.Fatalf("unexpected error")
		}

		var definitionIDs []int
		for _, definition := range ds {
			definitionIDs = append(definitionIDs, definition.ID)
		}

		expectedIDs := []int{11, 12}
		if diff := cmp.Diff(expectedIDs, definitionIDs); diff != "" {
			t.Fatalf("unexpected ids (-want +got):\n%s", diff)
		}
	})

	t.Run("missing migrations", func(t *testing.T) {
		// missing migration 10
		if _, err := definitions.UpFrom(9, 2); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestDownTo(t *testing.T) {
	definitions := newDefinitions([]Definition{
		{ID: 11, UpFilename: "11.up.sql"},
		{ID: 12, UpFilename: "12.up.sql"},
		{ID: 13, UpFilename: "13.up.sql"},
		{ID: 14, UpFilename: "14.up.sql"},
		{ID: 15, UpFilename: "15.up.sql"},
	})

	t.Run("zero", func(t *testing.T) {
		if _, err := definitions.DownTo(14, 0); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("with limit", func(t *testing.T) {
		// end of sequence
		ds, err := definitions.DownTo(15, 13)
		if err != nil {
			t.Fatalf("unexpected error")
		}

		var definitionIDs []int
		for _, definition := range ds {
			definitionIDs = append(definitionIDs, definition.ID)
		}

		expectedIDs := []int{15, 14}
		if diff := cmp.Diff(expectedIDs, definitionIDs); diff != "" {
			t.Fatalf("unexpected ids (-want +got):\n%s", diff)
		}
	})

	t.Run("missing migrations", func(t *testing.T) {
		// missing migration 16
		if _, err := definitions.DownTo(16, 14); err == nil {
			t.Fatalf("expected error %v", err)
		}
	})

	t.Run("wrong direction", func(t *testing.T) {
		if _, err := definitions.DownTo(12, 14); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestDownFrom(t *testing.T) {
	definitions := newDefinitions([]Definition{
		{ID: 11, UpFilename: "11.up.sql"},
		{ID: 12, UpFilename: "12.up.sql"},
		{ID: 13, UpFilename: "13.up.sql"},
		{ID: 14, UpFilename: "14.up.sql"},
		{ID: 15, UpFilename: "15.up.sql"},
	})

	t.Run("zero", func(t *testing.T) {
		// middle of sequence
		ds, err := definitions.DownFrom(14, 0)
		if err != nil {
			t.Fatalf("unexpected error")
		}
		if len(ds) != 0 {
			var definitionIDs []int
			for _, definition := range ds {
				definitionIDs = append(definitionIDs, definition.ID)
			}

			t.Fatalf("expected no definitions, got %v", definitionIDs)
		}
	})

	t.Run("empty", func(t *testing.T) {
		// before sequence
		ds, err := definitions.DownFrom(9, 0)
		if err != nil {
			t.Fatalf("unexpected error")
		}
		if len(ds) != 0 {
			t.Fatalf("expected no definitions")
		}
	})

	t.Run("with limit", func(t *testing.T) {
		// end of sequence
		ds, err := definitions.DownFrom(15, 2)
		if err != nil {
			t.Fatalf("unexpected error")
		}

		var definitionIDs []int
		for _, definition := range ds {
			definitionIDs = append(definitionIDs, definition.ID)
		}

		expectedIDs := []int{15, 14}
		if diff := cmp.Diff(expectedIDs, definitionIDs); diff != "" {
			t.Fatalf("unexpected ids (-want +got):\n%s", diff)
		}
	})

	t.Run("missing migrations", func(t *testing.T) {
		// missing migration 16
		if _, err := definitions.DownFrom(16, 2); err == nil {
			t.Fatalf("expected error %v", err)
		}
	})
}
