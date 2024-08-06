package build

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/buildkite/go-buildkite/v3/buildkite"
	"github.com/go-redsync/redsync/v4"
	"github.com/redis/go-redis/v9"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/dev/build-tracker/notify"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// Build keeps track of a buildkite.Build and it's associated jobs and pipeline.
// See BuildStore for where jobs are added to the build.
type Build struct {
	// Build is the buildkite.Build currently being executed by buildkite on a particular Pipeline
	buildkite.Build `json:"build"`

	// Pipeline is a wrapped buildkite.Pipeline that is running this build.
	Pipeline *Pipeline `json:"pipeline"`

	// steps is a map that keeps track of all the buildkite.Jobs associated with this build.
	// Each step keeps track of jobs associated with that step. Every job is wrapped to allow
	// for safer access to fields of the buildkite.Jobs. The name of the job is used as the key
	Steps map[string]*Step `json:"steps"`

	// ConsecutiveFailure indicates whether this build is the nth consecutive failure.
	ConsecutiveFailure int `json:"consecutiveFailures"`
}

type Step struct {
	Name string `json:"steps"`
	Jobs []*Job `json:"jobs"`
}

// Implement the notify.JobLine interface
var _ notify.JobLine = &Step{}

func (s *Step) Title() string {
	return s.Name
}

func (s *Step) LogURL() string {
	return s.LastJob().WebURL
}

// BuildStatus is the status of the build. The status is determined by the final status of contained Jobs of the build
type BuildStatus string

const (
	// The following are statuses we consider the build to be in
	BuildStatusUnknown BuildStatus = ""
	BuildPassed        BuildStatus = "Passed"
	BuildFailed        BuildStatus = "Failed"
	BuildFixed         BuildStatus = "Fixed"

	EventJobFinished   = "job.finished"
	EventBuildFinished = "build.finished"

	// The following are states the job received from buildkite can be in. These are terminal states
	JobFinishedState = "finished"
	JobPassedState   = "passed"
	JobFailedState   = "failed"
	JobTimedOutState = "timed_out"
	JobUnknnownState = "unknown"
)

func (b *Build) AddJob(j *Job) error {
	stepName := j.GetName()
	if stepName == "" {
		return errors.Newf("job %q name is empty", j.GetID())
	}
	step, ok := b.Steps[stepName]
	// We don't know about this step, so it must be a new one
	if !ok {
		step = NewStep(stepName)
		b.Steps[step.Name] = step
	}
	step.Jobs = append(step.Jobs, j)
	return nil
}

// updateFromEvent updates the current build with the build and pipeline from the event.
func (b *Build) updateFromEvent(e *Event) {
	b.Build = e.Build
	b.Pipeline = e.WrappedPipeline()
}

func (b *Build) IsFailed() bool {
	return b.GetState() == "failed"
}

func (b *Build) IsFinished() bool {
	switch b.GetState() {
	case "passed", "failed", "blocked", "canceled":
		return true
	default:
		return false
	}
}

func (b *Build) IsReleaseBuild() bool {
	// Release builds have two environment variables which distinguishes between internal / public releases
	for _, key := range []string{"RELEASE_PUBLIC", "RELEASE_INTERNAL"} {
		if v, ok := b.Env[key]; ok && v == "true" {
			return true
		}
	}

	return false
}

func (b *Build) GetBuildAuthor() buildkite.Author {
	var author buildkite.Author
	if b.Creator == nil {
		return author
	}

	author.Name = b.Creator.Name
	author.Email = b.Creator.Email
	return author
}

func (b *Build) GetCommitAuthor() buildkite.Author {
	return pointers.DerefZero(b.Author)
}

func (b *Build) GetWebURL() string {
	return pointers.DerefZero(b.WebURL)
}

func (b *Build) GetState() string {
	return pointers.DerefZero(b.State)
}

func (b *Build) GetCommit() string {
	return pointers.DerefZero(b.Commit)
}

func (b *Build) GetNumber() int {
	return pointers.DerefZero(b.Number)
}

func (b *Build) GetBranch() string {
	return pointers.DerefZero(b.Branch)
}

func (b *Build) GetMessage() string {
	return pointers.DerefZero(b.Message)
}

func (b *Build) AppendSteps(steps map[string]*Step) {
	for name, step := range steps {
		b.Steps[name] = step
	}
}

// Pipeline wraps a buildkite.Pipeline and provides convenience functions to access values of the wrapped pipeline in a safe maner
type Pipeline struct {
	buildkite.Pipeline `json:"pipeline"`
}

func (p *Pipeline) GetName() string {
	return pointers.DerefZero(p.Name)
}

// Event contains information about a buildkite event. Each event contains the build, pipeline, and job. Note that when the event
// is `build.*` then Job will be empty.
type Event struct {
	// Name is the name of the buildkite event that got triggered
	Name string `json:"event"`
	// Build is the buildkite.Build that triggered this event
	Build buildkite.Build `json:"build,omitempty"`
	// Pipeline is the buildkite.Pipeline that is running the build that triggered this event
	Pipeline buildkite.Pipeline `json:"pipeline,omitempty"`
	// Job is the current job being executed by the Build. When the event is not a job event variant, then this job will be empty
	Job buildkite.Job `json:"job,omitempty"`
	// Agent is the agent that is running the job that triggered this event. When the event is not an agent event variant, then this will be empty
	Agent buildkite.Agent `json:"agent,omitempty"`
}

func (b *Event) WrappedBuild() *Build {
	build := &Build{
		Build:    b.Build,
		Pipeline: b.WrappedPipeline(),
		Steps:    make(map[string]*Step),
	}

	return build
}

func (b *Event) WrappedJob() *Job {
	return &Job{Job: b.Job}
}

func (b *Event) WrappedPipeline() *Pipeline {
	return &Pipeline{Pipeline: b.Pipeline}
}

func (b *Event) IsBuildFinished() bool {
	return b.Name == EventBuildFinished
}

func (b *Event) IsJobFinished() bool {
	return b.Name == EventJobFinished
}

func (b *Event) GetJobName() string {
	return pointers.DerefZero(b.Job.Name)
}

func (b *Event) GetBuildNumber() int {
	return pointers.DerefZero(b.Build.Number)
}

// Store is a thread safe store which keeps track of Builds described by buildkite build events.
//
// The store is backed by a map and the build number is used as the key.
// When a build event is added the Buildkite Build, Pipeline and Job is extracted, if available. If the Build does not exist, Buildkite is wrapped
// in a Build and added to the map. When the event contains a Job the corresponding job is retrieved from the map and added to the Job it is for.
type Store struct {
	logger log.Logger

	r  RedisClient
	m1 Locker
}

type Locker interface {
	Lock() error
	Unlock() (bool, error)
}

type RedisClient interface {
	redis.StringCmdable
	redis.GenericCmdable
}

func NewBuildStore(logger log.Logger, rclient RedisClient, lock Locker) *Store {
	return &Store{
		logger: logger.Scoped("store"),

		r:  rclient,
		m1: lock,
	}
}

func (s *Store) lock() (func(), error) {
	for {
		err := s.m1.Lock()
		if err == redsync.ErrFailed {
			time.Sleep(100 * time.Millisecond)
			continue
		} else if err != nil {
			s.logger.Error("failed to acquire lock", log.Error(err))
			return nil, err
		}
		break
	}
	return func() {
		if _, err := s.m1.Unlock(); err != nil {
			s.logger.Error("failed to unlock", log.Error(err))
		}
	}, nil
}

func (s *Store) Add(event *Event) {
	unlock, err := s.lock()
	if err != nil {
		return
	}
	defer unlock()

	buildb, err := s.r.Get(context.Background(), "build/"+strconv.Itoa(event.GetBuildNumber())).Bytes()
	if err != nil && err != redis.Nil {
		s.logger.Error("failed to get build from redis", log.Error(err))
		return
	}

	var build *Build
	if err == nil {
		if err := json.Unmarshal(buildb, &build); err != nil {
			s.logger.Error("failed to unmarshal build", log.Error(err))
			return
		}
	}

	// if we don't know about this build, convert it and add it to the store
	if err == redis.Nil {
		build = event.WrappedBuild()
	}

	// write out the build to redis at the end, once all mutations are applied
	defer func() {
		buildb, _ = json.Marshal(build)
		s.r.Set(context.Background(), "build/"+strconv.Itoa(event.GetBuildNumber()), buildb, 0)
	}()

	// if the build is finished replace the original build with the replaced one since it
	// will be more up to date, and tack on some finalized data
	if event.IsBuildFinished() {
		build.updateFromEvent(event)

		s.logger.Debug("build finished", log.Int("buildNumber", event.GetBuildNumber()),
			log.Int("totalSteps", len(build.Steps)),
			log.String("status", build.GetState()))

		// If the build was triggered from another build, we need to update the "trigger-er" with the jobs
		// from the triggered build. This is so that any failures from the triggered build are reported as
		// failures in the triggerer.
		// We do this because we do not rely on the state of the build to determine if a build is "successful" or not.
		// We instead depend on the state of the jobs associated with said build.
		if event.Build.TriggeredFrom != nil {
			parentBuildb, err := s.r.Get(context.Background(), "build/"+strconv.Itoa(*event.Build.TriggeredFrom.BuildNumber)).Bytes()
			if err == nil {
				var parentBuild *Build
				if err := json.Unmarshal(parentBuildb, &parentBuild); err != nil {
					s.logger.Error("failed to unmarshal build", log.Error(err))
					return
				}
				parentBuild.AppendSteps(build.Steps)
				buildb, _ = json.Marshal(parentBuild)
				s.r.Set(context.Background(), "build/"+strconv.Itoa(event.GetBuildNumber()), buildb, 0)
			} else if err == redis.Nil {
				// If the triggered build doesn't exist, we'll just leave log a message
				s.logger.Warn(
					"build triggered from non-existent build",
					log.Int("buildNumber", event.GetBuildNumber()),
					log.String("pipeline", *event.Build.TriggeredFrom.BuildPipelineSlug),
					log.Int("triggeredFrom", *event.Build.TriggeredFrom.BuildNumber),
				)
			}
		}

		// Track consecutive failures by pipeline + branch
		// We update the global count of consecutiveFailures then we set the count on the individual build
		// if we get a pass, we reset the global count of consecutiveFailures
		failuresKey := fmt.Sprintf("%s/%s", build.Pipeline.GetName(), build.GetBranch())
		if build.IsFailed() {
			i, _ := s.r.Incr(context.Background(), failuresKey).Result()
			build.ConsecutiveFailure = int(i)
		} else {
			// We got a pass, reset the global count
			s.r.Set(context.Background(), failuresKey, 0, 0).Result()
		}
	}

	// Keep track of the job, if there is one
	newJob := event.WrappedJob()
	err = build.AddJob(newJob)
	if err != nil {
		s.logger.Warn("job not added",
			log.Error(err),
			log.Int("buildNumber", event.GetBuildNumber()),
			log.Object("job",
				log.String("name", newJob.GetName()),
				log.String("id", newJob.GetID()),
				log.String("status", string(newJob.status())),
				log.Int("exit", newJob.exitStatus())),
			log.Int("totalSteps", len(build.Steps)),
		)
	} else {
		s.logger.Debug("job added to step",
			log.Int("buildNumber", event.GetBuildNumber()),
			log.Object("step", log.String("name", newJob.GetName()),
				log.Object("job",
					log.String("name", newJob.GetName()),
					log.String("id", newJob.GetID()),
					log.String("status", string(newJob.status())),
					log.Int("exit", newJob.exitStatus())),
			),
			log.Int("totalSteps", len(build.Steps)),
		)
	}
}

func (s *Store) Set(build *Build) {
	unlock, err := s.lock()
	if err != nil {
		return
	}
	defer unlock()

	buildb, _ := json.Marshal(build)
	s.r.Set(context.Background(), "build/"+strconv.Itoa(*build.Number), buildb, 0)
}

func (s *Store) GetByBuildNumber(num int) *Build {
	unlock, err := s.lock()
	if err != nil {
		return nil
	}
	defer unlock()

	buildb, err := s.r.Get(context.Background(), "build/"+strconv.Itoa(num)).Bytes()
	if err != nil && err != redis.Nil {
		s.logger.Error("failed to get build from redis", log.Error(err))
		return nil
	}

	var build *Build
	if err == nil {
		if err := json.Unmarshal(buildb, &build); err != nil {
			s.logger.Error("failed to unmarshal build", log.Error(err))
			return nil
		}
	}
	return build
}

func (s *Store) DelByBuildNumber(buildNumbers ...int) {
	unlock, err := s.lock()
	if err != nil {
		return
	}
	defer unlock()

	nums := make([]string, 0, len(buildNumbers))
	for _, num := range buildNumbers {
		nums = append(nums, "build/"+strconv.Itoa(num))
	}

	s.r.Del(context.Background(), nums...)

	s.logger.Info("deleted builds", log.Int("totalBuilds", len(buildNumbers)))
}

func (s *Store) FinishedBuilds() []*Build {
	unlock, err := s.lock()
	if err != nil {
		return nil
	}
	defer unlock()

	buildKeys, err := s.r.Keys(context.Background(), "build/*").Result()
	if err != nil {
		s.logger.Error("failed to get build keys", log.Error(err))
		return nil
	}

	builds := make([]*Build, 0, len(buildKeys))

	values, err := s.r.MGet(context.Background(), buildKeys...).Result()
	if err != nil {
		s.logger.Error("failed to get build values", log.Error(err))
		return nil
	}

	for _, value := range values {
		if value == nil {
			continue
		}
		var build *Build
		if err := json.Unmarshal([]byte(value.(string)), &build); err != nil {
			s.logger.Error("failed to unmarshal build", log.Error(err))
			continue
		}
		builds = append(builds, build)
	}

	finished := make([]*Build, 0)
	for _, b := range builds {
		if b.IsFinished() {
			s.logger.Debug("build is finished", log.Int("buildNumber", b.GetNumber()), log.String("state", b.GetState()))
			finished = append(finished, b)
		}
	}

	return finished
}
