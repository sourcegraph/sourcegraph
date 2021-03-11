package reconciler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/batches"
)

var operationPrecedence = map[batches.ReconcilerOperation]int{
	batches.ReconcilerOperationPush:         0,
	batches.ReconcilerOperationDetach:       0,
	batches.ReconcilerOperationImport:       1,
	batches.ReconcilerOperationPublish:      1,
	batches.ReconcilerOperationPublishDraft: 1,
	batches.ReconcilerOperationClose:        1,
	batches.ReconcilerOperationReopen:       2,
	batches.ReconcilerOperationUndraft:      3,
	batches.ReconcilerOperationUpdate:       4,
	batches.ReconcilerOperationSleep:        5,
	batches.ReconcilerOperationSync:         6,
}

type Operations []batches.ReconcilerOperation

func (ops Operations) IsNone() bool {
	return len(ops) == 0
}

func (ops Operations) Equal(b Operations) bool {
	if len(ops) != len(b) {
		return false
	}
	bEntries := make(map[batches.ReconcilerOperation]struct{})
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

func (ops Operations) ExecutionOrder() []batches.ReconcilerOperation {
	uniqueOps := []batches.ReconcilerOperation{}

	// Make sure ops are unique.
	seenOps := make(map[batches.ReconcilerOperation]struct{})
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

// Plan represents the possible operations the reconciler needs to do
// to reconcile the current and the desired state of a changeset.
type Plan struct {
	// The changeset that is targeted in this plan.
	Changeset *batches.Changeset

	// The changeset spec that is used in this plan.
	ChangesetSpec *batches.ChangesetSpec

	// The operations that need to be done to reconcile the changeset.
	Ops Operations

	// The Delta between a possible previous ChangesetSpec and the current
	// ChangesetSpec.
	Delta *ChangesetSpecDelta
}

func (p *Plan) AddOp(op batches.ReconcilerOperation) { p.Ops = append(p.Ops, op) }
func (p *Plan) SetOp(op batches.ReconcilerOperation) { p.Ops = Operations{op} }

// DeterminePlan looks at the given changeset to determine what action the
// reconciler should take.
// It consumes the current and the previous changeset spec, if they exist. If
// the current ChangesetSpec is not applied to a batch change, it returns an
// error.
func DeterminePlan(previousSpec, currentSpec *batches.ChangesetSpec, ch *batches.Changeset) (*Plan, error) {
	pl := &Plan{
		Changeset:     ch,
		ChangesetSpec: currentSpec,
	}

	wantDetach := false
	isStillAttached := false
	wantDetachFromOwnerBatchChange := false
	for _, assoc := range ch.BatchChanges {
		if assoc.Detach {
			wantDetach = true
			if assoc.BatchChangeID == ch.OwnedByBatchChangeID {
				wantDetachFromOwnerBatchChange = true
			}
		} else {
			isStillAttached = true
		}
	}
	if wantDetach {
		pl.SetOp(batches.ReconcilerOperationDetach)
	}

	if ch.Closing {
		pl.AddOp(batches.ReconcilerOperationClose)
		// Close is a final operation, nothing else should overwrite it.
		return pl, nil
	} else if wantDetachFromOwnerBatchChange {
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
		if ch.Unpublished() && isStillAttached {
			pl.AddOp(batches.ReconcilerOperationImport)
		}
		return pl, nil
	}

	delta, err := compareChangesetSpecs(previousSpec, currentSpec)
	if err != nil {
		return pl, nil
	}
	pl.Delta = delta

	switch ch.PublicationState {
	case batches.ChangesetPublicationStateUnpublished:
		if currentSpec.Spec.Published.True() {
			pl.SetOp(batches.ReconcilerOperationPublish)
			pl.AddOp(batches.ReconcilerOperationPush)
		} else if currentSpec.Spec.Published.Draft() && ch.SupportsDraft() {
			// If configured to be opened as draft, and the changeset supports
			// draft mode, publish as draft. Otherwise, take no action.
			pl.SetOp(batches.ReconcilerOperationPublishDraft)
			pl.AddOp(batches.ReconcilerOperationPush)
		}

	case batches.ChangesetPublicationStatePublished:
		// Don't take any actions for merged changesets.
		if ch.ExternalState == batches.ChangesetExternalStateMerged {
			return pl, nil
		}
		if reopenAfterDetach(ch) {
			pl.SetOp(batches.ReconcilerOperationReopen)
		}

		// Only do undraft, when the codehost supports draft changesets.
		if delta.Undraft && batches.ExternalServiceSupports(ch.ExternalServiceType, batches.CodehostCapabilityDraftChangesets) {
			pl.AddOp(batches.ReconcilerOperationUndraft)
		}

		if delta.AttributesChanged() {
			if delta.NeedCommitUpdate() {
				pl.AddOp(batches.ReconcilerOperationPush)
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
				pl.AddOp(batches.ReconcilerOperationSleep)
				pl.AddOp(batches.ReconcilerOperationSync)
			} else {
				// Otherwise, we need to update the pull request on the code host or, if we
				// need to reopen it, update it to make sure it has the newest state.
				pl.AddOp(batches.ReconcilerOperationUpdate)
			}
		}

	default:
		return pl, fmt.Errorf("unknown changeset publication state: %s", ch.PublicationState)
	}

	return pl, nil
}

func reopenAfterDetach(ch *batches.Changeset) bool {
	closed := ch.ExternalState == batches.ChangesetExternalStateClosed
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

func compareChangesetSpecs(previous, current *batches.ChangesetSpec) (*ChangesetSpecDelta, error) {
	delta := &ChangesetSpecDelta{}

	if previous == nil {
		return delta, nil
	}

	if previous.Spec.Title != current.Spec.Title {
		delta.TitleChanged = true
	}
	if previous.Spec.Body != current.Spec.Body {
		delta.BodyChanged = true
	}
	if previous.Spec.BaseRef != current.Spec.BaseRef {
		delta.BaseRefChanged = true
	}

	// If was set to "draft" and now "true", need to undraft the changeset.
	// We currently ignore going from "true" to "draft".
	if previous.Spec.Published.Draft() && current.Spec.Published.True() {
		delta.Undraft = true
	}

	// Diff
	currentDiff, err := current.Spec.Diff()
	if err != nil {
		return nil, nil
	}
	previousDiff, err := previous.Spec.Diff()
	if err != nil {
		return nil, err
	}
	if previousDiff != currentDiff {
		delta.DiffChanged = true
	}

	// CommitMessage
	currentCommitMessage, err := current.Spec.CommitMessage()
	if err != nil {
		return nil, nil
	}
	previousCommitMessage, err := previous.Spec.CommitMessage()
	if err != nil {
		return nil, err
	}
	if previousCommitMessage != currentCommitMessage {
		delta.CommitMessageChanged = true
	}

	// AuthorName
	currentAuthorName, err := current.Spec.AuthorName()
	if err != nil {
		return nil, nil
	}
	previousAuthorName, err := previous.Spec.AuthorName()
	if err != nil {
		return nil, err
	}
	if previousAuthorName != currentAuthorName {
		delta.AuthorNameChanged = true
	}

	// AuthorEmail
	currentAuthorEmail, err := current.Spec.AuthorEmail()
	if err != nil {
		return nil, nil
	}
	previousAuthorEmail, err := previous.Spec.AuthorEmail()
	if err != nil {
		return nil, err
	}
	if previousAuthorEmail != currentAuthorEmail {
		delta.AuthorEmailChanged = true
	}

	return delta, nil
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
