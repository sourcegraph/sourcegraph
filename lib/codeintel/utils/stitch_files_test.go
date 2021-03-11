package codeintelutils

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestStitchMultipart(t *testing.T) {
	for _, compress := range []bool{true, false} {
		tempDir, err := ioutil.TempDir("", "codeintel-")
		if err != nil {
			t.Fatalf("unexpected error creating temp directory: %s", err)
		}
		defer os.RemoveAll(tempDir)

		filename := filepath.Join(tempDir, "target")
		partFilename := func(i int) string {
			return filepath.Join(tempDir, fmt.Sprintf("%d.part", i))
		}

		var expectedContents []byte
		for i := 0; i < 50; i++ {
			partContents := make([]byte, 20000)
			for j := 0; j < 20000; j++ {
				partContents[j] = byte(i)
			}
			expectedContents = append(expectedContents, partContents...)

			var buf bytes.Buffer
			gzipWriter := gzip.NewWriter(&buf)

			if _, err := io.Copy(gzipWriter, bytes.NewReader(partContents)); err != nil {
				t.Fatalf("unexpected error writing to buffer: %s", err)
			}
			if err := gzipWriter.Close(); err != nil {
				t.Fatalf("unexpected error closing gzip writer: %s", err)
			}

			if err := ioutil.WriteFile(partFilename(i), buf.Bytes(), os.ModePerm); err != nil {
				t.Fatalf("unexpected error writing file: %s", err)
			}
		}

		if err := StitchFiles(filename, partFilename, compress, compress); err != nil {
			t.Fatalf("unexpected error stitching files: %s", err)
		}

		f, err := os.Open(filename)
		if err != nil {
			t.Fatalf("unexpected error opening file: %s", err)
		}
		defer f.Close()

		gzipReader, err := gzip.NewReader(f)
		if err != nil {
			t.Fatalf("unexpected error opening gzip reader: %s", err)
		}

		contents, err := ioutil.ReadAll(gzipReader)
		if err != nil {
			t.Fatalf("unexpected error reading file: %s", err)
		}

		if diff := cmp.Diff(expectedContents, contents); diff != "" {
			t.Errorf("unexpected file contents (-want +got):\n%s", diff)
		}

		for i := 0; i < 50; i++ {
			if _, err := os.Stat(partFilename(i)); !os.IsNotExist(err) {
				t.Errorf("unexpected error. want=%q have=%q", os.ErrNotExist, err)
			}
		}
	}
}
