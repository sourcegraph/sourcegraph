package handler_test

import (
	"context"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/executorqueue/handler"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	executorstore "github.com/sourcegraph/sourcegraph/internal/executor/store"
	executortypes "github.com/sourcegraph/sourcegraph/internal/executor/types"
	metricsstore "github.com/sourcegraph/sourcegraph/internal/metrics/store"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	dbworkerstoremocks "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store/mocks"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

type dequeueEvent struct {
	queueName            string
	expectedStatusCode   int
	expectedResponseBody string
}

func transformerFunc[T workerutil.Record](ctx context.Context, version string, t T, resourceMetadata handler.ResourceMetadata) (executortypes.Job, error) {
	return executortypes.Job{ID: t.RecordID()}, nil
}

type dequeueTestCase struct {
	name string
	body string

	// If this is set, we expect all queues to be empty - otherwise, it's configured on a specific dequeue event
	// only valid status code for this field is http.StatusNoContent
	expectedStatusCode int

	mockFunc      func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore)
	assertionFunc func(t *testing.T, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore)

	dequeueEvents            []dequeueEvent
	codeintelTransformerFunc handler.TransformerFunc[uploadsshared.Index]
	batchesTransformerFunc   handler.TransformerFunc[*btypes.BatchSpecWorkspaceExecutionJob]
}

func TestMultiHandler_HandleDequeue(t *testing.T) {
	tests := []dequeueTestCase{
		{
			name: "Dequeue one record for each queue",
			body: `{"executorName": "test-executor", "numCPUs": 1, "memory": "1GB", "diskSpace": "10GB","queues": ["codeintel", "batches"]}`,
			mockFunc: func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				// QueuedCount gets called for each queue in queues on every invocation of HandleDequeue to filter empty queues,
				// so two calls are mocked for two dequeue events. Functionally it doesn't really matter what these return, but
				// for the sake of accuracy, the codeintel store returns 1 less. The batches store returns the same value because
				// the batches job isn't dequeued until after the second call to QueuedCount.
				codeintelMockStore.QueuedCountFunc.PushReturn(2, nil)
				codeintelMockStore.QueuedCountFunc.PushReturn(1, nil)
				batchesMockStore.QueuedCountFunc.PushReturn(2, nil)
				batchesMockStore.QueuedCountFunc.PushReturn(2, nil)

				codeintelMockStore.DequeueFunc.PushReturn(uploadsshared.Index{ID: 1}, true, nil)
				jobTokenStore.CreateFunc.PushReturn("token1", nil)
				batchesMockStore.DequeueFunc.PushReturn(&btypes.BatchSpecWorkspaceExecutionJob{ID: 2}, true, nil)
				jobTokenStore.CreateFunc.PushReturn("token2", nil)
			},
			assertionFunc: func(t *testing.T, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				require.Len(t, jobTokenStore.CreateFunc.History(), 2)

				require.Len(t, codeintelMockStore.QueuedCountFunc.History(), 2)
				require.Len(t, batchesMockStore.QueuedCountFunc.History(), 2)
				require.Len(t, codeintelMockStore.DequeueFunc.History(), 1)
				assert.Equal(t, "test-executor", codeintelMockStore.DequeueFunc.History()[0].Arg1)
				assert.Nil(t, codeintelMockStore.DequeueFunc.History()[0].Arg2)
				assert.Equal(t, 1, jobTokenStore.CreateFunc.History()[0].Arg1)
				assert.Equal(t, "codeintel", jobTokenStore.CreateFunc.History()[0].Arg2)

				require.Len(t, batchesMockStore.DequeueFunc.History(), 1)
				assert.Equal(t, "test-executor", batchesMockStore.DequeueFunc.History()[0].Arg1)
				assert.Nil(t, batchesMockStore.DequeueFunc.History()[0].Arg2)
				assert.Equal(t, 2, jobTokenStore.CreateFunc.History()[1].Arg1)
				assert.Equal(t, "batches", jobTokenStore.CreateFunc.History()[1].Arg2)
			},
			dequeueEvents: []dequeueEvent{
				{
					queueName:            "codeintel",
					expectedStatusCode:   http.StatusOK,
					expectedResponseBody: `{"id":1,"token":"token1","queue":"codeintel","repositoryName":"","repositoryDirectory":"","commit":"","fetchTags":false,"shallowClone":false,"sparseCheckout":null,"files":{},"dockerSteps":null,"cliSteps":null,"redactedValues":null}`,
				},
				{
					queueName:            "batches",
					expectedStatusCode:   http.StatusOK,
					expectedResponseBody: `{"id":2,"token":"token2","queue":"batches","repositoryName":"","repositoryDirectory":"","commit":"","fetchTags":false,"shallowClone":false,"sparseCheckout":null,"files":{},"dockerSteps":null,"cliSteps":null,"redactedValues":null}`,
				},
			},
		},
		{
			name: "Dequeue only codeintel record when requesting codeintel queue and batches record exists",
			body: `{"executorName": "test-executor", "numCPUs": 1, "memory": "1GB", "diskSpace": "10GB","queues": ["codeintel"]}`,
			mockFunc: func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				// On the second event, the queue will be empty and return an empty job
				codeintelMockStore.QueuedCountFunc.PushReturn(1, nil)
				codeintelMockStore.DequeueFunc.PushReturn(uploadsshared.Index{ID: 1}, true, nil)
				// Mock a non-empty queue that will never be reached because it's not requested in the dequeue body
				batchesMockStore.QueuedCountFunc.PushReturn(1, nil)
				batchesMockStore.DequeueFunc.PushReturn(&btypes.BatchSpecWorkspaceExecutionJob{ID: 2}, true, nil)
				jobTokenStore.CreateFunc.PushReturn("token1", nil)
			},
			assertionFunc: func(t *testing.T, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				require.Len(t, jobTokenStore.CreateFunc.History(), 1)

				// The queue will be empty after the first dequeue event, so no second dequeue happens
				require.Len(t, codeintelMockStore.DequeueFunc.History(), 1)
				assert.Equal(t, "test-executor", codeintelMockStore.DequeueFunc.History()[0].Arg1)
				assert.Nil(t, codeintelMockStore.DequeueFunc.History()[0].Arg2)
				assert.Equal(t, 1, jobTokenStore.CreateFunc.History()[0].Arg1)
				assert.Equal(t, "codeintel", jobTokenStore.CreateFunc.History()[0].Arg2)

				require.Len(t, batchesMockStore.QueuedCountFunc.History(), 0)
				require.Len(t, batchesMockStore.DequeueFunc.History(), 0)
			},
			dequeueEvents: []dequeueEvent{
				{
					queueName:            "codeintel",
					expectedStatusCode:   http.StatusOK,
					expectedResponseBody: `{"id":1,"token":"token1","queue":"codeintel","repositoryName":"","repositoryDirectory":"","commit":"","fetchTags":false,"shallowClone":false,"sparseCheckout":null,"files":{},"dockerSteps":null,"cliSteps":null,"redactedValues":null}`,
				},
				{
					queueName:          "batches",
					expectedStatusCode: http.StatusNoContent,
				},
			},
		},
		{
			name: "Dequeue only codeintel record when requesting both queues and batches record doesn't exists",
			body: `{"executorName": "test-executor", "numCPUs": 1, "memory": "1GB", "diskSpace": "10GB","queues": ["codeintel", "batches"]}`,
			mockFunc: func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				codeintelMockStore.QueuedCountFunc.PushReturn(1, nil)
				codeintelMockStore.QueuedCountFunc.PushReturn(0, nil)
				batchesMockStore.QueuedCountFunc.PushReturn(0, nil)
				codeintelMockStore.DequeueFunc.PushReturn(uploadsshared.Index{ID: 1}, true, nil)
				jobTokenStore.CreateFunc.PushReturn("token1", nil)
			},
			assertionFunc: func(t *testing.T, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				require.Len(t, jobTokenStore.CreateFunc.History(), 1)

				require.Len(t, codeintelMockStore.QueuedCountFunc.History(), 2)
				require.Len(t, batchesMockStore.QueuedCountFunc.History(), 2)
				require.Len(t, codeintelMockStore.DequeueFunc.History(), 1)
				assert.Equal(t, "test-executor", codeintelMockStore.DequeueFunc.History()[0].Arg1)
				assert.Nil(t, codeintelMockStore.DequeueFunc.History()[0].Arg2)
				assert.Equal(t, 1, jobTokenStore.CreateFunc.History()[0].Arg1)
				assert.Equal(t, "codeintel", jobTokenStore.CreateFunc.History()[0].Arg2)

				require.Len(t, batchesMockStore.DequeueFunc.History(), 0)
			},
			dequeueEvents: []dequeueEvent{
				{
					queueName:            "codeintel",
					expectedStatusCode:   http.StatusOK,
					expectedResponseBody: `{"id":1,"token":"token1","queue":"codeintel","repositoryName":"","repositoryDirectory":"","commit":"","fetchTags":false,"shallowClone":false,"sparseCheckout":null,"files":{},"dockerSteps":null,"cliSteps":null,"redactedValues":null}`,
				},
				{
					queueName:          "batches",
					expectedStatusCode: http.StatusNoContent,
				},
			},
		},
		{
			name: "Nothing to dequeue",
			body: `{"executorName": "test-executor", "numCPUs": 1, "memory": "1GB", "diskSpace": "10GB","queues": ["codeintel","batches"]}`,
			mockFunc: func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				codeintelMockStore.DequeueFunc.PushReturn(uploadsshared.Index{}, false, nil)
				batchesMockStore.DequeueFunc.PushReturn(&btypes.BatchSpecWorkspaceExecutionJob{}, false, nil)
			},
			assertionFunc: func(t *testing.T, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				require.Len(t, codeintelMockStore.DequeueFunc.History(), 1)
				require.Len(t, batchesMockStore.DequeueFunc.History(), 1)
				require.Len(t, jobTokenStore.CreateFunc.History(), 0)
			},
			expectedStatusCode: http.StatusNoContent,
		},
		{
			name: "No queue names provided",
			body: `{"executorName": "test-executor", "numCPUs": 1, "memory": "1GB", "diskSpace": "10GB","queues": []}`,
			assertionFunc: func(t *testing.T, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				require.Len(t, codeintelMockStore.QueuedCountFunc.History(), 0)
				require.Len(t, batchesMockStore.QueuedCountFunc.History(), 0)
				require.Len(t, codeintelMockStore.DequeueFunc.History(), 0)
				require.Len(t, batchesMockStore.DequeueFunc.History(), 0)
				require.Len(t, jobTokenStore.CreateFunc.History(), 0)
			},
			expectedStatusCode: http.StatusNoContent,
		},
		{
			name: "Invalid queue name",
			body: `{"executorName": "test-executor", "numCPUs": 1, "memory": "1GB", "diskSpace": "10GB","queues": ["invalidqueue"]}`,
			assertionFunc: func(t *testing.T, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				require.Len(t, codeintelMockStore.QueuedCountFunc.History(), 0)
				require.Len(t, batchesMockStore.QueuedCountFunc.History(), 0)
				require.Len(t, codeintelMockStore.DequeueFunc.History(), 0)
				require.Len(t, batchesMockStore.DequeueFunc.History(), 0)
				require.Len(t, jobTokenStore.CreateFunc.History(), 0)
			},
			dequeueEvents: []dequeueEvent{
				{
					expectedStatusCode:   http.StatusInternalServerError,
					expectedResponseBody: `{"error":"Invalid queue name(s) 'invalidqueue' found. Supported queue names are 'batches, codeintel'."}`,
				},
			},
		},
		{
			name: "Invalid version",
			body: `{"executorName": "test-executor", "version":"\n1.2", "numCPUs": 1, "memory": "1GB", "diskSpace": "10GB","queues": ["codeintel","batches"]}`,
			assertionFunc: func(t *testing.T, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				require.Len(t, codeintelMockStore.QueuedCountFunc.History(), 0)
				require.Len(t, batchesMockStore.QueuedCountFunc.History(), 0)
				require.Len(t, codeintelMockStore.DequeueFunc.History(), 0)
				require.Len(t, batchesMockStore.DequeueFunc.History(), 0)
				require.Len(t, jobTokenStore.CreateFunc.History(), 0)
			},
			dequeueEvents: []dequeueEvent{
				{
					expectedStatusCode:   http.StatusInternalServerError,
					expectedResponseBody: `{"error":"failed to check version \"\\n1.2\": Invalid Semantic Version"}`,
				},
			},
		},
		{
			name: "Dequeue error codeintel",
			body: `{"executorName": "test-executor", "numCPUs": 1, "memory": "1GB", "diskSpace": "10GB","queues": ["codeintel"]}`,
			mockFunc: func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				codeintelMockStore.QueuedCountFunc.PushReturn(1, nil)
				codeintelMockStore.DequeueFunc.PushReturn(uploadsshared.Index{}, false, errors.New("failed to dequeue"))
			},
			assertionFunc: func(t *testing.T, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				require.Len(t, codeintelMockStore.QueuedCountFunc.History(), 1)
				require.Len(t, batchesMockStore.QueuedCountFunc.History(), 0)
				require.Len(t, codeintelMockStore.DequeueFunc.History(), 1)
				require.Len(t, batchesMockStore.DequeueFunc.History(), 0)
				require.Len(t, jobTokenStore.CreateFunc.History(), 0)
			},
			dequeueEvents: []dequeueEvent{
				{
					queueName:            "codeintel",
					expectedStatusCode:   http.StatusInternalServerError,
					expectedResponseBody: `{"error":"dbworkerstore.Dequeue codeintel: failed to dequeue"}`,
				},
			},
		},
		{
			name: "Dequeue error batches",
			body: `{"executorName": "test-executor", "numCPUs": 1, "memory": "1GB", "diskSpace": "10GB","queues": ["batches"]}`,
			mockFunc: func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				batchesMockStore.QueuedCountFunc.PushReturn(1, nil)
				batchesMockStore.DequeueFunc.PushReturn(&btypes.BatchSpecWorkspaceExecutionJob{}, false, errors.New("failed to dequeue"))
			},
			assertionFunc: func(t *testing.T, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				require.Len(t, codeintelMockStore.QueuedCountFunc.History(), 0)
				require.Len(t, batchesMockStore.QueuedCountFunc.History(), 1)
				require.Len(t, codeintelMockStore.DequeueFunc.History(), 0)
				require.Len(t, batchesMockStore.DequeueFunc.History(), 1)
				require.Len(t, jobTokenStore.CreateFunc.History(), 0)
			},
			dequeueEvents: []dequeueEvent{
				{
					queueName:            "batches",
					expectedStatusCode:   http.StatusInternalServerError,
					expectedResponseBody: `{"error":"dbworkerstore.Dequeue batches: failed to dequeue"}`,
				},
			},
		},
		{
			name: "Failed to transform record codeintel",
			body: `{"executorName": "test-executor", "numCPUs": 1, "memory": "1GB", "diskSpace": "10GB","queues": ["codeintel"]}`,
			mockFunc: func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				codeintelMockStore.QueuedCountFunc.PushReturn(1, nil)
				codeintelMockStore.DequeueFunc.PushReturn(uploadsshared.Index{ID: 1}, true, nil)
				codeintelMockStore.MarkFailedFunc.PushReturn(true, nil)
			},
			assertionFunc: func(t *testing.T, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				require.Len(t, codeintelMockStore.QueuedCountFunc.History(), 1)
				require.Len(t, batchesMockStore.QueuedCountFunc.History(), 0)
				require.Len(t, codeintelMockStore.DequeueFunc.History(), 1)
				require.Len(t, batchesMockStore.DequeueFunc.History(), 0)
				require.Len(t, codeintelMockStore.MarkFailedFunc.History(), 1)
				assert.Equal(t, 1, codeintelMockStore.MarkFailedFunc.History()[0].Arg1)
				assert.Equal(t, "failed to transform record: failed", codeintelMockStore.MarkFailedFunc.History()[0].Arg2)
				assert.Equal(t, dbworkerstore.MarkFinalOptions{}, codeintelMockStore.MarkFailedFunc.History()[0].Arg3)
				require.Len(t, jobTokenStore.CreateFunc.History(), 0)
			},
			dequeueEvents: []dequeueEvent{
				{
					queueName:            "codeintel",
					expectedStatusCode:   http.StatusInternalServerError,
					expectedResponseBody: `{"error":"RecordTransformer codeintel: failed"}`,
				},
			},
			codeintelTransformerFunc: func(ctx context.Context, version string, record uploadsshared.Index, resourceMetadata handler.ResourceMetadata) (executortypes.Job, error) {
				return executortypes.Job{}, errors.New("failed")
			},
		},
		{
			name: "Failed to transform record batches",
			body: `{"executorName": "test-executor", "numCPUs": 1, "memory": "1GB", "diskSpace": "10GB","queues": ["batches"]}`,
			mockFunc: func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				batchesMockStore.QueuedCountFunc.PushReturn(1, nil)
				batchesMockStore.DequeueFunc.PushReturn(&btypes.BatchSpecWorkspaceExecutionJob{ID: 1}, true, nil)
				batchesMockStore.MarkFailedFunc.PushReturn(true, nil)
			},
			assertionFunc: func(t *testing.T, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				require.Len(t, codeintelMockStore.QueuedCountFunc.History(), 0)
				require.Len(t, batchesMockStore.QueuedCountFunc.History(), 1)
				require.Len(t, codeintelMockStore.DequeueFunc.History(), 0)
				require.Len(t, batchesMockStore.DequeueFunc.History(), 1)
				require.Len(t, batchesMockStore.MarkFailedFunc.History(), 1)
				assert.Equal(t, 1, batchesMockStore.MarkFailedFunc.History()[0].Arg1)
				assert.Equal(t, "failed to transform record: failed", batchesMockStore.MarkFailedFunc.History()[0].Arg2)
				assert.Equal(t, dbworkerstore.MarkFinalOptions{}, batchesMockStore.MarkFailedFunc.History()[0].Arg3)
				require.Len(t, jobTokenStore.CreateFunc.History(), 0)
			},
			dequeueEvents: []dequeueEvent{
				{
					queueName:            "batches",
					expectedStatusCode:   http.StatusInternalServerError,
					expectedResponseBody: `{"error":"RecordTransformer batches: failed"}`,
				},
			},
			batchesTransformerFunc: func(ctx context.Context, version string, record *btypes.BatchSpecWorkspaceExecutionJob, resourceMetadata handler.ResourceMetadata) (executortypes.Job, error) {
				return executortypes.Job{}, errors.New("failed")
			},
		},
		{
			name: "Failed to mark record as failed codeintel",
			body: `{"executorName": "test-executor", "numCPUs": 1, "memory": "1GB", "diskSpace": "10GB","queues": ["codeintel"]}`,
			mockFunc: func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				codeintelMockStore.QueuedCountFunc.PushReturn(1, nil)
				codeintelMockStore.DequeueFunc.PushReturn(uploadsshared.Index{ID: 1}, true, nil)
				codeintelMockStore.MarkFailedFunc.PushReturn(true, errors.New("failed to mark"))
			},
			assertionFunc: func(t *testing.T, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				require.Len(t, codeintelMockStore.QueuedCountFunc.History(), 1)
				require.Len(t, batchesMockStore.QueuedCountFunc.History(), 0)
				require.Len(t, codeintelMockStore.DequeueFunc.History(), 1)
				require.Len(t, batchesMockStore.DequeueFunc.History(), 0)
				require.Len(t, codeintelMockStore.MarkFailedFunc.History(), 1)
				assert.Equal(t, 1, codeintelMockStore.MarkFailedFunc.History()[0].Arg1)
				assert.Equal(t, "failed to transform record: failed", codeintelMockStore.MarkFailedFunc.History()[0].Arg2)
				assert.Equal(t, dbworkerstore.MarkFinalOptions{}, codeintelMockStore.MarkFailedFunc.History()[0].Arg3)
				require.Len(t, jobTokenStore.CreateFunc.History(), 0)
			},
			dequeueEvents: []dequeueEvent{
				{
					queueName:            "codeintel",
					expectedStatusCode:   http.StatusInternalServerError,
					expectedResponseBody: `{"error":"RecordTransformer codeintel: 2 errors occurred:\n\t* failed\n\t* failed to mark"}`,
				},
			},
			codeintelTransformerFunc: func(ctx context.Context, version string, record uploadsshared.Index, resourceMetadata handler.ResourceMetadata) (executortypes.Job, error) {
				return executortypes.Job{}, errors.New("failed")
			},
		},
		{
			name: "Failed to mark record as failed batches",
			body: `{"executorName": "test-executor", "numCPUs": 1, "memory": "1GB", "diskSpace": "10GB","queues": ["batches"]}`,
			mockFunc: func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				batchesMockStore.QueuedCountFunc.PushReturn(1, nil)
				batchesMockStore.DequeueFunc.PushReturn(&btypes.BatchSpecWorkspaceExecutionJob{ID: 1}, true, nil)
				batchesMockStore.MarkFailedFunc.PushReturn(true, errors.New("failed to mark"))
			},
			assertionFunc: func(t *testing.T, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				require.Len(t, codeintelMockStore.QueuedCountFunc.History(), 0)
				require.Len(t, batchesMockStore.QueuedCountFunc.History(), 1)
				require.Len(t, codeintelMockStore.DequeueFunc.History(), 0)
				require.Len(t, batchesMockStore.DequeueFunc.History(), 1)
				assert.Equal(t, 1, batchesMockStore.MarkFailedFunc.History()[0].Arg1)
				assert.Equal(t, "failed to transform record: failed", batchesMockStore.MarkFailedFunc.History()[0].Arg2)
				assert.Equal(t, dbworkerstore.MarkFinalOptions{}, batchesMockStore.MarkFailedFunc.History()[0].Arg3)
				require.Len(t, jobTokenStore.CreateFunc.History(), 0)
			},
			dequeueEvents: []dequeueEvent{
				{
					queueName:            "batches",
					expectedStatusCode:   http.StatusInternalServerError,
					expectedResponseBody: `{"error":"RecordTransformer batches: 2 errors occurred:\n\t* failed\n\t* failed to mark"}`,
				},
			},
			batchesTransformerFunc: func(ctx context.Context, version string, record *btypes.BatchSpecWorkspaceExecutionJob, resourceMetadata handler.ResourceMetadata) (executortypes.Job, error) {
				return executortypes.Job{}, errors.New("failed")
			},
		},
		{
			name: "Failed to create job token",
			body: `{"executorName": "test-executor", "numCPUs": 1, "memory": "1GB", "diskSpace": "10GB","queues": ["codeintel","batches"]}`,
			mockFunc: func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				codeintelMockStore.QueuedCountFunc.PushReturn(1, nil)
				codeintelMockStore.DequeueFunc.PushReturn(uploadsshared.Index{ID: 1}, true, nil)
				jobTokenStore.CreateFunc.PushReturn("", errors.New("failed to create token"))
			},
			assertionFunc: func(t *testing.T, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				require.Len(t, codeintelMockStore.QueuedCountFunc.History(), 1)
				require.Len(t, codeintelMockStore.DequeueFunc.History(), 1)
				require.Len(t, jobTokenStore.CreateFunc.History(), 1)
				require.Len(t, jobTokenStore.RegenerateFunc.History(), 0)
			},
			dequeueEvents: []dequeueEvent{
				{
					queueName:            "codeintel",
					expectedStatusCode:   http.StatusInternalServerError,
					expectedResponseBody: `{"error":"CreateToken: failed to create token"}`,
				},
			},
		},
		{
			name: "Job token already exists",
			body: `{"executorName": "test-executor","numCPUs": 1, "memory": "1GB", "diskSpace": "10GB","queues": ["codeintel","batches"]}`,
			mockFunc: func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				codeintelMockStore.QueuedCountFunc.PushReturn(1, nil)
				codeintelMockStore.DequeueFunc.PushReturn(uploadsshared.Index{ID: 1}, true, nil)
				jobTokenStore.CreateFunc.PushReturn("", executorstore.ErrJobTokenAlreadyCreated)
				jobTokenStore.RegenerateFunc.PushReturn("somenewtoken", nil)
			},
			assertionFunc: func(t *testing.T, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				require.Len(t, codeintelMockStore.QueuedCountFunc.History(), 1)
				require.Len(t, codeintelMockStore.DequeueFunc.History(), 1)
				require.Len(t, jobTokenStore.CreateFunc.History(), 1)
				require.Len(t, jobTokenStore.RegenerateFunc.History(), 1)
				assert.Equal(t, 1, jobTokenStore.RegenerateFunc.History()[0].Arg1)
				assert.Equal(t, "codeintel", jobTokenStore.RegenerateFunc.History()[0].Arg2)
			},
			dequeueEvents: []dequeueEvent{
				{
					queueName:            "codeintel",
					expectedStatusCode:   http.StatusOK,
					expectedResponseBody: `{"id":1,"token":"somenewtoken","queue":"codeintel", "repositoryName":"","repositoryDirectory":"","commit":"","fetchTags":false,"shallowClone":false,"sparseCheckout":null,"files":{},"dockerSteps":null,"cliSteps":null,"redactedValues":null}`,
				},
			},
		},
		{
			name: "Failed to regenerate token",
			body: `{"executorName": "test-executor","numCPUs": 1, "memory": "1GB", "diskSpace": "10GB","queues": ["codeintel","batches"]}`,
			mockFunc: func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				codeintelMockStore.QueuedCountFunc.PushReturn(1, nil)
				codeintelMockStore.DequeueFunc.PushReturn(uploadsshared.Index{ID: 1}, true, nil)
				jobTokenStore.CreateFunc.PushReturn("", executorstore.ErrJobTokenAlreadyCreated)
				jobTokenStore.RegenerateFunc.PushReturn("", errors.New("failed to regen token"))
			},
			assertionFunc: func(t *testing.T, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob], jobTokenStore *executorstore.MockJobTokenStore) {
				require.Len(t, codeintelMockStore.QueuedCountFunc.History(), 1)
				require.Len(t, codeintelMockStore.DequeueFunc.History(), 1)
				require.Len(t, jobTokenStore.CreateFunc.History(), 1)
				require.Len(t, jobTokenStore.RegenerateFunc.History(), 1)
			},
			dequeueEvents: []dequeueEvent{
				{
					queueName:            "codeintel",
					expectedStatusCode:   http.StatusInternalServerError,
					expectedResponseBody: `{"error":"RegenerateToken: failed to regen token"}`,
				},
			},
		},
	}

	realSelect := handler.DoSelectQueueForDequeueing
	mockSiteConfig()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rcache.SetupForTest(t)
			jobTokenStore := executorstore.NewMockJobTokenStore()
			codeIntelMockStore := dbworkerstoremocks.NewMockStore[uploadsshared.Index]()
			batchesMockStore := dbworkerstoremocks.NewMockStore[*btypes.BatchSpecWorkspaceExecutionJob]()

			mh := handler.NewMultiHandler(
				dbmocks.NewMockExecutorStore(),
				jobTokenStore,
				metricsstore.NewMockDistributedStore(),
				handler.QueueHandler[uploadsshared.Index]{Name: "codeintel", Store: codeIntelMockStore, RecordTransformer: transformerFunc[uploadsshared.Index]},
				handler.QueueHandler[*btypes.BatchSpecWorkspaceExecutionJob]{Name: "batches", Store: batchesMockStore, RecordTransformer: transformerFunc[*btypes.BatchSpecWorkspaceExecutionJob]},
			)

			router := mux.NewRouter()
			router.HandleFunc("/dequeue", mh.HandleDequeue)

			if test.mockFunc != nil {
				test.mockFunc(codeIntelMockStore, batchesMockStore, jobTokenStore)
			}

			if test.expectedStatusCode != 0 {
				evaluateEvent(test.body, test.expectedStatusCode, "", t, router)
			} else {
				for _, event := range test.dequeueEvents {
					if test.codeintelTransformerFunc != nil {
						mh.CodeIntelQueueHandler.RecordTransformer = test.codeintelTransformerFunc
					}
					if test.batchesTransformerFunc != nil {
						mh.BatchesQueueHandler.RecordTransformer = test.batchesTransformerFunc
					}
					// mock random queue picking to return the expected queue name
					handler.DoSelectQueueForDequeueing = func(candidateQueues []string, config *schema.DequeueCacheConfig) (string, error) {
						return event.queueName, nil
					}
					evaluateEvent(test.body, event.expectedStatusCode, event.expectedResponseBody, t, router)
				}

				if test.assertionFunc != nil {
					test.assertionFunc(t, codeIntelMockStore, batchesMockStore, jobTokenStore)
				}
			}
		})
	}
	// reset method to original for other tests
	handler.DoSelectQueueForDequeueing = realSelect
}

func TestMultiHandler_HandleHeartbeat(t *testing.T) {
	tests := []struct {
		name                 string
		body                 string
		mockFunc             func(metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob])
		expectedStatusCode   int
		expectedResponseBody string
		assertionFunc        func(t *testing.T, metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob])
	}{
		{
			name: "Heartbeat for multiple queues",
			body: `{"executorName": "test-executor", "queueNames": ["codeintel", "batches"], "jobIdsByQueue": [{"queueName": "codeintel", "jobIds": ["42", "7"]}, {"queueName": "batches", "jobIds": ["43", "8"]}], "os": "test-os", "architecture": "test-arch", "dockerVersion": "1.0", "executorVersion": "2.0", "gitVersion": "3.0", "igniteVersion": "4.0", "srcCliVersion": "5.0", "prometheusMetrics": ""}`,
			mockFunc: func(metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				executorStore.UpsertHeartbeatFunc.PushReturn(nil)
				codeintelMockStore.HeartbeatFunc.PushReturn([]string{"42", "7"}, nil, nil)
				batchesMockStore.HeartbeatFunc.PushReturn([]string{"43", "8"}, nil, nil)
			},
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: `{"knownIds":["42-codeintel", "7-codeintel", "43-batches", "8-batches"],"cancelIds":null}`,
			assertionFunc: func(t *testing.T, metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				require.Len(t, executorStore.UpsertHeartbeatFunc.History(), 1)

				assert.Equal(
					t,
					types.Executor{
						Hostname:        "test-executor",
						QueueNames:      []string{"codeintel", "batches"},
						OS:              "test-os",
						Architecture:    "test-arch",
						DockerVersion:   "1.0",
						ExecutorVersion: "2.0",
						GitVersion:      "3.0",
						IgniteVersion:   "4.0",
						SrcCliVersion:   "5.0",
					},
					executorStore.UpsertHeartbeatFunc.History()[0].Arg1,
				)
				require.Len(t, codeintelMockStore.HeartbeatFunc.History(), 1)
				assert.Equal(t, []string{"42", "7"}, codeintelMockStore.HeartbeatFunc.History()[0].Arg1)
				assert.Equal(t, dbworkerstore.HeartbeatOptions{WorkerHostname: "test-executor"}, codeintelMockStore.HeartbeatFunc.History()[0].Arg2)

				require.Len(t, batchesMockStore.HeartbeatFunc.History(), 1)
				assert.Equal(t, []string{"43", "8"}, batchesMockStore.HeartbeatFunc.History()[0].Arg1)
				assert.Equal(t, dbworkerstore.HeartbeatOptions{WorkerHostname: "test-executor"}, batchesMockStore.HeartbeatFunc.History()[0].Arg2)
			},
		},
		{
			name: "Heartbeat for single queue",
			body: `{"executorName": "test-executor", "queueNames": ["codeintel"], "jobIdsByQueue": [{"queueName": "codeintel", "jobIds": ["42", "7"]}], "os": "test-os", "architecture": "test-arch", "dockerVersion": "1.0", "executorVersion": "2.0", "gitVersion": "3.0", "igniteVersion": "4.0", "srcCliVersion": "5.0", "prometheusMetrics": ""}`,
			mockFunc: func(metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				executorStore.UpsertHeartbeatFunc.PushReturn(nil)
				codeintelMockStore.HeartbeatFunc.PushReturn([]string{"42", "7"}, nil, nil)
			},
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: `{"knownIds":["42-codeintel", "7-codeintel"],"cancelIds":null}`,
			assertionFunc: func(t *testing.T, metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				require.Len(t, executorStore.UpsertHeartbeatFunc.History(), 1)

				assert.Equal(
					t,
					types.Executor{
						Hostname:        "test-executor",
						QueueNames:      []string{"codeintel"},
						OS:              "test-os",
						Architecture:    "test-arch",
						DockerVersion:   "1.0",
						ExecutorVersion: "2.0",
						GitVersion:      "3.0",
						IgniteVersion:   "4.0",
						SrcCliVersion:   "5.0",
					},
					executorStore.UpsertHeartbeatFunc.History()[0].Arg1,
				)
				require.Len(t, codeintelMockStore.HeartbeatFunc.History(), 1)
				assert.Equal(t, []string{"42", "7"}, codeintelMockStore.HeartbeatFunc.History()[0].Arg1)
				assert.Equal(t, dbworkerstore.HeartbeatOptions{WorkerHostname: "test-executor"}, codeintelMockStore.HeartbeatFunc.History()[0].Arg2)

				require.Len(t, batchesMockStore.HeartbeatFunc.History(), 0)
			},
		},
		{
			name: "No running jobs",
			body: `{"executorName": "test-executor", "queueNames": ["codeintel", "batches"], "jobIdsByQueue": [], "os": "test-os", "architecture": "test-arch", "dockerVersion": "1.0", "executorVersion": "2.0", "gitVersion": "3.0", "igniteVersion": "4.0", "srcCliVersion": "5.0", "prometheusMetrics": ""}`,
			mockFunc: func(metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				executorStore.UpsertHeartbeatFunc.PushReturn(nil)
			},
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: `{"knownIds":null,"cancelIds":null}`,
			assertionFunc: func(t *testing.T, metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				require.Len(t, executorStore.UpsertHeartbeatFunc.History(), 1)

				assert.Equal(
					t,
					types.Executor{
						Hostname:        "test-executor",
						QueueNames:      []string{"codeintel", "batches"},
						OS:              "test-os",
						Architecture:    "test-arch",
						DockerVersion:   "1.0",
						ExecutorVersion: "2.0",
						GitVersion:      "3.0",
						IgniteVersion:   "4.0",
						SrcCliVersion:   "5.0",
					},
					executorStore.UpsertHeartbeatFunc.History()[0].Arg1,
				)
				require.Len(t, codeintelMockStore.HeartbeatFunc.History(), 0)
				require.Len(t, batchesMockStore.HeartbeatFunc.History(), 0)
			},
		},
		{
			name: "Known and canceled IDs",
			body: `{"executorName": "test-executor", "queueNames": ["codeintel", "batches"], "jobIdsByQueue": [{"queueName": "codeintel", "jobIds": ["42", "7"]}, {"queueName": "batches", "jobIds": ["43", "8"]}], "os": "test-os", "architecture": "test-arch", "dockerVersion": "1.0", "executorVersion": "2.0", "gitVersion": "3.0", "igniteVersion": "4.0", "srcCliVersion": "5.0", "prometheusMetrics": ""}`,
			mockFunc: func(metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				executorStore.UpsertHeartbeatFunc.PushReturn(nil)
				codeintelMockStore.HeartbeatFunc.PushReturn([]string{"42"}, []string{"7"}, nil)
				batchesMockStore.HeartbeatFunc.PushReturn([]string{"43"}, []string{"8"}, nil)
			},
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: `{"knownIds":["42-codeintel", "43-batches"],"cancelIds":["7-codeintel", "8-batches"]}`,
			assertionFunc: func(t *testing.T, metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				require.Len(t, executorStore.UpsertHeartbeatFunc.History(), 1)

				assert.Equal(
					t,
					types.Executor{
						Hostname:        "test-executor",
						QueueNames:      []string{"codeintel", "batches"},
						OS:              "test-os",
						Architecture:    "test-arch",
						DockerVersion:   "1.0",
						ExecutorVersion: "2.0",
						GitVersion:      "3.0",
						IgniteVersion:   "4.0",
						SrcCliVersion:   "5.0",
					},
					executorStore.UpsertHeartbeatFunc.History()[0].Arg1,
				)
				require.Len(t, codeintelMockStore.HeartbeatFunc.History(), 1)
				assert.Equal(t, []string{"42", "7"}, codeintelMockStore.HeartbeatFunc.History()[0].Arg1)
				assert.Equal(t, dbworkerstore.HeartbeatOptions{WorkerHostname: "test-executor"}, codeintelMockStore.HeartbeatFunc.History()[0].Arg2)

				require.Len(t, batchesMockStore.HeartbeatFunc.History(), 1)
				assert.Equal(t, []string{"43", "8"}, batchesMockStore.HeartbeatFunc.History()[0].Arg1)
				assert.Equal(t, dbworkerstore.HeartbeatOptions{WorkerHostname: "test-executor"}, batchesMockStore.HeartbeatFunc.History()[0].Arg2)
			},
		},
		{
			name:                 "Invalid worker hostname",
			body:                 `{"executorName": "", "queueNames": ["codeintel", "batches"], "jobIdsByQueue": [{"queueName": "codeintel", "jobIds": ["42", "7"]}, {"queueName": "batches", "jobIds": ["43", "8"]}], "os": "test-os", "architecture": "test-arch", "dockerVersion": "1.0", "executorVersion": "2.0", "gitVersion": "3.0", "igniteVersion": "4.0", "srcCliVersion": "5.0", "prometheusMetrics": ""}`,
			expectedStatusCode:   http.StatusInternalServerError,
			expectedResponseBody: `{"error":"worker hostname cannot be empty"}`,
			assertionFunc: func(t *testing.T, metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				require.Len(t, executorStore.UpsertHeartbeatFunc.History(), 0)
				require.Len(t, codeintelMockStore.HeartbeatFunc.History(), 0)
				require.Len(t, batchesMockStore.HeartbeatFunc.History(), 0)
			},
		},
		{
			name:                 "Job IDs by queue contains name not in queue names",
			body:                 `{"executorName": "test-executor", "queueNames": ["codeintel", "batches"], "jobIdsByQueue": [{"queueName": "foo", "jobIds": ["42"]}, {"queueName": "bar", "jobIds": ["43"]}], "os": "test-os", "architecture": "test-arch", "dockerVersion": "1.0", "executorVersion": "2.0", "gitVersion": "3.0", "igniteVersion": "4.0", "srcCliVersion": "5.0", "prometheusMetrics": ""}`,
			expectedStatusCode:   http.StatusInternalServerError,
			expectedResponseBody: `{"error":"unsupported queue name(s) 'foo, bar' submitted in queueJobIds, executor is configured for queues 'codeintel, batches'"}`,
			assertionFunc: func(t *testing.T, metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				require.Len(t, executorStore.UpsertHeartbeatFunc.History(), 0)
				require.Len(t, codeintelMockStore.HeartbeatFunc.History(), 0)
				require.Len(t, batchesMockStore.HeartbeatFunc.History(), 0)
			},
		},
		{
			name:                 "Queue names missing",
			body:                 `{"executorName": "test-executor", "jobIdsByQueue": [{"queueName": "codeintel", "jobIds": ["42"]}, {"queueName": "batches", "jobIds": ["43"]}], "os": "test-os", "architecture": "test-arch", "dockerVersion": "1.0", "executorVersion": "2.0", "gitVersion": "3.0", "igniteVersion": "4.0", "srcCliVersion": "5.0", "prometheusMetrics": ""}`,
			expectedStatusCode:   http.StatusInternalServerError,
			expectedResponseBody: `{"error":"queueNames must be set for multi-queue heartbeats"}`,
			assertionFunc: func(t *testing.T, metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				require.Len(t, executorStore.UpsertHeartbeatFunc.History(), 0)
				require.Len(t, codeintelMockStore.HeartbeatFunc.History(), 0)
				require.Len(t, batchesMockStore.HeartbeatFunc.History(), 0)
			},
		},
		{
			name: "Failed to upsert heartbeat",
			body: `{"executorName": "test-executor", "queueNames": ["codeintel", "batches"], "jobIdsByQueue": [{"queueName": "codeintel", "jobIds": ["42", "7"]}, {"queueName": "batches", "jobIds": ["43", "8"]}], "os": "test-os", "architecture": "test-arch", "dockerVersion": "1.0", "executorVersion": "2.0", "gitVersion": "3.0", "igniteVersion": "4.0", "srcCliVersion": "5.0", "prometheusMetrics": ""}`,
			mockFunc: func(metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				executorStore.UpsertHeartbeatFunc.PushReturn(errors.Newf("failed"))
				codeintelMockStore.HeartbeatFunc.PushReturn([]string{"42", "7"}, nil, nil)
				batchesMockStore.HeartbeatFunc.PushReturn([]string{"43", "8"}, nil, nil)
			},
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: `{"knownIds":["42-codeintel", "7-codeintel", "43-batches", "8-batches"],"cancelIds":null}`,
			assertionFunc: func(t *testing.T, metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				require.Len(t, executorStore.UpsertHeartbeatFunc.History(), 1)
				assert.Equal(
					t,
					types.Executor{
						Hostname:        "test-executor",
						QueueNames:      []string{"codeintel", "batches"},
						OS:              "test-os",
						Architecture:    "test-arch",
						DockerVersion:   "1.0",
						ExecutorVersion: "2.0",
						GitVersion:      "3.0",
						IgniteVersion:   "4.0",
						SrcCliVersion:   "5.0",
					},
					executorStore.UpsertHeartbeatFunc.History()[0].Arg1,
				)
				require.Len(t, codeintelMockStore.HeartbeatFunc.History(), 1)
				assert.Equal(t, []string{"42", "7"}, codeintelMockStore.HeartbeatFunc.History()[0].Arg1)
				assert.Equal(t, dbworkerstore.HeartbeatOptions{WorkerHostname: "test-executor"}, codeintelMockStore.HeartbeatFunc.History()[0].Arg2)

				require.Len(t, batchesMockStore.HeartbeatFunc.History(), 1)
				assert.Equal(t, []string{"43", "8"}, batchesMockStore.HeartbeatFunc.History()[0].Arg1)
				assert.Equal(t, dbworkerstore.HeartbeatOptions{WorkerHostname: "test-executor"}, batchesMockStore.HeartbeatFunc.History()[0].Arg2)
			},
		},
		{
			name: "Failed to heartbeat first queue, second is ignored",
			body: `{"executorName": "test-executor", "queueNames": ["codeintel", "batches"], "jobIdsByQueue": [{"queueName": "batches", "jobIds": ["43", "8"]}, {"queueName": "codeintel", "jobIds": ["42", "7"]}], "os": "test-os", "architecture": "test-arch", "dockerVersion": "1.0", "executorVersion": "2.0", "gitVersion": "3.0", "igniteVersion": "4.0", "srcCliVersion": "5.0", "prometheusMetrics": ""}`,
			mockFunc: func(metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				executorStore.UpsertHeartbeatFunc.PushReturn(nil)
				codeintelMockStore.HeartbeatFunc.PushReturn([]string{"42", "7"}, nil, nil)
				batchesMockStore.HeartbeatFunc.PushReturn(nil, nil, errors.New("failed"))
			},
			expectedStatusCode:   http.StatusInternalServerError,
			expectedResponseBody: `{"error":"multiqueue.UpsertHeartbeat: failed"}`,
			assertionFunc: func(t *testing.T, metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				require.Len(t, executorStore.UpsertHeartbeatFunc.History(), 1)
				assert.Equal(
					t,
					types.Executor{
						Hostname:        "test-executor",
						QueueNames:      []string{"codeintel", "batches"},
						OS:              "test-os",
						Architecture:    "test-arch",
						DockerVersion:   "1.0",
						ExecutorVersion: "2.0",
						GitVersion:      "3.0",
						IgniteVersion:   "4.0",
						SrcCliVersion:   "5.0",
					},
					executorStore.UpsertHeartbeatFunc.History()[0].Arg1,
				)
				// switch statement in MultiHandler.heartbeat starts with batches, which is first in `jobIdsByQueue`, so not called
				require.Len(t, codeintelMockStore.HeartbeatFunc.History(), 0)

				require.Len(t, batchesMockStore.HeartbeatFunc.History(), 1)
				assert.Equal(t, []string{"43", "8"}, batchesMockStore.HeartbeatFunc.History()[0].Arg1)
				assert.Equal(t, dbworkerstore.HeartbeatOptions{WorkerHostname: "test-executor"}, batchesMockStore.HeartbeatFunc.History()[0].Arg2)
			},
		},
		{
			name: "First queue successful heartbeat, failed to heartbeat second queue",
			body: `{"executorName": "test-executor", "queueNames": ["codeintel", "batches"], "jobIdsByQueue": [{"queueName": "codeintel", "jobIds": ["42", "7"]}, {"queueName": "batches", "jobIds": ["43", "8"]}], "os": "test-os", "architecture": "test-arch", "dockerVersion": "1.0", "executorVersion": "2.0", "gitVersion": "3.0", "igniteVersion": "4.0", "srcCliVersion": "5.0", "prometheusMetrics": ""}`,
			mockFunc: func(metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				executorStore.UpsertHeartbeatFunc.PushReturn(nil)
				codeintelMockStore.HeartbeatFunc.PushReturn([]string{"42", "7"}, nil, nil)
				batchesMockStore.HeartbeatFunc.PushReturn(nil, nil, errors.New("failed"))
			},
			expectedStatusCode:   http.StatusInternalServerError,
			expectedResponseBody: `{"error":"multiqueue.UpsertHeartbeat: failed"}`,
			assertionFunc: func(t *testing.T, metricsStore *metricsstore.MockDistributedStore, executorStore *dbmocks.MockExecutorStore, codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				require.Len(t, executorStore.UpsertHeartbeatFunc.History(), 1)
				assert.Equal(
					t,
					types.Executor{
						Hostname:        "test-executor",
						QueueNames:      []string{"codeintel", "batches"},
						OS:              "test-os",
						Architecture:    "test-arch",
						DockerVersion:   "1.0",
						ExecutorVersion: "2.0",
						GitVersion:      "3.0",
						IgniteVersion:   "4.0",
						SrcCliVersion:   "5.0",
					},
					executorStore.UpsertHeartbeatFunc.History()[0].Arg1,
				)
				// switch statement in MultiHandler.heartbeat starts with batches, which is first in `jobIdsByQueue`, so not called
				require.Len(t, codeintelMockStore.HeartbeatFunc.History(), 1)
				assert.Equal(t, []string{"42", "7"}, codeintelMockStore.HeartbeatFunc.History()[0].Arg1)
				assert.Equal(t, dbworkerstore.HeartbeatOptions{WorkerHostname: "test-executor"}, codeintelMockStore.HeartbeatFunc.History()[0].Arg2)

				require.Len(t, batchesMockStore.HeartbeatFunc.History(), 1)
				assert.Equal(t, []string{"43", "8"}, batchesMockStore.HeartbeatFunc.History()[0].Arg1)
				assert.Equal(t, dbworkerstore.HeartbeatOptions{WorkerHostname: "test-executor"}, batchesMockStore.HeartbeatFunc.History()[0].Arg2)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executorStore := dbmocks.NewMockExecutorStore()
			metricsStore := metricsstore.NewMockDistributedStore()
			codeIntelMockStore := dbworkerstoremocks.NewMockStore[uploadsshared.Index]()
			batchesMockStore := dbworkerstoremocks.NewMockStore[*btypes.BatchSpecWorkspaceExecutionJob]()

			mh := handler.NewMultiHandler(
				executorStore,
				executorstore.NewMockJobTokenStore(),
				metricsStore,
				handler.QueueHandler[uploadsshared.Index]{Name: "codeintel", Store: codeIntelMockStore},
				handler.QueueHandler[*btypes.BatchSpecWorkspaceExecutionJob]{Name: "batches", Store: batchesMockStore},
			)

			router := mux.NewRouter()
			router.HandleFunc("/heartbeat", mh.HandleHeartbeat)

			req, err := http.NewRequest(http.MethodPost, "/heartbeat", strings.NewReader(test.body))
			require.NoError(t, err)

			rw := httptest.NewRecorder()

			if test.mockFunc != nil {
				test.mockFunc(metricsStore, executorStore, codeIntelMockStore, batchesMockStore)
			}

			router.ServeHTTP(rw, req)

			assert.Equal(t, test.expectedStatusCode, rw.Code)

			b, err := io.ReadAll(rw.Body)
			require.NoError(t, err)

			if len(test.expectedResponseBody) > 0 {
				assert.JSONEq(t, test.expectedResponseBody, string(b))
			} else {
				assert.Empty(t, string(b))
			}

			if test.assertionFunc != nil {
				test.assertionFunc(t, metricsStore, executorStore, codeIntelMockStore, batchesMockStore)
			}
		})
	}
}

func evaluateEvent(
	requestBody string,
	expectedStatusCode int,
	expectedResponseBody string,
	t *testing.T,
	router *mux.Router,
) {
	req, err := http.NewRequest(http.MethodPost, "/dequeue", strings.NewReader(requestBody))
	require.NoError(t, err)

	rw := httptest.NewRecorder()

	router.ServeHTTP(rw, req)
	assert.Equal(t, expectedStatusCode, rw.Code)

	b, err := io.ReadAll(rw.Body)
	require.NoError(t, err)

	if len(expectedResponseBody) > 0 {
		assert.JSONEq(t, expectedResponseBody, string(b))
	} else {
		assert.Empty(t, string(b))
	}
}

// Note: this test passed multiple times with the bazel flag `--runs_per_test=1000` without failures,
// but statistically speaking this test _could_ flake. The chance of two subsequent failures is low enough
// that it shouldn't ever form an issue. If failures keep occurring something is actually broken.
func TestMultiHandler_SelectQueueForDequeueing(t *testing.T) {
	tests := []struct {
		name               string
		candidateQueues    []string
		dequeueCacheConfig schema.DequeueCacheConfig
		amountOfruns       int
		expectedErr        error
	}{
		{
			name:            "acceptable deviation",
			candidateQueues: []string{"batches", "codeintel"},
			dequeueCacheConfig: schema.DequeueCacheConfig{
				Batches: &schema.Batches{
					Limit:  50,
					Weight: 4,
				},
				Codeintel: &schema.Codeintel{
					Limit:  250,
					Weight: 1,
				},
			},
			amountOfruns: 5000,
		},
	}

	mockSiteConfig()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := handler.NewMultiHandler(
				nil,
				nil,
				nil,
				handler.QueueHandler[uploadsshared.Index]{Name: "codeintel"},
				handler.QueueHandler[*btypes.BatchSpecWorkspaceExecutionJob]{Name: "batches"},
			)

			selectCounts := make(map[string]int, len(tt.candidateQueues))
			for _, q := range tt.candidateQueues {
				selectCounts[q] = 0
			}

			for range tt.amountOfruns {
				selectedQueue, err := m.SelectQueueForDequeueing(tt.candidateQueues)
				if err != nil && err != tt.expectedErr {
					t.Fatalf("expected err %s, got err %s", tt.expectedErr, err)
				}
				selectCounts[selectedQueue]++
			}

			// calculate the sum of the candidate queue weights
			var totalWeight int
			for _, q := range tt.candidateQueues {
				switch q {
				case "batches":
					totalWeight += tt.dequeueCacheConfig.Batches.Weight
				case "codeintel":
					totalWeight += tt.dequeueCacheConfig.Codeintel.Weight
				}
			}
			// then calculate how many times each queue is expected to be chosen
			expectedSelectCounts := make(map[string]float64, len(tt.candidateQueues))
			for _, q := range tt.candidateQueues {
				switch q {
				case "batches":
					expectedSelectCounts[q] = float64(tt.dequeueCacheConfig.Batches.Weight) / float64(totalWeight) * float64(tt.amountOfruns)
				case "codeintel":
					expectedSelectCounts[q] = float64(tt.dequeueCacheConfig.Codeintel.Weight) / float64(totalWeight) * float64(tt.amountOfruns)
				}
			}
			for key := range selectCounts {
				// allow a 10% deviation of the expected count of selects per queue
				lower := int(math.Floor(expectedSelectCounts[key] - expectedSelectCounts[key]*0.1))
				upper := int(math.Floor(expectedSelectCounts[key] + expectedSelectCounts[key]*0.1))
				assert.True(t, selectCounts[key] >= lower && selectCounts[key] <= upper, "SelectQueueForDequeueing: %s = %d, lower = %d, upper = %d", key, selectCounts[key], lower, upper)
			}
		})
	}
}

func TestMultiHandler_SelectEligibleQueues(t *testing.T) {
	tests := []struct {
		name             string
		queues           []string
		mockCacheEntries map[string]int
		expectedQueues   []string
	}{
		{
			name:   "Nothing discarded",
			queues: []string{"batches", "codeintel"},
			mockCacheEntries: map[string]int{
				// both have dequeued 5 times
				"batches":   5,
				"codeintel": 5,
			},
			expectedQueues: []string{"batches", "codeintel"},
		},
		{
			name:   "All discarded",
			queues: []string{"batches", "codeintel"},
			mockCacheEntries: map[string]int{
				// both have dequeued their limit, so both will be returned
				"batches":   50,
				"codeintel": 250,
			},
			expectedQueues: []string{"batches", "codeintel"},
		},
		{
			name:   "Batches discarded",
			queues: []string{"batches", "codeintel"},
			mockCacheEntries: map[string]int{
				// batches has dequeued its limit, codeintel 5 times
				"batches":   50,
				"codeintel": 5,
			},
			expectedQueues: []string{"codeintel"},
		},
	}

	mockSiteConfig()

	m := handler.NewMultiHandler(
		nil,
		nil,
		nil,
		handler.QueueHandler[uploadsshared.Index]{Name: "codeintel"},
		handler.QueueHandler[*btypes.BatchSpecWorkspaceExecutionJob]{Name: "batches"},
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rcache.SetupForTest(t)
			for key, value := range tt.mockCacheEntries {
				for i := range value {
					// mock dequeues
					if err := m.DequeueCache.SetHashItem(key, strconv.Itoa(i), "job-id"); err != nil {
						t.Fatalf("unexpected error while setting hash item: %s", err)
					}
				}
			}

			queues, err := m.SelectEligibleQueues(tt.queues)
			if err != nil {
				t.Fatalf("unexpected error while discarding queues: %s", err)
			}
			assert.Equalf(t, tt.expectedQueues, queues, "SelectEligibleQueues(%v)", tt.queues)
		})
	}
}

func mockSiteConfig() {
	client := conf.DefaultClient()
	client.Mock(&conf.Unified{SiteConfiguration: schema.SiteConfiguration{
		ExecutorsMultiqueue: &schema.ExecutorsMultiqueue{
			DequeueCacheConfig: &schema.DequeueCacheConfig{
				Batches: &schema.Batches{
					Limit:  50,
					Weight: 4,
				},
				Codeintel: &schema.Codeintel{
					Limit:  250,
					Weight: 1,
				},
			},
		},
	}})
}

func TestMultiHandler_SelectNonEmptyQueues(t *testing.T) {
	tests := []struct {
		name           string
		queueNames     []string
		mockFunc       func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob])
		expectedQueues []string
	}{
		{
			name:       "Both contain jobs",
			queueNames: []string{"batches", "codeintel"},
			mockFunc: func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				codeintelMockStore.QueuedCountFunc.PushReturn(5, nil)
				batchesMockStore.QueuedCountFunc.PushReturn(5, nil)
			},
			expectedQueues: []string{"batches", "codeintel"},
		},
		{
			name:       "Only batches contains jobs",
			queueNames: []string{"batches", "codeintel"},
			mockFunc: func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				codeintelMockStore.QueuedCountFunc.PushReturn(0, nil)
				batchesMockStore.QueuedCountFunc.PushReturn(5, nil)
			},
			expectedQueues: []string{"batches"},
		},
		{
			name:       "None contain jobs",
			queueNames: []string{"batches", "codeintel"},
			mockFunc: func(codeintelMockStore *dbworkerstoremocks.MockStore[uploadsshared.Index], batchesMockStore *dbworkerstoremocks.MockStore[*btypes.BatchSpecWorkspaceExecutionJob]) {
				codeintelMockStore.QueuedCountFunc.PushReturn(0, nil)
				batchesMockStore.QueuedCountFunc.PushReturn(0, nil)
			},
			expectedQueues: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			codeIntelMockStore := dbworkerstoremocks.NewMockStore[uploadsshared.Index]()
			batchesMockStore := dbworkerstoremocks.NewMockStore[*btypes.BatchSpecWorkspaceExecutionJob]()
			m := &handler.MultiHandler{
				CodeIntelQueueHandler: handler.QueueHandler[uploadsshared.Index]{Name: "codeintel", Store: codeIntelMockStore},
				BatchesQueueHandler:   handler.QueueHandler[*btypes.BatchSpecWorkspaceExecutionJob]{Name: "batches", Store: batchesMockStore},
			}

			tt.mockFunc(codeIntelMockStore, batchesMockStore)

			got, err := m.SelectNonEmptyQueues(ctx, tt.queueNames)
			if err != nil {
				t.Fatalf("unexpected error while filtering non empty queues: %s", err)
			}
			assert.Equalf(t, tt.expectedQueues, got, "SelectNonEmptyQueues(%v)", tt.queueNames)
		})
	}
}
