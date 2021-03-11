package state

import (
	"github.com/sourcegraph/sourcegraph/internal/batches"
)

// ChangesetEvents is a collection of changeset events
type ChangesetEvents []*batches.ChangesetEvent

func (ce ChangesetEvents) Len() int      { return len(ce) }
func (ce ChangesetEvents) Swap(i, j int) { ce[i], ce[j] = ce[j], ce[i] }

// Less sorts changeset events by their Timestamps
func (ce ChangesetEvents) Less(i, j int) bool {
	return ce[i].Timestamp().Before(ce[j].Timestamp())
}
