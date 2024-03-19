package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProjectID(t *testing.T) {
	seenID := make(map[string]struct{})
	const (
		serviceID = "msp-test"
		envID     = "dev"
	)

	for range 100 {
		id, err := NewProjectID(serviceID, envID, DefaultSuffixLength)
		require.NoError(t, err)

		assert.Contains(t, id, serviceID)
		assert.Contains(t, id, envID)

		_, seenBefore := seenID[id]
		assert.False(t, seenBefore, id)

		seenID[id] = struct{}{}
	}
}
