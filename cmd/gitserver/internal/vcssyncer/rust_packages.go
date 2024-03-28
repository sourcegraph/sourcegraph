package vcssyncer

import (
	"context"
	"fmt"
	"io"
	"io/fs"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/crates"
	"github.com/sourcegraph/sourcegraph/internal/unpack"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

func NewRustPackagesSyncer(
	connection *schema.RustPackagesConnection,
	svc *dependencies.Service,
	client *crates.Client,
	fs gitserverfs.FS,
	getRemoteURLSource func(ctx context.Context, name api.RepoName) (RemoteURLSource, error),
) VCSSyncer {
	return &vcsPackagesSyncer{
		logger:             log.Scoped("RustPackagesSyncer"),
		typ:                "rust_packages",
		scheme:             dependencies.RustPackagesScheme,
		placeholder:        reposource.ParseRustVersionedPackage("sourcegraph.com/placeholder@0.0.0"),
		svc:                svc,
		configDeps:         connection.Dependencies,
		source:             &rustDependencySource{client: client},
		fs:                 fs,
		getRemoteURLSource: getRemoteURLSource,
	}
}

type rustDependencySource struct {
	client *crates.Client
}

func (rustDependencySource) ParseVersionedPackageFromNameAndVersion(name reposource.PackageName, version string) (reposource.VersionedPackage, error) {
	return reposource.ParseRustVersionedPackage(string(name) + "@" + version), nil
}

func (rustDependencySource) ParseVersionedPackageFromConfiguration(dep string) (reposource.VersionedPackage, error) {
	return reposource.ParseRustVersionedPackage(dep), nil
}

func (rustDependencySource) ParsePackageFromName(name reposource.PackageName) (reposource.Package, error) {
	return reposource.ParseRustPackageFromName(name), nil
}

func (rustDependencySource) ParsePackageFromRepoName(repoName api.RepoName) (reposource.Package, error) {
	return reposource.ParseRustPackageFromRepoName(repoName)
}

func (s *rustDependencySource) Download(ctx context.Context, dir string, dep reposource.VersionedPackage) error {
	packageURL := fmt.Sprintf("https://static.crates.io/crates/%s/%s-%s.crate", dep.PackageSyntax(), dep.PackageSyntax(), dep.PackageVersion())

	pkg, err := s.client.Get(ctx, packageURL)
	if err != nil {
		return errors.Wrapf(err, "error downloading crate with URL '%s'", packageURL)
	}
	defer pkg.Close()

	// TODO: we could add `.sourcegraph/repo.json` here with more information,
	// to be used by rust analyzer
	if err = unpackRustPackage(pkg, dir); err != nil {
		return errors.Wrap(err, "failed to unzip rust module")
	}

	return nil
}

// unpackRustPackages unpacks the given rust package archive into workDir, skipping any
// files that aren't valid or that are potentially malicious.
func unpackRustPackage(pkg io.Reader, workDir string) error {
	opts := unpack.Opts{
		SkipInvalid:    true,
		SkipDuplicates: true,
		Filter: func(path string, file fs.FileInfo) bool {
			size := file.Size()

			const sizeLimit = 15 * 1024 * 1024
			if size >= sizeLimit {
				return false
			}

			malicious := isPotentiallyMaliciousFilepathInArchive(path, workDir)
			return !malicious
		},
	}

	if err := unpack.Tgz(pkg, workDir, opts); err != nil {
		return err
	}

	return stripSingleOutermostDirectory(workDir)
}
