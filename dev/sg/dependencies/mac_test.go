package dependencies

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/check"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/usershell"
)

func TestMacFix(t *testing.T) {
	if !strings.Contains(*sgSetupTests, string(OSMac)) && !strings.Contains(*sgSetupTests, "macos") {
		t.Skip("Skipping Mac sg setup tests")
	}

	// Initialize context with user shell information
	ctx, err := usershell.Context(context.Background())
	require.NoError(t, err)

	input := bytes.NewReader([]byte("infra_devinfra\n\n"))
	// Set up runner with no input and simple output
	runner := check.NewRunner(input, std.NewSimpleOutput(os.Stdout, true), Mac)

	// automatically fix everything!
	t.Run("Fix", func(t *testing.T) {
		err = runner.Fix(ctx, testArgs)
		require.Nil(t, err)
	})

	// now check that everything was fixed
	t.Run("Check", func(t *testing.T) {
		err = runner.Check(ctx, testArgs)
		assert.Nil(t, err)
	})
}
