package codeintelutils

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
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
	for i := 0; i < 500; i++ {
		expectedPayload = append(expectedPayload, byte(i))
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("unexpected error reading request body: %s", err)
		}

		gzipReader, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("unexpected error creating gzip.Reader: %s", err)
		}
		decompressed, err := ioutil.ReadAll(gzipReader)
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

	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("unexpected error creating temp file: %s", err)
	}
	defer func() { os.Remove(f.Name()) }()
	_, _ = io.Copy(f, bytes.NewReader(expectedPayload))
	_ = f.Close()

	id, err := UploadIndex(UploadIndexOpts{
		Endpoint:            ts.URL,
		AccessToken:         "hunter2",
		Repo:                "foo/bar",
		Commit:              "deadbeef",
		Root:                "proj/",
		Indexer:             "lsif-go",
		GitHubToken:         "ght",
		File:                f.Name(),
		MaxPayloadSizeBytes: 1000,
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
	for i := 0; i < 20000; i++ {
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
			payload, err := ioutil.ReadAll(r.Body)
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

	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("unexpected error creating temp file: %s", err)
	}
	defer func() { os.Remove(f.Name()) }()
	_, _ = io.Copy(f, bytes.NewReader(expectedPayload))
	_ = f.Close()

	id, err := UploadIndex(UploadIndexOpts{
		Endpoint:            ts.URL,
		AccessToken:         "hunter2",
		Repo:                "foo/bar",
		Commit:              "deadbeef",
		Root:                "proj/",
		Indexer:             "lsif-go",
		GitHubToken:         "ght",
		File:                f.Name(),
		MaxPayloadSizeBytes: 100,
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
	for i := 0; i < 5; i++ {
		allPayloads = append(allPayloads, payloads[i]...)
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(allPayloads))
	if err != nil {
		t.Fatalf("unexpected error creating gzip.Reader: %s", err)
	}
	decompressed, err := ioutil.ReadAll(gzipReader)
	if err != nil {
		t.Fatalf("unexpected error reading from gzip.Reader: %s", err)
	}
	if diff := cmp.Diff(expectedPayload, decompressed); diff != "" {
		t.Errorf("unexpected gzipped contents (-want +got):\n%s", diff)
	}
}
