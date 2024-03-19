package main

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/run"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/sgconf"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/lib/output/outputtest"
)

func TestStartCommandSet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	buf := useOutputBuffer(t)

	commandSet := &sgconf.Commandset{Name: "test-set", Commands: []string{"test-cmd-1"}}
	command := &run.Command{
		Config: run.SGConfigCommandOptions{
			Name: "test-cmd-1",
		},
		Install: "echo 'booting up horsegraph'",
		Cmd:     "echo 'horsegraph booted up. mount your horse.' && echo 'quitting. not horsing around anymore.'",
	}

	testConf := &sgconf.Config{
		Commands:    map[string]*run.Command{"test-cmd-1": command},
		Commandsets: map[string]*sgconf.Commandset{"test-set": commandSet},
	}

	if err := startCommandSet(ctx, commandSet, testConf); err != nil {
		t.Errorf("failed to start: %s", err)
	}

	println(strings.Join(buf.Lines(), "\n"))
	expectOutput(t, buf, []string{
		"",
		"💡 Installing 1 commands...",
		"",
		"test-cmd-1 installed",
		"✅ 1/1 commands installed  ███████████████████████████████████████████████  100%",
		"",
		"✅ Everything installed! Booting up the system!",
		"",
		"Starting 1 cmds",
		"Running test-cmd-1...",
		"[     test-cmd-1] horsegraph booted up. mount your horse.",
		"[     test-cmd-1] quitting. not horsing around anymore.",
		"test-cmd-1 exited without error",
	})
}

func TestStartCommandSet_InstallError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	buf := useOutputBuffer(t)

	commandSet := &sgconf.Commandset{Name: "test-set", Commands: []string{"test-cmd-1"}}
	command := &run.Command{
		Config: run.SGConfigCommandOptions{
			Name: "test-cmd-1",
		},
		Install: "echo 'booting up horsegraph' && exit 1",
		Cmd:     "echo 'never appears'",
	}

	testConf := &sgconf.Config{
		Commands:    map[string]*run.Command{"test-cmd-1": command},
		Commandsets: map[string]*sgconf.Commandset{"test-set": commandSet},
	}

	err := startCommandSet(ctx, commandSet, testConf)
	if err == nil {
		t.Fatalf("err is nil unexpectedly")
	}
	if !strings.Contains(err.Error(), "failed to run test-cmd-1") {
		t.Errorf("err contains wrong message: %s", err.Error())
	}

	expectOutput(t, buf, []string{
		"",
		"💡 Installing 1 commands...",
		"--------------------------------------------------------------------------------",
		`Failed to build test-cmd-1: 'bash -c echo 'booting up horsegraph' && exit 1' failed with "exit status 1":`,
		"booting up horsegraph",
		"--------------------------------------------------------------------------------",
	})
}

func useOutputBuffer(t *testing.T) *outputtest.Buffer {
	t.Helper()

	buf := &outputtest.Buffer{}

	oldStdout := std.Out
	std.Out = std.NewFixedOutput(buf, true)
	t.Cleanup(func() { std.Out = oldStdout })

	return buf
}

func expectOutput(t *testing.T, buf *outputtest.Buffer, want []string) {
	t.Helper()

	have := buf.Lines()

	sort.Strings(want)
	sort.Strings(have)
	if !cmp.Equal(want, have) {
		t.Fatalf("wrong output:\n%s", cmp.Diff(want, have))
	}
}
