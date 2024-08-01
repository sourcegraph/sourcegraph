package workspace_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/workspace"
)

// TestNewKubernetesWorkspace tests the creation of a new Kubernetes workspace.
// It verifies that:
// - A workspace is created successfully with a logger
// - The created workspace is not nil
// - No error is returned during creation
//
// Basically, it does nothing
// because for Executors running on Kubernetes,
// now that single job pod is the only way to use it,
// There are no workspaces.
func TestNewKubernetesWorkspace(t *testing.T) {
	t.Run("creates workspace with logger", func(t *testing.T) {
		logger := workspace.NewMockLogger()
		ws, err := workspace.NewKubernetesWorkspace(logger)

		require.NoError(t, err)
		assert.NotNil(t, ws)
	})

	t.Run("returns non-nil workspace", func(t *testing.T) {
		logger := workspace.NewMockLogger()
		ws, _ := workspace.NewKubernetesWorkspace(logger)

		assert.NotNil(t, ws)
	})

	t.Run("returns no error", func(t *testing.T) {
		logger := workspace.NewMockLogger()
		_, err := workspace.NewKubernetesWorkspace(logger)

		assert.NoError(t, err)
	})
}
