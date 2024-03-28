package vcssyncer

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/npm"
	"github.com/sourcegraph/sourcegraph/internal/unpack"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// NewNpmPackagesSyncer create a new NpmPackageSyncer. If customClient is nil,
// the client for the syncer is configured based on the connection parameter.
func NewNpmPackagesSyncer(
	connection schema.NpmPackagesConnection,
	svc *dependencies.Service,
	client npm.Client,
	fs gitserverfs.FS,
	getRemoteURLSource func(ctx context.Context, name api.RepoName) (RemoteURLSource, error),
) VCSSyncer {
	placeholder, err := reposource.ParseNpmVersionedPackage("@sourcegraph/placeholder@1.0.0")
	if err != nil {
		panic(fmt.Sprintf("expected placeholder package to parse but got %v", err))
	}

	return &vcsPackagesSyncer{
		logger:             log.Scoped("NPMPackagesSyncer"),
		typ:                "npm_packages",
		scheme:             dependencies.NpmPackagesScheme,
		placeholder:        placeholder,
		svc:                svc,
		configDeps:         connection.Dependencies,
		fs:                 fs,
		source:             &npmPackagesSyncer{client: client},
		getRemoteURLSource: getRemoteURLSource,
	}
}

type npmPackagesSyncer struct {
	// The client to use for making queries against npm.
	client npm.Client
}

var (
	_ packagesSource         = &npmPackagesSyncer{}
	_ packagesDownloadSource = &npmPackagesSyncer{}
)

func (npmPackagesSyncer) ParseVersionedPackageFromNameAndVersion(name reposource.PackageName, version string) (reposource.VersionedPackage, error) {
	return reposource.ParseNpmVersionedPackage(string(name) + "@" + version)
}

func (npmPackagesSyncer) ParseVersionedPackageFromConfiguration(dep string) (reposource.VersionedPackage, error) {
	return reposource.ParseNpmVersionedPackage(dep)
}

func (s *npmPackagesSyncer) ParsePackageFromName(name reposource.PackageName) (reposource.Package, error) {
	return s.ParsePackageFromRepoName(api.RepoName("npm/" + strings.TrimPrefix(string(name), "@")))
}

func (npmPackagesSyncer) ParsePackageFromRepoName(repoName api.RepoName) (reposource.Package, error) {
	pkg, err := reposource.ParseNpmPackageFromRepoURL(repoName)
	if err != nil {
		return nil, err
	}
	return &reposource.NpmVersionedPackage{NpmPackageName: pkg}, nil
}

func (s npmPackagesSyncer) GetPackage(ctx context.Context, name reposource.PackageName) (reposource.Package, error) {
	dep, err := reposource.ParseNpmVersionedPackage(string(name) + "@")
	if err != nil {
		return nil, err
	}

	err = s.updateTarballURL(ctx, dep)
	if err != nil {
		return nil, err
	}

	return dep, nil
}

// updateTarballURL sends a GET request to find the URL to download the tarball of this package, and
// sets the `NpmVersionedPackage.TarballURL` field accordingly.
func (s *npmPackagesSyncer) updateTarballURL(ctx context.Context, dep *reposource.NpmVersionedPackage) error {
	f, err := s.client.GetDependencyInfo(ctx, dep)
	if err != nil {
		return err
	}
	dep.TarballURL = f.Dist.TarballURL
	return nil
}

func (s *npmPackagesSyncer) Download(ctx context.Context, dir string, dep reposource.VersionedPackage) error {
	npmDep := dep.(*reposource.NpmVersionedPackage)
	if npmDep.TarballURL == "" {
		err := s.updateTarballURL(ctx, npmDep)
		if err != nil {
			return err
		}
	}

	tgz, err := npm.FetchSources(ctx, s.client, npmDep)
	if err != nil {
		return errors.Wrap(err, "fetch tarball")
	}
	defer tgz.Close()

	if err = decompressTgz(tgz, dir); err != nil {
		return errors.Wrapf(err, "failed to decompress gzipped tarball for %s", dep.VersionedPackageSyntax())
	}

	return nil
}

// Decompress a tarball at tgzPath, putting the files under destination.
//
// Additionally, if all the files in the tarball have paths of the form
// dir/<blah> for the same directory 'dir', the 'dir' will be stripped.
func decompressTgz(tgz io.Reader, destination string) error {
	logger := log.Scoped("decompressTgz")

	err := unpack.Tgz(tgz, destination, unpack.Opts{
		SkipInvalid:    true,
		SkipDuplicates: true,
		Filter: func(path string, file fs.FileInfo) bool {
			size := file.Size()

			const sizeLimit = 15 * 1024 * 1024

			if size >= sizeLimit {
				logger.With(
					log.String("path", file.Name()),
					log.Int64("size", size),
					log.Int("limit", sizeLimit),
				).Warn("skipping large file in npm package")
				return false
			}

			malicious := isPotentiallyMaliciousFilepathInArchive(path, destination)
			return !malicious
		},
	})
	if err != nil {
		return err
	}

	return stripSingleOutermostDirectory(destination)
}

// stripSingleOutermostDirectory strips a single outermost directory in dir
// if it has no sibling files or directories.
//
// In practice, npm tarballs seem to contain a superfluous directory which
// contains the files. For example, if you extract react's tarball,
// all files will be under a package/ directory, and if you extract
// @types/lodash's files, all files are under lodash/.
//
// However, this additional directory has no meaning. Moreover, it makes
// the UX slightly worse, as when you navigate to a repo, you would see
// that it contains just 1 folder, and you'd need to click again to drill
// down further. So we strip the superfluous directory if we detect one.
//
// https://github.com/sourcegraph/sourcegraph/pull/28057#issuecomment-987890718
func stripSingleOutermostDirectory(dir string) error {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	if len(dirEntries) != 1 || !dirEntries[0].IsDir() {
		return nil
	}

	outermostDir := dirEntries[0].Name()
	tmpDir := dir + ".tmp"

	// mv $dir $tmpDir
	err = os.Rename(dir, tmpDir)
	if err != nil {
		return err
	}

	// mv $tmpDir/$(basename $outermostDir) $dir
	return os.Rename(path.Join(tmpDir, outermostDir), dir)
}
