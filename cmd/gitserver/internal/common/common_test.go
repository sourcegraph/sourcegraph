package common

import (
	"os/exec"
	"testing"
)

func TestGitDirPath(t *testing.T) {
	dir := GitDir("/repos/myrepo/.git")

	path := dir.Path("objects", "pack")
	if path != "/repos/myrepo/.git/objects/pack" {
		t.Errorf("Expected /repos/myrepo/.git/objects/pack, got %s", path)
	}

	path = dir.Path()
	if path != "/repos/myrepo/.git" {
		t.Errorf("Expected /repos/myrepo/.git, got %s", path)
	}
}

func TestGitDirSet(t *testing.T) {
	dir := GitDir("/repos/myrepo/.git")
	cmd := exec.Command("git", "log")

	dir.Set(cmd)

	if cmd.Dir != "/repos/myrepo/.git" {
		t.Errorf("Expected dir to be set to /repos/myrepo/.git, got %s", cmd.Dir)
	}

	foundGitDirEnv := false
	for _, env := range cmd.Env {
		if env == "GIT_DIR=/repos/myrepo/.git" {
			foundGitDirEnv = true
			break
		}
	}

	if !foundGitDirEnv {
		t.Error("Expected GIT_DIR env to be set")
	}
}
