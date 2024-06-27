package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModelRef(t *testing.T) {
	testMRef := ModelRef("foo::bar::baz")
	assert.EqualValues(t, "foo", testMRef.ProviderID())
	assert.EqualValues(t, "baz", testMRef.ModelID())
}
