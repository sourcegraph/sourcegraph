package loki

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/bk"
	"github.com/sourcegraph/sourcegraph/internal/randstring"
)

func TestChunkEntry(t *testing.T) {
	ts := time.Now().UnixNano()
	line := "0123456789"
	entry := [2]string{fmt.Sprintf("%d", ts), line}

	results, err := chunkEntry(entry, 2)
	if err != nil {
		t.Error(err)
	}

	if len(results) != len(line)/2 {
		t.Errorf("Got %d chunks wanted %d", len(results), len(line)/2)
	}

	allLines := bytes.NewBuffer(nil)
	for i := range len(results) {
		expectedTs := fmt.Sprintf("%d", ts+int64(i))
		if results[i][0] != expectedTs {
			t.Errorf("wrong timestamp at %d. Got %s wanted %s", i, results[i][0], expectedTs)
		}
		allLines.WriteString(results[i][1])
	}

	if allLines.String() != line {
		t.Errorf("reconstructed chunked line differs from original line. Got %q wanted %q", allLines.String(), line)
	}
}

func TestSplitIntoChunks(t *testing.T) {
	t.Run("general split into chunks", func(t *testing.T) {
		line := randstring.NewLen(100)

		result := splitIntoChunks([]byte(line), 10)
		if len(result) != 10 {
			t.Errorf("expected string of size 100 to be split into 10 chunks. Got %d wanted %d", len(result), 10)
		}
	})

	t.Run("chunk size larger than string", func(t *testing.T) {
		line := randstring.NewLen(100)

		result := splitIntoChunks([]byte(line), len(line)+1)
		if len(result) != 1 {
			t.Errorf("expected string of size 100 to be split into 10 chunks. Got %d wanted %d", len(result), 1)
		}
	})

	t.Run("line size larger by 1 than chunk size", func(t *testing.T) {
		line := randstring.NewLen(100)

		result := splitIntoChunks([]byte(line), 99)
		if len(result) != 2 {
			t.Errorf("expected string of size 100 to be split into 10 chunks. Got %d wanted %d", len(result), 2)
		}
	})

	t.Run("check chunk content", func(t *testing.T) {
		line := "123456789"

		result := splitIntoChunks([]byte(line), 5)

		if bytes.Compare(result[0], []byte("12345")) != 0 {
			t.Errorf("incorrect chunk content for 0 idx. Got %s wanted %s", string(result[0]), "12345")
		}

		if bytes.Compare(result[1], []byte("6789")) != 0 {
			t.Errorf("incorrect chunk content for 0 idx. Got %s wanted %s", string(result[0]), "12345")
		}
	})

	t.Run("chunk sizes", func(t *testing.T) {
		line := randstring.NewLen(1337)

		results := splitIntoChunks([]byte(line), 1024)

		for i, r := range results {
			if len(r) > 1024 {
				t.Errorf("incorrect sized chunk found at %d with size %d", i, len(r))
			}
		}
	})
}

func TestNewStreamFromJobLogs(t *testing.T) {
	type args struct {
		log string
	}
	tests := []struct {
		name    string
		args    args
		want    [][2]string
		wantErr bool
	}{
		{
			name: "parse empty content",
			args: args{
				log: ``,
			},
			want: [][2]string{},
		},
		{
			name: "parse invalid line",
			args: args{
				log: `~~~ Preparing working directory`,
			},
			wantErr: true,
		},
		{
			name: "parse line",
			args: args{
				log: `_bk;t=1633575941106~~~ Preparing working directory`,
			},
			want: [][2]string{
				{"1633575941106000000", "~~~ Preparing working directory"},
			},
		},
		{
			name: "merge timestamps",
			args: args{
				log: `_bk;t=1633575941106~~~ Preparing working directory
_bk;t=1633575941106$ cd /buildkite/builds/buildkite-agent-77bfc969fc-4zfqc-1/sourcegraph/sourcegraph
_bk;t=1633575941112$ git remote set-url origin git@github.com:sourcegraph/sourcegraph.git
_bk;t=1633575946276remote: Enumerating objects: 25, done._bk;t=1633575947202

_bk;t=1633575947202remote: Counting objects:   4% (1/25)_bk;t=1633575947202
remote: Counting objects:   8% (2/25)_bk;t=1633575947202`,
			},
			want: [][2]string{
				{"1633575941106000000", "~~~ Preparing working directory\n$ cd /buildkite/builds/buildkite-agent-77bfc969fc-4zfqc-1/sourcegraph/sourcegraph"},
				{"1633575941112000000", "$ git remote set-url origin git@github.com:sourcegraph/sourcegraph.git"},
				{"1633575946276000000", "remote: Enumerating objects: 25, done."},
				{"1633575947202000000", "remote: Counting objects:   4% (1/25)\nremote: Counting objects:   8% (2/25)"},
			},
		},
		{
			name: "weird ansi things",
			args: args{
				log: `_bk;t=1633575951822[38;5;48m2021-10-07 03:05:51 INFO  [0m [0mUpdating BUILDKITE_COMMIT to "d4b6e13eab2216ea2a934607df5c97a25e920207"[0m

_bk;t=1633575951838[38;5;48m2021-10-07 03:05:54 INFO  [0m [0mSuccessfully uploaded and parsed pipeline config[0m`,
			},
			want: [][2]string{
				{"1633575951822000000", "2021-10-07 03:05:51 INFO   Updating BUILDKITE_COMMIT to \"d4b6e13eab2216ea2a934607df5c97a25e920207\""},
				{"1633575951838000000", "2021-10-07 03:05:54 INFO   Successfully uploaded and parsed pipeline config"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewStreamFromJobLogs(&bk.JobLogs{
				JobMeta: bk.JobMeta{Job: tt.name},
				Content: &tt.args.log,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("NewStreamFromJobLogs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if diff := cmp.Diff(tt.want, got.Values); diff != "" {
				t.Fatalf("(-want +got):\n%s", diff)
			}
		})
	}
}
