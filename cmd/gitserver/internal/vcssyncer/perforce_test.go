package vcssyncer

import (
	"strings"
	"testing"

	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
)

func TestP4DepotSyncer_p4CommandEnv(t *testing.T) {
	fs := gitserverfs.NewMockFS()
	fs.P4HomeDirFunc.SetDefaultReturn("p4home", nil)
	syncer := &perforceDepotSyncer{
		logger:                  logtest.Scoped(t),
		recordingCommandFactory: wrexec.NewNoOpRecordingCommandFactory(),
		p4Client:                "client",
		fs:                      fs,
	}

	cwd := t.TempDir()
	vars, err := syncer.p4CommandEnv(cwd, "host", "username", "password")
	require.NoError(t, err)
	assertEnv := func(key, value string) {
		var match string
		for _, s := range vars {
			parts := strings.SplitN(s, "=", 2)
			if len(parts) != 2 {
				t.Errorf("Expected 2 parts, got %d in %q", len(parts), s)
				continue
			}
			if parts[0] != key {
				continue
			}
			// Last match wins
			if parts[1] == value {
				match = parts[1]
			}
		}
		if match == "" {
			t.Errorf("No match found for %q", key)
		} else if match != value {
			t.Errorf("Want %q, got %q", value, match)
		}
	}
	assertEnv("HOME", "p4home")
	assertEnv("P4CLIENT", "client")
	assertEnv("P4PORT", "host")
	assertEnv("P4USER", "username")
	assertEnv("P4PASSWD", "password")
	assertEnv("P4CLIENTPATH", cwd)
}
