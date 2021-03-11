package batches

import "time"

type ChangesetSyncState struct {
	BaseRefOid string
	HeadRefOid string

	// This is essentially the result of c.ExternalState != BatchChangeStateOpen
	// the last time a sync occured. We use this to short circuit computing the
	// sync state if the changeset remains closed.
	IsComplete bool
}

func (state *ChangesetSyncState) Equals(old *ChangesetSyncState) bool {
	return state.BaseRefOid == old.BaseRefOid && state.HeadRefOid == old.HeadRefOid && state.IsComplete == old.IsComplete
}

// ChangesetSyncData represents data about the sync status of a changeset
type ChangesetSyncData struct {
	ChangesetID int64
	// UpdatedAt is the time we last updated / synced the changeset in our DB
	UpdatedAt time.Time
	// LatestEvent is the time we received the most recent changeset event
	LatestEvent time.Time
	// ExternalUpdatedAt is the time the external changeset last changed
	ExternalUpdatedAt time.Time
	// RepoExternalServiceID is the external_service_id in the repo table, usually
	// represented by the code host URL
	RepoExternalServiceID string
}
