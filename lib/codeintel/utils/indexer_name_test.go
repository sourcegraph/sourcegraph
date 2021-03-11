package codeintelutils

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

const testMetaDataVertex = `{"label": "metaData", "toolInfo": {"name": "test"}}`
const testVertex = `{"id": "a", "type": "edge", "label": "textDocument/references", "outV": "b", "inV": "c"}`

func TestReadIndexerName(t *testing.T) {
	name, err := ReadIndexerName(generateTestIndex(testMetaDataVertex))
	if err != nil {
		t.Fatalf("unexpected error reading indexer name: %s", err)
	}
	if name != "test" {
		t.Errorf("unexpected indexer name. want=%s have=%s", "test", name)
	}
}

func TestReadIndexerNameMalformed(t *testing.T) {
	for _, metaDataVertex := range []string{`invalid json`, `{"label": "textDocument/references"}`} {
		if _, err := ReadIndexerName(generateTestIndex(metaDataVertex)); err != ErrInvalidMetaDataVertex {
			t.Fatalf("unexpected error reading indexer name. want=%q have=%q", ErrInvalidMetaDataVertex, err)
		}
	}
}

func generateTestIndex(metaDataVertex string) io.Reader {
	lines := []string{metaDataVertex}
	for i := 0; i < 20000; i++ {
		lines = append(lines, testVertex)
	}

	return bytes.NewReader([]byte(strings.Join(lines, "\n") + "\n"))
}
