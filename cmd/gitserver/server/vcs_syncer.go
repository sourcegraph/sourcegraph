package server

import (
	"context"
	"os/exec"

	"github.com/sourcegraph/sourcegraph/internal/vcs"
)

// VCSSyncer describes whether and how to sync content from a VCS remote to
// local disk.
type VCSSyncer interface {
	// Type returns the type of the syncer.
	Type() string
	// IsCloneable checks to see if the VCS remote URL is cloneable. Any non-nil
	// error indicates there is a problem.
	IsCloneable(ctx context.Context, remoteURL *vcs.URL) error
	// CloneCommand returns the command to be executed for cloning from remote.
	CloneCommand(ctx context.Context, remoteURL *vcs.URL, tmpPath string) (cmd *exec.Cmd, err error)
	// Fetch tries to fetch updates from the remote to given directory.
	// The revspec parameter is optional and specifies that the client is specifically
	// interested in fetching the provided revspec (example "v2.3.4^0").
	// For package hosts (vcsPackagesSyncer, npm/pypi/crates.io), the revspec is used
	// to lazily fetch package versions. More details at
	// https://github.com/sourcegraph/sourcegraph/issues/37921#issuecomment-1184301885
	Fetch(ctx context.Context, remoteURL *vcs.URL, dir GitDir, revspec string) error
	// RemoteShowCommand returns the command to be executed for showing remote.
	RemoteShowCommand(ctx context.Context, remoteURL *vcs.URL) (cmd *exec.Cmd, err error)
}

type notFoundError struct{ error }

func (e notFoundError) NotFound() bool { return true }
