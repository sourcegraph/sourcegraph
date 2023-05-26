package queue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/enterprise/cmd/executor/internal/apiclient"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/executor/types"
	internalexecutor "github.com/sourcegraph/sourcegraph/internal/executor"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Client is the client used to communicate with a remote job queue API.
type Client struct {
	options         Options
	client          *apiclient.BaseClient
	logger          log.Logger
	metricsGatherer prometheus.Gatherer
	operations      *operations
}

// Compile time validation.
var _ workerutil.Store[types.Job] = &Client{}
var _ command.ExecutionLogEntryStore = &Client{}

func New(observationCtx *observation.Context, options Options, metricsGatherer prometheus.Gatherer) (*Client, error) {
	logger := log.Scoped("executor-api-queue-client", "The API client adapter for executors to use dbworkers over HTTP")
	client, err := apiclient.NewBaseClient(logger, options.BaseClientOptions)
	if err != nil {
		return nil, err
	}
	return &Client{
		options:         options,
		client:          client,
		logger:          logger,
		metricsGatherer: metricsGatherer,
		operations:      newOperations(observationCtx),
	}, nil
}

func (c *Client) QueuedCount(ctx context.Context) (int, error) {
	return 0, errors.New("unimplemented")
}

func (c *Client) Dequeue(ctx context.Context, workerHostname string, extraArguments any) (job types.Job, _ bool, err error) {
	var queueAttr attribute.KeyValue
	var endpoint string
	dequeueRequest := types.DequeueRequest{
		Version:      version.Version(),
		ExecutorName: c.options.ExecutorName,
		NumCPUs:      c.options.ResourceOptions.NumCPUs,
		Memory:       c.options.ResourceOptions.Memory,
		DiskSpace:    c.options.ResourceOptions.DiskSpace,
	}

	if len(c.options.QueueNames) > 0 {
		queueAttr = attribute.StringSlice("queueNames", c.options.QueueNames)
		endpoint = "/dequeue"
		dequeueRequest.Queues = c.options.QueueNames
	} else {
		queueAttr = attribute.String("queueName", c.options.QueueName)
		endpoint = fmt.Sprintf("%s/dequeue", c.options.QueueName)
	}

	ctx, _, endObservation := c.operations.dequeue.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		queueAttr,
	}})
	defer endObservation(1, observation.Args{})

	req, err := c.client.NewJSONRequest(http.MethodPost, endpoint, dequeueRequest)
	if err != nil {
		return job, false, err
	}

	decoded, err := c.client.DoAndDecode(ctx, req, &job)
	return job, decoded, err
}

func (c *Client) MarkComplete(ctx context.Context, job types.Job) (_ bool, err error) {
	queue := c.inferQueueName(job)
	ctx, _, endObservation := c.operations.markComplete.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("queueName", queue),
		attribute.Int("jobID", job.ID),
	}})
	defer endObservation(1, observation.Args{})

	req, err := c.client.NewJSONJobRequest(job.ID, http.MethodPost, fmt.Sprintf("%s/markComplete", queue), job.Token, types.MarkCompleteRequest{
		JobOperationRequest: types.JobOperationRequest{
			ExecutorName: c.options.ExecutorName,
			JobID:        job.ID,
		},
	})
	if err != nil {
		return false, err
	}

	if err = c.client.DoAndDrop(ctx, req); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Client) MarkErrored(ctx context.Context, job types.Job, failureMessage string) (_ bool, err error) {
	queue := c.inferQueueName(job)
	ctx, _, endObservation := c.operations.markErrored.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("queueName", queue),
		attribute.Int("jobID", job.ID),
	}})
	defer endObservation(1, observation.Args{})

	req, err := c.client.NewJSONJobRequest(job.ID, http.MethodPost, fmt.Sprintf("%s/markErrored", queue), job.Token, types.MarkErroredRequest{
		JobOperationRequest: types.JobOperationRequest{
			ExecutorName: c.options.ExecutorName,
			JobID:        job.ID,
		},
		ErrorMessage: failureMessage,
	})
	if err != nil {
		return false, err
	}

	if err = c.client.DoAndDrop(ctx, req); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Client) MarkFailed(ctx context.Context, job types.Job, failureMessage string) (_ bool, err error) {
	queue := c.inferQueueName(job)
	ctx, _, endObservation := c.operations.markFailed.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("queueName", queue),
		attribute.Int("jobID", job.ID),
	}})
	defer endObservation(1, observation.Args{})

	req, err := c.client.NewJSONJobRequest(job.ID, http.MethodPost, fmt.Sprintf("%s/markFailed", queue), job.Token, types.MarkErroredRequest{
		JobOperationRequest: types.JobOperationRequest{
			ExecutorName: c.options.ExecutorName,
			JobID:        job.ID,
		},
		ErrorMessage: failureMessage,
	})
	if err != nil {
		return false, err
	}

	if err = c.client.DoAndDrop(ctx, req); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Client) Heartbeat(ctx context.Context, jobIDs []string) (knownIDs, cancelIDs []string, err error) {
	metrics, err := gatherMetrics(c.logger, c.metricsGatherer)
	if err != nil {
		c.logger.Error("Failed to collect prometheus metrics for heartbeat", log.Error(err))
		// Continue, no metric errors should prevent heartbeats.
	}

	var queueAttr attribute.KeyValue
	var endpoint string
	heartbeatRequest := types.HeartbeatRequest{
		ExecutorName: c.options.ExecutorName,

		OS:              c.options.TelemetryOptions.OS,
		Architecture:    c.options.TelemetryOptions.Architecture,
		DockerVersion:   c.options.TelemetryOptions.DockerVersion,
		ExecutorVersion: c.options.TelemetryOptions.ExecutorVersion,
		GitVersion:      c.options.TelemetryOptions.GitVersion,
		IgniteVersion:   c.options.TelemetryOptions.IgniteVersion,
		SrcCliVersion:   c.options.TelemetryOptions.SrcCliVersion,

		PrometheusMetrics: metrics,
	}
	if len(c.options.QueueNames) > 0 {
		queueAttr = attribute.StringSlice("queueNames", c.options.QueueNames)
		endpoint = "/heartbeat"
		queueJobIDs, err := parseJobIDs(jobIDs)
		if err != nil {
			c.logger.Error("failed to parse job IDs", log.Error(err))
			return nil, nil, err
		}
		heartbeatRequest.JobIDsByQueue = queueJobIDs
		heartbeatRequest.QueueNames = c.options.QueueNames
	} else {
		queueAttr = attribute.String("queueName", c.options.QueueName)
		endpoint = fmt.Sprintf("%s/heartbeat", c.options.QueueName)
		heartbeatRequest.JobIDs = jobIDs
	}
	ctx, _, endObservation := c.operations.heartbeat.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		queueAttr,
		attribute.StringSlice("jobIDs", jobIDs),
	}})
	defer endObservation(1, observation.Args{})

	req, err := c.client.NewJSONRequest(http.MethodPost, endpoint, heartbeatRequest)
	if err != nil {
		return nil, nil, err
	}

	// Do the request and get the reader for the response body.
	_, body, err := c.client.Do(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	// Now read the response body into a buffer, so that we can decode it twice.
	// This will always be small, so no problem that we don't stream this.
	defer body.Close()
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, nil, err
	}

	// First, try to unmarshal the response into a V2 response object.
	var respV2 types.HeartbeatResponse
	if err := json.Unmarshal(bodyBytes, &respV2); err == nil {
		// If that works, we can return the data.
		return respV2.KnownIDs, respV2.CancelIDs, nil
	}
	return nil, nil, err
}

func parseJobIDs(jobIDs []string) ([]types.QueueJobIDs, error) {
	var queueJobIDs []types.QueueJobIDs
	queueIds := map[string][]string{}
	for _, stringID := range jobIDs {
		parts := strings.Split(stringID, "-")
		if len(parts) != 2 {
			return nil, errors.Newf("Unexpected job ID format: %s", stringID)
		}

		id, queueName := parts[0], parts[1]

		if _, ok := queueIds[queueName]; ok {
			queueIds[queueName] = append(queueIds[queueName], id)
		} else {
			queueIds[queueName] = []string{id}
		}
	}
	for q, ids := range queueIds {
		queueJobIDs = append(queueJobIDs, types.QueueJobIDs{QueueName: q, JobIDs: ids})
	}
	return queueJobIDs, nil
}

func gatherMetrics(logger log.Logger, gatherer prometheus.Gatherer) (string, error) {
	maxDuration := 3 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), maxDuration)
	defer cancel()
	go func() {
		<-ctx.Done()
		if ctx.Err() == context.DeadlineExceeded {
			logger.Warn("gathering metrics took longer than expected", log.Duration("maxDuration", maxDuration))
		}
	}()
	mfs, err := gatherer.Gather()
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	enc := expfmt.NewEncoder(&buf, expfmt.FmtText)
	for _, mf := range mfs {
		if err = enc.Encode(mf); err != nil {
			return "", errors.Wrap(err, "encoding metric family")
		}
	}
	return buf.String(), nil
}

func (c *Client) Ping(ctx context.Context) (err error) {
	var req *http.Request
	if len(c.options.QueueNames) > 0 {
		req, err = c.client.NewJSONRequest(http.MethodPost, fmt.Sprintf("/heartbeat"), types.HeartbeatRequest{
			ExecutorName: c.options.ExecutorName,
			QueueNames:   c.options.QueueNames,
		})
	} else {
		req, err = c.client.NewJSONRequest(http.MethodPost, fmt.Sprintf("%s/heartbeat", c.options.QueueName), types.HeartbeatRequest{
			ExecutorName: c.options.ExecutorName,
		})
	}
	if err != nil {
		return err
	}

	return c.client.DoAndDrop(ctx, req)
}

func (c *Client) AddExecutionLogEntry(ctx context.Context, job types.Job, entry internalexecutor.ExecutionLogEntry) (entryID int, err error) {
	queue := c.inferQueueName(job)

	ctx, _, endObservation := c.operations.addExecutionLogEntry.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("queueName", queue),
		attribute.Int("jobID", job.ID),
	}})
	defer endObservation(1, observation.Args{})

	req, err := c.client.NewJSONJobRequest(job.ID, http.MethodPost, fmt.Sprintf("%s/addExecutionLogEntry", queue), job.Token, types.AddExecutionLogEntryRequest{
		JobOperationRequest: types.JobOperationRequest{
			ExecutorName: c.options.ExecutorName,
			JobID:        job.ID,
		},
		ExecutionLogEntry: entry,
	})
	if err != nil {
		return entryID, err
	}

	_, err = c.client.DoAndDecode(ctx, req, &entryID)
	return entryID, err
}

func (c *Client) UpdateExecutionLogEntry(ctx context.Context, job types.Job, entryID int, entry internalexecutor.ExecutionLogEntry) (err error) {
	queue := c.inferQueueName(job)

	ctx, _, endObservation := c.operations.updateExecutionLogEntry.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("queueName", queue),
		attribute.Int("jobID", job.ID),
		attribute.Int("entryID", entryID),
	}})
	defer endObservation(1, observation.Args{})

	req, err := c.client.NewJSONJobRequest(job.ID, http.MethodPost, fmt.Sprintf("%s/updateExecutionLogEntry", queue), job.Token, types.UpdateExecutionLogEntryRequest{
		JobOperationRequest: types.JobOperationRequest{
			ExecutorName: c.options.ExecutorName,
			JobID:        job.ID,
		},
		EntryID:           entryID,
		ExecutionLogEntry: entry,
	})
	if err != nil {
		return err
	}

	return c.client.DoAndDrop(ctx, req)
}

// inferQueueName returns the queue name if it is specified on the job, which is the case
// when an executor is configured to listen to multiple queues. If the queue name is empty,
// return the specific queue that is configured.
func (c *Client) inferQueueName(job types.Job) string {
	if job.Queue != "" {
		return job.Queue
	} else {
		return c.options.QueueName
	}
}
