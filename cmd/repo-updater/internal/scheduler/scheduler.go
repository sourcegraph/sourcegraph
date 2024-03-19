package scheduler

import (
	"container/heap"
	"context"
	"math/rand"
	"strings"
	"time"

	"github.com/grafana/regexp"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/limiter"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

const (
	// minDelay is the minimum amount of time between scheduled updates for a single repository.
	minDelay = 45 * time.Second

	// maxDelay is the maximum amount of time between scheduled updates for a single repository.
	maxDelay = 8 * time.Hour
)

// UpdateScheduler schedules repo update (or clone) requests to gitserver.
//
// Repository metadata is synced from configured code hosts and added to the scheduler.
//
// Updates are scheduled based on the time that has elapsed since the last commit
// divided by a constant factor of 2. For example, if a repo's last commit was 8 hours ago
// then the next update will be scheduled 4 hours from now. If there are still no new commits,
// then the next update will be scheduled 6 hours from then.
// This heuristic is simple to compute and has nice backoff properties.
//
// If an error occurs when attempting to fetch a repo we perform exponential
// backoff by doubling the current interval. This ensures that problematic repos
// don't stay in the front of the schedule clogging up the queue.
//
// When it is time for a repo to update, the scheduler inserts the repo into a queue.
//
// A worker continuously dequeues repos and sends updates to gitserver, but its concurrency
// is limited by the gitMaxConcurrentClones site configuration.
type UpdateScheduler struct {
	db              database.DB
	gitserverClient gitserver.Client
	updateQueue     *updateQueue
	schedule        *schedule
	logger          log.Logger
	cancelCtx       context.CancelFunc
}

// A configuredRepo represents the configuration data for a given repo from
// a configuration source, such as information retrieved from GitHub for a
// given GitHubConnection.
type configuredRepo struct {
	ID   api.RepoID
	Name api.RepoName
}

// notifyChanBuffer controls the buffer size of notification channels.
// It is important that this value is 1 so that we can perform lossless
// non-blocking sends.
const notifyChanBuffer = 1

// NewUpdateScheduler returns a new scheduler.
func NewUpdateScheduler(logger log.Logger, db database.DB, gitserverClient gitserver.Client) *UpdateScheduler {
	updateSchedLogger := logger.Scoped("UpdateScheduler")

	return &UpdateScheduler{
		db:              db,
		gitserverClient: gitserverClient,
		updateQueue: &updateQueue{
			index:         make(map[api.RepoID]*repoUpdate),
			notifyEnqueue: make(chan struct{}, notifyChanBuffer),
		},
		schedule: &schedule{
			index:         make(map[api.RepoID]*scheduledRepoUpdate),
			wakeup:        make(chan struct{}, notifyChanBuffer),
			randGenerator: rand.New(rand.NewSource(time.Now().UnixNano())),
			logger:        updateSchedLogger.Scoped("Schedule"),
		},
		logger: updateSchedLogger,
	}
}

func (s *UpdateScheduler) Start() {
	// Make sure the update scheduler acts as an internal actor, so it can see all
	// repos.
	ctx, cancel := context.WithCancel(actor.WithInternalActor(context.Background()))
	s.cancelCtx = cancel

	go s.runUpdateLoop(ctx)
	go s.runScheduleLoop(ctx)
}

func (s *UpdateScheduler) Stop() {
	if s.cancelCtx != nil {
		s.cancelCtx()
	}
}

// runScheduleLoop starts the loop that schedules updates by enqueuing them into the updateQueue.
func (s *UpdateScheduler) runScheduleLoop(ctx context.Context) {
	for {
		select {
		case <-s.schedule.wakeup:
		case <-ctx.Done():
			s.schedule.reset()
			return
		}

		if conf.Get().DisableAutoGitUpdates {
			continue
		}

		s.runSchedule()
		schedLoops.Inc()
	}
}

func (s *UpdateScheduler) runSchedule() {
	s.schedule.mu.Lock()
	defer s.schedule.mu.Unlock()
	defer s.schedule.rescheduleTimer()

	for len(s.schedule.heap) != 0 {
		repoUpdate := s.schedule.heap[0]
		if !repoUpdate.Due.Before(timeNow().Add(time.Millisecond)) {
			break
		}

		schedAutoFetch.Inc()
		s.updateQueue.enqueue(repoUpdate.Repo, priorityLow)
		repoUpdate.Due = timeNow().Add(repoUpdate.Interval)
		heap.Fix(s.schedule, 0)
	}
}

// runUpdateLoop sends repo update requests to gitserver.
func (s *UpdateScheduler) runUpdateLoop(ctx context.Context) {
	limiter := configuredLimiter()

	for {
		select {
		case <-s.updateQueue.notifyEnqueue:
		case <-ctx.Done():
			s.updateQueue.reset()
			return
		}

		for {
			ctx, cancel, err := limiter.Acquire(ctx)
			if err != nil {
				// context is canceled; shutdown
				return
			}

			repo, ok := s.updateQueue.acquireNext()
			if !ok {
				cancel()
				break
			}

			subLogger := s.logger.Scoped("RunUpdateLoop")

			go func() {
				defer cancel()
				defer s.updateQueue.remove(repo, true)

				// This is a blocking call since the repo will be cloned synchronously by gitserver
				// if it doesn't exist or update it if it does. The timeout of this request depends
				// on the value of conf.GitLongCommandTimeout() or if the passed context has a set
				// deadline shorter than the value of this config.
				resp, err := s.gitserverClient.RequestRepoUpdate(ctx, repo.Name)
				if err != nil {
					schedError.WithLabelValues("requestRepoUpdate").Inc()
					subLogger.Error("error requesting repo update", log.Error(err), log.String("uri", string(repo.Name)))
				} else if resp != nil && resp.Error != "" {
					schedError.WithLabelValues("repoUpdateResponse").Inc()
					// We don't want to spam our logs when the rate limiter has been set to block all
					// updates
					if !strings.Contains(resp.Error, ratelimit.ErrBlockAll.Error()) {
						subLogger.Error("error updating repo", log.String("err", resp.Error), log.String("uri", string(repo.Name)))
					}
				}

				if interval := getCustomInterval(subLogger, conf.Get(), string(repo.Name)); interval > 0 {
					s.schedule.updateInterval(repo, interval)
					return
				}

				if err != nil || (resp != nil && resp.Error != "") {
					// On error we will double the current interval so that we back off and don't
					// get stuck with problematic repos with low intervals.
					if currentInterval, ok := s.schedule.getCurrentInterval(repo); ok {
						s.schedule.updateInterval(repo, currentInterval*2)
					}
				} else if resp != nil && resp.LastFetched != nil && resp.LastChanged != nil {
					// This is the heuristic that is described in the UpdateScheduler documentation.
					// Update that documentation if you update this logic.
					interval := resp.LastFetched.Sub(*resp.LastChanged) / 2
					s.schedule.updateInterval(repo, interval)
				}
			}()
		}
	}
}

func getCustomInterval(logger log.Logger, c *conf.Unified, repoName string) time.Duration {
	if c == nil {
		return 0
	}
	for _, rule := range c.GitUpdateInterval {
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			logger.Warn("error compiling GitUpdateInterval pattern", log.Error(err))
			continue
		}
		if re.MatchString(repoName) {
			return time.Duration(rule.Interval) * time.Minute
		}
	}
	return 0
}

// configuredLimiter returns a mutable limiter that is
// configured with the maximum number of concurrent update
// requests that repo-updater should send to gitserver.
var configuredLimiter = func() *limiter.MutableLimiter {
	limiter := limiter.NewMutable(1)
	conf.Watch(func() {
		limiter.SetLimit(conf.GitMaxConcurrentClones())
	})
	return limiter
}

// UpdateFromDiff updates the scheduled and queued repos from the given sync
// diff.
//
// We upsert all repos that exist to the scheduler. This is so the
// scheduler can track the repositories and periodically update
// them.
//
// Items on the update queue will be cloned/fetched as soon as
// possible. We treat repos differently depending on which part of the
// diff they are:
//
//	Deleted    - remove from scheduler and queue.
//	Added      - new repo, enqueue for asap clone.
//	Modified   - likely new url or name. May also be a sign of new
//	             commits. Enqueue for asap clone (or fetch).
//	Unmodified - we likely already have this cloned. Just rely on
//	             the scheduler and do not enqueue.
func (s *UpdateScheduler) UpdateFromDiff(diff types.RepoSyncDiff) {
	for _, r := range diff.Deleted {
		s.remove(r)
	}

	for _, r := range diff.Added {
		s.upsert(r, true)
	}
	for _, r := range diff.Modified.Repos() {
		s.upsert(r, true)
	}

	known := len(diff.Added) + len(diff.Modified)
	for _, r := range diff.Unmodified {
		if r.IsDeleted() {
			s.remove(r)
			continue
		}

		known++
		s.upsert(r, false)
	}
}

// PrioritiseUncloned will treat any repos listed in ids as uncloned, which in
// effect will move them to the front of the queue for updating ASAP.
//
// This method should be called periodically with the list of all repositories
// managed by the scheduler that are not cloned on gitserver.
func (s *UpdateScheduler) PrioritiseUncloned(repos []types.MinimalRepo) {
	s.schedule.prioritiseUncloned(repos)
}

// EnsureScheduled ensures that all repos in repos exist in the scheduler.
func (s *UpdateScheduler) EnsureScheduled(repos []types.MinimalRepo) {
	s.schedule.insertNew(repos)
}

// ListRepoIDs lists the ids of all repos managed by the scheduler
func (s *UpdateScheduler) ListRepoIDs() []api.RepoID {
	s.schedule.mu.Lock()
	defer s.schedule.mu.Unlock()

	ids := make([]api.RepoID, len(s.schedule.heap))
	for i := range s.schedule.heap {
		ids[i] = s.schedule.heap[i].Repo.ID
	}
	return ids
}

// upsert adds r to the scheduler for periodic updates. If r.ID is already in
// the scheduler, then the fields are updated (upsert).
//
// If enqueue is true then r is also enqueued to the update queue for a git
// fetch/clone soon.
func (s *UpdateScheduler) upsert(r *types.Repo, enqueue bool) {
	repo := configuredRepoFromRepo(r)
	logger := s.logger.With(log.String("repo", string(r.Name)))

	updated := s.schedule.upsert(repo)
	logger.Debug("scheduler.schedule.upserted", log.Bool("updated", updated))

	if !enqueue {
		return
	}
	updated = s.updateQueue.enqueue(repo, priorityLow)
	logger.Debug("scheduler.updateQueue.enqueued", log.Bool("updated", updated))
}

func (s *UpdateScheduler) remove(r *types.Repo) {
	repo := configuredRepoFromRepo(r)
	logger := s.logger.With(log.String("repo", string(r.Name)))

	if s.schedule.remove(repo) {
		logger.Debug("scheduler.schedule.removed")
	}

	if s.updateQueue.remove(repo, false) {
		logger.Debug("scheduler.updateQueue.removed")
	}
}

func configuredRepoFromRepo(r *types.Repo) configuredRepo {
	repo := configuredRepo{
		ID:   r.ID,
		Name: r.Name,
	}

	return repo
}

// UpdateOnce causes a single update of the given repository.
// It neither adds nor removes the repo from the schedule.
func (s *UpdateScheduler) UpdateOnce(id api.RepoID, name api.RepoName) {
	repo := configuredRepo{
		ID:   id,
		Name: name,
	}
	schedManualFetch.Inc()
	s.updateQueue.enqueue(repo, priorityHigh)
}

// DebugDump returns the state of the update scheduler for debugging.
func (s *UpdateScheduler) DebugDump(ctx context.Context) any {
	data := struct {
		Name        string
		UpdateQueue []*repoUpdate
		Schedule    []*scheduledRepoUpdate
		SyncJobs    []*types.ExternalServiceSyncJob
	}{
		Name: "repos",
	}

	s.schedule.mu.Lock()
	schedule := schedule{
		heap: make([]*scheduledRepoUpdate, len(s.schedule.heap)),
	}
	for i, update := range s.schedule.heap {
		// Copy the scheduledRepoUpdate as a value so that
		// popping off the heap here won't update the index value of the real heap, and
		// we don't do a racy read on the repo pointer which may change concurrently in the real heap.
		updateCopy := *update
		schedule.heap[i] = &updateCopy
	}
	s.schedule.mu.Unlock()

	for len(schedule.heap) > 0 {
		update := heap.Pop(&schedule).(*scheduledRepoUpdate)
		data.Schedule = append(data.Schedule, update)
	}

	s.updateQueue.mu.Lock()
	updateQueue := updateQueue{
		heap: make([]*repoUpdate, len(s.updateQueue.heap)),
	}
	for i, update := range s.updateQueue.heap {
		// Copy the repoUpdate as a value so that
		// popping off the heap here won't update the index value of the real heap, and
		// we don't do a racy read on the repo pointer which may change concurrently in the real heap.
		updateCopy := *update
		updateQueue.heap[i] = &updateCopy
	}
	s.updateQueue.mu.Unlock()

	for len(updateQueue.heap) > 0 {
		// Copy the scheduledRepoUpdate as a value so that the repo pointer
		// won't change concurrently after we release the lock.
		update := heap.Pop(&updateQueue).(*repoUpdate)
		data.UpdateQueue = append(data.UpdateQueue, update)
	}

	var err error
	data.SyncJobs, err = s.db.ExternalServices().GetSyncJobs(ctx, database.ExternalServicesGetSyncJobsOptions{})
	if err != nil {
		s.logger.Warn("getting external service sync jobs for debug page", log.Error(err))
	}

	return &data
}

// ScheduleInfo returns the current schedule info for a repo.
func (s *UpdateScheduler) ScheduleInfo(id api.RepoID) *protocol.RepoUpdateSchedulerInfoResult {
	var result protocol.RepoUpdateSchedulerInfoResult

	s.schedule.mu.Lock()
	if update := s.schedule.index[id]; update != nil {
		result.Schedule = &protocol.RepoScheduleState{
			Index:           update.Index,
			Total:           len(s.schedule.index),
			IntervalSeconds: int(update.Interval / time.Second),
			Due:             update.Due,
		}
	}
	s.schedule.mu.Unlock()

	s.updateQueue.mu.Lock()
	if update := s.updateQueue.index[id]; update != nil {
		result.Queue = &protocol.RepoQueueState{
			Index:    update.Index,
			Total:    len(s.updateQueue.index),
			Updating: update.Updating,
			Priority: int(update.Priority),
		}
	}
	s.updateQueue.mu.Unlock()

	return &result
}

type priority int

const (
	priorityLow priority = iota
	priorityHigh
)

// repoUpdate is a repository that has been queued for an update.
type repoUpdate struct {
	Repo     configuredRepo
	Priority priority
	Seq      uint64 // the sequence number of the update
	Updating bool   // whether the repo has been acquired for update
	Index    int    `json:"-"` // the index in the heap
}

// scheduledRepoUpdate is the update schedule for a single repo.
type scheduledRepoUpdate struct {
	Repo     configuredRepo // the repo to update
	Interval time.Duration  // how regularly the repo is updated
	Due      time.Time      // the next time that the repo will be enqueued for a update
	Index    int            `json:"-"` // the index in the heap
}

// notify performs a non-blocking send on the channel.
// The channel should be buffered.
var notify = func(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

// Mockable time functions for testing.
var (
	timeNow       = time.Now
	timeAfterFunc = time.AfterFunc
)
