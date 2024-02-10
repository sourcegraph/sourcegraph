package ui

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/fileutil"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

// initHTTPTestGitServer instantiates an httptest.Server to make it return an HTTP response as set
// by httpStatusCode and a body as set by resp. It also ensures that the server is closed during
// test cleanup, thus ensuring that the caller does not have to remember to close the server.
//
// Finally, initHTTPTestGitServer patches the gitserver.Client.Addrs to the URL of the test
// HTTP server, so that API calls to the gitserver are received by the test HTTP server.
//
// TL;DR: This function helps us to mock the gitserver without having to define mock functions for
// each of the gitserver client methods.
func initHTTPTestGitServer(t *testing.T, httpStatusCode int, resp string) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Trailer", "X-Exec-Error")
		w.Header().Add("Trailer", "X-Exec-Exit-Status")
		w.Header().Add("Trailer", "X-Exec-Stderr")
		w.Header().Set("X-Exec-Error", "")
		w.Header().Set("X-Exec-Exit-Status", "0")
		w.Header().Set("X-Exec-Stderr", "")
		w.WriteHeader(httpStatusCode)
		_, err := w.Write([]byte(resp))
		if err != nil {
			t.Fatalf("Failed to write to httptest server: %v", err)
		}
	}))

	t.Cleanup(func() {
		s.Close()
	})
}

func Test_serveRawWithHTTPRequestMethodHEAD(t *testing.T) {
	// mockNewCommon ensures that we do not need the repo-updater running for this unit test.
	mockNewCommon = func(w http.ResponseWriter, r *http.Request, title string, serveError serveErrorHandler) (*Common, error) {
		return &Common{
			Repo: &types.Repo{
				Name: "test",
			},
			CommitID: api.CommitID("12345"),
		}, nil
	}
	defer func() {
		mockNewCommon = nil
	}()

	logger := logtest.Scoped(t)

	t.Run("success response for HEAD request", func(t *testing.T) {
		// httptest server will return a 200 OK, so gitserver.Client.RepoInfo will not return
		// an error.
		initHTTPTestGitServer(t, http.StatusOK, "{}")

		req := httptest.NewRequest("HEAD", "/github.com/sourcegraph/sourcegraph/-/raw", nil)
		w := httptest.NewRecorder()

		db := dbmocks.NewMockDB()
		rstore := dbmocks.NewMockRepoStore()
		db.ReposFunc.SetDefaultReturn(rstore)
		rstore.GetByNameFunc.SetDefaultReturn(&types.Repo{ID: 123}, nil)

		err := serveRaw(logger, db, gitserver.NewTestClient(t))(w, req)
		if err != nil {
			t.Fatalf("Failed to invoke serveRaw: %v", err)
		}

		if w.Code != http.StatusOK {
			t.Fatalf("Want %d but got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("failure response for HEAD request", func(t *testing.T) {
		// httptest server will return a 404 Not Found, so gitserver.Client.RepoInfo will
		// return an error.
		initHTTPTestGitServer(t, http.StatusNotFound, "{}")

		req := httptest.NewRequest("HEAD", "/github.com/sourcegraph/sourcegraph/-/raw", nil)
		w := httptest.NewRecorder()

		db := dbmocks.NewMockDB()
		rstore := dbmocks.NewMockRepoStore()
		db.ReposFunc.SetDefaultReturn(rstore)
		rstore.GetByNameFunc.SetDefaultReturn(nil, &database.RepoNotFoundErr{ID: 123})

		err := serveRaw(logger, db, gitserver.NewTestClient(t))(w, req)
		if err == nil {
			t.Fatal("Want error but got nil")
		}

		if w.Code != http.StatusNotFound {
			t.Fatalf("Want %d but got %d", http.StatusNotFound, w.Code)
		}
	})
}

func Test_serveRawWithContentArchive(t *testing.T) {
	mockNewCommon = func(w http.ResponseWriter, r *http.Request, title string, serveError serveErrorHandler) (*Common, error) {
		return &Common{
			Repo: &types.Repo{
				Name: "test",
			},
			CommitID: api.CommitID("12345"),
		}, nil
	}
	defer func() {
		mockNewCommon = nil
	}()

	logger := logtest.Scoped(t)

	mockGitServerResponse := "this is a gitserver archive response"

	t.Run("success response for format=zip", func(t *testing.T) {
		// httptest server will return a 200 OK, so gitserver.Client.RepoInfo will not return an error.

		initHTTPTestGitServer(t, http.StatusOK, mockGitServerResponse)

		req := httptest.NewRequest("GET", "/github.com/sourcegraph/sourcegraph/-/raw?format=zip", nil)
		w := httptest.NewRecorder()

		db := dbmocks.NewMockDB()
		err := serveRaw(logger, db, gitserver.NewTestClient(t))(w, req)
		if err != nil {
			t.Fatalf("Failed to invoke serveRaw: %v", err)
		}

		if w.Code != http.StatusOK {
			t.Fatalf("Want %d but got %d", http.StatusOK, w.Code)
		}

		expectedHeaders := map[string]string{
			"X-Content-Type-Options": "nosniff",
			"Content-Type":           "application/zip",
			"Content-Disposition":    mime.FormatMediaType("Attachment", map[string]string{"filename": "test.zip"}),
		}

		if len(w.Header()) != len(expectedHeaders) {
			t.Errorf("Want %d headers but got %d headers", len(w.Header()), len(expectedHeaders))
		}

		for k, v := range expectedHeaders {
			if h := w.Header().Get(k); h != v {
				t.Errorf("Expected header %q to have value %q but got %q", k, v, h)
			}
		}

		body := string(w.Body.Bytes())
		if body != mockGitServerResponse {
			t.Errorf("Want %q in body, but got %q", mockGitServerResponse, body)
		}
	})

	t.Run("success response for format=tar", func(t *testing.T) {
		// httptest server will return a 200 OK, so gitserver.Client.RepoInfo will not return an error.

		initHTTPTestGitServer(t, http.StatusOK, mockGitServerResponse)

		req := httptest.NewRequest("GET", "/github.com/sourcegraph/sourcegraph/-/raw?format=tar", nil)
		w := httptest.NewRecorder()

		db := dbmocks.NewMockDB()
		err := serveRaw(logger, db, gitserver.NewTestClient(t))(w, req)
		if err != nil {
			t.Fatalf("Failed to invoke serveRaw: %v", err)
		}

		if w.Code != http.StatusOK {
			t.Fatalf("Want %d but got %d", http.StatusOK, w.Code)
		}

		expectedHeaders := map[string]string{
			"X-Content-Type-Options": "nosniff",
			"Content-Type":           "application/x-tar",
			"Content-Disposition":    mime.FormatMediaType("Attachment", map[string]string{"filename": "test.tar"}),
		}

		if len(w.Header()) != len(expectedHeaders) {
			t.Errorf("Want %d headers but got %d headers", len(w.Header()), len(expectedHeaders))
		}

		for k, v := range expectedHeaders {
			if h := w.Header().Get(k); h != v {
				t.Errorf("Expected header %q to have value %q but got %q", k, v, h)
			}
		}

		body := string(w.Body.Bytes())
		if body != mockGitServerResponse {
			t.Errorf("Want %q in body, but got %q", mockGitServerResponse, body)
		}
	})

}

func Test_serveRawWithContentTypePlain(t *testing.T) {
	mockNewCommon = func(w http.ResponseWriter, r *http.Request, title string, serveError serveErrorHandler) (*Common, error) {
		return &Common{
			Repo: &types.Repo{
				Name: "test",
			},
			CommitID: api.CommitID("12345"),
		}, nil
	}
	defer func() {
		mockNewCommon = nil
	}()

	logger := logtest.Scoped(t)

	assertHeaders := func(w http.ResponseWriter) {
		t.Helper()

		expectedHeaders := map[string]string{
			"X-Content-Type-Options": "nosniff",
			"Content-Type":           "text/plain; charset=utf-8",
		}

		if len(w.Header()) != len(expectedHeaders) {
			t.Errorf("Want %d headers but got %d headers", len(w.Header()), len(expectedHeaders))
		}

		for k, v := range expectedHeaders {
			if h := w.Header().Get(k); h != v {
				t.Errorf("Want header %q to have value %q but got %q", k, v, h)
			}
		}
	}

	t.Run("404 Not Found for non existent directory", func(t *testing.T) {
		// httptest server will return a 200 OK, so gitserver.Client.RepoInfo will not return an error.
		initHTTPTestGitServer(t, http.StatusOK, "{}")

		gsClient := gitserver.NewMockClient()
		gsClient.StatFunc.SetDefaultReturn(&fileutil.FileInfo{}, os.ErrNotExist)

		req := httptest.NewRequest("GET", "/github.com/sourcegraph/sourcegraph/-/raw", nil)
		w := httptest.NewRecorder()

		db := dbmocks.NewMockDB()
		err := serveRaw(logger, db, gsClient)(w, req)
		if err != nil {
			t.Fatalf("Failed to invoke serveRaw: %v", err)
		}

		if w.Code != http.StatusNotFound {
			t.Fatalf("Want %d but got %d", http.StatusOK, w.Code)
		}

		assertHeaders(w)
	})

	t.Run("success response for existing directory", func(t *testing.T) {
		// httptest server will return a 200 OK, so gitserver.Client.RepoInfo will not return an error.
		initHTTPTestGitServer(t, http.StatusOK, "{}")

		gsClient := gitserver.NewMockClient()
		gsClient.StatFunc.SetDefaultReturn(&fileutil.FileInfo{Mode_: os.ModeDir}, nil)
		gsClient.ReadDirFunc.SetDefaultHook(func(context.Context, api.RepoName, api.CommitID, string, bool) ([]fs.FileInfo, error) {
			return []fs.FileInfo{
				&fileutil.FileInfo{Name_: "test/a", Mode_: os.ModeDir},
				&fileutil.FileInfo{Name_: "test/b", Mode_: os.ModeDir},
				&fileutil.FileInfo{Name_: "c.go", Mode_: 0},
			}, nil
		})

		req := httptest.NewRequest("GET", "/github.com/sourcegraph/sourcegraph/-/raw", nil)
		w := httptest.NewRecorder()

		db := dbmocks.NewMockDB()
		err := serveRaw(logger, db, gsClient)(w, req)
		if err != nil {
			t.Fatalf("Failed to invoke serveRaw: %v", err)
		}

		if w.Code != http.StatusOK {
			t.Fatalf("Want %d but got %d", http.StatusOK, w.Code)
		}

		assertHeaders(w)

		want := `a/
b/
c.go`
		body := string(w.Body.Bytes())
		if body != want {
			t.Errorf("Want %q in body, but got %q", want, body)
		}
	})

	t.Run("success response for existing file", func(t *testing.T) {
		// httptest server will return a 200 OK, so gitserver.Client.RepoInfo will not return an error.
		initHTTPTestGitServer(t, http.StatusOK, "{}")

		gitserverClient := gitserver.NewMockClient()
		gitserverClient.StatFunc.SetDefaultReturn(&fileutil.FileInfo{Mode_: 0}, nil)
		gitserverClient.NewFileReaderFunc.SetDefaultHook(func(context.Context, api.RepoName, api.CommitID, string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("this is a test file")), nil
		})

		req := httptest.NewRequest("GET", "/github.com/sourcegraph/sourcegraph/-/raw", nil)
		w := httptest.NewRecorder()

		err := serveRaw(logger, dbmocks.NewMockDB(), gitserverClient)(w, req)
		if err != nil {
			t.Fatalf("Failed to invoke serveRaw: %v", err)
		}

		if w.Code != http.StatusOK {
			t.Fatalf("Want %d but got %d", http.StatusOK, w.Code)
		}

		assertHeaders(w)

		want := "this is a test file"

		body := string(w.Body.Bytes())
		if body != want {
			t.Errorf("Want %q in body, but got %q", want, body)
		}
	})

	// Ensure that anything apart from tar/zip/text is still handled with a text/plain content type.
	t.Run("success response for existing file with format=exe", func(t *testing.T) {
		// httptest server will return a 200 OK, so gitserver.Client.RepoInfo will not return an error.
		initHTTPTestGitServer(t, http.StatusOK, "{}")

		gitserverClient := gitserver.NewMockClient()
		gitserverClient.StatFunc.SetDefaultReturn(&fileutil.FileInfo{Mode_: 0}, nil)
		gitserverClient.NewFileReaderFunc.SetDefaultHook(func(context.Context, api.RepoName, api.CommitID, string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("this is a test file")), nil
		})

		req := httptest.NewRequest("GET", "/github.com/sourcegraph/sourcegraph/-/raw?format=exe", nil)
		w := httptest.NewRecorder()

		err := serveRaw(logger, dbmocks.NewMockDB(), gitserverClient)(w, req)
		if err != nil {
			t.Fatalf("Failed to invoke serveRaw: %v", err)
		}

		if w.Code != http.StatusOK {
			t.Fatalf("Want %d but got %d", http.StatusOK, w.Code)
		}

		assertHeaders(w)

		want := "this is a test file"

		body := string(w.Body.Bytes())
		if body != want {
			t.Errorf("Want %q in body, but got %q", want, body)
		}
	})
}

func Test_serveRawRepoCloning(t *testing.T) {
	mockNewCommon = func(w http.ResponseWriter, r *http.Request, title string, serveError serveErrorHandler) (*Common, error) {
		return &Common{
			Repo: nil,
		}, nil
	}
	t.Cleanup(func() {
		mockNewCommon = nil
	})
	logger := logtest.Scoped(t)
	// Fail git server calls, as they should not be invoked for a cloning repo.
	initHTTPTestGitServer(t, http.StatusInternalServerError, "{should not be invoked}")
	gsClient := gitserver.NewMockClient()
	gsClient.StatFunc.SetDefaultReturn(nil, fmt.Errorf("should not be invoked"))

	req := httptest.NewRequest("GET", "/github.com/sourcegraph/sourcegraph/-/raw", nil)
	w := httptest.NewRecorder()
	db := dbmocks.NewMockDB()
	// Former implementation would sleep awaiting repository to be available.
	// Await request to be served with a timeout by racing done channel with time.After.
	err := serveRaw(logger, db, gsClient)(w, req)
	if err != nil {
		t.Fatalf("Failed to invoke serveRaw: %v", err)
	}
	assert.Equal(t, http.StatusNotFound, w.Code, "http response status")
	assert.Equal(t, "Repository unavailable while cloning.", string(w.Body.Bytes()), "http response body")
}
