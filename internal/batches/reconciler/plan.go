package reconciler

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var operationPrecedence = map[btypes.ReconcilerOperation]int{
	btypes.ReconcilerOperationPush:         0,
	btypes.ReconcilerOperationDetach:       0,
	btypes.ReconcilerOperationArchive:      0,
	btypes.ReconcilerOperationReattach:     0,
	btypes.ReconcilerOperationImport:       1,
	btypes.ReconcilerOperationPublish:      1,
	btypes.ReconcilerOperationPublishDraft: 1,
	btypes.ReconcilerOperationClose:        1,
	btypes.ReconcilerOperationReopen:       2,
	btypes.ReconcilerOperationUndraft:      3,
	btypes.ReconcilerOperationUpdate:       4,
	btypes.ReconcilerOperationSleep:        5,
	btypes.ReconcilerOperationSync:         6,
}

type Operations []btypes.ReconcilerOperation

func (ops Operations) IsNone() bool {
	return len(ops) == 0
}

func (ops Operations) Equal(b Operations) bool {
	if len(ops) != len(b) {
		return false
	}
	bEntries := make(map[btypes.ReconcilerOperation]struct{})
	for _, e := range b {
		bEntries[e] = struct{}{}
	}

	for _, op := range ops {
		if _, ok := bEntries[op]; !ok {
			return false
		}
	}

	return true
}

func (ops Operations) String() string {
	if ops.IsNone() {
		return "No operations required"
	}
	eo := ops.ExecutionOrder()
	ss := make([]string, len(eo))
	for i, val := range eo {
		ss[i] = strings.ToLower(string(val))
	}
	return strings.Join(ss, " => ")
}

func (ops Operations) ExecutionOrder() []btypes.ReconcilerOperation {
	uniqueOps := []btypes.ReconcilerOperation{}

	// Make sure ops are unique.
	seenOps := make(map[btypes.ReconcilerOperation]struct{})
	for _, op := range ops {
		if _, ok := seenOps[op]; ok {
			continue
		}

		seenOps[op] = struct{}{}
		uniqueOps = append(uniqueOps, op)
	}

	sort.Slice(uniqueOps, func(i, j int) bool {
		return operationPrecedence[uniqueOps[i]] < operationPrecedence[uniqueOps[j]]
	})

	return uniqueOps
}

func (ops Operations) Contains(op btypes.ReconcilerOperation) bool {
	for _, o := range ops {
		if o == op {
			return true
		}
	}
	return false
}

// Plan represents the possible operations the reconciler needs to do
// to reconcile the current and the desired state of a changeset.
type Plan struct {
	// The changeset that is targeted in this plan.
	Changeset *btypes.Changeset

	// The changeset spec that is used in this plan.
	ChangesetSpec *btypes.ChangesetSpec

	// The operations that need to be done to reconcile the changeset.
	Ops Operations

	// The Delta between a possible previous ChangesetSpec and the current
	// ChangesetSpec.
	Delta *ChangesetSpecDelta
}

func (p *Plan) AddOp(op btypes.ReconcilerOperation) { p.Ops = append(p.Ops, op) }
func (p *Plan) SetOp(op btypes.ReconcilerOperation) { p.Ops = Operations{op} }

// DeterminePlan looks at the given changeset to determine what action the
// reconciler should take.
// It consumes the current and the previous changeset spec, if they exist. If
// the current ChangesetSpec is not applied to a batch change, it returns an
// error.
func DeterminePlan(previousSpec, currentSpec *btypes.ChangesetSpec, currentChangeset, wantedChangeset *btypes.Changeset) (*Plan, error) {
	pl := &Plan{
		Changeset:     wantedChangeset,
		ChangesetSpec: currentSpec,
	}

	wantDetach := false
	wantArchive := false
	isArchived := false
	isStillAttached := false
	isReattach := false
	wantDetachFromOwnerBatchChange := false
	for _, assoc := range wantedChangeset.BatchChanges {
		if assoc.Detach {
			wantDetach = true
			if assoc.BatchChangeID == wantedChangeset.OwnedByBatchChangeID {
				wantDetachFromOwnerBatchChange = true
			}
		} else if assoc.Archive && assoc.BatchChangeID == wantedChangeset.OwnedByBatchChangeID && wantedChangeset.Published() {
			wantArchive = !assoc.IsArchived
			isArchived = assoc.IsArchived
		} else if currentChangeset != nil && len(currentChangeset.BatchChanges) == 0 {
			isReattach = true
		} else {
			isStillAttached = true
		}
	}
	if wantDetach {
		pl.SetOp(btypes.ReconcilerOperationDetach)
	}

	if wantArchive {
		pl.SetOp(btypes.ReconcilerOperationArchive)
	}

	if wantedChangeset.Closing {
		if wantedChangeset.ExternalState != btypes.ChangesetExternalStateReadOnly {
			pl.AddOp(btypes.ReconcilerOperationClose)
		}
		// Close is a final operation, nothing else should overwrite it.
		return pl, nil
	} else if wantDetachFromOwnerBatchChange || wantArchive || isArchived {
		// If the owner batch change detaches the changeset, we don't need to do
		// any additional writing operations, we can just return operation
		// "detach".
		// If some other batch change detached, but the owner batch change
		// didn't, detach, update is a valid combination, since we'll detach
		// from one batch change but still update the changeset because the
		// owning batch change changed the spec.
		return pl, nil
	}

	// If it doesn't have a spec, it's an imported changeset and we can't do
	// anything.
	if currentSpec == nil {
		// If still more than one remains attached, we still want to import the changeset.
		if wantedChangeset.Unpublished() && isStillAttached {
			pl.AddOp(btypes.ReconcilerOperationImport)
		} else if isReattach && !wantDetach {
			pl.AddOp(btypes.ReconcilerOperationReattach)
		}
		return pl, nil
	}

	if previousSpec != nil && isReattach && !wantDetach {
		pl.AddOp(btypes.ReconcilerOperationReattach)
	}

	delta := compareChangesetSpecs(previousSpec, currentSpec, wantedChangeset.UiPublicationState)
	pl.Delta = delta

	switch wantedChangeset.PublicationState {
	case btypes.ChangesetPublicationStateUnpublished:
		calc := calculatePublicationState(currentSpec.Published, wantedChangeset.UiPublicationState)
		if calc.IsPublished() {
			pl.SetOp(btypes.ReconcilerOperationPublish)
			pl.AddOp(btypes.ReconcilerOperationPush)
		} else if calc.IsDraft() && wantedChangeset.SupportsDraft() {
			// If configured to be opened as draft, and the changeset supports
			// draft mode, publish as draft. Otherwise, take no action.
			pl.SetOp(btypes.ReconcilerOperationPublishDraft)
			pl.AddOp(btypes.ReconcilerOperationPush)
		}
		// TODO: test for Published.Nil() and then plan based on the UI
		// publication state. For now, we'll let it fall through and treat it
		// the same as being unpublished.

	case btypes.ChangesetPublicationStatePublished:
		// Don't take any actions for merged or read-only changesets.
		if wantedChangeset.ExternalState == btypes.ChangesetExternalStateMerged ||
			wantedChangeset.ExternalState == btypes.ChangesetExternalStateReadOnly {
			return pl, nil
		}
		if reopenAfterDetach(wantedChangeset) {
			pl.SetOp(btypes.ReconcilerOperationReopen)
		}

		// Figure out if we need to do an undraft, assuming the code host
		// supports draft changesets. This may be due to a new spec being
		// applied, which would mean delta.Undraft is set, or because the UI
		// publication state has been changed, for which we need to compare the
		// current changeset state against the desired state.
		if btypes.ExternalServiceSupports(wantedChangeset.ExternalServiceType, btypes.CodehostCapabilityDraftChangesets) {
			if delta.Undraft {
				pl.AddOp(btypes.ReconcilerOperationUndraft)
			} else if calc := calculatePublicationState(currentSpec.Published, wantedChangeset.UiPublicationState); calc.IsPublished() && wantedChangeset.ExternalState == btypes.ChangesetExternalStateDraft {
				pl.AddOp(btypes.ReconcilerOperationUndraft)
			}
		}

		if delta.AttributesChanged() {
			if delta.NeedCommitUpdate() {
				pl.AddOp(btypes.ReconcilerOperationPush)
			}

			// If we only need to update the diff and we didn't change the state of the changeset,
			// we're done, because we already pushed the commit. We don't need to
			// update anything on the codehost.
			if !delta.NeedCodeHostUpdate() {
				// But we need to sync the changeset so that it has the new commit.
				//
				// The problem: the code host might not have updated the changeset to
				// have the new commit SHA as its head ref oid (and the check states,
				// ...).
				//
				// That's why we give them 3 seconds to update the changesets.
				//
				// Why 3 seconds? Well... 1 or 2 seem to be too short and 4 too long?
				pl.AddOp(btypes.ReconcilerOperationSleep)
				pl.AddOp(btypes.ReconcilerOperationSync)
			} else {
				// Otherwise, we need to update the pull request on the code host or, if we
				// need to reopen it, update it to make sure it has the newest state.
				pl.AddOp(btypes.ReconcilerOperationUpdate)
			}
		}

	default:
		return pl, errors.Errorf("unknown changeset publication state: %s", wantedChangeset.PublicationState)
	}

	return pl, nil
}

func reopenAfterDetach(ch *btypes.Changeset) bool {
	closed := ch.ExternalState == btypes.ChangesetExternalStateClosed ||
		ch.ExternalState == btypes.ChangesetExternalStateReadOnly
	if !closed {
		return false
	}

	// Sanity check: if it's not owned by a batch change, it's simply being tracked.
	if ch.OwnedByBatchChangeID == 0 {
		return false
	}
	// Sanity check 2: if it's marked as to-be-closed, then we don't reopen it.
	if ch.Closing {
		return false
	}

	// At this point the changeset is closed and not marked as to-be-closed.

	// TODO: What if somebody closed the changeset on purpose on the codehost?
	return ch.AttachedTo(ch.OwnedByBatchChangeID)
}

func compareChangesetSpecs(previous, current *btypes.ChangesetSpec, uiPublicationState *btypes.ChangesetUiPublicationState) *ChangesetSpecDelta {
	delta := &ChangesetSpecDelta{}

	if previous == nil {
		return delta
	}

	if previous.Title != current.Title {
		delta.TitleChanged = true
	}
	if previous.Body != current.Body {
		delta.BodyChanged = true
	}
	if previous.BaseRef != current.BaseRef {
		delta.BaseRefChanged = true
	}

	// If was set to "draft" and now "true", need to undraft the changeset.
	// We currently ignore going from "true" to "draft".
	previousCalc := calculatePublicationState(previous.Published, uiPublicationState)
	currentCalc := calculatePublicationState(current.Published, uiPublicationState)
	if previousCalc.IsDraft() && currentCalc.IsPublished() {
		delta.Undraft = true
	}

	// Diff
	currentDiff := current.Diff
	previousDiff := previous.Diff
	if !bytes.Equal(previousDiff, currentDiff) {
		delta.DiffChanged = true
	}

	// CommitMessage
	currentCommitMessage := current.CommitMessage
	previousCommitMessage := previous.CommitMessage
	if previousCommitMessage != currentCommitMessage {
		delta.CommitMessageChanged = true
	}

	// AuthorName
	currentAuthorName := current.CommitAuthorName
	previousAuthorName := previous.CommitAuthorName
	if previousAuthorName != currentAuthorName {
		delta.AuthorNameChanged = true
	}

	// AuthorEmail
	currentAuthorEmail := current.CommitAuthorEmail
	previousAuthorEmail := previous.CommitAuthorEmail
	if previousAuthorEmail != currentAuthorEmail {
		delta.AuthorEmailChanged = true
	}

	return delta
}

type ChangesetSpecDelta struct {
	TitleChanged         bool
	BodyChanged          bool
	Undraft              bool
	BaseRefChanged       bool
	DiffChanged          bool
	CommitMessageChanged bool
	AuthorNameChanged    bool
	AuthorEmailChanged   bool
}

func (d *ChangesetSpecDelta) String() string { return fmt.Sprintf("%#v", d) }

func (d *ChangesetSpecDelta) NeedCommitUpdate() bool {
	return d.DiffChanged || d.CommitMessageChanged || d.AuthorNameChanged || d.AuthorEmailChanged
}

func (d *ChangesetSpecDelta) NeedCodeHostUpdate() bool {
	return d.TitleChanged || d.BodyChanged || d.BaseRefChanged
}

func (d *ChangesetSpecDelta) AttributesChanged() bool {
	return d.NeedCommitUpdate() || d.NeedCodeHostUpdate()
}
