package codeintelutils

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSplitFile(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("unexpected error creating temp file: %s", err)
	}
	defer func() { os.Remove(f.Name()) }()

	var expectedContents []byte
	for i := 0; i < 50; i++ {
		var partContents []byte
		for j := 0; j < 20000; j++ {
			partContents = append(partContents, byte(j))
		}

		expectedContents = append(expectedContents, partContents...)
		_, _ = f.Write(partContents)
	}
	f.Close()

	files, cleanup, err := SplitFile(f.Name(), 18000)
	if err != nil {
		t.Fatalf("unexpected error splitting file: %s", err)
	}

	var contents []byte
	for _, file := range files {
		temp, err := ioutil.ReadFile(file)
		if err != nil {
			t.Fatalf("unexpected error reading file: %s", err)
		}

		if len(temp) > 18000 {
			t.Errorf("chunk too large: want<%d have=%d", 18000, len(temp))
		}

		contents = append(contents, temp...)
	}

	if diff := cmp.Diff(expectedContents, contents); diff != "" {
		t.Errorf("unexpected split contents (-want +got):\n%s", diff)
	}

	cleanup(nil)

	for _, file := range files {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Errorf("unexpected error. want=%q have=%q", os.ErrNotExist, err)
		}
	}
}
