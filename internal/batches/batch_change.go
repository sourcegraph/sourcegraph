package batches

import "time"

// BatchChangeState defines the possible states of a BatchChange
type BatchChangeState string

const (
	BatchChangeStateAny    BatchChangeState = "ANY"
	BatchChangeStateOpen   BatchChangeState = "OPEN"
	BatchChangeStateClosed BatchChangeState = "CLOSED"
)

// A BatchChange of changesets over multiple Repos over time.
type BatchChange struct {
	ID          int64
	Name        string
	Description string

	BatchSpecID int64

	InitialApplierID int32
	LastApplierID    int32
	LastAppliedAt    time.Time

	NamespaceUserID int32
	NamespaceOrgID  int32

	ClosedAt time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Clone returns a clone of a BatchChange.
func (c *BatchChange) Clone() *BatchChange {
	cc := *c
	return &cc
}

// Closed returns true when the ClosedAt timestamp has been set.
func (c *BatchChange) Closed() bool { return !c.ClosedAt.IsZero() }
