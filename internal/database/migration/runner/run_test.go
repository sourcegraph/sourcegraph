package runner

import (
	"context"
	"strings"
	"testing"

	mockassert "github.com/derision-test/go-mockgen/v2/testutil/assert"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestRun(t *testing.T) {
	overrideSchemas(t)
	ctx := context.Background()

	t.Run("upgrade (empty)", func(t *testing.T) {
		store := testStoreWithVersion(0, false)

		if err := makeTestRunner(t, store).Run(ctx, Options{
			Operations: []MigrationOperation{
				{
					SchemaName: "well-formed",
					Type:       MigrationOperationTypeUpgrade,
				},
			},
		}); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		mockassert.CalledN(t, store.UpFunc, 4)
		mockassert.NotCalled(t, store.DownFunc)
	})

	t.Run("upgrade (partially applied)", func(t *testing.T) {
		store := testStoreWithVersion(10002, false)

		if err := makeTestRunner(t, store).Run(ctx, Options{
			Operations: []MigrationOperation{
				{
					SchemaName: "well-formed",
					Type:       MigrationOperationTypeUpgrade,
				},
			},
		}); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		mockassert.CalledN(t, store.UpFunc, 2)
		mockassert.NotCalled(t, store.DownFunc)
	})

	t.Run("upgrade (fully applied)", func(t *testing.T) {
		store := testStoreWithVersion(10004, false)

		if err := makeTestRunner(t, store).Run(ctx, Options{
			Operations: []MigrationOperation{
				{
					SchemaName: "well-formed",
					Type:       MigrationOperationTypeUpgrade,
				},
			},
		}); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		mockassert.NotCalled(t, store.UpFunc)
		mockassert.NotCalled(t, store.DownFunc)
	})

	t.Run("upgrade (future schema applied)", func(t *testing.T) {
		store := testStoreWithVersion(10008, false)

		if err := makeTestRunner(t, store).Run(ctx, Options{
			Operations: []MigrationOperation{
				{
					SchemaName: "well-formed",
					Type:       MigrationOperationTypeUpgrade,
				},
			},
		}); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		mockassert.NotCalled(t, store.UpFunc)
		mockassert.NotCalled(t, store.DownFunc)
	})

	t.Run("revert", func(t *testing.T) {
		store := testStoreWithVersion(10003, false)

		if err := makeTestRunner(t, store).Run(ctx, Options{
			Operations: []MigrationOperation{
				{
					SchemaName: "well-formed",
					Type:       MigrationOperationTypeRevert,
				},
			},
		}); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		mockassert.NotCalled(t, store.UpFunc)
		mockassert.CalledN(t, store.DownFunc, 1)
	})

	t.Run("revert (ambiguous)", func(t *testing.T) {
		store := testStoreWithVersion(10004, false)
		expectedErrorMessage := "ambiguous revert"

		if err := makeTestRunner(t, store).Run(ctx, Options{
			Operations: []MigrationOperation{
				{
					SchemaName: "well-formed",
					Type:       MigrationOperationTypeRevert,
				},
			},
		}); err == nil || !strings.Contains(err.Error(), expectedErrorMessage) {
			t.Fatalf("unexpected error: expected=%q have=%s", expectedErrorMessage, err)
		}
	})

	t.Run("upgrade (dirty database)", func(t *testing.T) {
		store := testStoreWithVersion(10003, true)
		expectedErrorMessage := "dirty database"

		if err := makeTestRunner(t, store).Run(ctx, Options{
			Operations: []MigrationOperation{
				{
					SchemaName: "well-formed",
					Type:       MigrationOperationTypeUpgrade,
				},
			},
		}); err == nil || !strings.Contains(err.Error(), expectedErrorMessage) {
			t.Fatalf("unexpected error: expected=%q have=%s", expectedErrorMessage, err)
		}
	})

	t.Run("upgrade (dirty database, ignore single dirty log)", func(t *testing.T) {
		store := testStoreWithVersion(10003, true)

		if err := makeTestRunner(t, store).Run(ctx, Options{
			Operations: []MigrationOperation{
				{
					SchemaName: "well-formed",
					Type:       MigrationOperationTypeUpgrade,
				},
			},
			IgnoreSingleDirtyLog: true,
		}); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	})

	t.Run("upgrade (dirty database, ignore single pending log)", func(t *testing.T) {
		store := testStoreWithVersion(10003, false)
		store.VersionsFunc.SetDefaultHook(func(ctx context.Context) ([]int, []int, []int, error) {
			return nil, []int{10001}, nil, nil
		})

		if err := makeTestRunner(t, store).Run(ctx, Options{
			Operations: []MigrationOperation{
				{
					SchemaName: "well-formed",
					Type:       MigrationOperationTypeUpgrade,
				},
			},
			IgnoreSinglePendingLog: true,
		}); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	})

	t.Run("upgrade (dirty database/dead migrations)", func(t *testing.T) {
		store := testStoreWithVersion(10003, true)
		store.VersionsFunc.SetDefaultReturn(nil, []int{10003}, nil, nil)
		expectedErrorMessage := "dirty database"

		if err := makeTestRunner(t, store).Run(ctx, Options{
			Operations: []MigrationOperation{
				{
					SchemaName: "well-formed",
					Type:       MigrationOperationTypeUpgrade,
				},
			},
		}); err == nil || !strings.Contains(err.Error(), expectedErrorMessage) {
			t.Fatalf("unexpected error: expected=%q have=%s", expectedErrorMessage, err)
		}
	})

	t.Run("upgrade (query error)", func(t *testing.T) {
		store := testStoreWithVersion(10000, false)
		expectedErrorMessage := "database connection error"
		store.UpFunc.PushReturn(errors.Newf(expectedErrorMessage))

		if err := makeTestRunner(t, store).Run(ctx, Options{
			Operations: []MigrationOperation{
				{
					SchemaName: "query-error",
					Type:       MigrationOperationTypeUpgrade,
				},
			},
		}); err == nil || !strings.Contains(err.Error(), expectedErrorMessage) {
			t.Fatalf("unexpected error running upgrade. want=%q have=%q", expectedErrorMessage, err)
		}

		mockassert.CalledN(t, store.UpFunc, 1)
		mockassert.NotCalled(t, store.DownFunc)
	})

	t.Run("upgrade (create index concurrently)", func(t *testing.T) {
		store := testStoreWithVersion(0, false)

		if err := makeTestRunner(t, store).Run(ctx, Options{
			Operations: []MigrationOperation{
				{
					SchemaName: "concurrent-index",
					Type:       MigrationOperationTypeUpgrade,
				},
			},
		}); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		mockassert.CalledN(t, store.UpFunc, 2)
		mockassert.NotCalled(t, store.DownFunc)
	})
}
