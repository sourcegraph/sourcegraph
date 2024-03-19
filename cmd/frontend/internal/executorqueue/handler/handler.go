package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/database"
	internalexecutor "github.com/sourcegraph/sourcegraph/internal/executor"
	executorstore "github.com/sourcegraph/sourcegraph/internal/executor/store"
	executortypes "github.com/sourcegraph/sourcegraph/internal/executor/types"
	metricsstore "github.com/sourcegraph/sourcegraph/internal/metrics/store"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	"github.com/sourcegraph/sourcegraph/lib/api"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// ExecutorHandler handles the HTTP requests of an executor for a single queue. See MultiHandler for multi-queue implementation.
type ExecutorHandler interface {
	// Name is the name of the queue the handler processes.
	Name() string
	// HandleDequeue retrieves the next executor.Job to be processed in the queue.
	HandleDequeue(w http.ResponseWriter, r *http.Request)
	// HandleAddExecutionLogEntry adds the log entry for the executor.Job.
	HandleAddExecutionLogEntry(w http.ResponseWriter, r *http.Request)
	// HandleUpdateExecutionLogEntry updates the log entry for the executor.Job.
	HandleUpdateExecutionLogEntry(w http.ResponseWriter, r *http.Request)
	// HandleMarkComplete updates the executor.Job to have a completed status.
	HandleMarkComplete(w http.ResponseWriter, r *http.Request)
	// HandleMarkErrored updates the executor.Job to have an errored status.
	HandleMarkErrored(w http.ResponseWriter, r *http.Request)
	// HandleMarkFailed updates the executor.Job to have a failed status.
	HandleMarkFailed(w http.ResponseWriter, r *http.Request)
	// HandleHeartbeat handles the heartbeat of an executor.
	HandleHeartbeat(w http.ResponseWriter, r *http.Request)
}

var _ ExecutorHandler = &handler[workerutil.Record]{}

type handler[T workerutil.Record] struct {
	queueHandler  QueueHandler[T]
	executorStore database.ExecutorStore
	jobTokenStore executorstore.JobTokenStore
	metricsStore  metricsstore.DistributedStore
	logger        log.Logger
}

// QueueHandler the specific logic for handling a queue.
type QueueHandler[T workerutil.Record] struct {
	// Name signifies the type of work the queue serves to executors.
	Name string
	// Store is a required dbworker store.
	Store store.Store[T]
	// RecordTransformer is a required hook for each registered queue that transforms a generic
	// record from that queue into the job to be given to an executor.
	RecordTransformer TransformerFunc[T]
}

// TransformerFunc is the function to transform a workerutil.Record into an executor.Job.
type TransformerFunc[T workerutil.Record] func(ctx context.Context, version string, record T, resourceMetadata ResourceMetadata) (executortypes.Job, error)

// NewHandler creates a new ExecutorHandler.
func NewHandler[T workerutil.Record](
	executorStore database.ExecutorStore,
	jobTokenStore executorstore.JobTokenStore,
	metricsStore metricsstore.DistributedStore,
	queueHandler QueueHandler[T],
) ExecutorHandler {
	return &handler[T]{
		executorStore: executorStore,
		jobTokenStore: jobTokenStore,
		metricsStore:  metricsStore,
		logger: log.Scoped(
			fmt.Sprintf("executor-queue-handler-%s", queueHandler.Name),
		),
		queueHandler: queueHandler,
	}
}

func (h *handler[T]) Name() string {
	return h.queueHandler.Name
}

func (h *handler[T]) HandleDequeue(w http.ResponseWriter, r *http.Request) {
	var payload executortypes.DequeueRequest

	wrapHandler(w, r, &payload, h.logger, func() (int, any, error) {
		job, dequeued, err := h.dequeue(r.Context(), mux.Vars(r)["queueName"], executorMetadata{
			name:    payload.ExecutorName,
			version: payload.Version,
			resources: ResourceMetadata{
				NumCPUs:   payload.NumCPUs,
				Memory:    payload.Memory,
				DiskSpace: payload.DiskSpace,
			},
		})
		if !dequeued {
			return http.StatusNoContent, nil, err
		}

		return http.StatusOK, job, err
	})
}

// dequeue selects a job record from the database and stashes metadata including
// the job record and the locking transaction. If no job is available for processing,
// a false-valued flag is returned.
func (h *handler[T]) dequeue(ctx context.Context, queueName string, metadata executorMetadata) (executortypes.Job, bool, error) {
	if err := validateWorkerHostname(metadata.name); err != nil {
		return executortypes.Job{}, false, err
	}

	version2Supported := false
	if metadata.version != "" {
		var err error
		version2Supported, err = api.CheckSourcegraphVersion(metadata.version, ">= 4.3.0-0", "2022-11-24")
		if err != nil {
			return executortypes.Job{}, false, errors.Wrapf(err, "failed to check version %q", metadata.version)
		}
	}

	// executorName is supposed to be unique.
	record, dequeued, err := h.queueHandler.Store.Dequeue(ctx, metadata.name, nil)
	if err != nil {
		return executortypes.Job{}, false, errors.Wrap(err, "dbworkerstore.Dequeue")
	}
	if !dequeued {
		return executortypes.Job{}, false, nil
	}

	logger := log.Scoped("dequeue")
	job, err := h.queueHandler.RecordTransformer(ctx, metadata.version, record, metadata.resources)
	if err != nil {
		if _, err := h.queueHandler.Store.MarkFailed(ctx, record.RecordID(), fmt.Sprintf("failed to transform record: %s", err), store.MarkFinalOptions{}); err != nil {
			logger.Error("Failed to mark record as failed",
				log.Int("recordID", record.RecordID()),
				log.Error(err))
		}

		return executortypes.Job{}, false, errors.Wrap(err, "RecordTransformer")
	}

	// If this executor supports v2, return a v2 payload. Based on this field,
	// marshalling will be switched between old and new payload.
	if version2Supported {
		job.Version = 2
	}

	token, err := h.jobTokenStore.Create(ctx, job.ID, queueName, job.RepositoryName)
	if err != nil {
		if errors.Is(err, executorstore.ErrJobTokenAlreadyCreated) {
			// Token has already been created, regen it.
			token, err = h.jobTokenStore.Regenerate(ctx, job.ID, queueName)
			if err != nil {
				return executortypes.Job{}, false, errors.Wrap(err, "RegenerateToken")
			}
		} else {
			return executortypes.Job{}, false, errors.Wrap(err, "CreateToken")
		}
	}
	job.Token = token

	return job, true, nil
}

type executorMetadata struct {
	name      string
	version   string
	resources ResourceMetadata
}

// ResourceMetadata is the specific resource data for an executor instance.
type ResourceMetadata struct {
	NumCPUs   int
	Memory    string
	DiskSpace string
}

func (h *handler[T]) HandleAddExecutionLogEntry(w http.ResponseWriter, r *http.Request) {
	var payload executortypes.AddExecutionLogEntryRequest

	wrapHandler(w, r, &payload, h.logger, func() (int, any, error) {
		id, err := h.addExecutionLogEntry(r.Context(), payload.ExecutorName, payload.JobID, payload.ExecutionLogEntry)
		return http.StatusOK, id, err
	})
}

func (h *handler[T]) addExecutionLogEntry(ctx context.Context, executorName string, jobID int, entry internalexecutor.ExecutionLogEntry) (int, error) {
	entryID, err := h.queueHandler.Store.AddExecutionLogEntry(ctx, jobID, entry, store.ExecutionLogEntryOptions{
		// We pass the WorkerHostname, so the store enforces the record to be owned by this executor. When
		// the previous executor didn't report heartbeats anymore, but is still alive and reporting logs,
		// both executors that ever got the job would be writing to the same record. This prevents it.
		WorkerHostname: executorName,
		// We pass state to enforce adding log entries is only possible while the record is still dequeued.
		State: "processing",
	})
	if err == store.ErrExecutionLogEntryNotUpdated {
		return 0, ErrUnknownJob
	}
	return entryID, errors.Wrap(err, "dbworkerstore.AddExecutionLogEntry")
}

func (h *handler[T]) HandleUpdateExecutionLogEntry(w http.ResponseWriter, r *http.Request) {
	var payload executortypes.UpdateExecutionLogEntryRequest

	wrapHandler(w, r, &payload, h.logger, func() (int, any, error) {
		err := h.updateExecutionLogEntry(r.Context(), payload.ExecutorName, payload.JobID, payload.EntryID, payload.ExecutionLogEntry)
		return http.StatusNoContent, nil, err
	})
}

func (h *handler[T]) updateExecutionLogEntry(ctx context.Context, executorName string, jobID int, entryID int, entry internalexecutor.ExecutionLogEntry) error {
	err := h.queueHandler.Store.UpdateExecutionLogEntry(ctx, jobID, entryID, entry, store.ExecutionLogEntryOptions{
		// We pass the WorkerHostname, so the store enforces the record to be owned by this executor. When
		// the previous executor didn't report heartbeats anymore, but is still alive and reporting logs,
		// both executors that ever got the job would be writing to the same record. This prevents it.
		WorkerHostname: executorName,
		// We pass state to enforce adding log entries is only possible while the record is still dequeued.
		State: "processing",
	})
	if err == store.ErrExecutionLogEntryNotUpdated {
		return ErrUnknownJob
	}
	return errors.Wrap(err, "dbworkerstore.UpdateExecutionLogEntry")
}

func (h *handler[T]) HandleMarkComplete(w http.ResponseWriter, r *http.Request) {
	var payload executortypes.MarkCompleteRequest

	wrapHandler(w, r, &payload, h.logger, func() (int, any, error) {
		err := h.markComplete(r.Context(), mux.Vars(r)["queueName"], payload.ExecutorName, payload.JobID)
		if err == ErrUnknownJob {
			return http.StatusNotFound, nil, nil
		}

		return http.StatusNoContent, nil, err
	})
}

func (h *handler[T]) markComplete(ctx context.Context, queueName string, executorName string, jobID int) error {
	ok, err := h.queueHandler.Store.MarkComplete(ctx, jobID, store.MarkFinalOptions{
		// We pass the WorkerHostname, so the store enforces the record to be owned by this executor. When
		// the previous executor didn't report heartbeats anymore, but is still alive and reporting state,
		// both executors that ever got the job would be writing to the same record. This prevents it.
		WorkerHostname: executorName,
	})
	if err != nil {
		return errors.Wrap(err, "dbworkerstore.MarkComplete")
	}
	if !ok {
		return ErrUnknownJob
	}

	if err = h.jobTokenStore.Delete(ctx, jobID, queueName); err != nil {
		return errors.Wrap(err, "jobTokenStore.Delete")
	}

	return nil
}

func (h *handler[T]) HandleMarkErrored(w http.ResponseWriter, r *http.Request) {
	var payload executortypes.MarkErroredRequest

	wrapHandler(w, r, &payload, h.logger, func() (int, any, error) {
		err := h.markErrored(r.Context(), mux.Vars(r)["queueName"], payload.ExecutorName, payload.JobID, payload.ErrorMessage)
		if err == ErrUnknownJob {
			return http.StatusNotFound, nil, nil
		}

		return http.StatusNoContent, nil, err
	})
}

func (h *handler[T]) markErrored(ctx context.Context, queueName string, executorName string, jobID int, errorMessage string) error {
	ok, err := h.queueHandler.Store.MarkErrored(ctx, jobID, errorMessage, store.MarkFinalOptions{
		// We pass the WorkerHostname, so the store enforces the record to be owned by this executor. When
		// the previous executor didn't report heartbeats anymore, but is still alive and reporting state,
		// both executors that ever got the job would be writing to the same record. This prevents it.
		WorkerHostname: executorName,
	})
	if err != nil {
		return errors.Wrap(err, "dbworkerstore.MarkErrored")
	}
	if !ok {
		return ErrUnknownJob
	}

	if err = h.jobTokenStore.Delete(ctx, jobID, queueName); err != nil {
		return errors.Wrap(err, "jobTokenStore.Delete")
	}

	return nil
}

func (h *handler[T]) HandleMarkFailed(w http.ResponseWriter, r *http.Request) {
	var payload executortypes.MarkErroredRequest

	wrapHandler(w, r, &payload, h.logger, func() (int, any, error) {
		err := h.markFailed(r.Context(), mux.Vars(r)["queueName"], payload.ExecutorName, payload.JobID, payload.ErrorMessage)
		if err == ErrUnknownJob {
			return http.StatusNotFound, nil, nil
		}

		return http.StatusNoContent, nil, err
	})
}

// ErrUnknownJob error when the job does not exist.
var ErrUnknownJob = errors.New("unknown job")

func (h *handler[T]) markFailed(ctx context.Context, queueName string, executorName string, jobID int, errorMessage string) error {
	ok, err := h.queueHandler.Store.MarkFailed(ctx, jobID, errorMessage, store.MarkFinalOptions{
		// We pass the WorkerHostname, so the store enforces the record to be owned by this executor. When
		// the previous executor didn't report heartbeats anymore, but is still alive and reporting state,
		// both executors that ever got the job would be writing to the same record. This prevents it.
		WorkerHostname: executorName,
	})
	if err != nil {
		return errors.Wrap(err, "dbworkerstore.MarkFailed")
	}
	if !ok {
		return ErrUnknownJob
	}

	if err = h.jobTokenStore.Delete(ctx, jobID, queueName); err != nil {
		return errors.Wrap(err, "jobTokenStore.Delete")
	}

	return nil
}

func (h *handler[T]) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var payload executortypes.HeartbeatRequest

	wrapHandler(w, r, &payload, h.logger, func() (int, any, error) {
		e := types.Executor{
			Hostname:        payload.ExecutorName,
			QueueName:       mux.Vars(r)["queueName"],
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
				h.logger.Error("failed to decode metrics and apply labels for executor heartbeat", log.Error(err))
				return
			}

			if err = h.metricsStore.Ingest(payload.ExecutorName, metrics); err != nil {
				// Just log the error but don't panic. The heartbeat is more important.
				h.logger.Error("failed to ingest metrics for executor heartbeat", log.Error(err))
			}
		}()

		knownIDs, cancelIDs, err := h.heartbeat(r.Context(), e, payload.JobIDs)

		return http.StatusOK, executortypes.HeartbeatResponse{KnownIDs: knownIDs, CancelIDs: cancelIDs}, err
	})
}

func (h *handler[T]) heartbeat(ctx context.Context, executor types.Executor, ids []string) ([]string, []string, error) {
	if err := validateWorkerHostname(executor.Hostname); err != nil {
		return nil, nil, err
	}

	logger := log.Scoped("heartbeat")

	// Write this heartbeat to the database so that we can populate the UI with recent executor activity.
	if err := h.executorStore.UpsertHeartbeat(ctx, executor); err != nil {
		logger.Error("Failed to upsert executor heartbeat", log.Error(err))
	}

	knownIDs, cancelIDs, err := h.queueHandler.Store.Heartbeat(ctx, ids, store.HeartbeatOptions{
		// We pass the WorkerHostname, so the store enforces the record to be owned by this executor. When
		// the previous executor didn't report heartbeats anymore, but is still alive and reporting state,
		// both executors that ever got the job would be writing to the same record. This prevents it.
		WorkerHostname: executor.Hostname,
	})
	return knownIDs, cancelIDs, errors.Wrap(err, "dbworkerstore.UpsertHeartbeat")
}

// wrapHandler decodes the request body into the given payload pointer, then calls the given
// handler function. If the body cannot be decoded, a 400 BadRequest is returned and the handler
// function is not called. If the handler function returns an error, a 500 Internal Server Error
// is returned. Otherwise, the response status will match the status code value returned from the
// handler, and the payload value returned from the handler is encoded and written to the
// response body.
func wrapHandler(w http.ResponseWriter, r *http.Request, payload any, logger log.Logger, handler func() (int, any, error)) {
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		logger.Error("Failed to unmarshal payload", log.Error(err))
		http.Error(w, fmt.Sprintf("Failed to unmarshal payload: %s", err.Error()), http.StatusBadRequest)
		return
	}

	status, payload, err := handler()
	if err != nil {
		logger.Error("Handler returned an error", log.Error(err))

		status = http.StatusInternalServerError
		payload = errorResponse{Error: err.Error()}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		logger.Error("Failed to serialize payload", log.Error(err))
		http.Error(w, fmt.Sprintf("Failed to serialize payload: %s", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(status)

	if status != http.StatusNoContent {
		_, _ = io.Copy(w, bytes.NewReader(data))
	}
}

// decodeAndLabelMetrics decodes the text serialized prometheus metrics dump and then
// applies common labels.
func decodeAndLabelMetrics(encodedMetrics, instanceName string) ([]*dto.MetricFamily, error) {
	var data []*dto.MetricFamily

	dec := expfmt.NewDecoder(strings.NewReader(encodedMetrics), expfmt.FmtText)
	for {
		var mf dto.MetricFamily
		if err := dec.Decode(&mf); err != nil {
			if err == io.EOF {
				break
			}

			return nil, errors.Wrap(err, "decoding metric family")
		}

		// Attach the extra labels.
		metricLabelInstance := "sg_instance"
		metricLabelJob := "sg_job"
		executorJob := "sourcegraph-executors"
		registryJob := "sourcegraph-executors-registry"
		for _, m := range mf.Metric {
			var metricLabelInstanceValue string
			for _, l := range m.Label {
				if *l.Name == metricLabelInstance {
					metricLabelInstanceValue = l.GetValue()
					break
				}
			}
			// if sg_instance not set, set it as the executor name sent in the heartbeat.
			// this is done for the executor's own and it's node_exporter metrics, executors
			// set sg_instance for metrics scraped from the registry+registry's node_exporter
			if metricLabelInstanceValue == "" {
				m.Label = append(m.Label, &dto.LabelPair{Name: &metricLabelInstance, Value: &instanceName})
			}

			if metricLabelInstanceValue == "docker-registry" {
				m.Label = append(m.Label, &dto.LabelPair{Name: &metricLabelJob, Value: &registryJob})
			} else {
				m.Label = append(m.Label, &dto.LabelPair{Name: &metricLabelJob, Value: &executorJob})
			}
		}

		data = append(data, &mf)
	}

	return data, nil
}

type errorResponse struct {
	Error string `json:"error"`
}

// validateWorkerHostname validates the WorkerHostname field sent for all the endpoints.
// We don't allow empty hostnames, as it would bypass the hostname verification, which
// could lead to stray workers updating records they no longer own.
func validateWorkerHostname(hostname string) error {
	if hostname == "" {
		return errors.New("worker hostname cannot be empty")
	}
	return nil
}
