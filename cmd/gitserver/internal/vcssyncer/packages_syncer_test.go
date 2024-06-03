package vcssyncer

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestVcsDependenciesSyncer_Fetch(t *testing.T) {
	ctx := context.Background()
	placeholder, _ := parseFakeDependency("sourcegraph/placeholder@0.0.0")

	depsSource := &fakeDepsSource{
		deps:          map[string]reposource.VersionedPackage{},
		download:      map[string]error{},
		downloadCount: map[string]int{},
	}
	depsService := &fakeDepsService{deps: map[reposource.PackageName]dependencies.PackageRepoReference{}}

	root := t.TempDir()
	fs := gitserverfs.New(observation.TestContextTB(t), root)
	require.NoError(t, fs.Initialize())
	remoteURL := &vcs.URL{URL: url.URL{Path: "fake/foo"}}

	s := vcsPackagesSyncer{
		logger:      logtest.Scoped(t),
		typ:         "fake",
		scheme:      "fake",
		placeholder: placeholder,
		source:      depsSource,
		svc:         depsService,
		fs:          fs,
		getRemoteURLSource: func(ctx context.Context, name api.RepoName) (RemoteURLSource, error) {
			return RemoteURLSourceFunc(func(_ context.Context) (*vcs.URL, error) {
				return remoteURL, nil
			}), nil
		},
	}

	dir := common.GitDir(t.TempDir())
	err := os.MkdirAll(string(dir), os.ModePerm)
	require.NoError(t, err)
	err = git.MakeBareRepo(ctx, string(dir))
	require.NoError(t, err)
	err = s.Fetch(ctx, "repo", dir, io.Discard)
	require.NoError(t, err)

	depsService.Add("foo@0.0.1")
	depsSource.Add("foo@0.0.1")

	t.Run("one version from service", func(t *testing.T) {
		err := s.Fetch(ctx, "", dir, io.Discard)
		require.NoError(t, err)

		s.assertRefs(t, dir, map[string]string{
			"refs/heads/latest":   "759dab7e4a7fc384522cb75519660cb0d6f6e49d",
			"refs/tags/v0.0.1":    "b47eb15deed08abc9d437c81f42c1635febaa218",
			"refs/tags/v0.0.1^{}": "759dab7e4a7fc384522cb75519660cb0d6f6e49d",
		})
		s.assertDownloadCounts(t, depsSource, map[string]int{"foo@0.0.1": 1})
	})

	s.configDeps = []string{"foo@0.0.2"}
	depsSource.Add("foo@0.0.2")
	allVersionsHaveRefs := map[string]string{
		"refs/heads/latest":   "6cff53ec57702e8eec10569a3d981dacbaee4ed3",
		"refs/tags/v0.0.1":    "b47eb15deed08abc9d437c81f42c1635febaa218",
		"refs/tags/v0.0.1^{}": "759dab7e4a7fc384522cb75519660cb0d6f6e49d",
		"refs/tags/v0.0.2":    "7e2e4506ef1f5cd97187917a67bfb7a310f78687",
		"refs/tags/v0.0.2^{}": "6cff53ec57702e8eec10569a3d981dacbaee4ed3",
	}
	oneVersionOneDownload := map[string]int{"foo@0.0.1": 1, "foo@0.0.2": 1}

	t.Run("two versions, service and config", func(t *testing.T) {
		err := s.Fetch(ctx, "", dir, io.Discard)
		require.NoError(t, err)

		s.assertRefs(t, dir, allVersionsHaveRefs)
		s.assertDownloadCounts(t, depsSource, oneVersionOneDownload)
	})

	depsSource.Delete("foo@0.0.2")

	t.Run("cached tag not re-downloaded (404 not found)", func(t *testing.T) {
		err := s.Fetch(ctx, "", dir, io.Discard)
		require.NoError(t, err)

		// v0.0.2 is still present in the git repo because we didn't send a second download request.
		s.assertRefs(t, dir, allVersionsHaveRefs)
		s.assertDownloadCounts(t, depsSource, oneVersionOneDownload)
	})

	depsSource.Add("foo@0.0.2")
	depsSource.download["foo@0.0.1"] = errors.New("401 unauthorized")

	t.Run("cached tag not re-downloaded (401 unauthorized)", func(t *testing.T) {
		err := s.Fetch(ctx, "", dir, io.Discard)
		// v0.0.1 is still present in the git repo because we didn't send a second download request.
		require.NoError(t, err)
		s.assertRefs(t, dir, allVersionsHaveRefs)
		s.assertDownloadCounts(t, depsSource, oneVersionOneDownload)
	})

	depsService.Delete("foo@0.0.1")
	onlyV2Refs := map[string]string{
		"refs/heads/latest":   "6cff53ec57702e8eec10569a3d981dacbaee4ed3",
		"refs/tags/v0.0.2":    "7e2e4506ef1f5cd97187917a67bfb7a310f78687",
		"refs/tags/v0.0.2^{}": "6cff53ec57702e8eec10569a3d981dacbaee4ed3",
	}

	t.Run("service version deleted", func(t *testing.T) {
		err := s.Fetch(ctx, "", dir, io.Discard)
		require.NoError(t, err)

		s.assertRefs(t, dir, onlyV2Refs)
		s.assertDownloadCounts(t, depsSource, oneVersionOneDownload)
	})

	s.configDeps = []string{}

	t.Run("all versions deleted", func(t *testing.T) {
		err := s.Fetch(ctx, "", dir, io.Discard)
		require.NoError(t, err)

		s.assertRefs(t, dir, map[string]string{})
		s.assertDownloadCounts(t, depsSource, oneVersionOneDownload)
	})

	depsService.Add("foo@0.0.1")
	depsSource.Add("foo@0.0.1")
	depsService.Add("foo@0.0.2")
	depsSource.Add("foo@0.0.2")
	t.Run("error aggregation", func(t *testing.T) {
		err := s.Fetch(ctx, "", dir, io.Discard)
		require.ErrorContains(t, err, "401 unauthorized")

		// The foo@0.0.1 tag was not created because of the 401 error.
		// The foo@0.0.2 tag was created despite the 401 error for foo@0.0.1
		s.assertRefs(t, dir, onlyV2Refs)

		// We re-downloaded both v0.0.1 and v0.0.2 since their git refs had been deleted.
		s.assertDownloadCounts(t, depsSource, map[string]int{"foo@0.0.1": 2, "foo@0.0.2": 2})
	})

	depsSource.download["foo@0.0.4"] = errors.New("0.0.4 not found")
	s.svc.(*fakeDepsService).upsertedDeps = []dependencies.MinimalPackageRepoRef{}

	depsSource.download["org.springframework.boot:spring-boot:3.0"] = notFoundError{errors.New("Please contact Josh Long")}

	t.Run("trying to download non-existent Maven dependency", func(t *testing.T) {
		springBootDep, err := reposource.ParseMavenVersionedPackage("org.springframework.boot:spring-boot:3.0")
		if err != nil {
			t.Fatal("Cannot parse Maven dependency")
		}
		err = s.gitPushDependencyTag(ctx, string(dir), springBootDep)
		require.NotNil(t, err)
	})
}

type fakeDepsService struct {
	deps         map[reposource.PackageName]dependencies.PackageRepoReference
	upsertedDeps []dependencies.MinimalPackageRepoRef
}

func (s *fakeDepsService) InsertPackageRepoRefs(_ context.Context, depsToAdd []dependencies.MinimalPackageRepoRef) (newRepos []dependencies.PackageRepoReference, newVersions []dependencies.PackageRepoRefVersion, _ error) {
	for i := range depsToAdd {
		depsToAdd[i].LastCheckedAt = nil
		for j := range depsToAdd[i].Versions {
			depsToAdd[i].Versions[j].LastCheckedAt = nil
		}
	}
	s.upsertedDeps = append(s.upsertedDeps, depsToAdd...)
	for _, depToAdd := range depsToAdd {
		if existingDep, exists := s.deps[depToAdd.Name]; exists {
			for _, version := range depToAdd.Versions {
				if !slices.ContainsFunc(existingDep.Versions, func(v dependencies.PackageRepoRefVersion) bool {
					return v.Version == version.Version
				}) {
					existingDep.Versions = append(existingDep.Versions, dependencies.PackageRepoRefVersion{
						PackageRefID: existingDep.ID,
						Version:      version.Version,
						Blocked:      version.Blocked,
					})
					s.deps[depToAdd.Name] = existingDep
					newVersions = append(newVersions, dependencies.PackageRepoRefVersion{
						Version: version.Version,
						Blocked: version.Blocked,
					})
				}
			}
		} else {
			versionsForDep := make([]dependencies.PackageRepoRefVersion, 0, len(depToAdd.Versions))
			for _, version := range depToAdd.Versions {
				versionsForDep = append(versionsForDep, dependencies.PackageRepoRefVersion{
					Version: version.Version,
					Blocked: version.Blocked,
				})
			}
			s.deps[depToAdd.Name] = dependencies.PackageRepoReference{
				Scheme:   depToAdd.Scheme,
				Name:     depToAdd.Name,
				Versions: versionsForDep,
			}
			newRepos = append(newRepos, dependencies.PackageRepoReference{
				Scheme:   depToAdd.Scheme,
				Name:     depToAdd.Name,
				Versions: versionsForDep,
			})
		}
	}
	return
}

func (s *fakeDepsService) IsPackageRepoVersionAllowed(_ context.Context, _ string, _ reposource.PackageName, _ string) (bool, error) {
	return true, nil
}

func (s *fakeDepsService) ListPackageRepoRefs(_ context.Context, opts dependencies.ListDependencyReposOpts) ([]dependencies.PackageRepoReference, int, bool, error) {
	return []dependencies.PackageRepoReference{s.deps[opts.Name]}, 1, false, nil
}

func (s *fakeDepsService) Add(deps ...string) {
	for _, d := range deps {
		dep, _ := parseFakeDependency(d)
		name := dep.PackageSyntax()
		if d, ok := s.deps[name]; !ok {
			s.deps[name] = dependencies.PackageRepoReference{
				Scheme: dep.Scheme(),
				Name:   name,
				Versions: []dependencies.PackageRepoRefVersion{
					{Version: dep.PackageVersion()},
				},
			}
		} else {
			d.Versions = append(d.Versions, dependencies.PackageRepoRefVersion{Version: dep.PackageVersion()})
			s.deps[name] = d
		}
	}
}

func (s *fakeDepsService) Delete(deps ...string) {
	for _, d := range deps {
		depToDelete, _ := parseFakeDependency(d)
		name := depToDelete.PackageSyntax()
		version := depToDelete.PackageVersion()
		dep := s.deps[name]
		if idx := slices.IndexFunc(dep.Versions, func(v dependencies.PackageRepoRefVersion) bool {
			return v.Version == version
		}); idx > -1 {
			dep.Versions = slices.Delete(dep.Versions, idx, idx+1)
			s.deps[name] = dep
		}
	}
}

type fakeDepsSource struct {
	deps          map[string]reposource.VersionedPackage
	download      map[string]error
	downloadCount map[string]int
}

func (s *fakeDepsSource) Add(deps ...string) {
	for _, d := range deps {
		dep, _ := parseFakeDependency(d)
		s.deps[d] = dep
	}
}

func (s *fakeDepsSource) Delete(deps ...string) {
	for _, d := range deps {
		delete(s.deps, d)
	}
}

func (s *fakeDepsSource) Download(_ context.Context, dir string, dep reposource.VersionedPackage) error {
	s.downloadCount[dep.VersionedPackageSyntax()] = 1 + s.downloadCount[dep.VersionedPackageSyntax()]

	err := s.download[dep.VersionedPackageSyntax()]
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "README.md"), []byte("README for "+dep.VersionedPackageSyntax()), 0o666)
}

func (fakeDepsSource) ParseVersionedPackageFromNameAndVersion(name reposource.PackageName, version string) (reposource.VersionedPackage, error) {
	return parseFakeDependency(string(name) + "@" + version)
}

func (fakeDepsSource) ParseVersionedPackageFromConfiguration(dep string) (reposource.VersionedPackage, error) {
	return parseFakeDependency(dep)
}

func (fakeDepsSource) ParsePackageFromName(name reposource.PackageName) (reposource.Package, error) {
	return parseFakeDependency(string(name))
}

func (s *fakeDepsSource) ParsePackageFromRepoName(repoName api.RepoName) (reposource.Package, error) {
	return s.ParsePackageFromName(reposource.PackageName(strings.TrimPrefix(string(repoName), "fake/")))
}

type fakeVersionedPackage struct {
	name    reposource.PackageName
	version string
}

func parseFakeDependency(dep string) (reposource.VersionedPackage, error) {
	i := strings.LastIndex(dep, "@")
	if i == -1 {
		return fakeVersionedPackage{name: reposource.PackageName(dep)}, nil
	}
	return fakeVersionedPackage{name: reposource.PackageName(dep[:i]), version: dep[i+1:]}, nil
}

func (f fakeVersionedPackage) Scheme() string                        { return "fake" }
func (f fakeVersionedPackage) PackageSyntax() reposource.PackageName { return f.name }
func (f fakeVersionedPackage) VersionedPackageSyntax() string {
	return string(f.name) + "@" + f.version
}
func (f fakeVersionedPackage) PackageVersion() string    { return f.version }
func (f fakeVersionedPackage) Description() string       { return string(f.name) + "@" + f.version }
func (f fakeVersionedPackage) RepoName() api.RepoName    { return api.RepoName("fake/" + f.name) }
func (f fakeVersionedPackage) GitTagFromVersion() string { return "v" + f.version }
func (f fakeVersionedPackage) Less(other reposource.VersionedPackage) bool {
	return f.VersionedPackageSyntax() > other.VersionedPackageSyntax()
}

func (s *vcsPackagesSyncer) runCloneCommand(t *testing.T, bareGitDirectory string, dependencies []string) {

	s.configDeps = dependencies
	err := os.MkdirAll(bareGitDirectory, os.ModePerm)
	require.NoError(t, err)
	err = git.MakeBareRepo(context.Background(), bareGitDirectory)
	require.NoError(t, err)
	err = s.Fetch(context.Background(), "repo", common.GitDir(bareGitDirectory), io.Discard)
	assert.Nil(t, err)
}

func (s *vcsPackagesSyncer) assertDownloadCounts(t *testing.T, depsSource *fakeDepsSource, want map[string]int) {
	t.Helper()

	require.Equal(t, want, depsSource.downloadCount)
}

func (s *vcsPackagesSyncer) assertRefs(t *testing.T, dir common.GitDir, want map[string]string) {
	t.Helper()

	cmd := exec.Command("git", "show-ref", "--head", "--dereference")
	cmd.Dir = string(dir)

	out, _ := cmd.CombinedOutput()

	sc := bufio.NewScanner(bytes.NewReader(out))
	have := map[string]string{}
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		have[fs[1]] = fs[0]
	}

	require.NoError(t, sc.Err())
	require.Equal(t, want, have)
}
