package upload

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestUploadIndex(t *testing.T) {
	var expectedPayload []byte
	for i := range 500 {
		expectedPayload = append(expectedPayload, byte(i))
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("unexpected error reading request body: %s", err)
		}

		if r.Header.Get("Content-Type") != "application/x-ndjson+lsif" {
			t.Fatalf("Content-Type header expected to be '%s', got '%s'", "application/x-ndjson+lsif", r.Header.Get("Content-Type"))
		}

		if r.Header.Get("Authorization") != "token hunter2" {
			t.Fatalf("Authorization header expected to be '%s', got '%s'", "token hunter2", r.Header.Get("Authorization"))
		}

		gzipReader, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("unexpected error creating gzip.Reader: %s", err)
		}
		decompressed, err := io.ReadAll(gzipReader)
		if err != nil {
			t.Fatalf("unexpected error reading from gzip.Reader: %s", err)
		}

		if diff := cmp.Diff(expectedPayload, decompressed); diff != "" {
			t.Errorf("unexpected request payload (-want +got):\n%s", diff)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"42"}`))
	}))
	defer ts.Close()

	f, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("unexpected error creating temp file: %s", err)
	}
	defer func() { os.Remove(f.Name()) }()
	_, _ = io.Copy(f, bytes.NewReader(expectedPayload))
	_ = f.Close()

	id, err := UploadIndex(context.Background(), f.Name(), http.DefaultClient, UploadOptions{
		UploadRecordOptions: UploadRecordOptions{
			Repo:    "foo/bar",
			Commit:  "deadbeef",
			Root:    "proj/",
			Indexer: "lsif-go",
		},
		SourcegraphInstanceOptions: SourcegraphInstanceOptions{
			SourcegraphURL:      ts.URL,
			AccessToken:         "hunter2",
			GitHubToken:         "ght",
			MaxPayloadSizeBytes: 1000,
			AdditionalHeaders:   map[string]string{"Content-Type": "application/x-ndjson+lsif"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error uploading index: %s", err)
	}

	if id != 42 {
		t.Errorf("unexpected id. want=%d have=%d", 42, id)
	}
}

func TestUploadIndexMultipart(t *testing.T) {
	var expectedPayload []byte
	for i := range 20000 {
		expectedPayload = append(expectedPayload, byte(i))
	}

	var m sync.Mutex
	payloads := map[int][]byte{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("multiPart") != "" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"42"}`)) // graphql id is TFNJRlVwbG9hZDoiNDIi
			return
		}

		if r.URL.Query().Get("index") != "" {
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("unexpected error reading request body: %s", err)
			}

			index, _ := strconv.Atoi(r.URL.Query().Get("index"))
			m.Lock()
			payloads[index] = payload
			m.Unlock()
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	f, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("unexpected error creating temp file: %s", err)
	}
	defer func() { os.Remove(f.Name()) }()
	_, _ = io.Copy(f, bytes.NewReader(expectedPayload))
	_ = f.Close()

	id, err := UploadIndex(context.Background(), f.Name(), http.DefaultClient, UploadOptions{
		UploadRecordOptions: UploadRecordOptions{
			Repo:    "foo/bar",
			Commit:  "deadbeef",
			Root:    "proj/",
			Indexer: "lsif-go",
		},
		SourcegraphInstanceOptions: SourcegraphInstanceOptions{
			SourcegraphURL:      ts.URL,
			AccessToken:         "hunter2",
			GitHubToken:         "ght",
			MaxPayloadSizeBytes: 100,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error uploading index: %s", err)
	}

	if id != 42 {
		t.Errorf("unexpected id. want=%d have=%d", 42, id)
	}

	if len(payloads) != 5 {
		t.Errorf("unexpected payloads size. want=%d have=%d", 5, len(payloads))
	}

	var allPayloads []byte
	for i := range 5 {
		allPayloads = append(allPayloads, payloads[i]...)
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(allPayloads))
	if err != nil {
		t.Fatalf("unexpected error creating gzip.Reader: %s", err)
	}
	decompressed, err := io.ReadAll(gzipReader)
	if err != nil {
		t.Fatalf("unexpected error reading from gzip.Reader: %s", err)
	}
	if diff := cmp.Diff(expectedPayload, decompressed); diff != "" {
		t.Errorf("unexpected gzipped contents (-want +got):\n%s", diff)
	}
}
