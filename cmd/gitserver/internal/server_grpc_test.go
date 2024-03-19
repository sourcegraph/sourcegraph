package internal

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	mockassert "github.com/derision-test/go-mockgen/v2/testutil/assert"
	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	v1 "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	internalgrpc "github.com/sourcegraph/sourcegraph/internal/grpc"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func TestGRPCServer_Blame(t *testing.T) {
	mockSS := gitserver.NewMockGitserverService_BlameServer()
	// Add an actor to the context.
	a := actor.FromUser(1)
	mockSS.ContextFunc.SetDefaultReturn(actor.WithActor(context.Background(), a))
	t.Run("argument validation", func(t *testing.T) {
		gs := &grpcServer{}
		err := gs.Blame(&v1.BlameRequest{RepoName: "", Path: "thepath"}, mockSS)
		require.ErrorContains(t, err, "repo must be specified")
		assertGRPCStatusCode(t, err, codes.InvalidArgument)
		err = gs.Blame(&v1.BlameRequest{RepoName: "therepo", Commit: ""}, mockSS)
		require.ErrorContains(t, err, "commit must be specified")
		assertGRPCStatusCode(t, err, codes.InvalidArgument)
		err = gs.Blame(&v1.BlameRequest{RepoName: "therepo", Commit: "deadbeef", Path: ""}, mockSS)
		require.ErrorContains(t, err, "path must be specified")
		assertGRPCStatusCode(t, err, codes.InvalidArgument)
	})
	t.Run("checks for uncloned repo", func(t *testing.T) {
		svc := NewMockService()
		svc.MaybeStartCloneFunc.SetDefaultReturn(&protocol.NotFoundPayload{CloneInProgress: true, CloneProgress: "cloning"}, false)
		gs := &grpcServer{svc: svc}
		err := gs.Blame(&v1.BlameRequest{RepoName: "therepo", Commit: "deadbeef", Path: "thepath"}, mockSS)
		require.Error(t, err)
		assertGRPCStatusCode(t, err, codes.NotFound)
		assertHasGRPCErrorDetailOfType(t, err, &proto.RepoNotFoundPayload{})
		require.Contains(t, err.Error(), "repo not found")
		mockassert.Called(t, svc.MaybeStartCloneFunc)
	})
	t.Run("checks for subrepo perms access to given path", func(t *testing.T) {
		srp := authz.NewMockSubRepoPermissionChecker()
		svc := NewMockService()
		// Repo is cloned, proceed!
		svc.MaybeStartCloneFunc.SetDefaultReturn(nil, true)
		gs := &grpcServer{
			subRepoChecker: srp,
			svc:            svc,
			getBackendFunc: func(common.GitDir, api.RepoName) git.GitBackend {
				b := git.NewMockGitBackend()
				hr := git.NewMockBlameHunkReader()
				hr.ReadFunc.SetDefaultReturn(nil, io.EOF)
				b.BlameFunc.SetDefaultReturn(hr, nil)
				return b
			},
		}

		t.Run("subrepo perms are not enabled", func(t *testing.T) {
			srp.EnabledFunc.SetDefaultReturn(false)
			err := gs.Blame(&v1.BlameRequest{RepoName: "therepo", Commit: "deadbeef", Path: "thepath"}, mockSS)
			assert.NoError(t, err)
			mockassert.Called(t, srp.EnabledFunc)
		})

		t.Run("subrepo perms are enabled, permission granted", func(t *testing.T) {
			srp.EnabledFunc.SetDefaultReturn(true)
			srp.PermissionsFunc.SetDefaultReturn(authz.Read, nil)
			err := gs.Blame(&v1.BlameRequest{RepoName: "therepo", Commit: "deadbeef", Path: "thepath"}, mockSS)
			assert.NoError(t, err)
			mockassert.Called(t, srp.EnabledFunc)
			mockassert.Called(t, srp.PermissionsFunc)
		})

		t.Run("subrepo perms are enabled, permission denied", func(t *testing.T) {
			srp.EnabledFunc.SetDefaultReturn(true)
			srp.PermissionsFunc.SetDefaultReturn(authz.None, nil)
			err := gs.Blame(&v1.BlameRequest{RepoName: "therepo", Commit: "deadbeef", Path: "thepath"}, mockSS)
			require.Error(t, err)
			mockassert.Called(t, srp.EnabledFunc)
			mockassert.Called(t, srp.PermissionsFunc)
			assertGRPCStatusCode(t, err, codes.PermissionDenied)
		})
	})
	t.Run("e2e", func(t *testing.T) {
		srp := authz.NewMockSubRepoPermissionChecker()
		// Skip subrepo perms checks.
		srp.EnabledFunc.SetDefaultReturn(false)
		svc := NewMockService()
		// Repo is cloned, proceed!
		svc.MaybeStartCloneFunc.SetDefaultReturn(nil, true)
		b := git.NewMockGitBackend()
		hr := git.NewMockBlameHunkReader()
		hr.ReadFunc.PushReturn(&gitdomain.Hunk{CommitID: "deadbeef"}, nil)
		hr.ReadFunc.PushReturn(nil, io.EOF)
		b.BlameFunc.PushReturn(hr, nil)
		gs := &grpcServer{
			subRepoChecker: srp,
			svc:            svc,
			getBackendFunc: func(common.GitDir, api.RepoName) git.GitBackend {
				return b
			},
		}

		cli := spawnServer(t, gs)
		r, err := cli.Blame(context.Background(), &v1.BlameRequest{
			RepoName: "therepo",
			Commit:   "deadbeef",
			Path:     "thepath",
		})
		require.NoError(t, err)
		for {
			msg, err := r.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}
			if diff := cmp.Diff(&proto.BlameResponse{
				Hunk: &proto.BlameHunk{
					Commit: "deadbeef",
					Author: &v1.BlameAuthor{
						Date: timestamppb.New(time.Time{}),
					},
				},
			}, msg, cmpopts.IgnoreUnexported(proto.BlameResponse{}, proto.BlameHunk{}, proto.BlameAuthor{}, timestamppb.Timestamp{})); diff != "" {
				t.Fatalf("unexpected response (-want +got):\n%s", diff)
			}
		}

		{
			b.BlameFunc.PushReturn(nil, &os.PathError{Op: "open", Path: "thepath", Err: os.ErrNotExist})
			r, err = cli.Blame(context.Background(), &v1.BlameRequest{
				RepoName: "therepo",
				Commit:   "deadbeef",
				Path:     "thepath",
			})
			require.NoError(t, err)

			_, err := r.Recv()
			assertGRPCStatusCode(t, err, codes.NotFound)
			assertHasGRPCErrorDetailOfType(t, err, &proto.FileNotFoundPayload{})
		}

		{
			b.BlameFunc.PushReturn(nil, &gitdomain.RevisionNotFoundError{Repo: "therepo", Spec: "deadbeef"})
			r, err = cli.Blame(context.Background(), &v1.BlameRequest{
				RepoName: "therepo",
				Commit:   "deadbeef",
				Path:     "thepath",
			})
			require.NoError(t, err)

			_, err := r.Recv()
			assertGRPCStatusCode(t, err, codes.NotFound)
			assertHasGRPCErrorDetailOfType(t, err, &proto.RevisionNotFoundPayload{})
		}
	})
}

func TestGRPCServer_DefaultBranch(t *testing.T) {
	ctx := context.Background()
	t.Run("argument validation", func(t *testing.T) {
		gs := &grpcServer{}
		_, err := gs.DefaultBranch(ctx, &v1.DefaultBranchRequest{RepoName: ""})
		require.ErrorContains(t, err, "repo must be specified")
		assertGRPCStatusCode(t, err, codes.InvalidArgument)
	})
	t.Run("checks for uncloned repo", func(t *testing.T) {
		svc := NewMockService()
		svc.MaybeStartCloneFunc.SetDefaultReturn(&protocol.NotFoundPayload{CloneInProgress: true, CloneProgress: "cloning"}, false)
		gs := &grpcServer{svc: svc}
		_, err := gs.DefaultBranch(ctx, &v1.DefaultBranchRequest{RepoName: "therepo"})
		require.Error(t, err)
		assertGRPCStatusCode(t, err, codes.NotFound)
		assertHasGRPCErrorDetailOfType(t, err, &proto.RepoNotFoundPayload{})
		require.Contains(t, err.Error(), "repo not found")
		mockassert.Called(t, svc.MaybeStartCloneFunc)
	})
	t.Run("e2e", func(t *testing.T) {
		svc := NewMockService()
		// Repo is cloned, proceed!
		svc.MaybeStartCloneFunc.SetDefaultReturn(nil, true)
		b := git.NewMockGitBackend()
		b.SymbolicRefHeadFunc.SetDefaultReturn("refs/heads/main", nil)
		b.RevParseHeadFunc.SetDefaultReturn("deadbeef", nil)
		gs := &grpcServer{
			svc: svc,
			getBackendFunc: func(common.GitDir, api.RepoName) git.GitBackend {
				return b
			},
		}

		cli := spawnServer(t, gs)
		res, err := cli.DefaultBranch(ctx, &v1.DefaultBranchRequest{
			RepoName: "therepo",
		})
		require.NoError(t, err)
		if diff := cmp.Diff(&proto.DefaultBranchResponse{
			RefName: "refs/heads/main",
			Commit:  "deadbeef",
		}, res, cmpopts.IgnoreUnexported(proto.DefaultBranchResponse{})); diff != "" {
			t.Fatalf("unexpected response (-want +got):\n%s", diff)
		}

		// Check that RevNotFoundErrors are mapped correctly:
		b.RevParseHeadFunc.SetDefaultReturn("", &gitdomain.RevisionNotFoundError{Repo: "therepo", Spec: "HEAD"})
		_, err = cli.DefaultBranch(ctx, &v1.DefaultBranchRequest{
			RepoName: "therepo",
		})
		require.Error(t, err)
		assertGRPCStatusCode(t, err, codes.NotFound)
		assertHasGRPCErrorDetailOfType(t, err, &proto.RevisionNotFoundPayload{})
	})
}

func TestGRPCServer_MergeBase(t *testing.T) {
	ctx := context.Background()
	t.Run("argument validation", func(t *testing.T) {
		gs := &grpcServer{}
		_, err := gs.MergeBase(ctx, &v1.MergeBaseRequest{RepoName: ""})
		require.ErrorContains(t, err, "repo must be specified")
		assertGRPCStatusCode(t, err, codes.InvalidArgument)
		_, err = gs.MergeBase(ctx, &v1.MergeBaseRequest{RepoName: "therepo", Base: []byte{}})
		require.ErrorContains(t, err, "base must be specified")
		assertGRPCStatusCode(t, err, codes.InvalidArgument)
		_, err = gs.MergeBase(ctx, &v1.MergeBaseRequest{RepoName: "therepo", Base: []byte("master")})
		require.ErrorContains(t, err, "head must be specified")
		assertGRPCStatusCode(t, err, codes.InvalidArgument)
	})
	t.Run("checks for uncloned repo", func(t *testing.T) {
		svc := NewMockService()
		svc.MaybeStartCloneFunc.SetDefaultReturn(&protocol.NotFoundPayload{CloneInProgress: true, CloneProgress: "cloning"}, false)
		gs := &grpcServer{svc: svc}
		_, err := gs.MergeBase(ctx, &v1.MergeBaseRequest{RepoName: "therepo", Base: []byte("master"), Head: []byte("b2")})
		require.Error(t, err)
		assertGRPCStatusCode(t, err, codes.NotFound)
		assertHasGRPCErrorDetailOfType(t, err, &proto.RepoNotFoundPayload{})
		require.Contains(t, err.Error(), "repo not found")
		mockassert.Called(t, svc.MaybeStartCloneFunc)
	})
	t.Run("revision not found", func(t *testing.T) {
		svc := NewMockService()
		svc.MaybeStartCloneFunc.SetDefaultReturn(nil, true)
		gs := &grpcServer{
			svc: svc,
			getBackendFunc: func(common.GitDir, api.RepoName) git.GitBackend {
				b := git.NewMockGitBackend()
				b.MergeBaseFunc.SetDefaultReturn("", &gitdomain.RevisionNotFoundError{Repo: "therepo", Spec: "b2"})
				return b
			},
		}
		_, err := gs.MergeBase(ctx, &v1.MergeBaseRequest{RepoName: "therepo", Base: []byte("master"), Head: []byte("b2")})
		require.Error(t, err)
		assertGRPCStatusCode(t, err, codes.NotFound)
		assertHasGRPCErrorDetailOfType(t, err, &proto.RevisionNotFoundPayload{})
		require.Contains(t, err.Error(), "revision not found")
	})
	t.Run("e2e", func(t *testing.T) {
		svc := NewMockService()
		// Repo is cloned, proceed!
		svc.MaybeStartCloneFunc.SetDefaultReturn(nil, true)
		b := git.NewMockGitBackend()
		b.MergeBaseFunc.SetDefaultReturn("deadbeef", nil)
		gs := &grpcServer{
			svc: svc,
			getBackendFunc: func(common.GitDir, api.RepoName) git.GitBackend {
				return b
			},
		}

		cli := spawnServer(t, gs)
		res, err := cli.MergeBase(ctx, &v1.MergeBaseRequest{
			RepoName: "therepo",
			Base:     []byte("master"),
			Head:     []byte("b2"),
		})
		require.NoError(t, err)
		if diff := cmp.Diff(&proto.MergeBaseResponse{
			MergeBaseCommitSha: "deadbeef",
		}, res, cmpopts.IgnoreUnexported(proto.MergeBaseResponse{})); diff != "" {
			t.Fatalf("unexpected response (-want +got):\n%s", diff)
		}
	})
}

func TestGRPCServer_ReadFile(t *testing.T) {
	mockSS := gitserver.NewMockGitserverService_ReadFileServer()
	// Add an actor to the context.
	a := actor.FromUser(1)
	mockSS.ContextFunc.SetDefaultReturn(actor.WithActor(context.Background(), a))
	t.Run("argument validation", func(t *testing.T) {
		gs := &grpcServer{}
		err := gs.ReadFile(&v1.ReadFileRequest{RepoName: ""}, mockSS)
		require.ErrorContains(t, err, "repo must be specified")
		assertGRPCStatusCode(t, err, codes.InvalidArgument)
		err = gs.ReadFile(&v1.ReadFileRequest{RepoName: "therepo", Path: ""}, mockSS)
		require.ErrorContains(t, err, "path must be specified")
		assertGRPCStatusCode(t, err, codes.InvalidArgument)
		err = gs.ReadFile(&v1.ReadFileRequest{RepoName: "therepo", Path: "thepath", Commit: ""}, mockSS)
		require.ErrorContains(t, err, "commit must be specified")
		assertGRPCStatusCode(t, err, codes.InvalidArgument)
	})
	t.Run("checks for uncloned repo", func(t *testing.T) {
		svc := NewMockService()
		svc.MaybeStartCloneFunc.SetDefaultReturn(&protocol.NotFoundPayload{CloneInProgress: true, CloneProgress: "cloning"}, false)
		gs := &grpcServer{svc: svc}
		err := gs.ReadFile(&v1.ReadFileRequest{RepoName: "therepo", Commit: "deadbeef", Path: "thepath"}, mockSS)
		require.Error(t, err)
		assertGRPCStatusCode(t, err, codes.NotFound)
		assertHasGRPCErrorDetailOfType(t, err, &proto.RepoNotFoundPayload{})
		require.Contains(t, err.Error(), "repo not found")
		mockassert.Called(t, svc.MaybeStartCloneFunc)
	})
	t.Run("checks for subrepo perms access to given path", func(t *testing.T) {
		srp := authz.NewMockSubRepoPermissionChecker()
		svc := NewMockService()
		// Repo is cloned, proceed!
		svc.MaybeStartCloneFunc.SetDefaultReturn(nil, true)
		gs := &grpcServer{
			subRepoChecker: srp,
			svc:            svc,
			getBackendFunc: func(common.GitDir, api.RepoName) git.GitBackend {
				b := git.NewMockGitBackend()
				b.ReadFileFunc.SetDefaultReturn(io.NopCloser(bytes.NewReader([]byte("filecontent"))), nil)
				return b
			},
		}

		t.Run("subrepo perms are not enabled", func(t *testing.T) {
			srp.EnabledFunc.SetDefaultReturn(false)
			err := gs.ReadFile(&v1.ReadFileRequest{RepoName: "therepo", Commit: "deadbeef", Path: "thepath"}, mockSS)
			assert.NoError(t, err)
			mockassert.Called(t, srp.EnabledFunc)
		})

		t.Run("subrepo perms are enabled, permission granted", func(t *testing.T) {
			srp.EnabledFunc.SetDefaultReturn(true)
			srp.PermissionsFunc.SetDefaultReturn(authz.Read, nil)
			err := gs.ReadFile(&v1.ReadFileRequest{RepoName: "therepo", Commit: "deadbeef", Path: "thepath"}, mockSS)
			assert.NoError(t, err)
			mockassert.Called(t, srp.EnabledFunc)
			mockassert.Called(t, srp.PermissionsFunc)
		})

		t.Run("subrepo perms are enabled, permission denied", func(t *testing.T) {
			srp.EnabledFunc.SetDefaultReturn(true)
			srp.PermissionsFunc.SetDefaultReturn(authz.None, nil)
			err := gs.ReadFile(&v1.ReadFileRequest{RepoName: "therepo", Commit: "deadbeef", Path: "thepath"}, mockSS)
			require.Error(t, err)
			mockassert.Called(t, srp.EnabledFunc)
			mockassert.Called(t, srp.PermissionsFunc)
			assertGRPCStatusCode(t, err, codes.PermissionDenied)
		})
	})
	t.Run("e2e", func(t *testing.T) {
		srp := authz.NewMockSubRepoPermissionChecker()
		// Skip subrepo perms checks.
		srp.EnabledFunc.SetDefaultReturn(false)
		svc := NewMockService()
		// Repo is cloned, proceed!
		svc.MaybeStartCloneFunc.SetDefaultReturn(nil, true)
		b := git.NewMockGitBackend()
		b.ReadFileFunc.SetDefaultReturn(io.NopCloser(bytes.NewReader([]byte("filecontent"))), nil)
		gs := &grpcServer{
			subRepoChecker: srp,
			svc:            svc,
			getBackendFunc: func(common.GitDir, api.RepoName) git.GitBackend {
				return b
			},
		}

		cli := spawnServer(t, gs)
		r, err := cli.ReadFile(context.Background(), &v1.ReadFileRequest{
			RepoName: "therepo",
			Commit:   "deadbeef",
			Path:     "thepath",
		})
		require.NoError(t, err)
		for {
			msg, err := r.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}
			if diff := cmp.Diff(&proto.ReadFileResponse{
				Data: []byte("filecontent"),
			}, msg, cmpopts.IgnoreUnexported(proto.ReadFileResponse{})); diff != "" {
				t.Fatalf("unexpected response (-want +got):\n%s", diff)
			}
		}

		b.ReadFileFunc.SetDefaultReturn(nil, os.ErrNotExist)
		cc, err := cli.ReadFile(context.Background(), &v1.ReadFileRequest{
			RepoName: "therepo",
			Commit:   "deadbeef",
			Path:     "thepath",
		})
		require.NoError(t, err)
		_, err = cc.Recv()
		require.Error(t, err)
		assertGRPCStatusCode(t, err, codes.NotFound)
		assertHasGRPCErrorDetailOfType(t, err, &proto.FileNotFoundPayload{})

		b.ReadFileFunc.SetDefaultReturn(nil, &gitdomain.RevisionNotFoundError{})
		cc, err = cli.ReadFile(context.Background(), &v1.ReadFileRequest{
			RepoName: "therepo",
			Commit:   "deadbeef",
			Path:     "thepath",
		})
		require.NoError(t, err)
		_, err = cc.Recv()
		require.Error(t, err)
		assertGRPCStatusCode(t, err, codes.NotFound)
		assertHasGRPCErrorDetailOfType(t, err, &proto.RevisionNotFoundPayload{})
	})
}

func TestGRPCServer_Archive(t *testing.T) {
	mockSS := gitserver.NewMockGitserverService_ArchiveServer()
	// Add an actor to the context.
	a := actor.FromUser(1)
	mockSS.ContextFunc.SetDefaultReturn(actor.WithActor(context.Background(), a))
	t.Run("argument validation", func(t *testing.T) {
		gs := &grpcServer{}
		err := gs.Archive(&v1.ArchiveRequest{Repo: ""}, mockSS)
		require.ErrorContains(t, err, "repo must be specified")
		assertGRPCStatusCode(t, err, codes.InvalidArgument)

		err = gs.Archive(&v1.ArchiveRequest{Repo: "therepo", Format: proto.ArchiveFormat_ARCHIVE_FORMAT_TAR}, mockSS)
		require.ErrorContains(t, err, "treeish must be specified")
		assertGRPCStatusCode(t, err, codes.InvalidArgument)

		err = gs.Archive(&v1.ArchiveRequest{Repo: "therepo", Treeish: "HEAD"}, mockSS)
		require.ErrorContains(t, err, "unknown archive format")
		assertGRPCStatusCode(t, err, codes.InvalidArgument)
	})
	t.Run("checks for uncloned repo", func(t *testing.T) {
		svc := NewMockService()
		svc.MaybeStartCloneFunc.SetDefaultReturn(&protocol.NotFoundPayload{CloneInProgress: true, CloneProgress: "cloning"}, false)
		gs := &grpcServer{svc: svc}
		err := gs.Archive(&v1.ArchiveRequest{Repo: "therepo", Treeish: "HEAD", Format: proto.ArchiveFormat_ARCHIVE_FORMAT_ZIP}, mockSS)
		require.Error(t, err)
		assertGRPCStatusCode(t, err, codes.NotFound)
		assertHasGRPCErrorDetailOfType(t, err, &proto.RepoNotFoundPayload{})
		require.Contains(t, err.Error(), "repo not found")
		mockassert.Called(t, svc.MaybeStartCloneFunc)
	})
	t.Run("checks if sub-repo perms are enabled for repo", func(t *testing.T) {
		srp := authz.NewMockSubRepoPermissionChecker()
		svc := NewMockService()
		// Repo is cloned, proceed!
		svc.MaybeStartCloneFunc.SetDefaultReturn(nil, true)
		gs := &grpcServer{
			subRepoChecker: srp,
			svc:            svc,
			getBackendFunc: func(common.GitDir, api.RepoName) git.GitBackend {
				b := git.NewMockGitBackend()
				b.ArchiveReaderFunc.SetDefaultReturn(io.NopCloser(bytes.NewReader([]byte("filecontent"))), nil)
				return b
			},
		}

		t.Run("subrepo perms are enabled but actor is internal", func(t *testing.T) {
			srp.EnabledForRepoFunc.SetDefaultReturn(true, nil)
			mockSS := gitserver.NewMockGitserverService_ArchiveServer()
			// Add an internal actor to the context.
			mockSS.ContextFunc.SetDefaultReturn(actor.WithInternalActor(context.Background()))
			err := gs.Archive(&v1.ArchiveRequest{Repo: "therepo", Treeish: "HEAD", Format: proto.ArchiveFormat_ARCHIVE_FORMAT_ZIP}, mockSS)
			assert.NoError(t, err)
			mockassert.NotCalled(t, srp.EnabledForRepoFunc)
		})

		t.Run("subrepo perms are not enabled", func(t *testing.T) {
			srp.EnabledForRepoFunc.SetDefaultReturn(false, nil)
			err := gs.Archive(&v1.ArchiveRequest{Repo: "therepo", Treeish: "HEAD", Format: proto.ArchiveFormat_ARCHIVE_FORMAT_ZIP}, mockSS)
			assert.NoError(t, err)
			mockassert.Called(t, srp.EnabledForRepoFunc)
		})

		t.Run("subrepo perms are enabled, returns error", func(t *testing.T) {
			srp.EnabledForRepoFunc.SetDefaultReturn(true, nil)
			err := gs.Archive(&v1.ArchiveRequest{Repo: "therepo", Treeish: "HEAD", Format: proto.ArchiveFormat_ARCHIVE_FORMAT_ZIP}, mockSS)
			assert.Error(t, err)
			assertGRPCStatusCode(t, err, codes.Unimplemented)
			require.Contains(t, err.Error(), "archiveReader invoked for a repo with sub-repo permissions")
			mockassert.Called(t, srp.EnabledForRepoFunc)
		})
	})
	t.Run("e2e", func(t *testing.T) {
		srp := authz.NewMockSubRepoPermissionChecker()
		// Skip subrepo perms checks.
		srp.EnabledForRepoFunc.SetDefaultReturn(false, nil)
		svc := NewMockService()
		// Repo is cloned, proceed!
		svc.MaybeStartCloneFunc.SetDefaultReturn(nil, true)
		b := git.NewMockGitBackend()
		b.ArchiveReaderFunc.SetDefaultReturn(io.NopCloser(bytes.NewReader([]byte("filecontent"))), nil)
		gs := &grpcServer{
			subRepoChecker: srp,
			svc:            svc,
			getBackendFunc: func(common.GitDir, api.RepoName) git.GitBackend {
				return b
			},
		}

		cli := spawnServer(t, gs)
		r, err := cli.Archive(context.Background(), &v1.ArchiveRequest{
			Repo:    "therepo",
			Treeish: "HEAD",
			Format:  proto.ArchiveFormat_ARCHIVE_FORMAT_ZIP,
		})
		require.NoError(t, err)
		for {
			msg, err := r.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}
			if diff := cmp.Diff(&proto.ArchiveResponse{
				Data: []byte("filecontent"),
			}, msg, cmpopts.IgnoreUnexported(proto.ArchiveResponse{})); diff != "" {
				t.Fatalf("unexpected response (-want +got):\n%s", diff)
			}
		}

		// Invalid file path.
		b.ArchiveReaderFunc.SetDefaultReturn(nil, os.ErrNotExist)
		cc, err := cli.Archive(context.Background(), &v1.ArchiveRequest{
			Repo:    "therepo",
			Treeish: "HEAD",
			Format:  proto.ArchiveFormat_ARCHIVE_FORMAT_ZIP,
		})
		require.NoError(t, err)
		_, err = cc.Recv()
		require.Error(t, err)
		assertGRPCStatusCode(t, err, codes.NotFound)
		assertHasGRPCErrorDetailOfType(t, err, &proto.FileNotFoundPayload{})

		// TODO: Do we return this?
		b.ArchiveReaderFunc.SetDefaultReturn(nil, &gitdomain.RevisionNotFoundError{})
		cc, err = cli.Archive(context.Background(), &v1.ArchiveRequest{
			Repo:    "therepo",
			Treeish: "HEAD",
			Format:  proto.ArchiveFormat_ARCHIVE_FORMAT_ZIP,
		})
		require.NoError(t, err)
		_, err = cc.Recv()
		require.Error(t, err)
		assertGRPCStatusCode(t, err, codes.NotFound)
		assertHasGRPCErrorDetailOfType(t, err, &proto.RevisionNotFoundPayload{})
	})
}

func TestGRPCServer_GetCommit(t *testing.T) {
	// Add an actor to the context.
	a := actor.FromUser(1)
	ctx := actor.WithActor(context.Background(), a)
	t.Run("argument validation", func(t *testing.T) {
		gs := &grpcServer{}
		_, err := gs.GetCommit(ctx, &v1.GetCommitRequest{RepoName: ""})
		require.ErrorContains(t, err, "repo must be specified")
		assertGRPCStatusCode(t, err, codes.InvalidArgument)
		_, err = gs.GetCommit(ctx, &v1.GetCommitRequest{RepoName: "therepo", Commit: ""})
		require.ErrorContains(t, err, "commit must be specified")
		assertGRPCStatusCode(t, err, codes.InvalidArgument)
	})
	t.Run("checks for uncloned repo", func(t *testing.T) {
		svc := NewMockService()
		svc.MaybeStartCloneFunc.SetDefaultReturn(&protocol.NotFoundPayload{CloneInProgress: true, CloneProgress: "cloning"}, false)
		gs := &grpcServer{svc: svc}
		_, err := gs.GetCommit(ctx, &v1.GetCommitRequest{RepoName: "therepo", Commit: "deadbeef"})
		require.Error(t, err)
		assertGRPCStatusCode(t, err, codes.NotFound)
		assertHasGRPCErrorDetailOfType(t, err, &proto.RepoNotFoundPayload{})
		require.Contains(t, err.Error(), "repo not found")
		mockassert.Called(t, svc.MaybeStartCloneFunc)
	})
	t.Run("checks for subrepo perms access to commit", func(t *testing.T) {
		srp := authz.NewMockSubRepoPermissionChecker()
		svc := NewMockService()
		// Repo is cloned, proceed!
		svc.MaybeStartCloneFunc.SetDefaultReturn(nil, true)
		b := git.NewMockGitBackend()
		gs := &grpcServer{
			subRepoChecker: srp,
			svc:            svc,
			getBackendFunc: func(common.GitDir, api.RepoName) git.GitBackend {
				return b
			},
		}

		t.Run("subrepo perms are not enabled", func(t *testing.T) {
			srp.EnabledFunc.SetDefaultReturn(false)
			srp.EnabledForRepoFunc.SetDefaultReturn(false, nil)
			b.GetCommitFunc.PushReturn(&git.GitCommitWithFiles{Commit: &gitdomain.Commit{Committer: &gitdomain.Signature{}}}, nil)
			_, err := gs.GetCommit(ctx, &v1.GetCommitRequest{RepoName: "therepo", Commit: "deadbeef"})
			assert.NoError(t, err)
			mockassert.Called(t, srp.EnabledFunc)
		})

		t.Run("subrepo perms are enabled, no file paths", func(t *testing.T) {
			srp.EnabledFunc.SetDefaultReturn(true)
			srp.EnabledForRepoFunc.SetDefaultReturn(true, nil)
			b.GetCommitFunc.PushReturn(&git.GitCommitWithFiles{Commit: &gitdomain.Commit{Committer: &gitdomain.Signature{}}}, nil)
			_, err := gs.GetCommit(ctx, &v1.GetCommitRequest{RepoName: "therepo", Commit: "deadbeef"})
			assert.NoError(t, err)
			mockassert.Called(t, srp.EnabledFunc)
			mockassert.NotCalled(t, srp.PermissionsFunc)
		})

		t.Run("subrepo perms are enabled, permission granted", func(t *testing.T) {
			srp.EnabledFunc.SetDefaultReturn(true)
			srp.EnabledForRepoFunc.SetDefaultReturn(true, nil)
			srp.PermissionsFunc.SetDefaultReturn(authz.Read, nil)
			b.GetCommitFunc.PushReturn(&git.GitCommitWithFiles{Commit: &gitdomain.Commit{Committer: &gitdomain.Signature{}}, ModifiedFiles: []string{"file1"}}, nil)
			_, err := gs.GetCommit(ctx, &v1.GetCommitRequest{RepoName: "therepo", Commit: "deadbeef"})
			assert.NoError(t, err)
			mockassert.Called(t, srp.EnabledFunc)
			mockassert.Called(t, srp.PermissionsFunc)
		})

		t.Run("subrepo perms are enabled, permission denied", func(t *testing.T) {
			srp.EnabledFunc.SetDefaultReturn(true)
			srp.EnabledForRepoFunc.SetDefaultReturn(true, nil)
			srp.PermissionsFunc.SetDefaultReturn(authz.None, nil)
			b.GetCommitFunc.PushReturn(&git.GitCommitWithFiles{Commit: &gitdomain.Commit{Committer: &gitdomain.Signature{}}, ModifiedFiles: []string{"file1"}}, nil)
			_, err := gs.GetCommit(ctx, &v1.GetCommitRequest{RepoName: "therepo", Commit: "deadbeef"})
			require.Error(t, err)
			mockassert.Called(t, srp.EnabledFunc)
			mockassert.Called(t, srp.PermissionsFunc)
			assertGRPCStatusCode(t, err, codes.NotFound)
			assertHasGRPCErrorDetailOfType(t, err, &proto.RevisionNotFoundPayload{})
		})
	})
	t.Run("e2e", func(t *testing.T) {
		srp := authz.NewMockSubRepoPermissionChecker()
		// Skip subrepo perms checks.
		srp.EnabledFunc.SetDefaultReturn(false)
		srp.EnabledForRepoFunc.SetDefaultReturn(false, nil)
		svc := NewMockService()
		// Repo is cloned, proceed!
		svc.MaybeStartCloneFunc.SetDefaultReturn(nil, true)
		b := git.NewMockGitBackend()
		b.GetCommitFunc.PushReturn(&git.GitCommitWithFiles{Commit: &gitdomain.Commit{Committer: &gitdomain.Signature{}}}, nil)
		gs := &grpcServer{
			subRepoChecker: srp,
			svc:            svc,
			getBackendFunc: func(common.GitDir, api.RepoName) git.GitBackend {
				return b
			},
		}

		cli := spawnServer(t, gs)
		_, err := cli.GetCommit(ctx, &v1.GetCommitRequest{
			RepoName: "therepo",
			Commit:   "deadbeef",
		})
		require.NoError(t, err)

		b.GetCommitFunc.PushReturn(nil, &gitdomain.RevisionNotFoundError{Repo: "therepo", Spec: "deadbeef"})
		_, err = cli.GetCommit(ctx, &v1.GetCommitRequest{
			RepoName: "therepo",
			Commit:   "deadbeef",
		})
		require.Error(t, err)
		assertGRPCStatusCode(t, err, codes.NotFound)
		assertHasGRPCErrorDetailOfType(t, err, &proto.RevisionNotFoundPayload{})
	})
}

func TestGRPCServer_ResolveRevision(t *testing.T) {
	ctx := context.Background()
	t.Run("argument validation", func(t *testing.T) {
		gs := &grpcServer{}
		_, err := gs.ResolveRevision(ctx, &v1.ResolveRevisionRequest{RepoName: ""})
		require.ErrorContains(t, err, "repo must be specified")
		assertGRPCStatusCode(t, err, codes.InvalidArgument)
	})
	t.Run("checks for uncloned repo", func(t *testing.T) {
		svc := NewMockService()
		svc.MaybeStartCloneFunc.SetDefaultReturn(&protocol.NotFoundPayload{CloneInProgress: true, CloneProgress: "cloning"}, false)
		gs := &grpcServer{svc: svc}
		_, err := gs.ResolveRevision(ctx, &v1.ResolveRevisionRequest{RepoName: "therepo"})
		require.Error(t, err)
		assertGRPCStatusCode(t, err, codes.NotFound)
		assertHasGRPCErrorDetailOfType(t, err, &proto.RepoNotFoundPayload{})
		require.Contains(t, err.Error(), "repo not found")
		mockassert.Called(t, svc.MaybeStartCloneFunc)
	})
	t.Run("e2e", func(t *testing.T) {
		svc := NewMockService()
		// Repo is cloned, proceed!
		svc.MaybeStartCloneFunc.SetDefaultReturn(nil, true)
		b := git.NewMockGitBackend()
		b.ResolveRevisionFunc.SetDefaultReturn("deadbeef", nil)
		gs := &grpcServer{
			svc: svc,
			getBackendFunc: func(common.GitDir, api.RepoName) git.GitBackend {
				return b
			},
		}

		cli := spawnServer(t, gs)
		res, err := cli.ResolveRevision(ctx, &v1.ResolveRevisionRequest{
			RepoName: "therepo",
		})
		require.NoError(t, err)
		if diff := cmp.Diff(&proto.ResolveRevisionResponse{
			CommitSha: "deadbeef",
		}, res, cmpopts.IgnoreUnexported(proto.ResolveRevisionResponse{})); diff != "" {
			t.Fatalf("unexpected response (-want +got):\n%s", diff)
		}

		// Check that RevNotFoundErrors are mapped correctly:
		b.ResolveRevisionFunc.SetDefaultReturn("", &gitdomain.RevisionNotFoundError{Repo: "therepo", Spec: "HEAD"})
		_, err = cli.ResolveRevision(ctx, &v1.ResolveRevisionRequest{
			RepoName: "therepo",
		})
		require.Error(t, err)
		assertGRPCStatusCode(t, err, codes.NotFound)
		assertHasGRPCErrorDetailOfType(t, err, &proto.RevisionNotFoundPayload{})

		// Test EnsureRevision is called correctly.
		// Initially, the revision is not found.
		b.ResolveRevisionFunc.PushReturn("", &gitdomain.RevisionNotFoundError{Repo: "therepo", Spec: "HEAD"})
		// EnsureRevision was able to run a fetch, retry.
		svc.EnsureRevisionFunc.SetDefaultReturn(true)
		// After the fetch, resolve revision succeeds.
		b.ResolveRevisionFunc.PushReturn("deadbeef", nil)
		_, err = cli.ResolveRevision(ctx, &v1.ResolveRevisionRequest{
			RepoName:       "therepo",
			RevSpec:        []byte("HEAD"),
			EnsureRevision: pointers.Ptr(true),
		})
		require.NoError(t, err)
		if diff := cmp.Diff(&proto.ResolveRevisionResponse{
			CommitSha: "deadbeef",
		}, res, cmpopts.IgnoreUnexported(proto.ResolveRevisionResponse{})); diff != "" {
			t.Fatalf("unexpected response (-want +got):\n%s", diff)
		}
		mockrequire.Called(t, svc.EnsureRevisionFunc)
	})
}

func assertGRPCStatusCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	s, ok := status.FromError(err)
	require.True(t, ok, "expected status.FromError to succeed")
	require.Equal(t, want, s.Code())
}

func assertHasGRPCErrorDetailOfType(t *testing.T, err error, typ any) {
	t.Helper()
	s, ok := status.FromError(err)
	require.True(t, ok, "expected status.FromError to succeed")
	for _, d := range s.Details() {
		// Compare types of d and typ:
		if reflect.TypeOf(d) == reflect.TypeOf(typ) {
			return
		}
	}
	t.Fatalf("error %v does not implement error detail type %T", err, typ)
}

func spawnServer(t *testing.T, server *grpcServer) proto.GitserverServiceClient {
	t.Helper()
	grpcServer := defaults.NewServer(logtest.Scoped(t))
	proto.RegisterGitserverServiceServer(grpcServer, server)
	handler := internalgrpc.MultiplexHandlers(grpcServer, http.NotFoundHandler())
	srv := httptest.NewServer(handler)
	t.Cleanup(func() {
		srv.Close()
	})

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	cc, err := defaults.Dial(u.Host, logtest.Scoped(t))
	require.NoError(t, err)

	return proto.NewGitserverServiceClient(cc)
}
