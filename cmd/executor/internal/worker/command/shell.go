package command

import (
	"path/filepath"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/files"
)

// NewShellSpec creates a new spec for a shell command.
func NewShellSpec(workingDir string, scriptPath string, spec Spec, options DockerOptions) Spec {
	// TODO - remove this once src-cli is not required anymore for SSBC.
	if spec.Image == "" {
		env := spec.Env
		return Spec{
			Key:       spec.Key,
			Command:   spec.Command,
			Dir:       filepath.Join(workingDir, spec.Dir),
			Env:       env,
			Operation: spec.Operation,
		}
	}

	hostDir := workingDir
	if options.Resources.DockerHostMountPath != "" {
		hostDir = filepath.Join(options.Resources.DockerHostMountPath, filepath.Base(workingDir))
	}

	return Spec{
		Key: spec.Key,
		Dir: filepath.Join(hostDir, spec.Dir),
		Env: spec.Env,
		Command: Flatten(
			"/bin/sh",
			filepath.Join(hostDir, files.ScriptsPath, scriptPath),
		),
		Operation: spec.Operation,
	}
}
