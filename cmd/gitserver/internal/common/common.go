package common

import (
	"os"
	"os/exec"
	"path/filepath"
)

// GitDir is an absolute path to a GIT_DIR.
// They will all follow the form:
//
//	${s.ReposDir}/${name}/.git
type GitDir string

// Path is a helper which returns filepath.Join(dir, elem...)
func (dir GitDir) Path(elem ...string) string {
	if len(elem) == 0 {
		return string(dir)
	}
	return filepath.Join(append([]string{string(dir)}, elem...)...)
}

// Set updates cmd so that it will run in dir.
//
// Note: GitDir is always a valid GIT_DIR, so we additionally set the
// environment variable GIT_DIR. This is to avoid git doing discovery in case
// of a bad repo, leading to hard to diagnose error messages.
func (dir GitDir) Set(cmd *exec.Cmd) {
	cmd.Dir = string(dir)
	if cmd.Env == nil {
		// Do not strip out existing env when setting.
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "GIT_DIR="+string(dir))
}

// ErrRepoCorrupted is an error indicating that the repository is potentially
// corrupted.
type ErrRepoCorrupted struct {
	Reason string
}

func (e ErrRepoCorrupted) Error() string {
	return "repository is corrupted: " + e.Reason
}
