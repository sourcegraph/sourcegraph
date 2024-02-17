package service

import (
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	batcheslib "github.com/sourcegraph/sourcegraph/lib/batches"
)

func TestUiPublicationStates_Add(t *testing.T) {
	var ps UiPublicationStates

	// Add a single publication state, ensuring that ps.rand is initialised.
	if err := ps.Add("foo", batcheslib.PublishedValue{Val: true}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(ps.rand) != 1 {
		t.Errorf("unexpected number of elements: %d", len(ps.rand))
	}

	// Add another publication state.
	if err := ps.Add("bar", batcheslib.PublishedValue{Val: true}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(ps.rand) != 2 {
		t.Errorf("unexpected number of elements: %d", len(ps.rand))
	}

	// Try to add a duplicate publication state.
	if err := ps.Add("bar", batcheslib.PublishedValue{Val: true}); err == nil {
		t.Error("unexpected nil error")
	}
	if len(ps.rand) != 2 {
		t.Errorf("unexpected number of elements: %d", len(ps.rand))
	}
}

func TestUiPublicationStates_get(t *testing.T) {
	var ps UiPublicationStates

	// Verify that an uninitialised UiPublicationStates can have get() called
	// without panicking.
	ps.get(0)

	ps.id = map[int64]*btypes.ChangesetUiPublicationState{
		1: &btypes.ChangesetUiPublicationStateDraft,
		2: &btypes.ChangesetUiPublicationStateUnpublished,
		3: nil,
	}

	for id, want := range map[int64]*btypes.ChangesetUiPublicationState{
		1: &btypes.ChangesetUiPublicationStateDraft,
		2: &btypes.ChangesetUiPublicationStateUnpublished,
		3: nil,
		4: nil,
	} {
		t.Run(strconv.FormatInt(id, 10), func(t *testing.T) {
			if have := ps.get(id); have != want {
				t.Errorf("unexpected result: have=%v want=%v", have, want)
			}
		})
	}
}

func TestUiPublicationStates_prepareAndValidate(t *testing.T) {
	var (
		changesetUI = &btypes.ChangesetSpec{
			ID:        1,
			RandID:    "1",
			Published: batcheslib.PublishedValue{Val: nil},
			Type:      btypes.ChangesetSpecTypeBranch,
		}
		changesetPublished = &btypes.ChangesetSpec{
			ID:        2,
			RandID:    "2",
			Published: batcheslib.PublishedValue{Val: true},
			Type:      btypes.ChangesetSpecTypeBranch,
		}
		changesetUnwired = &btypes.ChangesetSpec{
			ID:        3,
			RandID:    "3",
			Published: batcheslib.PublishedValue{Val: true},
			Type:      btypes.ChangesetSpecTypeBranch,
		}

		mappings = btypes.RewirerMappings{
			{
				// This should be ignored, since it has a zero ChangesetSpecID.
				ChangesetSpecID: 0,
				ChangesetSpec:   changesetUnwired,
			},
			{
				ChangesetSpecID: 1,
				ChangesetSpec:   changesetUI,
			},
			{
				ChangesetSpecID: 2,
				ChangesetSpec:   changesetPublished,
			},
		}
	)

	t.Run("errors", func(t *testing.T) {
		for name, tc := range map[string]struct {
			changesetUIs map[string]batcheslib.PublishedValue
		}{
			"spec not in mappings": {
				changesetUIs: map[string]batcheslib.PublishedValue{
					changesetUnwired.RandID: {Val: true},
				},
			},
			"spec with published field": {
				changesetUIs: map[string]batcheslib.PublishedValue{
					changesetPublished.RandID: {Val: true},
				},
			},
		} {
			t.Run(name, func(t *testing.T) {
				var ps UiPublicationStates
				for rid, pv := range tc.changesetUIs {
					require.NoError(t, ps.Add(rid, pv))
				}

				if err := ps.prepareAndValidate(mappings); err == nil {
					t.Error("unexpected nil error")
				}
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		var ps UiPublicationStates

		require.NoError(t, ps.Add(changesetUI.RandID, batcheslib.PublishedValue{Val: true}))
		if err := ps.prepareAndValidate(mappings); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(ps.rand) != 0 {
			t.Errorf("unexpected elements remaining in ps.rand: %+v", ps.rand)
		}

		want := map[int64]*btypes.ChangesetUiPublicationState{
			changesetUI.ID: &btypes.ChangesetUiPublicationStatePublished,
		}
		if diff := cmp.Diff(want, ps.id); diff != "" {
			t.Errorf("unexpected ps.id (-want +have):\n%s", diff)
		}
	})
}

func TestUiPublicationStates_prepareEmpty(t *testing.T) {
	for name, ps := range map[string]UiPublicationStates{
		"nil":   {},
		"empty": {rand: map[string]batcheslib.PublishedValue{}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ps.prepareAndValidate(btypes.RewirerMappings{}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
