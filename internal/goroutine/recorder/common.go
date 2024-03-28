package recorder

import (
	"math"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/rcache"
)

// JobInfo contains information about a job, including all its routines.
type JobInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Routines []RoutineInfo
}

// RoutineInfo contains information about a routine.
type RoutineInfo struct {
	Name        string      `json:"name"`
	Type        RoutineType `json:"type"`
	JobName     string      `json:"jobName"`
	Description string      `json:"description"`
	IntervalMs  int32       `json:"intervalMs"` // Assumes that the routine runs at a fixed interval across all hosts.
	Instances   []RoutineInstanceInfo
	RecentRuns  []RoutineRun
	Stats       RoutineRunStats
}

// serializableRoutineInfo represents a single routine in a job, and is used for serialization in Redis.
type serializableRoutineInfo struct {
	Name        string        `json:"name"`
	Type        RoutineType   `json:"type"`
	JobName     string        `json:"jobName"`
	Description string        `json:"description"`
	Interval    time.Duration `json:"interval"`
}

// RoutineInstanceInfo contains information about a routine instance.
// That is, a single version that's running (or ran) on a single node.
type RoutineInstanceInfo struct {
	HostName      string     `json:"hostName"`
	LastStartedAt *time.Time `json:"lastStartedAt"`
	LastStoppedAt *time.Time `json:"LastStoppedAt"`
}

// RoutineRun contains information about a single run of a routine.
// That is, a single action that a running instance of a routine performed.
type RoutineRun struct {
	At           time.Time `json:"at"`
	HostName     string    `json:"hostname"`
	DurationMs   int32     `json:"durationMs"`
	ErrorMessage string    `json:"errorMessage"`
}

// RoutineRunStats contains statistics about a routine.
type RoutineRunStats struct {
	Since         time.Time `json:"since"`
	RunCount      int32     `json:"runCount"`
	ErrorCount    int32     `json:"errorCount"`
	MinDurationMs int32     `json:"minDurationMs"`
	AvgDurationMs int32     `json:"avgDurationMs"`
	MaxDurationMs int32     `json:"maxDurationMs"`
}

type RoutineType string

const (
	PeriodicRoutine     RoutineType = "PERIODIC"
	PeriodicWithMetrics RoutineType = "PERIODIC_WITH_METRICS"
	DBBackedRoutine     RoutineType = "DB_BACKED"
	CustomRoutine       RoutineType = "CUSTOM"
)

const ttlSeconds = 604800 // 7 days

func GetCache() *rcache.Cache {
	return rcache.NewWithTTL(keyPrefix, ttlSeconds, false)
}

// mergeStats returns the given stats updated with the given run data.
func mergeStats(a RoutineRunStats, b RoutineRunStats) RoutineRunStats {
	// Calculate earlier "since"
	var since time.Time
	if a.Since.IsZero() {
		since = b.Since
	}
	if b.Since.IsZero() {
		since = a.Since
	}
	if !a.Since.IsZero() && !b.Since.IsZero() && a.Since.Before(b.Since) {
		since = a.Since
	}

	// Calculate durations
	var minDurationMs int32
	if a.MinDurationMs == 0 || b.MinDurationMs < a.MinDurationMs {
		minDurationMs = b.MinDurationMs
	} else {
		minDurationMs = a.MinDurationMs
	}
	avgDurationMs := int32(math.Round((float64(a.AvgDurationMs)*float64(a.RunCount) + float64(b.AvgDurationMs)*float64(b.RunCount)) / (float64(a.RunCount) + float64(b.RunCount))))
	var maxDurationMs int32
	if b.MaxDurationMs > a.MaxDurationMs {
		maxDurationMs = b.MaxDurationMs
	} else {
		maxDurationMs = a.MaxDurationMs
	}

	// Return merged stats
	return RoutineRunStats{
		Since:         since,
		RunCount:      a.RunCount + b.RunCount,
		ErrorCount:    a.ErrorCount + b.ErrorCount,
		MinDurationMs: minDurationMs,
		AvgDurationMs: avgDurationMs,
		MaxDurationMs: maxDurationMs,
	}
}
