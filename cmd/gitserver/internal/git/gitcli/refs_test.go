package gitcli

import (
	"context"
	"io"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestGitCLIBackend_ListRefs(t *testing.T) {
	// Prepare repo state:
	backend := BackendWithRepoCommands(t,
		"echo 'hello\nworld\nfrom\nblame\n' > foo.txt",
		"git add foo.txt",
		"git commit -m foo --author='Foo Author <foo@sourcegraph.com>'",
		// Add an annotated tag.
		"git tag -a foo-tag -m foo-tag",
		// Add a lightweight tag.
		"git tag light-tag",
		// Add a second commit on a different branch.
		"git checkout -b foo",
		"echo 'hello\nworld\nfrom\nthe best blame\n' > foo.txt",
		"git add foo.txt",
		"git commit -m bar --author='Bar Author <bar@sourcegraph.com>'",
		"git checkout master",
		"mkdir -p .git/refs/pull/100",
		"echo $(git rev-parse HEAD) > .git/refs/pull/100/head",
	)

	ctx := context.Background()

	commit, err := backend.RevParseHead(ctx)
	require.NoError(t, err)

	// Verify that the for-each-ref output is correct and that the iterator correctly
	// terminates.
	t.Run("stream refs", func(t *testing.T) {
		it, err := backend.ListRefs(ctx, git.ListRefsOpts{})
		require.NoError(t, err)

		ref, err := it.Next()
		require.NoError(t, err)

		// HEAD comes first.
		assert.Equal(t, &gitdomain.Ref{
			Name:        "refs/heads/master",
			ShortName:   "master",
			CommitID:    commit,
			RefOID:      commit,
			IsHead:      true,
			Type:        gitdomain.RefTypeBranch,
			CreatedDate: ref.CreatedDate,
		}, ref)

		ref, err = it.Next()
		require.NoError(t, err)

		assert.Equal(t, &gitdomain.Ref{
			Name:      "refs/tags/light-tag",
			ShortName: "light-tag",
			CommitID:  commit,
			// for lightweight tags, the RefOID is the same as the CommitID.
			RefOID:      commit,
			IsHead:      false,
			Type:        gitdomain.RefTypeTag,
			CreatedDate: ref.CreatedDate,
		}, ref)

		ref, err = it.Next()
		require.NoError(t, err)

		assert.Equal(t, &gitdomain.Ref{
			Name:      "refs/tags/foo-tag",
			ShortName: "foo-tag",
			CommitID:  commit,
			// note that this is NOT the OID of the commit pointed to by the tag, but the one of the tag itself.
			RefOID:      "957e5bad2c7c68722287ef5c298bfe9e09eb8b3f",
			IsHead:      false,
			Type:        gitdomain.RefTypeTag,
			CreatedDate: ref.CreatedDate,
		}, ref)

		ref, err = it.Next()
		require.NoError(t, err)

		assert.Equal(t, &gitdomain.Ref{
			Name:        "refs/pull/100/head",
			ShortName:   "pull/100/head",
			CommitID:    commit,
			RefOID:      commit,
			IsHead:      false,
			Type:        gitdomain.RefTypeBranch,
			CreatedDate: ref.CreatedDate,
		}, ref)

		ref, err = it.Next()
		require.NoError(t, err)

		assert.Equal(t, &gitdomain.Ref{
			Name:        "refs/heads/foo",
			ShortName:   "foo",
			CommitID:    "53e63d6dd6e61a58369bbc637b0ead2ee58d993c",
			RefOID:      "53e63d6dd6e61a58369bbc637b0ead2ee58d993c",
			IsHead:      false,
			Type:        gitdomain.RefTypeBranch,
			CreatedDate: ref.CreatedDate,
		}, ref)

		_, err = it.Next()
		require.Equal(t, io.EOF, err)

		require.NoError(t, it.Close())
	})

	t.Run("heads and tags", func(t *testing.T) {
		it, err := backend.ListRefs(ctx, git.ListRefsOpts{HeadsOnly: true, TagsOnly: true})
		require.NoError(t, err)

		ref, err := it.Next()
		require.NoError(t, err)

		// HEAD comes first.
		assert.Equal(t, &gitdomain.Ref{
			Name:        "refs/heads/master",
			ShortName:   "master",
			CommitID:    commit,
			RefOID:      commit,
			IsHead:      true,
			Type:        gitdomain.RefTypeBranch,
			CreatedDate: ref.CreatedDate,
		}, ref)

		ref, err = it.Next()
		require.NoError(t, err)

		assert.Equal(t, &gitdomain.Ref{
			Name:      "refs/tags/light-tag",
			ShortName: "light-tag",
			CommitID:  commit,
			// for lightweight tags, the RefOID is the same as the CommitID.
			RefOID:      commit,
			IsHead:      false,
			Type:        gitdomain.RefTypeTag,
			CreatedDate: ref.CreatedDate,
		}, ref)

		ref, err = it.Next()
		require.NoError(t, err)

		assert.Equal(t, &gitdomain.Ref{
			Name:      "refs/tags/foo-tag",
			ShortName: "foo-tag",
			CommitID:  commit,
			// note that this is NOT the OID of the commit pointed to by the tag, but the one of the tag itself.
			RefOID:      "957e5bad2c7c68722287ef5c298bfe9e09eb8b3f",
			IsHead:      false,
			Type:        gitdomain.RefTypeTag,
			CreatedDate: ref.CreatedDate,
		}, ref)

		ref, err = it.Next()
		require.NoError(t, err)

		assert.Equal(t, &gitdomain.Ref{
			Name:        "refs/heads/foo",
			ShortName:   "foo",
			CommitID:    "53e63d6dd6e61a58369bbc637b0ead2ee58d993c",
			RefOID:      "53e63d6dd6e61a58369bbc637b0ead2ee58d993c",
			IsHead:      false,
			Type:        gitdomain.RefTypeBranch,
			CreatedDate: ref.CreatedDate,
		}, ref)

		_, err = it.Next()
		require.Equal(t, io.EOF, err)

		require.NoError(t, it.Close())
	})

	t.Run("tags only", func(t *testing.T) {
		it, err := backend.ListRefs(ctx, git.ListRefsOpts{TagsOnly: true})
		require.NoError(t, err)

		ref, err := it.Next()
		require.NoError(t, err)

		assert.Equal(t, &gitdomain.Ref{
			Name:      "refs/tags/light-tag",
			ShortName: "light-tag",
			CommitID:  commit,
			// for lightweight tags, the RefOID is the same as the CommitID.
			RefOID:      commit,
			IsHead:      false,
			Type:        gitdomain.RefTypeTag,
			CreatedDate: ref.CreatedDate,
		}, ref)

		ref, err = it.Next()
		require.NoError(t, err)

		assert.Equal(t, &gitdomain.Ref{
			Name:      "refs/tags/foo-tag",
			ShortName: "foo-tag",
			CommitID:  commit,
			// note that this is NOT the OID of the commit pointed to by the tag, but the one of the tag itself.
			RefOID:      "957e5bad2c7c68722287ef5c298bfe9e09eb8b3f",
			IsHead:      false,
			Type:        gitdomain.RefTypeTag,
			CreatedDate: ref.CreatedDate,
		}, ref)

		_, err = it.Next()
		require.Equal(t, io.EOF, err)

		require.NoError(t, it.Close())
	})

	t.Run("heads only", func(t *testing.T) {
		it, err := backend.ListRefs(ctx, git.ListRefsOpts{HeadsOnly: true})
		require.NoError(t, err)

		ref, err := it.Next()
		require.NoError(t, err)

		// HEAD comes first.
		assert.Equal(t, &gitdomain.Ref{
			Name:        "refs/heads/master",
			ShortName:   "master",
			CommitID:    commit,
			RefOID:      commit,
			IsHead:      true,
			Type:        gitdomain.RefTypeBranch,
			CreatedDate: ref.CreatedDate,
		}, ref)

		ref, err = it.Next()
		require.NoError(t, err)

		assert.Equal(t, &gitdomain.Ref{
			Name:        "refs/heads/foo",
			ShortName:   "foo",
			CommitID:    "53e63d6dd6e61a58369bbc637b0ead2ee58d993c",
			RefOID:      "53e63d6dd6e61a58369bbc637b0ead2ee58d993c",
			IsHead:      false,
			Type:        gitdomain.RefTypeBranch,
			CreatedDate: ref.CreatedDate,
		}, ref)

		_, err = it.Next()
		require.Equal(t, io.EOF, err)

		require.NoError(t, it.Close())
	})

	t.Run("points at", func(t *testing.T) {
		it, err := backend.ListRefs(ctx, git.ListRefsOpts{PointsAtCommit: []api.CommitID{commit}})
		require.NoError(t, err)

		ref, err := it.Next()
		require.NoError(t, err)

		// HEAD comes first.
		assert.Equal(t, &gitdomain.Ref{
			Name:        "refs/heads/master",
			ShortName:   "master",
			CommitID:    commit,
			RefOID:      commit,
			IsHead:      true,
			Type:        gitdomain.RefTypeBranch,
			CreatedDate: ref.CreatedDate,
		}, ref)

		ref, err = it.Next()
		require.NoError(t, err)

		assert.Equal(t, &gitdomain.Ref{
			Name:      "refs/tags/light-tag",
			ShortName: "light-tag",
			CommitID:  commit,
			// for lightweight tags, the RefOID is the same as the CommitID.
			RefOID:      commit,
			IsHead:      false,
			Type:        gitdomain.RefTypeTag,
			CreatedDate: ref.CreatedDate,
		}, ref)

		ref, err = it.Next()
		require.NoError(t, err)

		assert.Equal(t, &gitdomain.Ref{
			Name:      "refs/tags/foo-tag",
			ShortName: "foo-tag",
			CommitID:  commit,
			// note that this is NOT the OID of the commit pointed to by the tag, but the one of the tag itself.
			RefOID:      "957e5bad2c7c68722287ef5c298bfe9e09eb8b3f",
			IsHead:      false,
			Type:        gitdomain.RefTypeTag,
			CreatedDate: ref.CreatedDate,
		}, ref)

		ref, err = it.Next()
		require.NoError(t, err)

		assert.Equal(t, &gitdomain.Ref{
			Name:        "refs/pull/100/head",
			ShortName:   "pull/100/head",
			CommitID:    commit,
			RefOID:      commit,
			IsHead:      false,
			Type:        gitdomain.RefTypeBranch,
			CreatedDate: ref.CreatedDate,
		}, ref)

		_, err = it.Next()
		require.Equal(t, io.EOF, err)

		require.NoError(t, it.Close())
	})

	t.Run("contains", func(t *testing.T) {
		it, err := backend.ListRefs(ctx, git.ListRefsOpts{Contains: []api.CommitID{commit}})
		require.NoError(t, err)

		ref, err := it.Next()
		require.NoError(t, err)

		// HEAD comes first.
		assert.Equal(t, &gitdomain.Ref{
			Name:        "refs/heads/master",
			ShortName:   "master",
			CommitID:    commit,
			RefOID:      commit,
			IsHead:      true,
			Type:        gitdomain.RefTypeBranch,
			CreatedDate: ref.CreatedDate,
		}, ref)

		ref, err = it.Next()
		require.NoError(t, err)

		assert.Equal(t, &gitdomain.Ref{
			Name:      "refs/tags/light-tag",
			ShortName: "light-tag",
			CommitID:  commit,
			// for lightweight tags, the RefOID is the same as the CommitID.
			RefOID:      commit,
			IsHead:      false,
			Type:        gitdomain.RefTypeTag,
			CreatedDate: ref.CreatedDate,
		}, ref)

		ref, err = it.Next()
		require.NoError(t, err)

		assert.Equal(t, &gitdomain.Ref{
			Name:      "refs/tags/foo-tag",
			ShortName: "foo-tag",
			CommitID:  commit,
			// note that this is NOT the OID of the commit pointed to by the tag, but the one of the tag itself.
			RefOID:      "957e5bad2c7c68722287ef5c298bfe9e09eb8b3f",
			IsHead:      false,
			Type:        gitdomain.RefTypeTag,
			CreatedDate: ref.CreatedDate,
		}, ref)

		ref, err = it.Next()
		require.NoError(t, err)

		assert.Equal(t, &gitdomain.Ref{
			Name:        "refs/pull/100/head",
			ShortName:   "pull/100/head",
			CommitID:    commit,
			RefOID:      commit,
			IsHead:      false,
			Type:        gitdomain.RefTypeBranch,
			CreatedDate: ref.CreatedDate,
		}, ref)

		ref, err = it.Next()
		require.NoError(t, err)

		assert.Equal(t, &gitdomain.Ref{
			Name:        "refs/heads/foo",
			ShortName:   "foo",
			CommitID:    "53e63d6dd6e61a58369bbc637b0ead2ee58d993c",
			RefOID:      "53e63d6dd6e61a58369bbc637b0ead2ee58d993c",
			IsHead:      false,
			Type:        gitdomain.RefTypeBranch,
			CreatedDate: ref.CreatedDate,
		}, ref)

		_, err = it.Next()
		require.Equal(t, io.EOF, err)

		require.NoError(t, it.Close())
	})

	// Verify that if the context is canceled, the iterator returns an error.
	t.Run("context cancelation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		t.Cleanup(cancel)

		it, err := backend.ListRefs(ctx, git.ListRefsOpts{})
		require.NoError(t, err)

		cancel()

		_, err = it.Next()
		require.Error(t, err)
		require.True(t, errors.Is(err, context.Canceled), "unexpected error: %v", err)

		require.True(t, errors.Is(it.Close(), context.Canceled), "unexpected error: %v", err)
	})

	// For now, we don't want to error for this case.
	t.Run("points-at target not found", func(t *testing.T) {
		// Ambiguous ref, could be commit, could be a ref.
		_, err := backend.ListRefs(ctx, git.ListRefsOpts{PointsAtCommit: []api.CommitID{api.CommitID("deadbeef")}})
		require.NoError(t, err)

		// Definitely a commit (yes, those can yield different errors from git).
		_, err = backend.ListRefs(ctx, git.ListRefsOpts{PointsAtCommit: []api.CommitID{api.CommitID("e3889dff4263a2273459471739aafabc10269885")}})
		require.NoError(t, err)
	})
}

func TestGitCLIBackend_emptyrepo(t *testing.T) {
	// Prepare repo state:
	backend := BackendWithRepoCommands(t)

	ctx := context.Background()

	it, err := backend.ListRefs(ctx, git.ListRefsOpts{})
	require.NoError(t, err)

	_, err = it.Next()
	require.Equal(t, io.EOF, err)

	require.NoError(t, it.Close())
}

func TestGitCLIBackend_ListRefs_GoroutineLeak(t *testing.T) {
	ctx := context.Background()

	// Prepare repo state:
	backend := BackendWithRepoCommands(t,
		"echo abcd > file1",
		"git add file1",
		"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",
		"git tag -a tag1 -m tag1",
	)

	routinesBefore := runtime.NumGoroutine()

	it, err := backend.ListRefs(ctx, git.ListRefsOpts{})
	require.NoError(t, err)

	// Read one entry, so one more would need to be read.
	ref, err := it.Next()
	require.NoError(t, err)
	require.Equal(t, "refs/heads/master", ref.Name)

	// Don't complete reading all the output, instead, bail and close the reader.
	require.NoError(t, it.Close())

	time.Sleep(time.Millisecond)

	// Expect no leaked routines.
	routinesAfter := runtime.NumGoroutine()
	require.Equal(t, routinesBefore, routinesAfter)
}

func TestBuildListRefsArgs(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		args := buildListRefsArgs(git.ListRefsOpts{})
		require.Equal(t,
			[]Argument{FlagArgument{"--sort"}, FlagArgument{"-refname"}, FlagArgument{"--sort"}, FlagArgument{"-creatordate"}, FlagArgument{"--sort"}, FlagArgument{"-HEAD"}, FlagArgument{"--format"}, FlagArgument{"%(objecttype)%00%(HEAD)%00%(refname)%00%(refname:short)%00%(objectname)%00%(*objectname)%00%(creatordate:unix)"}},
			args,
		)
	})

	t.Run("heads only", func(t *testing.T) {
		args := buildListRefsArgs(git.ListRefsOpts{HeadsOnly: true})
		require.Equal(t,
			[]Argument{FlagArgument{"--sort"}, FlagArgument{"-refname"}, FlagArgument{"--sort"}, FlagArgument{"-creatordate"}, FlagArgument{"--sort"}, FlagArgument{"-HEAD"}, FlagArgument{"--format"}, FlagArgument{"%(objecttype)%00%(HEAD)%00%(refname)%00%(refname:short)%00%(objectname)%00%(*objectname)%00%(creatordate:unix)"}, FlagArgument{"--"}, FlagArgument{"refs/heads/"}},
			args,
		)
	})

	t.Run("tags only", func(t *testing.T) {
		args := buildListRefsArgs(git.ListRefsOpts{TagsOnly: true})
		require.Equal(t,
			[]Argument{FlagArgument{"--sort"}, FlagArgument{"-refname"}, FlagArgument{"--sort"}, FlagArgument{"-creatordate"}, FlagArgument{"--sort"}, FlagArgument{"-HEAD"}, FlagArgument{"--format"}, FlagArgument{"%(objecttype)%00%(HEAD)%00%(refname)%00%(refname:short)%00%(objectname)%00%(*objectname)%00%(creatordate:unix)"}, FlagArgument{"--"}, FlagArgument{"refs/tags/"}},
			args,
		)
	})

	t.Run("heads and tags only", func(t *testing.T) {
		args := buildListRefsArgs(git.ListRefsOpts{HeadsOnly: true, TagsOnly: true})
		require.Equal(t,
			[]Argument{FlagArgument{"--sort"}, FlagArgument{"-refname"}, FlagArgument{"--sort"}, FlagArgument{"-creatordate"}, FlagArgument{"--sort"}, FlagArgument{"-HEAD"}, FlagArgument{"--format"}, FlagArgument{"%(objecttype)%00%(HEAD)%00%(refname)%00%(refname:short)%00%(objectname)%00%(*objectname)%00%(creatordate:unix)"}, FlagArgument{"--"}, FlagArgument{"refs/heads/"}, FlagArgument{"refs/tags/"}},
			args,
		)
	})

	t.Run("points at commit", func(t *testing.T) {
		commit := api.CommitID("f00ba4")
		args := buildListRefsArgs(git.ListRefsOpts{PointsAtCommit: []api.CommitID{commit}})
		require.Equal(t,
			[]Argument{FlagArgument{"--sort"}, FlagArgument{"-refname"}, FlagArgument{"--sort"}, FlagArgument{"-creatordate"}, FlagArgument{"--sort"}, FlagArgument{"-HEAD"}, FlagArgument{"--format"}, FlagArgument{"%(objecttype)%00%(HEAD)%00%(refname)%00%(refname:short)%00%(objectname)%00%(*objectname)%00%(creatordate:unix)"}, SpecSafeValueArgument{"--points-at=f00ba4"}},
			args,
		)
	})

	t.Run("contains commit", func(t *testing.T) {
		commit := api.CommitID("f00ba4")
		args := buildListRefsArgs(git.ListRefsOpts{Contains: []api.CommitID{commit}})
		require.Equal(t,
			[]Argument{FlagArgument{"--sort"}, FlagArgument{"-refname"}, FlagArgument{"--sort"}, FlagArgument{"-creatordate"}, FlagArgument{"--sort"}, FlagArgument{"-HEAD"}, FlagArgument{"--format"}, FlagArgument{"%(objecttype)%00%(HEAD)%00%(refname)%00%(refname:short)%00%(objectname)%00%(*objectname)%00%(creatordate:unix)"}, SpecSafeValueArgument{"--contains=f00ba4"}},
			args,
		)
	})
}

func TestGitCLIBackend_RefHash(t *testing.T) {
	ctx := context.Background()
	rcf := wrexec.NewNoOpRecordingCommandFactory()

	// Prepare repo state:

	dir := RepoWithCommands(t,
		"echo line1 > f",
		"git add f",
		`GIT_COMMITTER_DATE="2015-01-01 00:00 Z" git commit -m foo --author='Foo Author <foo@sourcegraph.com>'`,
		"git checkout -b branch",
		"echo line1 > f2",
		"git add f2",
		`GIT_COMMITTER_DATE="2015-01-01 00:00 Z" git commit -m foo --author='Foo Author <foo@sourcegraph.com>'`,
	)
	backend := NewBackend(logtest.Scoped(t), rcf, dir, api.RepoName(t.Name()))

	first, err := backend.RefHash(ctx)
	require.NoError(t, err)

	second, err := backend.RefHash(ctx)
	require.NoError(t, err)

	// The hash should be stable across runs.
	require.Equal(t, first, second)

	// The hash should also be stable for a reclone of the same repo:
	dir = RepoWithCommands(t,
		"echo line1 > f",
		"git add f",
		`GIT_COMMITTER_DATE="2015-01-01 00:00 Z" git commit -m foo --author='Foo Author <foo@sourcegraph.com>'`,
		"git checkout -b branch",
		"echo line1 > f2",
		"git add f2",
		`GIT_COMMITTER_DATE="2015-01-01 00:00 Z" git commit -m foo --author='Foo Author <foo@sourcegraph.com>'`,
	)
	backend = NewBackend(logtest.Scoped(t), rcf, dir, api.RepoName(t.Name()))

	third, err := backend.RefHash(ctx)
	require.NoError(t, err)
	require.Equal(t, first, third)
}
