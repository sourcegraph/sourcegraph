package conf

import (
	"reflect"
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/dotcom"
)

func TestAuthPublic(t *testing.T) {
	t.Run("Default, self-hosted instance non-public auth", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, false)
		got := AuthPublic()
		want := false
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("Sourcegraph.com public auth", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)
		got := AuthPublic()
		want := true
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}
