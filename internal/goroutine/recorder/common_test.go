package recorder

import (
	"testing"
	"time"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/stretchr/testify/assert"
)

// TestLoggerAndReaderHappyPaths tests pretty much everything in the happy path of both the logger and the log reader.
func TestLoggerAndReaderHappyPaths(t *testing.T) {
	rcache.SetupForTest(t)

	// Create logger
	c := rcache.NewWithTTL(keyPrefix, 1)
	recorder := New(log.NoOp(), "test", c)

	// Create routines
	routine1 := newRoutineMock("routine-1", "a routine", 2*time.Minute)
	routine1.SetJobName("job-1")
	routine2 := newRoutineMock("routine-2", "another routine", 2*time.Minute)
	routine2.SetJobName("job-1")
	routine3 := newRoutineMock("routine-3", "a third routine", 2*time.Minute)
	routine3.SetJobName("job-2")

	// Register routines
	recorder.Register(routine1)
	recorder.Register(routine2)
	recorder.Register(routine3)
	recorder.RegistrationDone()

	// Get infos
	jobInfos, err := GetBackgroundJobInfos(c, "", 5, 7)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, jobInfos, 2)
	assert.Equal(t, "job-1", jobInfos[0].ID)
	assert.Equal(t, "job-1", jobInfos[0].Name)
	assert.Equal(t, 2, len(jobInfos[0].Routines))
	assert.Equal(t, "a routine", jobInfos[0].Routines[0].Description)
	assert.Equal(t, 1, len(jobInfos[0].Routines[0].Instances))
	assert.Equal(t, "test", jobInfos[0].Routines[0].Instances[0].HostName)
	assertRoutineStats(t, jobInfos[0].Routines[0], "routine-1", false, false, 0, 0, 0, 0, 0, 0)
	assertRoutineStats(t, jobInfos[0].Routines[1], "routine-2", false, false, 0, 0, 0, 0, 0, 0)
	assertRoutineStats(t, jobInfos[1].Routines[0], "routine-3", false, false, 0, 0, 0, 0, 0, 0)

	// Log some runs: 3x routine-1 (1x with error), 200x routine-2, 0x routine-3 (and stops)
	recorder.LogStart(routine1)
	recorder.LogStart(routine2)
	recorder.LogStart(routine3)
	recorder.LogRun(routine1, 10*time.Millisecond, nil)
	recorder.LogRun(routine1, 20*time.Millisecond, errors.New("test error"))
	for range 100 { // Make sure int32 overflow doesn't happen
		recorder.LogRun(routine2, 10*time.Hour, nil)
		recorder.LogRun(routine2, 20*time.Hour, nil)
	}
	recorder.LogStop(routine3)

	// Get infos again
	jobInfos, err = GetBackgroundJobInfos(c, "", 5, 7)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, jobInfos, 2)
	assertRoutineStats(t, jobInfos[0].Routines[0], "routine-1", true, false, 2, 2, 1, 10, 15, 20)
	assertRoutineStats(t, jobInfos[0].Routines[1], "routine-2", true, false, 5, 200, 0, 1000*60*60*10, 1500*60*60*10, 2000*60*60*10)
	assertRoutineStats(t, jobInfos[1].Routines[0], "routine-3", true, true, 0, 0, 0, 0, 0, 0)
}

func assertRoutineStats(t *testing.T, r RoutineInfo, name string,
	started bool, stopped bool, rRuns int, sRuns int32, sErrors int32, sMin int32, sAvg int32, sMax int32,
) {
	assert.Equal(t, name, r.Name)
	if started {
		assert.NotNil(t, r.Instances[0].LastStartedAt)
	} else {
		assert.Nil(t, r.Instances[0].LastStartedAt)
	}
	assert.Equal(t, rRuns, len(r.RecentRuns))
	assert.Equal(t, sRuns, r.Stats.RunCount)
	assert.Equal(t, sErrors, r.Stats.ErrorCount)
	assert.Equal(t, sMin, r.Stats.MinDurationMs)
	assert.Equal(t, sAvg, r.Stats.AvgDurationMs)
	assert.Equal(t, sMax, r.Stats.MaxDurationMs)
	if stopped {
		assert.NotNil(t, r.Instances[0].LastStoppedAt)
	} else {
		assert.Nil(t, r.Instances[0].LastStoppedAt)
	}
}

type RoutineMock struct {
	name        string
	description string
	jobName     string
	interval    time.Duration
}

var _ Recordable = &RoutineMock{}

func newRoutineMock(name string, description string, interval time.Duration) *RoutineMock {
	return &RoutineMock{
		name:        name,
		description: description,
		interval:    interval,
	}
}

func (r *RoutineMock) Start() {
	// Do nothing
}

func (r *RoutineMock) Stop() {
	// Do nothing
}

func (r *RoutineMock) Name() string {
	return r.name
}

func (r *RoutineMock) Type() RoutineType {
	return CustomRoutine
}

func (r *RoutineMock) JobName() string {
	return r.jobName
}

func (r *RoutineMock) SetJobName(jobName string) {
	r.jobName = jobName
}

func (r *RoutineMock) Description() string {
	return r.description
}

func (r *RoutineMock) Interval() time.Duration {
	return r.interval
}

func (r *RoutineMock) RegisterRecorder(*Recorder) {
	// Do nothing
}
