package syncer

import (
	"time"

	"github.com/sourcegraph/sourcegraph/internal/batches"
)

var (
	minSyncDelay = 2 * time.Minute
	maxSyncDelay = 8 * time.Hour
)

// NextSync computes the time we want the next sync to happen.
func NextSync(clock func() time.Time, h *batches.ChangesetSyncData) time.Time {
	lastSync := h.UpdatedAt

	if lastSync.IsZero() {
		// Edge case where we've never synced
		return clock()
	}

	var lastChange time.Time
	// When we perform a sync, event timestamps are all updated even if nothing has changed.
	// We should fall back to h.ExternalUpdated if the diff is small
	// TODO: This is a workaround while we try to implement syncing without always updating events. See: https://github.com/sourcegraph/sourcegraph/pull/8771
	// Once the above issue is fixed we can simply use maxTime(h.ExternalUpdatedAt, h.LatestEvent)
	if diff := h.LatestEvent.Sub(lastSync); !h.LatestEvent.IsZero() && absDuration(diff) < minSyncDelay {
		lastChange = h.ExternalUpdatedAt
	} else {
		lastChange = maxTime(h.ExternalUpdatedAt, h.LatestEvent)
	}

	// Simple linear backoff for now
	diff := lastSync.Sub(lastChange)

	// If the last change has happened AFTER our last sync this indicates a webhook
	// has arrived. In this case, we should check again in minSyncDelay after
	// the hook arrived. If multiple webhooks arrive in close succession this will
	// cause us to wait for a quiet period of at least minSyncDelay
	if diff < 0 {
		return lastChange.Add(minSyncDelay)
	}

	if diff > maxSyncDelay {
		diff = maxSyncDelay
	}
	if diff < minSyncDelay {
		diff = minSyncDelay
	}
	return lastSync.Add(diff)
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func absDuration(d time.Duration) time.Duration {
	if d >= 0 {
		return d
	}
	return -1 * d
}
