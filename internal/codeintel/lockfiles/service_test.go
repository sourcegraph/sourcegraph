package lockfiles

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/sebdah/goldie/v2"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
)

func TestListDependencies(t *testing.T) {
	ctx := context.Background()

	t.Run("empty ls-files return", func(t *testing.T) {
		gitSvc := NewMockGitService()
		gitSvc.ArchiveFunc.SetDefaultHook(zipArchive(t, map[string]io.Reader{}))

		got, err := TestService(gitSvc).ListDependencies(ctx, "foo", "")
		if err != nil {
			t.Fatal(err)
		}

		if len(got) != 0 {
			t.Fatalf("expected no dependencies")
		}
	})

	t.Run("npm", func(t *testing.T) {
		gitSvc := NewMockGitService()

		yarnLock, err := os.Open("testdata/parse/yarn.lock/yarn_normal.lock")
		if err != nil {
			t.Fatal(err)
		}

		gitSvc.ArchiveFunc.SetDefaultHook(zipArchive(t, map[string]io.Reader{
			"client/package-lock.json": strings.NewReader(`{"dependencies": { "@octokit/request": {"version": "5.6.2"} }}`),
			// promise@8.0.3 is also in yarn.lock. We test that it gets de-duplicated.
			"package-lock.json": strings.NewReader(`{"dependencies": { "promise": {"version": "8.0.3"} }}`),
			"yarn.lock":         yarnLock,
		}))

		deps, err := TestService(gitSvc).ListDependencies(ctx, "foo", "HEAD")
		if err != nil {
			t.Fatal(err)
		}

		have := make([]string, 0, len(deps))
		for _, dep := range deps {
			have = append(have, dep.PackageManagerSyntax())
		}

		sort.Strings(have)

		g := goldie.New(t, goldie.WithFixtureDir("testdata/svc"))
		g.AssertJson(t, t.Name(), have)
	})

	t.Run("go", func(t *testing.T) {
		gitSvc := NewMockGitService()
		gitSvc.LsFilesFunc.SetDefaultReturn([]string{
			"subpkg/go.sum",
			"go.sum",
		}, nil)

		gitSvc.ArchiveFunc.SetDefaultHook(zipArchive(t, map[string]io.Reader{
			// github.com/google/uuid@v1.0.0 is also in go.sum. We test that it gets de-duplicated.
			"subpkg/go.sum": strings.NewReader(`
modernc.org/cc v1.0.0/go.mod h1:1Sk4//wdnYJiUIxnW8ddKpaOJCF37yAdqYnkxUpaYxw=
modernc.org/golex v1.0.0/go.mod h1:b/QX9oBD/LhixY6NDh+IdGv17hgB+51fET1i2kPSmvk=
github.com/google/uuid v1.0.0 h1:b4Gk+7WdP/d3HZH8EJsZpvV7EtDOgaZLtnaNGIu1adA=
github.com/google/uuid v1.0.0/go.mod h1:TIyPZe4MgqvfeYDBFedMoGGpEw/LqOeaOT+nhxU+yHo=
`),
			"go.sum": strings.NewReader(`
github.com/google/uuid v1.0.0 h1:b4Gk+7WdP/d3HZH8EJsZpvV7EtDOgaZLtnaNGIu1adA=
github.com/google/uuid v1.0.0/go.mod h1:TIyPZe4MgqvfeYDBFedMoGGpEw/LqOeaOT+nhxU+yHo=
github.com/pborman/uuid v1.2.1 h1:+ZZIw58t/ozdjRaXh/3awHfmWRbzYxJoAdNJxe/3pvw=
github.com/pborman/uuid v1.2.1/go.mod h1:X/NO0urCmaxf9VXbdlT7C2Yzkj2IKimNn4k+gtPdI/k=
`),
		}))

		deps, err := TestService(gitSvc).ListDependencies(ctx, "foo", "HEAD")
		if err != nil {
			t.Fatal(err)
		}

		have := make([]string, 0, len(deps))
		for _, dep := range deps {
			have = append(have, dep.PackageManagerSyntax())
		}

		sort.Strings(have)

		g := goldie.New(t, goldie.WithFixtureDir("testdata/svc"))
		g.AssertJson(t, t.Name(), have)
	})
}

func zipArchive(t testing.TB, files map[string]io.Reader) func(context.Context, api.RepoName, gitserver.ArchiveOptions) (io.ReadCloser, error) {
	return func(ctx context.Context, name api.RepoName, options gitserver.ArchiveOptions) (io.ReadCloser, error) {
		var b bytes.Buffer
		zw := zip.NewWriter(&b)
		defer zw.Close()

		for name, f := range files {
			w, err := zw.Create(name)
			if err != nil {
				t.Fatal(err)
			}

			_, err = io.Copy(w, f)
			if err != nil {
				t.Fatal(err)
			}
		}

		return io.NopCloser(&b), nil
	}
}
