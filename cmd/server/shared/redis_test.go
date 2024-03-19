package shared

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRedisFixAOF(t *testing.T) {
	if _, err := exec.LookPath("redis-check-aof"); err != nil {
		t.Skip("redis-check-aof not on path: ", err)
	}
	dataDir := t.TempDir()

	var b bytes.Buffer
	redisCmd(&b, "PUT", "foo", "bar")
	want := b.String()

	// now add another command which we will corrupt, and write that out to
	// disk
	redisCmd(&b, "PUT", "bad", "baaaaad")
	bad := b.Bytes()
	bad = bad[:len(bad)-4]
	aofPath := filepath.Join(dataDir, "appendonly.aof")
	if err := os.WriteFile(aofPath, bad, 0o600); err != nil {
		t.Fatal(err)
	}

	// We run redisFixAOF twice. First time it will repair, second time should
	// be a noop since the file will be fine.
	for range 2 {
		redisFixAOF(filepath.Dir(dataDir), redisProcfileConfig{
			name:    "redis-test",
			dataDir: filepath.Base(dataDir),
		})

		got, err := os.ReadFile(aofPath)
		if err != nil {
			t.Fatal(err)
		}

		if string(got) != want {
			t.Errorf("mismatch (-want +got):\n%s", cmp.Diff(want, string(got)))
		}
	}
}

func redisCmd(out io.Writer, parts ...string) {
	_, _ = fmt.Fprintf(out, "*%d\r\n", len(parts))
	for _, p := range parts {
		_, _ = fmt.Fprintf(out, "$%d\r\n%s\r\n", len(p), p)
	}
}

func TestYesReader(t *testing.T) {
	r := &yesReader{Expletive: []byte("y\n")}
	got := make([]byte, 1000)
	n := 0
	for n < len(got) {
		for size := 1; size < 10 && n < len(got); size++ {
			if n+size >= len(got) {
				size = len(got) - n
			}
			m, err := r.Read(got[n : n+size])
			if err != nil {
				t.Fatal(err)
			}
			n += m
		}
	}

	want := bytes.Repeat([]byte("y\n"), 1000)[:1000]
	if !bytes.Equal(got, want) {
		t.Errorf("mismatch (-want +got):\n%s", cmp.Diff(want, got))
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	verbose = testing.Verbose()
	os.Exit(m.Run())
}
