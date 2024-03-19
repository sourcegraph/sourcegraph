package webhooks

import (
	"context"
	"testing"

	mockassert "github.com/derision-test/go-mockgen/v2/testutil/assert"
	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestHandler(t *testing.T) {
	t.Run("store error", func(t *testing.T) {
		want := errors.New("error")
		store := dbmocks.NewMockWebhookLogStore()
		store.DeleteStaleFunc.SetDefaultReturn(want)

		ph := &handler{
			store: store,
		}

		err := ph.Handle(context.Background())
		assert.ErrorIs(t, err, want)
		mockassert.CalledOnce(t, store.DeleteStaleFunc)
	})

	t.Run("success", func(t *testing.T) {
		store := dbmocks.NewMockWebhookLogStore()
		ph := &handler{
			store: store,
		}

		err := ph.Handle(context.Background())
		assert.Nil(t, err)
		mockassert.CalledOnce(t, store.DeleteStaleFunc)
	})
}
