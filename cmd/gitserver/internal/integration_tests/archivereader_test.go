package inttests

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"

	server "github.com/sourcegraph/sourcegraph/cmd/gitserver/internal"
	common "github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git/gitcli"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/vcssyncer"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	internalgrpc "github.com/sourcegraph/sourcegraph/internal/grpc"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestClient_ArchiveReader(t *testing.T) {
	root := gitserver.CreateRepoDir(t)

	type test struct {
		name string

		remote      string
		revision    string
		want        map[string]string
		clientErr   error
		readerError error
		skipReader  bool
	}

	tests := []test{
		{
			name: "simple",

			remote:   createSimpleGitRepo(t, root),
			revision: "HEAD",
			want: map[string]string{
				"dir1/":      "",
				"dir1/file1": "infile1",
				"file 2":     "infile2",
			},
			skipReader: false,
		},
		{
			name: "repo-with-dotgit-dir",

			remote:   createRepoWithDotGitDir(t, root),
			revision: "HEAD",
			want: map[string]string{
				"file1":            "hello\n",
				".git/mydir/file2": "milton\n",
				".git/mydir/":      "",
				".git/":            "",
			},
			skipReader: false,
		},
		{
			name: "not-found",

			revision:   "HEAD",
			clientErr:  errors.New("repository does not exist: not-found"),
			skipReader: false,
		},
		{
			name: "revision-not-found",

			remote:      createRepoWithDotGitDir(t, root),
			revision:    "revision-not-found",
			clientErr:   nil,
			readerError: &gitdomain.RevisionNotFoundError{Repo: "revision-not-found", Spec: "revision-not-found"},
			skipReader:  true,
		},
	}

	runArchiveReaderTestfunc := func(t *testing.T, mkClient func(t *testing.T, addrs []string) gitserver.Client, name api.RepoName, test test) {
		t.Run(string(name), func(t *testing.T) {
			// Setup: Prepare the test Gitserver server + register the gRPC server
			s := &server.Server{
				Logger:   logtest.Scoped(t),
				ReposDir: filepath.Join(root, "repos"),
				DB:       newMockDB(),
				GetBackendFunc: func(dir common.GitDir, repoName api.RepoName) (git.GitBackend, error) {
					return gitcli.NewBackend(logtest.Scoped(t), wrexec.NewNoOpRecordingCommandFactory(), dir, repoName), nil
				},
				GetRemoteURLFunc: func(_ context.Context, name api.RepoName) (string, error) {
					if test.remote != "" {
						return test.remote, nil
					}
					return "", errors.Errorf("no remote for %s", test.name)
				},
				GetVCSSyncer: func(ctx context.Context, name api.RepoName) (vcssyncer.VCSSyncer, error) {
					return vcssyncer.NewGitRepoSyncer(logtest.Scoped(t), wrexec.NewNoOpRecordingCommandFactory()), nil
				},
				RecordingCommandFactory: wrexec.NewNoOpRecordingCommandFactory(),
				Locker:                  server.NewRepositoryLocker(),
				RPSLimiter:              ratelimit.NewInstrumentedLimiter("GitserverTest", rate.NewLimiter(100, 10)),
			}

			grpcServer := defaults.NewServer(logtest.Scoped(t))

			proto.RegisterGitserverServiceServer(grpcServer, server.NewGRPCServer(s))
			handler := internalgrpc.MultiplexHandlers(grpcServer, s.Handler())
			srv := httptest.NewServer(handler)
			defer srv.Close()

			u, _ := url.Parse(srv.URL)

			addrs := []string{u.Host}
			cli := mkClient(t, addrs)
			ctx := context.Background()

			if test.remote != "" {
				if _, err := cli.RequestRepoUpdate(ctx, name, 0); err != nil {
					t.Fatal(err)
				}
			}

			rc, err := cli.ArchiveReader(ctx, name, gitserver.ArchiveOptions{Treeish: test.revision, Format: gitserver.ArchiveFormatZip})
			if have, want := fmt.Sprint(err), fmt.Sprint(test.clientErr); have != want {
				t.Errorf("archive: have err %v, want %v", have, want)
			}
			if rc == nil {
				return
			}
			t.Cleanup(func() {
				if err := rc.Close(); err != nil {
					t.Fatal(err)
				}
			})

			data, readErr := io.ReadAll(rc)
			if readErr != nil {
				if readErr.Error() != test.readerError.Error() {
					t.Errorf("archive: have reader err %v, want %v", readErr.Error(), test.readerError.Error())
				}

				if test.skipReader {
					return
				}

				t.Fatal(readErr)
			}

			zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal(err)
			}

			got := map[string]string{}
			for _, f := range zr.File {
				r, err := f.Open()
				if err != nil {
					t.Errorf("failed to open %q because %s", f.Name, err)
					continue
				}
				contents, err := io.ReadAll(r)
				_ = r.Close()
				if err != nil {
					t.Errorf("Read(%q): %s", f.Name, err)
					continue
				}
				got[f.Name] = string(contents)
			}

			if !cmp.Equal(test.want, got) {
				t.Errorf("mismatch (-want +got):\n%s", cmp.Diff(test.want, got))
			}
		})
	}

	for _, test := range tests {
		repoName := api.RepoName(test.name)
		called := false

		mkClient := func(t *testing.T, addrs []string) gitserver.Client {
			t.Helper()

			source := gitserver.NewTestClientSource(t, addrs, func(o *gitserver.TestClientSourceOptions) {
				o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
					base := proto.NewGitserverServiceClient(cc)

					mockArchive := func(ctx context.Context, in *proto.ArchiveRequest, opts ...grpc.CallOption) (proto.GitserverService_ArchiveClient, error) {
						called = true
						return base.Archive(ctx, in, opts...)
					}
					mockRepoUpdate := func(ctx context.Context, in *proto.RepoUpdateRequest, opts ...grpc.CallOption) (*proto.RepoUpdateResponse, error) {
						base := proto.NewGitserverServiceClient(cc)
						return base.RepoUpdate(ctx, in, opts...)
					}
					cli := gitserver.NewMockGitserverServiceClient()
					cli.ArchiveFunc.SetDefaultHook(mockArchive)
					cli.RepoUpdateFunc.SetDefaultHook(mockRepoUpdate)
					return cli
				}
			})

			return gitserver.NewTestClient(t).WithClientSource(source)
		}

		runArchiveReaderTestfunc(t, mkClient, repoName, test)
		if !called {
			t.Error("archiveReader: GitserverServiceClient should have been called")
		}

	}
}

func createSimpleGitRepo(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, "remotes", "simple")

	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}

	for _, cmd := range []string{
		"git init",
		"mkdir dir1",
		"echo -n infile1 > dir1/file1",
		"touch --date=2006-01-02T15:04:05Z dir1 dir1/file1 || touch -t 200601021704.05 dir1 dir1/file1",
		"git add dir1/file1",
		"GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_AUTHOR_DATE=2006-01-02T15:04:05Z GIT_COMMITTER_DATE=2006-01-02T15:04:05Z git commit -m commit1 --author='a <a@a.com>' --date 2006-01-02T15:04:05Z",
		"echo -n infile2 > 'file 2'",
		"touch --date=2014-05-06T19:20:21Z 'file 2' || touch -t 201405062120.21 'file 2'",
		"git add 'file 2'",
		"GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_AUTHOR_DATE=2006-01-02T15:04:05Z GIT_COMMITTER_DATE=2014-05-06T19:20:21Z git commit -m commit2 --author='a <a@a.com>' --date 2014-05-06T19:20:21Z",
		"git branch test-ref HEAD~1",
		"git branch test-nested-ref test-ref",
	} {
		c := exec.Command("bash", "-c", `GIT_CONFIG_GLOBAL="" GIT_CONFIG_SYSTEM="" `+cmd)
		c.Dir = dir
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("Command %q failed. Output was:\n\n%s", cmd, out)
		}
	}

	return dir
}

func createRepoWithDotGitDir(t *testing.T, root string) string {
	t.Helper()
	b64 := func(s string) string {
		t.Helper()
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	dir := filepath.Join(root, "remotes", "repo-with-dot-git-dir")

	// This repo was synthesized by hand to contain a file whose path is `.git/mydir/file2` (the Git
	// CLI will not let you create a file with a `.git` path component).
	//
	// The synthesized bad commit is:
	//
	// commit aa600fc517ea6546f31ae8198beb1932f13b0e4c (HEAD -> master)
	// Author: Quinn Slack <qslack@qslack.com>
	// 	Date:   Tue Jun 5 16:17:20 2018 -0700
	//
	// wip
	//
	// diff --git a/.git/mydir/file2 b/.git/mydir/file2
	// new file mode 100644
	// index 0000000..82b919c
	// --- /dev/null
	// +++ b/.git/mydir/file2
	// @@ -0,0 +1 @@
	// +milton
	files := map[string]string{
		"config": `
[core]
repositoryformatversion=0
filemode=true
`,
		"HEAD":              `ref: refs/heads/master`,
		"refs/heads/master": `aa600fc517ea6546f31ae8198beb1932f13b0e4c`,
		"objects/e7/9c5e8f964493290a409888d5413a737e8e5dd5": b64("eAFLyslPUrBgyMzLLMlMzOECACgtBOw="),
		"objects/ce/013625030ba8dba906f756967f9e9ca394464a": b64("eAFLyslPUjBjyEjNycnnAgAdxQQU"),
		"objects/82/b919c9c565d162c564286d9d6a2497931be47e": b64("eAFLyslPUjBnyM3MKcnP4wIAIw8ElA=="),
		"objects/e5/231c1d547df839dce09809e43608fe6c537682": b64("eAErKUpNVTAzYTAxAAIFvfTMEgbb8lmsKdJ+zz7ukeMOulcqZqOllmloYGBmYqKQlpmTashwjtFMlZl7xe2VbN/DptXPm7N4ipsXACOoGDo="),
		"objects/da/5ecc846359eaf23e8abe907b3125fdd7abdbc0": b64("eAErKUpNVTA2ZjA0MDAzMVFIy8xJNWJo2il58mjqxaSjKRq5c7NUpk+WflIHABZRD2I="),
		"objects/d0/01d287018593691c36042e1c8089fde7415296": b64("eAErKUpNVTA2ZjA0MDAzMVFIy8xJNWQ4x2imysy94vZKtu9h0+rnzVk8xc0LAP2TDiQ="),
		"objects/b4/009ecbf1eba01c5279f25840e2afc0d15f5005": b64("eAGdjdsJAjEQRf1OFdOAMpPN5gEitiBWEJIRBzcJu2b7N2IHfh24nMtJrRTpQA4PfWOGjEhZe4fk5zDZQGmyaDRT8ujDI7MzNOtgVdz7s21w26VWuC8xveC8vr+8/nBKrVxgyF4bJBfgiA5RjXUEO/9xVVKlS1zUB/JxNbA="),
		"objects/3d/779a05641b4ee6f1bc1e0b52de75163c2a2669": b64("eAErKUpNVTA2YjAxAAKF3MqUzCKGW3FnWpIjX32y69o3odpQ9e/11bcPAAAipRGQ"),
		"objects/aa/600fc517ea6546f31ae8198beb1932f13b0e4c": b64("eAGdjlkKAjEQBf3OKfoCSmfpLCDiFcQTZDodHHQWxwxe3xFv4FfBKx4UT8PQNzDa7doiAkLGataFXCg12lRYMEVM4qzHWMUz2eCjUXNeZGzQOdwkd1VLl1EzmZCqoehQTK6MRVMlRFJ5bbdpgcvajyNcH5nvcHy+vjz/cOBpOIEmE41D7xD2GBDVtm6BTf64qnc/qw9c4UKS"),
		"objects/e6/9de29bb2d1d6434b8b29ae775ad8c2e48c5391": b64("eAFLyslPUjBgAAAJsAHw"),
	}
	for name, data := range files {
		name = filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}
