package workspace

import (
	"context"
	"fmt"
	"os"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/cmdlogger"
)

type kubernetesWorkspace struct {
	scriptFilenames []string
	workspaceDir    string
	logger          cmdlogger.Logger
}

// NewKubernetesWorkspace creates a new workspace for a job.
func NewKubernetesWorkspace(
	logger cmdlogger.Logger,
) (Workspace, error) {
	return &kubernetesWorkspace{logger: logger}, nil
}

func (w kubernetesWorkspace) Path() string {
	return w.workspaceDir
}

func (w kubernetesWorkspace) WorkingDirectory() string {
	return w.workspaceDir
}

func (w kubernetesWorkspace) ScriptFilenames() []string {
	return w.scriptFilenames
}

func (w kubernetesWorkspace) Remove(ctx context.Context, keepWorkspace bool) {
	handle := w.logger.LogEntry("teardown.fs", nil)
	defer func() {
		// We always finish this with exit code 0 even if it errored, because workspace
		// cleanup doesn't fail the execution job. We can deal with it separately.
		handle.Finalize(0)
		handle.Close()
	}()

	if keepWorkspace {
		fmt.Fprintf(handle, "Preserving workspace (%s) as per config", w.workspaceDir)
		return
	}

	if w.workspaceDir != "" {
		fmt.Fprintf(handle, "Removing %s\n", w.workspaceDir)
		if rmErr := os.RemoveAll(w.workspaceDir); rmErr != nil {
			fmt.Fprintf(handle, "Operation failed: %s\n", rmErr.Error())
		}
	}
}
