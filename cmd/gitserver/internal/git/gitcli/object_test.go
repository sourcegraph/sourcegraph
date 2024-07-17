package gitcli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestGitCLIBackend_GetObject(t *testing.T) {
	ctx := context.Background()

	// Prepare repo state:
	backend := BackendWithRepoCommands(t,
		"echo line1 > f",
		"git add f",
		"git commit -m foo --author='Foo Author <foo@sourcegraph.com>'",
		`git tag -m "Test base tag" testbase`,
	)

	// Commit by ref.
	obj, err := backend.GetObject(ctx, "master")
	require.NoError(t, err)
	require.Equal(t, mustDecodeOID(t, "3580f4105887559aa530eb2b1744f7cad676578a"), obj.ID)
	require.Equal(t, gitdomain.ObjectTypeCommit, obj.Type)

	// Tag.
	obj, err = backend.GetObject(ctx, "testbase")
	require.NoError(t, err)
	require.Equal(t, mustDecodeOID(t, "548fa239e1ac249b9ccfaad00f0fba56461442d8"), obj.ID)
	require.Equal(t, gitdomain.ObjectTypeTag, obj.Type)

	// Tree.
	obj, err = backend.GetObject(ctx, "88e98d1e8b909b8935c06d5a6cea5eb835c433eb")
	require.NoError(t, err)
	require.Equal(t, mustDecodeOID(t, "88e98d1e8b909b8935c06d5a6cea5eb835c433eb"), obj.ID)
	require.Equal(t, gitdomain.ObjectTypeTree, obj.Type)

	// Tree.
	obj, err = backend.GetObject(ctx, "master^{tree}")
	require.NoError(t, err)
	require.Equal(t, mustDecodeOID(t, "88e98d1e8b909b8935c06d5a6cea5eb835c433eb"), obj.ID)
	require.Equal(t, gitdomain.ObjectTypeTree, obj.Type)

	// Blob.
	obj, err = backend.GetObject(ctx, "a29bdeb434d874c9b1d8969c40c42161b03fafdc")
	require.NoError(t, err)
	require.Equal(t, mustDecodeOID(t, "a29bdeb434d874c9b1d8969c40c42161b03fafdc"), obj.ID)
	require.Equal(t, gitdomain.ObjectTypeBlob, obj.Type)

	// Unknown revision.
	_, err = backend.GetObject(ctx, "master2")
	require.Error(t, err)
	require.True(t, errors.HasType[*gitdomain.RevisionNotFoundError](err))

	// Unknown commit.
	_, err = backend.GetObject(ctx, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	require.Error(t, err)
	require.True(t, errors.HasType[*gitdomain.RevisionNotFoundError](err))

	// Invalid commit sha (invalid hex format).
	_, err = backend.GetObject(ctx, "notacommitsha")
	require.Error(t, err)
	require.True(t, errors.HasType[*gitdomain.RevisionNotFoundError](err))

	t.Run("HEAD in empty repo", func(t *testing.T) {
		backend := BackendWithRepoCommands(t)

		_, err := backend.GetObject(ctx, "HEAD")
		require.Error(t, err)
		require.True(t, errors.HasType[*gitdomain.RevisionNotFoundError](err))
	})
}

func mustDecodeOID(t *testing.T, s string) gitdomain.OID {
	t.Helper()

	oid, err := gitdomain.NewOID(api.CommitID(s))
	require.NoError(t, err)
	return oid
}
