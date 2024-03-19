package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bt "github.com/sourcegraph/sourcegraph/internal/batches/testing"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
)

func testStoreBatchSpecWorkspaceFiles(t *testing.T, ctx context.Context, s *Store, clock bt.Clock) {
	files := make([]*btypes.BatchSpecWorkspaceFile, 0, 5)

	spec := &btypes.BatchSpec{
		UserID:          int32(1234),
		NamespaceUserID: int32(1234),
	}
	err := s.CreateBatchSpec(ctx, spec)
	require.NoError(t, err)

	t.Run("Create", func(t *testing.T) {
		for i := range cap(files) {
			file := &btypes.BatchSpecWorkspaceFile{
				BatchSpecID: spec.ID,
				FileName:    fmt.Sprintf("hello-%d.txt", i),
				Path:        "foo/bar",
				Size:        12,
				Content:     []byte("hello, world!"),
				ModifiedAt:  clock.Now(),
			}
			expected := file.Clone()

			err := s.UpsertBatchSpecWorkspaceFile(ctx, file)
			require.NoError(t, err)

			expected.ID = file.ID
			expected.RandID = file.RandID
			expected.CreatedAt = file.CreatedAt
			expected.UpdatedAt = file.UpdatedAt

			diff := cmp.Diff(file, expected)
			require.Empty(t, diff)

			files = append(files, file)
		}
		assert.Len(t, files, 5)
	})

	t.Run("Update", func(t *testing.T) {
		clock.Add(1 * time.Second)

		file := files[0]
		file.Size = 20

		expected := file.Clone()
		expected.UpdatedAt = clock.Now()

		err := s.UpsertBatchSpecWorkspaceFile(ctx, file)
		require.NoError(t, err)

		diff := cmp.Diff(file, expected)
		assert.Empty(t, diff)
	})

	t.Run("Get", func(t *testing.T) {
		t.Run("ByID", func(t *testing.T) {
			expected := files[0]
			actual, err := s.GetBatchSpecWorkspaceFile(ctx, GetBatchSpecWorkspaceFileOpts{ID: expected.ID})
			require.NoError(t, err)

			diff := cmp.Diff(actual, expected)
			assert.Empty(t, diff)
		})

		t.Run("ByRandID", func(t *testing.T) {
			expected := files[0]
			actual, err := s.GetBatchSpecWorkspaceFile(ctx, GetBatchSpecWorkspaceFileOpts{RandID: expected.RandID})
			require.NoError(t, err)

			diff := cmp.Diff(actual, expected)
			assert.Empty(t, diff)
		})

		t.Run("No Options", func(t *testing.T) {
			file, err := s.GetBatchSpecWorkspaceFile(ctx, GetBatchSpecWorkspaceFileOpts{})
			assert.Error(t, err)
			assert.Nil(t, file)
		})
	})

	t.Run("Count", func(t *testing.T) {
		t.Run("ByID", func(t *testing.T) {
			count, err := s.CountBatchSpecWorkspaceFiles(ctx, ListBatchSpecWorkspaceFileOpts{ID: files[0].ID})
			require.NoError(t, err)
			assert.Equal(t, 1, count)
		})

		t.Run("ByRandID", func(t *testing.T) {
			count, err := s.CountBatchSpecWorkspaceFiles(ctx, ListBatchSpecWorkspaceFileOpts{RandID: files[0].RandID})
			require.NoError(t, err)
			assert.Equal(t, 1, count)
		})

		t.Run("ByBatchSpecID", func(t *testing.T) {
			count, err := s.CountBatchSpecWorkspaceFiles(ctx, ListBatchSpecWorkspaceFileOpts{BatchSpecID: spec.ID})
			require.NoError(t, err)
			assert.Equal(t, len(files), count)
		})

		t.Run("ByBatchSpecRandID", func(t *testing.T) {
			count, err := s.CountBatchSpecWorkspaceFiles(ctx, ListBatchSpecWorkspaceFileOpts{BatchSpecRandID: spec.RandID})
			require.NoError(t, err)
			assert.Equal(t, len(files), count)
		})
	})

	t.Run("List", func(t *testing.T) {
		t.Run("ByBatchSpecID", func(t *testing.T) {
			actual, next, err := s.ListBatchSpecWorkspaceFiles(ctx, ListBatchSpecWorkspaceFileOpts{BatchSpecID: spec.ID})
			require.NoError(t, err)
			assert.Zero(t, next)
			assert.Len(t, actual, len(files))

			for _, f := range actual {
				expected := getExpectedFile(f.ID, files)
				diff := cmp.Diff(f, expected)
				require.Empty(t, diff)
			}
		})

		t.Run("ByBatchSpecRandID", func(t *testing.T) {
			actual, next, err := s.ListBatchSpecWorkspaceFiles(ctx, ListBatchSpecWorkspaceFileOpts{BatchSpecRandID: spec.RawSpec})
			require.NoError(t, err)
			assert.Zero(t, next)
			assert.Len(t, actual, len(files))

			for _, f := range actual {
				expected := getExpectedFile(f.ID, files)
				diff := cmp.Diff(f, expected)
				require.Empty(t, diff)
			}
		})

		t.Run("Large Limit", func(t *testing.T) {
			opts := ListBatchSpecWorkspaceFileOpts{
				LimitOpts: LimitOpts{
					Limit: 100,
				},
				BatchSpecRandID: spec.RawSpec,
			}
			actual, next, err := s.ListBatchSpecWorkspaceFiles(ctx, opts)
			require.NoError(t, err)
			assert.Zero(t, next)
			assert.Len(t, actual, len(files))

			for _, f := range actual {
				expected := getExpectedFile(f.ID, files)
				diff := cmp.Diff(f, expected)
				require.Empty(t, diff)
			}
		})

		t.Run("Small Limit", func(t *testing.T) {
			opts := ListBatchSpecWorkspaceFileOpts{
				LimitOpts: LimitOpts{
					Limit: 2,
				},
				BatchSpecRandID: spec.RawSpec,
			}
			actual, next, err := s.ListBatchSpecWorkspaceFiles(ctx, opts)
			require.NoError(t, err)
			// Limit
			assert.Equal(t, files[2].ID, next)
			assert.Len(t, actual, 2)

			for _, f := range actual {
				expected := getExpectedFile(f.ID, files)
				diff := cmp.Diff(f, expected)
				require.Empty(t, diff)
			}
		})

		t.Run("From Cursor", func(t *testing.T) {
			opts := ListBatchSpecWorkspaceFileOpts{
				LimitOpts: LimitOpts{
					Limit: 3,
				},
				Cursor:          int64(3),
				BatchSpecRandID: spec.RawSpec,
			}
			actual, next, err := s.ListBatchSpecWorkspaceFiles(ctx, opts)
			require.NoError(t, err)
			assert.Zero(t, next)
			assert.Len(t, actual, 3)

			for _, f := range actual {
				expected := getExpectedFile(f.ID, files)
				diff := cmp.Diff(f, expected)
				require.Empty(t, diff)
			}
		})
	})

	t.Run("Delete", func(t *testing.T) {
		t.Run("No Options", func(t *testing.T) {
			err := s.DeleteBatchSpecWorkspaceFile(ctx, DeleteBatchSpecWorkspaceFileOpts{})
			assert.Error(t, err)
		})

		t.Run("ByID", func(t *testing.T) {
			err := s.DeleteBatchSpecWorkspaceFile(ctx, DeleteBatchSpecWorkspaceFileOpts{ID: files[0].ID})
			require.NoError(t, err)

			deletedFile, err := s.GetBatchSpecWorkspaceFile(ctx, GetBatchSpecWorkspaceFileOpts{ID: files[0].ID})
			require.ErrorIs(t, err, ErrNoResults)
			assert.Nil(t, deletedFile)
		})

		t.Run("ByBatchSpecID", func(t *testing.T) {
			// Add one more file just in case current ones have been deleted
			newFile := &btypes.BatchSpecWorkspaceFile{
				BatchSpecID: spec.ID,
				FileName:    "by-spec-id.txt",
				Path:        "foo/bar",
				Size:        12,
				Content:     []byte("hello, world!"),
				ModifiedAt:  clock.Now(),
			}
			err := s.UpsertBatchSpecWorkspaceFile(ctx, newFile)
			require.NoError(t, err)

			err = s.DeleteBatchSpecWorkspaceFile(ctx, DeleteBatchSpecWorkspaceFileOpts{BatchSpecID: spec.ID})
			require.NoError(t, err)

			for _, f := range files {
				deletedFile, err := s.GetBatchSpecWorkspaceFile(ctx, GetBatchSpecWorkspaceFileOpts{ID: f.ID})
				require.ErrorIs(t, err, ErrNoResults)
				assert.Nil(t, deletedFile)
			}

			// And check if the new one has also been deleted
			deletedFile, err := s.GetBatchSpecWorkspaceFile(ctx, GetBatchSpecWorkspaceFileOpts{ID: newFile.ID})
			require.ErrorIs(t, err, ErrNoResults)
			assert.Nil(t, deletedFile)
		})
	})
}

func getExpectedFile(id int64, expects []*btypes.BatchSpecWorkspaceFile) *btypes.BatchSpecWorkspaceFile {
	for _, expect := range expects {
		if id == expect.ID {
			return expect
		}
	}
	return nil
}
