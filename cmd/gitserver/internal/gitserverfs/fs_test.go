package gitserverfs

import (
	"path/filepath"
	"testing"

	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
)

func TestIgnorePath(t *testing.T) {
	reposDir := "/data/repos"

	for _, tc := range []struct {
		path         string
		shouldIgnore bool
	}{
		{path: filepath.Join(reposDir, tempDirName), shouldIgnore: true},
		{path: filepath.Join(reposDir, p4HomeName), shouldIgnore: true},
		// Double check handling of trailing space
		{path: filepath.Join(reposDir, p4HomeName+"   "), shouldIgnore: true},
		{path: filepath.Join(reposDir, "sourcegraph/sourcegraph"), shouldIgnore: false},
	} {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tc.shouldIgnore, ignorePath(reposDir, tc.path))
		})
	}
}

func TestRemoveRepoDirectory(t *testing.T) {
	logger := logtest.Scoped(t)
	root := t.TempDir()

	mkFiles(t, root,
		"github.com/foo/baz/.git/HEAD",
		"github.com/foo/survivor/.git/HEAD",
		"github.com/bam/bam/.git/HEAD",
		"example.com/repo/.git/HEAD",
	)

	// Remove everything but github.com/foo/survivor
	for _, d := range []string{
		"github.com/foo/baz/.git",
		"github.com/bam/bam/.git",
		"example.com/repo/.git",
	} {
		if err := removeRepoDirectory(logger, root, common.GitDir(filepath.Join(root, d))); err != nil {
			t.Fatalf("failed to remove %s: %s", d, err)
		}
	}

	// Removing them a second time is safe
	for _, d := range []string{
		"github.com/foo/baz/.git",
		"github.com/bam/bam/.git",
		"example.com/repo/.git",
	} {
		if err := removeRepoDirectory(logger, root, common.GitDir(filepath.Join(root, d))); err != nil {
			t.Fatalf("failed to remove %s: %s", d, err)
		}
	}

	assertPaths(t, root,
		"github.com/foo/survivor/.git/HEAD",
		".tmp",
	)
}

func TestRemoveRepoDirectory_Empty(t *testing.T) {
	root := t.TempDir()

	mkFiles(t, root,
		"github.com/foo/baz/.git/HEAD",
	)
	logger := logtest.Scoped(t)

	if err := removeRepoDirectory(logger, root, common.GitDir(filepath.Join(root, "github.com/foo/baz/.git"))); err != nil {
		t.Fatal(err)
	}

	assertPaths(t, root,
		".tmp",
	)
}
