package registry

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/dotcom"
)

func TestHandleRegistry(t *testing.T) {
	dotcom.MockSourcegraphDotComMode(t, true)

	t.Run("list", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rr.Body = new(bytes.Buffer)
		req, _ := http.NewRequest("GET", "/.api/registry/extensions", nil)
		req.Header.Set("Accept", "application/vnd.sourcegraph.api+json;version=20180621")
		handleRegistry(rr, req)
		if want := 200; rr.Result().StatusCode != want {
			t.Errorf("got HTTP status %d, want %d", rr.Result().StatusCode, want)
		}
		body, _ := io.ReadAll(rr.Result().Body)
		if want := []byte("sourcegraph/go"); !bytes.Contains(body, want) {
			t.Error("unexpected result")
		}
	})

	t.Run("get", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rr.Body = new(bytes.Buffer)
		req, _ := http.NewRequest("GET", "/.api/registry/extensions/extension-id/sourcegraph/go", nil)
		req.Header.Set("Accept", "application/vnd.sourcegraph.api+json;version=20180621")
		handleRegistry(rr, req)
		if want := 200; rr.Result().StatusCode != want {
			t.Errorf("got HTTP status %d, want %d", rr.Result().StatusCode, want)
		}
		body, _ := io.ReadAll(rr.Result().Body)
		if want := []byte("contributes"); !bytes.Contains(body, want) {
			t.Error("unexpected result")
		}
	})
}
