package handler

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/sourcegraph/log"

	"github.com/mroth/weightedrand/v2"

	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	executorstore "github.com/sourcegraph/sourcegraph/internal/executor/store"
	executortypes "github.com/sourcegraph/sourcegraph/internal/executor/types"
	metricsstore "github.com/sourcegraph/sourcegraph/internal/metrics/store"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	"github.com/sourcegraph/sourcegraph/lib/api"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// MultiHandler handles the HTTP requests of an executor for more than one queue. See ExecutorHandler for single-queue implementation.
type MultiHandler struct {
	executorStore         database.ExecutorStore
	jobTokenStore         executorstore.JobTokenStore
	metricsStore          metricsstore.DistributedStore
	CodeIntelQueueHandler QueueHandler[uploadsshared.Index]
	BatchesQueueHandler   QueueHandler[*btypes.BatchSpecWorkspaceExecutionJob]
	DequeueCache          *rcache.Cache
	dequeueCacheConfig    *schema.DequeueCacheConfig
	logger                log.Logger
}

// NewMultiHandler creates a new MultiHandler.
func NewMultiHandler(
	executorStore database.ExecutorStore,
	jobTokenStore executorstore.JobTokenStore,
	metricsStore metricsstore.DistributedStore,
	codeIntelQueueHandler QueueHandler[uploadsshared.Index],
	batchesQueueHandler QueueHandler[*btypes.BatchSpecWorkspaceExecutionJob],
) MultiHandler {
	siteConfig := conf.Get().SiteConfiguration
	dequeueCache := rcache.New(executortypes.DequeueCachePrefix)
	dequeueCacheConfig := executortypes.DequeuePropertiesPerQueue
	if siteConfig.ExecutorsMultiqueue != nil && siteConfig.ExecutorsMultiqueue.DequeueCacheConfig != nil {
		dequeueCacheConfig = siteConfig.ExecutorsMultiqueue.DequeueCacheConfig
	}
	multiHandler := MultiHandler{
		executorStore:         executorStore,
		jobTokenStore:         jobTokenStore,
		metricsStore:          metricsStore,
		CodeIntelQueueHandler: codeIntelQueueHandler,
		BatchesQueueHandler:   batchesQueueHandler,
		DequeueCache:          dequeueCache,
		dequeueCacheConfig:    dequeueCacheConfig,
		logger:                log.Scoped("executor-multi-queue-handler"),
	}
	return multiHandler
}

// HandleDequeue is the equivalent of ExecutorHandler.HandleDequeue for multiple queues.
func (m *MultiHandler) HandleDequeue(w http.ResponseWriter, r *http.Request) {
	var payload executortypes.DequeueRequest
	wrapHandler(w, r, &payload, m.logger, func() (int, any, error) {
		job, dequeued, err := m.dequeue(r.Context(), payload)
		if !dequeued {
			return http.StatusNoContent, nil, err
		}

		return http.StatusOK, job, err
	})
}

func (m *MultiHandler) dequeue(ctx context.Context, req executortypes.DequeueRequest) (executortypes.Job, bool, error) {
	if err := validateWorkerHostname(req.ExecutorName); err != nil {
		m.logger.Error(err.Error())
		return executortypes.Job{}, false, err
	}

	version2Supported := false
	if req.Version != "" {
		var err error
		version2Supported, err = api.CheckSourcegraphVersion(req.Version, ">= 4.3.0-0", "2022-11-24")
		if err != nil {
			return executortypes.Job{}, false, errors.Wrapf(err, "failed to check version %q", req.Version)
		}
	}

	if len(req.Queues) == 0 {
		m.logger.Info("Dequeue requested without any queue names", log.String("executorName", req.ExecutorName))
		return executortypes.Job{}, false, nil
	}

	if invalidQueues := m.validateQueues(req.Queues); len(invalidQueues) > 0 {
		message := fmt.Sprintf("Invalid queue name(s) '%s' found. Supported queue names are '%s'.", strings.Join(invalidQueues, ", "), strings.Join(executortypes.ValidQueueNames, ", "))
		m.logger.Error(message)
		return executortypes.Job{}, false, errors.New(message)
	}

	// discard empty queues
	nonEmptyQueues, err := m.SelectNonEmptyQueues(ctx, req.Queues)
	if err != nil {
		return executortypes.Job{}, false, err
	}

	var selectedQueue string
	if len(nonEmptyQueues) == 0 {
		// all queues are empty, dequeue nothing
		return executortypes.Job{}, false, nil
	} else if len(nonEmptyQueues) == 1 {
		// only one queue contains items, select as candidate
		selectedQueue = nonEmptyQueues[0]
	} else {
		// multiple populated queues, discard queues at dequeue limit
		candidateQueues, err := m.SelectEligibleQueues(nonEmptyQueues)
		if err != nil {
			return executortypes.Job{}, false, err
		}
		if len(candidateQueues) == 1 {
			// only one queue hasn't reached dequeue limit for this window, select as candidate
			selectedQueue = candidateQueues[0]
		} else {
			// final list of candidates: multiple not at limit or all at limit.
			selectedQueue, err = m.SelectQueueForDequeueing(candidateQueues)
			if err != nil {
				return executortypes.Job{}, false, err
			}
		}
	}

	resourceMetadata := ResourceMetadata{
		NumCPUs:   req.NumCPUs,
		Memory:    req.Memory,
		DiskSpace: req.DiskSpace,
	}

	logger := m.logger.Scoped("dequeue")
	var job executortypes.Job
	switch selectedQueue {
	case m.BatchesQueueHandler.Name:
		record, dequeued, err := m.BatchesQueueHandler.Store.Dequeue(ctx, req.ExecutorName, nil)
		if err != nil {
			err = errors.Wrapf(err, "dbworkerstore.Dequeue %s", selectedQueue)
			logger.Error("Failed to dequeue", log.String("queue", selectedQueue), log.Error(err))
			return executortypes.Job{}, false, err
		}
		if !dequeued {
			// no batches job to dequeue. Even though the queue was populated before, another executor
			// instance could have dequeued in the meantime
			return executortypes.Job{}, false, nil
		}

		job, err = m.BatchesQueueHandler.RecordTransformer(ctx, req.Version, record, resourceMetadata)
		if err != nil {
			markErr := markRecordAsFailed(ctx, m.BatchesQueueHandler.Store, record.RecordID(), err, logger)
			err = errors.Wrapf(errors.Append(err, markErr), "RecordTransformer %s", selectedQueue)
			logger.Error("Failed to transform record", log.String("queue", selectedQueue), log.Error(err))
			return executortypes.Job{}, false, err
		}
	case m.CodeIntelQueueHandler.Name:
		record, dequeued, err := m.CodeIntelQueueHandler.Store.Dequeue(ctx, req.ExecutorName, nil)
		if err != nil {
			err = errors.Wrapf(err, "dbworkerstore.Dequeue %s", selectedQueue)
			logger.Error("Failed to dequeue", log.String("queue", selectedQueue), log.Error(err))
			return executortypes.Job{}, false, err
		}
		if !dequeued {
			// no codeintel job to dequeue. Even though the queue was populated before, another executor
			// instance could have dequeued in the meantime
			return executortypes.Job{}, false, nil
		}

		job, err = m.CodeIntelQueueHandler.RecordTransformer(ctx, req.Version, record, resourceMetadata)
		if err != nil {
			markErr := markRecordAsFailed(ctx, m.CodeIntelQueueHandler.Store, record.RecordID(), err, logger)
			err = errors.Wrapf(errors.Append(err, markErr), "RecordTransformer %s", selectedQueue)
			logger.Error("Failed to transform record", log.String("queue", selectedQueue), log.Error(err))
			return executortypes.Job{}, false, err
		}
	}
	job.Queue = selectedQueue

	// If this executor supports v2, return a v2 payload. Based on this field,
	// marshalling will be switched between old and new payload.
	if version2Supported {
		job.Version = 2
	}

	logger = m.logger.Scoped("token")
	token, err := m.jobTokenStore.Create(ctx, job.ID, job.Queue, job.RepositoryName)
	if err != nil {
		if errors.Is(err, executorstore.ErrJobTokenAlreadyCreated) {
			// Token has already been created, regen it.
			token, err = m.jobTokenStore.Regenerate(ctx, job.ID, job.Queue)
			if err != nil {
				err = errors.Wrap(err, "RegenerateToken")
				logger.Error("Failed to regenerate token", log.Error(err))
				return executortypes.Job{}, false, err
			}
		} else {
			err = errors.Wrap(err, "CreateToken")
			logger.Error("Failed to create token", log.Error(err))
			return executortypes.Job{}, false, err
		}
	}
	job.Token = token

	// increment dequeue counter
	err = m.DequeueCache.SetHashItem(selectedQueue, fmt.Sprint(time.Now().UnixNano()), job.Token)
	if err != nil {
		m.logger.Error("failed to increment dequeue count", log.String("queue", selectedQueue), log.Error(err))
	}

	return job, true, nil
}

// SelectQueueForDequeueing selects a queue from the provided list with weighted randomness.
func (m *MultiHandler) SelectQueueForDequeueing(candidateQueues []string) (string, error) {
	return DoSelectQueueForDequeueing(candidateQueues, m.dequeueCacheConfig)
}

var DoSelectQueueForDequeueing = func(candidateQueues []string, config *schema.DequeueCacheConfig) (string, error) {
	// pick a queue based on the defined weights
	var choices []weightedrand.Choice[string, int]
	for _, queue := range candidateQueues {
		var weight int
		switch queue {
		case "batches":
			weight = config.Batches.Weight
		case "codeintel":
			weight = config.Codeintel.Weight
		}
		choices = append(choices, weightedrand.NewChoice(queue, weight))
	}
	chooser, err := weightedrand.NewChooser(choices...)
	if err != nil {
		return "", errors.Wrap(err, "failed to randomly select candidate queue to dequeue")
	}
	return chooser.Pick(), nil
}

// SelectEligibleQueues returns a list of queues that have not yet reached the limit of dequeues in the
// current time window.
func (m *MultiHandler) SelectEligibleQueues(queues []string) ([]string, error) {
	var candidateQueues []string
	for _, queue := range queues {
		dequeues, err := m.DequeueCache.GetHashAll(queue)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to check dequeue count for queue '%s'", queue)
		}
		var limit int
		switch queue {
		case m.BatchesQueueHandler.Name:
			limit = m.dequeueCacheConfig.Batches.Limit
		case m.CodeIntelQueueHandler.Name:
			limit = m.dequeueCacheConfig.Codeintel.Limit
		}
		if len(dequeues) < limit {
			candidateQueues = append(candidateQueues, queue)
		}
	}
	if len(candidateQueues) == 0 {
		// all queues are at limit, so make all candidate
		candidateQueues = queues
	}
	return candidateQueues, nil
}

// SelectNonEmptyQueues gets the queue size from the store of each provided queue name and returns
// only those names that have at least one job queued.
func (m *MultiHandler) SelectNonEmptyQueues(ctx context.Context, queueNames []string) ([]string, error) {
	var nonEmptyQueues []string
	for _, queue := range queueNames {
		var err error
		var count int
		switch queue {
		case m.BatchesQueueHandler.Name:
			count, err = m.BatchesQueueHandler.Store.QueuedCount(ctx, false)
		case m.CodeIntelQueueHandler.Name:
			count, err = m.CodeIntelQueueHandler.Store.QueuedCount(ctx, false)
		}
		if err != nil {
			m.logger.Error("fetching queue size", log.Error(err), log.String("queue", queue))
			return nil, err
		}
		if count != 0 {
			nonEmptyQueues = append(nonEmptyQueues, queue)
		}
	}
	return nonEmptyQueues, nil
}

// HandleHeartbeat processes a heartbeat from a multi-queue executor.
func (m *MultiHandler) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var payload executortypes.HeartbeatRequest

	wrapHandler(w, r, &payload, m.logger, func() (int, any, error) {
		e := types.Executor{
			Hostname:        payload.ExecutorName,
			QueueNames:      payload.QueueNames,
			OS:              payload.OS,
			Architecture:    payload.Architecture,
			DockerVersion:   payload.DockerVersion,
			ExecutorVersion: payload.ExecutorVersion,
			GitVersion:      payload.GitVersion,
			IgniteVersion:   payload.IgniteVersion,
			SrcCliVersion:   payload.SrcCliVersion,
		}

		// Handle metrics in the background, this should not delay the heartbeat response being
		// delivered. It is critical for keeping jobs alive.
		go func() {
			metrics, err := decodeAndLabelMetrics(payload.PrometheusMetrics, payload.ExecutorName)
			if err != nil {
				// Just log the error but don't panic. The heartbeat is more important.
				m.logger.Error("failed to decode metrics and apply labels for executor heartbeat", log.Error(err))
				return
			}

			if err = m.metricsStore.Ingest(payload.ExecutorName, metrics); err != nil {
				// Just log the error but don't panic. The heartbeat is more important.
				m.logger.Error("failed to ingest metrics for executor heartbeat", log.Error(err))
			}
		}()

		knownIDs, cancelIDs, err := m.heartbeat(r.Context(), e, payload.JobIDsByQueue)

		return http.StatusOK, executortypes.HeartbeatResponse{KnownIDs: knownIDs, CancelIDs: cancelIDs}, err
	})
}

func (m *MultiHandler) heartbeat(ctx context.Context, executor types.Executor, idsByQueue []executortypes.QueueJobIDs) (knownIDs, cancelIDs []string, err error) {
	if err = validateWorkerHostname(executor.Hostname); err != nil {
		return nil, nil, err
	}

	if len(executor.QueueNames) == 0 {
		return nil, nil, errors.Newf("queueNames must be set for multi-queue heartbeats")
	}

	var invalidQueueNames []string
	for _, queue := range idsByQueue {
		if !slices.Contains(executor.QueueNames, queue.QueueName) {
			invalidQueueNames = append(invalidQueueNames, queue.QueueName)
		}
	}
	if len(invalidQueueNames) > 0 {
		return nil, nil, errors.Newf(
			"unsupported queue name(s) '%s' submitted in queueJobIds, executor is configured for queues '%s'",
			strings.Join(invalidQueueNames, ", "),
			strings.Join(executor.QueueNames, ", "),
		)
	}

	logger := log.Scoped("multiqueue.heartbeat")

	// Write this heartbeat to the database so that we can populate the UI with recent executor activity.
	if err = m.executorStore.UpsertHeartbeat(ctx, executor); err != nil {
		logger.Error("Failed to upsert executor heartbeat", log.Error(err), log.Strings("queues", executor.QueueNames))
	}

	for _, queue := range idsByQueue {
		heartbeatOptions := dbworkerstore.HeartbeatOptions{
			// see handler.heartbeat for explanation of this field
			WorkerHostname: executor.Hostname,
		}

		var known []string
		var cancel []string

		switch queue.QueueName {
		case m.BatchesQueueHandler.Name:
			known, cancel, err = m.BatchesQueueHandler.Store.Heartbeat(ctx, queue.JobIDs, heartbeatOptions)
		case m.CodeIntelQueueHandler.Name:
			known, cancel, err = m.CodeIntelQueueHandler.Store.Heartbeat(ctx, queue.JobIDs, heartbeatOptions)
		}

		if err != nil {
			return nil, nil, errors.Wrap(err, "multiqueue.UpsertHeartbeat")
		}

		// TODO: this could move into the executor client's Heartbeat impl, but considering this is
		// multi-queue specific code, it's a bit ambiguous where it should live. Having it here allows
		// types.HeartbeatResponse to be simpler and enables the client to pass the ID sets back to the worker
		// without further single/multi queue logic
		for i, knownID := range known {
			known[i] = knownID + "-" + queue.QueueName
		}
		for i, cancelID := range cancel {
			cancel[i] = cancelID + "-" + queue.QueueName
		}
		knownIDs = append(knownIDs, known...)
		cancelIDs = append(cancelIDs, cancel...)
	}

	return knownIDs, cancelIDs, nil
}

func (m *MultiHandler) validateQueues(queues []string) []string {
	var invalidQueues []string
	for _, queue := range queues {
		if !slices.Contains(executortypes.ValidQueueNames, queue) {
			invalidQueues = append(invalidQueues, queue)
		}
	}
	return invalidQueues
}

func markRecordAsFailed[T workerutil.Record](context context.Context, store dbworkerstore.Store[T], recordID int, err error, logger log.Logger) error {
	_, markErr := store.MarkFailed(context, recordID, fmt.Sprintf("failed to transform record: %s", err), dbworkerstore.MarkFinalOptions{})
	if markErr != nil {
		logger.Error("Failed to mark record as failed",
			log.Int("recordID", recordID),
			log.Error(markErr))
	}
	return markErr
}
