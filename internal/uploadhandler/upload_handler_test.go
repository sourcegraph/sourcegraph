package uploadhandler

import (
	"bytes"
	"compress/gzip"
	"context"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/uploadstore"
	uploadstoremocks "github.com/sourcegraph/sourcegraph/internal/uploadstore/mocks"
)

func TestMain(m *testing.M) {
	flag.Parse()
	if !testing.Verbose() {
		logtest.InitWithLevel(m, log.LevelNone)
	}
	os.Exit(m.Run())
}

const testCommit = "deadbeef01deadbeef02deadbeef03deadbeef04"

func TestHandleEnqueueSinglePayload(t *testing.T) {
	setupRepoMocks(t)

	mockDBStore := NewMockDBStore[testUploadMetadata]()
	mockUploadStore := uploadstoremocks.NewMockStore()

	mockDBStore.WithTransactionFunc.SetDefaultHook(func(ctx context.Context, f func(tx DBStore[testUploadMetadata]) error) error { return f(mockDBStore) })
	mockDBStore.InsertUploadFunc.SetDefaultReturn(42, nil)

	testURL, err := url.Parse("http://test.com/upload")
	if err != nil {
		t.Fatalf("unexpected error constructing url: %s", err)
	}
	testURL.RawQuery = (url.Values{
		"commit":      []string{testCommit},
		"root":        []string{"proj/"},
		"repository":  []string{"github.com/test/test"},
		"indexerName": []string{"lsif-go"},
	}).Encode()

	var expectedContents []byte
	for i := range 20000 {
		expectedContents = append(expectedContents, byte(i))
	}

	w := httptest.NewRecorder()
	r, err := http.NewRequest("POST", testURL.String(), bytes.NewReader(expectedContents))
	if err != nil {
		t.Fatalf("unexpected error constructing request: %s", err)
	}
	r.Header.Set("X-Uncompressed-Size", "21")

	newTestUploadHandler(t, mockDBStore, mockUploadStore).ServeHTTP(w, r)

	if w.Code != http.StatusAccepted {
		t.Errorf("unexpected status code. want=%d have=%d", http.StatusAccepted, w.Code)
	}
	if diff := cmp.Diff([]byte(`{"id":"42"}`), w.Body.Bytes()); diff != "" {
		t.Errorf("unexpected response payload (-want +got):\n%s", diff)
	}

	if len(mockDBStore.InsertUploadFunc.History()) != 1 {
		t.Errorf("unexpected number of InsertUpload calls. want=%d have=%d", 1, len(mockDBStore.InsertUploadFunc.History()))
	} else {
		call := mockDBStore.InsertUploadFunc.History()[0]
		if call.Arg1.Metadata.Commit != testCommit {
			t.Errorf("unexpected commit. want=%q have=%q", testCommit, call.Arg1.Metadata.Commit)
		}
		if call.Arg1.Metadata.Root != "proj/" {
			t.Errorf("unexpected root. want=%q have=%q", "proj/", call.Arg1.Metadata.Root)
		}
		if call.Arg1.Metadata.RepositoryID != 50 {
			t.Errorf("unexpected repository id. want=%d have=%d", 50, call.Arg1.Metadata.RepositoryID)
		}
		if call.Arg1.Metadata.Indexer != "lsif-go" {
			t.Errorf("unexpected indexer name. want=%q have=%q", "lsif-go", call.Arg1.Metadata.Indexer)
		}
		if *call.Arg1.UncompressedSize != 21 {
			t.Errorf("unexpected uncompressed size. want=%d have%d", 21, *call.Arg1.UncompressedSize)
		}
	}

	if len(mockUploadStore.UploadFunc.History()) != 1 {
		t.Errorf("unexpected number of SendUpload calls. want=%d have=%d", 1, len(mockUploadStore.UploadFunc.History()))
	} else {
		call := mockUploadStore.UploadFunc.History()[0]
		if call.Arg1 != "upload-42.lsif.gz" {
			t.Errorf("unexpected bundle id. want=%s have=%s", "upload-42.lsif.gz", call.Arg1)
		}

		contents, err := io.ReadAll(call.Arg2)
		if err != nil {
			t.Fatalf("unexpected error reading payload: %s", err)
		}

		if diff := cmp.Diff(expectedContents, contents); diff != "" {
			t.Errorf("unexpected file contents (-want +got):\n%s", diff)
		}
	}
}

func TestHandleEnqueueSinglePayloadNoIndexerName(t *testing.T) {
	setupRepoMocks(t)

	mockDBStore := NewMockDBStore[testUploadMetadata]()
	mockUploadStore := uploadstoremocks.NewMockStore()

	mockDBStore.WithTransactionFunc.SetDefaultHook(func(ctx context.Context, f func(tx DBStore[testUploadMetadata]) error) error { return f(mockDBStore) })
	mockDBStore.InsertUploadFunc.SetDefaultReturn(42, nil)

	testURL, err := url.Parse("http://test.com/upload")
	if err != nil {
		t.Fatalf("unexpected error constructing url: %s", err)
	}
	testURL.RawQuery = (url.Values{
		"commit":     []string{testCommit},
		"root":       []string{"proj/"},
		"repository": []string{"github.com/test/test"},
	}).Encode()

	var lines []string
	lines = append(lines, `{"label": "metaData", "toolInfo": {"name": "lsif-go"}}`)
	for range 20000 {
		lines = append(lines, `{"id": "a", "type": "edge", "label": "textDocument/references", "outV": "b", "inV": "c"}`)
	}

	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	_, _ = io.Copy(gzipWriter, bytes.NewReader([]byte(strings.Join(lines, "\n"))))
	gzipWriter.Close()
	expectedContents := buf.Bytes()

	w := httptest.NewRecorder()
	r, err := http.NewRequest("POST", testURL.String(), bytes.NewReader(expectedContents))
	if err != nil {
		t.Fatalf("unexpected error constructing request: %s", err)
	}

	newTestUploadHandler(t, mockDBStore, mockUploadStore).ServeHTTP(w, r)

	if w.Code != http.StatusAccepted {
		t.Errorf("unexpected status code. want=%d have=%d", http.StatusAccepted, w.Code)
	}

	if len(mockUploadStore.UploadFunc.History()) != 1 {
		t.Errorf("unexpected number of Upload calls. want=%d have=%d", 1, len(mockUploadStore.UploadFunc.History()))
	} else {
		call := mockUploadStore.UploadFunc.History()[0]
		if call.Arg1 != "upload-42.lsif.gz" {
			t.Errorf("unexpected bundle id. want=%s have=%s", "upload-42.lsif.gz", call.Arg1)
		}

		contents, err := io.ReadAll(call.Arg2)
		if err != nil {
			t.Fatalf("unexpected error reading payload: %s", err)
		}

		if diff := cmp.Diff(expectedContents, contents); diff != "" {
			t.Errorf("unexpected file contents (-want +got):\n%s", diff)
		}
	}
}

func TestHandleEnqueueMultipartSetup(t *testing.T) {
	setupRepoMocks(t)

	mockDBStore := NewMockDBStore[testUploadMetadata]()
	mockUploadStore := uploadstoremocks.NewMockStore()

	mockDBStore.WithTransactionFunc.SetDefaultHook(func(ctx context.Context, f func(tx DBStore[testUploadMetadata]) error) error { return f(mockDBStore) })
	mockDBStore.InsertUploadFunc.SetDefaultReturn(42, nil)

	testURL, err := url.Parse("http://test.com/upload")
	if err != nil {
		t.Fatalf("unexpected error constructing url: %s", err)
	}
	testURL.RawQuery = (url.Values{
		"commit":      []string{testCommit},
		"root":        []string{"proj/"},
		"repository":  []string{"github.com/test/test"},
		"indexerName": []string{"lsif-go"},
		"multiPart":   []string{"true"},
		"numParts":    []string{"3"},
	}).Encode()

	w := httptest.NewRecorder()
	r, err := http.NewRequest("POST", testURL.String(), nil)
	if err != nil {
		t.Fatalf("unexpected error constructing request: %s", err)
	}
	r.Header.Set("X-Uncompressed-Size", "50")

	newTestUploadHandler(t, mockDBStore, mockUploadStore).ServeHTTP(w, r)

	if w.Code != http.StatusAccepted {
		t.Errorf("unexpected status code. want=%d have=%d", http.StatusAccepted, w.Code)
	}
	if diff := cmp.Diff([]byte(`{"id":"42"}`), w.Body.Bytes()); diff != "" {
		t.Errorf("unexpected response payload (-want +got):\n%s", diff)
	}

	if len(mockDBStore.InsertUploadFunc.History()) != 1 {
		t.Errorf("unexpected number of InsertUpload calls. want=%d have=%d", 1, len(mockDBStore.InsertUploadFunc.History()))
	} else {
		call := mockDBStore.InsertUploadFunc.History()[0]
		if call.Arg1.Metadata.Commit != testCommit {
			t.Errorf("unexpected commit. want=%q have=%q", testCommit, call.Arg1.Metadata.Commit)
		}
		if call.Arg1.Metadata.Root != "proj/" {
			t.Errorf("unexpected root. want=%q have=%q", "proj/", call.Arg1.Metadata.Root)
		}
		if call.Arg1.Metadata.RepositoryID != 50 {
			t.Errorf("unexpected repository id. want=%d have=%d", 50, call.Arg1.Metadata.RepositoryID)
		}
		if call.Arg1.Metadata.Indexer != "lsif-go" {
			t.Errorf("unexpected indexer name. want=%q have=%q", "lsif-go", call.Arg1.Metadata.Indexer)
		}
		if *call.Arg1.UncompressedSize != 50 {
			t.Errorf("unexpected uncompressed size. want=%d have%d", 21, *call.Arg1.UncompressedSize)
		}
	}
}

func TestHandleEnqueueMultipartUpload(t *testing.T) {
	setupRepoMocks(t)

	mockDBStore := NewMockDBStore[testUploadMetadata]()
	mockUploadStore := uploadstoremocks.NewMockStore()

	upload := Upload[testUploadMetadata]{
		ID:            42,
		NumParts:      5,
		UploadedParts: []int{0, 1, 2, 3, 4},
	}

	mockDBStore.WithTransactionFunc.SetDefaultHook(func(ctx context.Context, f func(tx DBStore[testUploadMetadata]) error) error { return f(mockDBStore) })
	mockDBStore.GetUploadByIDFunc.SetDefaultReturn(upload, true, nil)

	testURL, err := url.Parse("http://test.com/upload")
	if err != nil {
		t.Fatalf("unexpected error constructing url: %s", err)
	}
	testURL.RawQuery = (url.Values{
		"uploadId": []string{"42"},
		"index":    []string{"3"},
	}).Encode()

	var expectedContents []byte
	for i := range 20000 {
		expectedContents = append(expectedContents, byte(i))
	}

	w := httptest.NewRecorder()
	r, err := http.NewRequest("POST", testURL.String(), bytes.NewReader(expectedContents))
	if err != nil {
		t.Fatalf("unexpected error constructing request: %s", err)
	}

	newTestUploadHandler(t, mockDBStore, mockUploadStore).ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("unexpected status code. want=%d have=%d", http.StatusNoContent, w.Code)
	}

	if len(mockDBStore.AddUploadPartFunc.History()) != 1 {
		t.Errorf("unexpected number of AddUploadPart calls. want=%d have=%d", 1, len(mockDBStore.AddUploadPartFunc.History()))
	} else {
		call := mockDBStore.AddUploadPartFunc.History()[0]
		if call.Arg1 != 42 {
			t.Errorf("unexpected commit. want=%q have=%q", 42, call.Arg1)
		}
		if call.Arg2 != 3 {
			t.Errorf("unexpected root. want=%q have=%q", 3, call.Arg2)
		}
	}

	if len(mockUploadStore.UploadFunc.History()) != 1 {
		t.Errorf("unexpected number of Upload calls. want=%d have=%d", 1, len(mockUploadStore.UploadFunc.History()))
	} else {
		call := mockUploadStore.UploadFunc.History()[0]
		if call.Arg1 != "upload-42.3.lsif.gz" {
			t.Errorf("unexpected bundle id. want=%s have=%s", "upload-42.3.lsif.gz", call.Arg1)
		}

		contents, err := io.ReadAll(call.Arg2)
		if err != nil {
			t.Fatalf("unexpected error reading payload: %s", err)
		}

		if diff := cmp.Diff(expectedContents, contents); diff != "" {
			t.Errorf("unexpected file contents (-want +got):\n%s", diff)
		}
	}
}

func TestHandleEnqueueMultipartFinalize(t *testing.T) {
	setupRepoMocks(t)

	mockDBStore := NewMockDBStore[testUploadMetadata]()
	mockUploadStore := uploadstoremocks.NewMockStore()

	upload := Upload[testUploadMetadata]{
		ID:            42,
		NumParts:      5,
		UploadedParts: []int{0, 1, 2, 3, 4},
	}
	mockDBStore.WithTransactionFunc.SetDefaultHook(func(ctx context.Context, f func(tx DBStore[testUploadMetadata]) error) error { return f(mockDBStore) })
	mockDBStore.GetUploadByIDFunc.SetDefaultReturn(upload, true, nil)

	testURL, err := url.Parse("http://test.com/upload")
	if err != nil {
		t.Fatalf("unexpected error constructing url: %s", err)
	}
	testURL.RawQuery = (url.Values{
		"uploadId": []string{"42"},
		"done":     []string{"true"},
	}).Encode()

	w := httptest.NewRecorder()
	r, err := http.NewRequest("POST", testURL.String(), nil)
	if err != nil {
		t.Fatalf("unexpected error constructing request: %s", err)
	}

	newTestUploadHandler(t, mockDBStore, mockUploadStore).ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("unexpected status code. want=%d have=%d", http.StatusNoContent, w.Code)
	}

	if len(mockDBStore.MarkQueuedFunc.History()) != 1 {
		t.Errorf("unexpected number of MarkQueued calls. want=%d have=%d", 1, len(mockDBStore.MarkQueuedFunc.History()))
	} else if call := mockDBStore.MarkQueuedFunc.History()[0]; call.Arg1 != 42 {
		t.Errorf("unexpected upload id. want=%d have=%d", 42, call.Arg1)
	}

	if len(mockUploadStore.ComposeFunc.History()) != 1 {
		t.Errorf("unexpected number of Compose calls. want=%d have=%d", 1, len(mockUploadStore.ComposeFunc.History()))
	} else {
		call := mockUploadStore.ComposeFunc.History()[0]

		if call.Arg1 != "upload-42.lsif.gz" {
			t.Errorf("unexpected bundle id. want=%s have=%s", "upload-42.lsif.gz", call.Arg1)
		}

		expectedFilenames := []string{
			"upload-42.0.lsif.gz",
			"upload-42.1.lsif.gz",
			"upload-42.2.lsif.gz",
			"upload-42.3.lsif.gz",
			"upload-42.4.lsif.gz",
		}
		if diff := cmp.Diff(expectedFilenames, call.Arg2); diff != "" {
			t.Errorf("unexpected source filenames (-want +got):\n%s", diff)
		}
	}
}

func TestHandleEnqueueMultipartFinalizeIncompleteUpload(t *testing.T) {
	setupRepoMocks(t)

	mockDBStore := NewMockDBStore[testUploadMetadata]()
	mockUploadStore := uploadstoremocks.NewMockStore()

	upload := Upload[testUploadMetadata]{
		ID:            42,
		NumParts:      5,
		UploadedParts: []int{0, 1, 3, 4},
	}
	mockDBStore.GetUploadByIDFunc.SetDefaultReturn(upload, true, nil)

	testURL, err := url.Parse("http://test.com/upload")
	if err != nil {
		t.Fatalf("unexpected error constructing url: %s", err)
	}
	testURL.RawQuery = (url.Values{
		"uploadId": []string{"42"},
		"done":     []string{"true"},
	}).Encode()

	w := httptest.NewRecorder()
	r, err := http.NewRequest("POST", testURL.String(), nil)
	if err != nil {
		t.Fatalf("unexpected error constructing request: %s", err)
	}

	h := &UploadHandler[testUploadMetadata]{
		dbStore:     mockDBStore,
		uploadStore: mockUploadStore,
		operations:  NewOperations(&observation.TestContext, "test"),
		logger:      logtest.Scoped(t),
	}
	h.handleEnqueue(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected status code. want=%d have=%d", http.StatusBadRequest, w.Code)
	}
}

type testUploadMetadata struct {
	RepositoryID      int
	Commit            string
	Root              string
	Indexer           string
	IndexerVersion    string
	AssociatedIndexID int
}

func newTestUploadHandler(t *testing.T, dbStore DBStore[testUploadMetadata], uploadStore uploadstore.Store) http.Handler {
	metadataFromRequest := func(ctx context.Context, r *http.Request) (testUploadMetadata, int, error) {
		return testUploadMetadata{
			RepositoryID:      50,
			Commit:            getQuery(r, "commit"),
			Root:              getQuery(r, "root"),
			Indexer:           getQuery(r, "indexerName"),
			IndexerVersion:    getQuery(r, "indexerVersion"),
			AssociatedIndexID: getQueryInt(r, "associatedIndexId"),
		}, 0, nil
	}

	return NewUploadHandler(
		logtest.Scoped(t),
		dbStore,
		uploadStore,
		NewOperations(&observation.TestContext, "test"),
		metadataFromRequest,
	)
}

func setupRepoMocks(t testing.TB) {
	t.Cleanup(func() {
		backend.Mocks.Repos.GetByName = nil
		backend.Mocks.Repos.ResolveRev = nil
	})

	backend.Mocks.Repos.GetByName = func(ctx context.Context, name api.RepoName) (*types.Repo, error) {
		if name != "github.com/test/test" {
			t.Errorf("unexpected repository name. want=%s have=%s", "github.com/test/test", name)
		}
		return &types.Repo{ID: 50}, nil
	}

	backend.Mocks.Repos.ResolveRev = func(ctx context.Context, repo api.RepoName, rev string) (api.CommitID, error) {
		if rev != testCommit {
			t.Errorf("unexpected commit. want=%s have=%s", testCommit, rev)
		}
		return "", nil
	}
}
