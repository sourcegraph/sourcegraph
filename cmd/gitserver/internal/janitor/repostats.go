package janitor

import "time"

type RepositoryStats struct {
	DiskSize uint64
	// if true, a corruption event should be logged and the repo should be deleted.
	MissingHead bool
	// if true, a corruption event should be logged and the repo should be deleted.
	NonBare bool
	// if true, a corruption event should have been logged and the repo should be deleted.
	SawCorruptionInCommand bool
	// If non-zero, GC failed recently. If this is non-zero, the repo should be
	// deleted eventually.
	GCFailedAt time.Time
}
