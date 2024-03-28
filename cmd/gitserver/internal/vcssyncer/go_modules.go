package vcssyncer

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"

	"github.com/sourcegraph/log"
	"golang.org/x/mod/module"
	modzip "golang.org/x/mod/zip"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/internal/api"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gomodproxy"
	"github.com/sourcegraph/sourcegraph/internal/unpack"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

func NewGoModulesSyncer(
	connection *schema.GoModulesConnection,
	svc *dependencies.Service,
	client *gomodproxy.Client,
	fs gitserverfs.FS,
	getRemoteURLSource func(ctx context.Context, name api.RepoName) (RemoteURLSource, error),
) VCSSyncer {
	placeholder, err := reposource.ParseGoVersionedPackage("sourcegraph.com/placeholder@v0.0.0")
	if err != nil {
		panic(fmt.Sprintf("expected placeholder dependency to parse but got %v", err))
	}

	return &vcsPackagesSyncer{
		logger:             log.Scoped("GoModulesSyncer"),
		typ:                "go_modules",
		scheme:             dependencies.GoPackagesScheme,
		placeholder:        placeholder,
		svc:                svc,
		configDeps:         connection.Dependencies,
		source:             &goModulesSyncer{client: client, fs: fs},
		fs:                 fs,
		getRemoteURLSource: getRemoteURLSource,
	}
}

type goModulesSyncer struct {
	client *gomodproxy.Client
	fs     gitserverfs.FS
}

func (s goModulesSyncer) ParseVersionedPackageFromNameAndVersion(name reposource.PackageName, version string) (reposource.VersionedPackage, error) {
	return reposource.ParseGoVersionedPackage(string(name) + "@" + version)
}

func (goModulesSyncer) ParseVersionedPackageFromConfiguration(dep string) (reposource.VersionedPackage, error) {
	return reposource.ParseGoVersionedPackage(dep)
}

func (goModulesSyncer) ParsePackageFromName(name reposource.PackageName) (reposource.Package, error) {
	return reposource.ParseGoDependencyFromName(name)
}

func (goModulesSyncer) ParsePackageFromRepoName(repoName api.RepoName) (reposource.Package, error) {
	return reposource.ParseGoDependencyFromRepoName(repoName)
}

func (s *goModulesSyncer) Download(ctx context.Context, dir string, dep reposource.VersionedPackage) error {
	zip, err := s.client.GetZip(ctx, dep.PackageSyntax(), dep.PackageVersion())
	if err != nil {
		return errors.Wrap(err, "get zip")
	}
	defer zip.Close()

	tmpdir, err := s.fs.TempDir("gomod-zips")
	if err != nil {
		return errors.Wrap(err, "create temp dir")
	}
	defer os.RemoveAll(tmpdir)

	mod := dep.(*reposource.GoVersionedPackage).Module
	if err = unzip(mod, zip, tmpdir, dir); err != nil {
		return errors.Wrap(err, "failed to unzip go module")
	}

	return nil
}

// unzip the given go module zip into workDir, skipping any files that aren't
// valid according to modzip.CheckZip or that are potentially malicious.
func unzip(mod module.Version, zipContent io.Reader, tmpdir, workDir string) (err error) {
	// We cannot unzip in a streaming fashion, so we write the zip file to
	// a temporary file. Otherwise, we would need to load the entire zip into
	// memory, which isn't great for multi-megabyte+ files.

	// Write the whole package to a temporary file.
	zip, zipLen, err := writeZipToTemp(tmpdir, zipContent)
	if err != nil {
		return err
	}
	defer zip.Close()

	files, err := modzip.CheckZip(mod, zip.Name())
	if err != nil && len(files.Valid) == 0 {
		return errors.Wrapf(err, "failed to check go module zip %q", zip.Name())
	}

	if len(files.Valid) == 0 {
		return nil
	}

	valid := make(map[string]struct{}, len(files.Valid))
	for _, f := range files.Valid {
		valid[f] = struct{}{}
	}

	err = unpack.Zip(zip, zipLen, workDir, unpack.Opts{
		SkipInvalid:    true,
		SkipDuplicates: true,
		Filter: func(path string, file fs.FileInfo) bool {
			malicious := isPotentiallyMaliciousFilepathInArchive(path, workDir)
			_, ok := valid[path]
			return ok && !malicious
		},
	})

	if err != nil {
		return err
	}

	// All files in module zips are prefixed by prefix below, but we don't want
	// those nested directories in our actual repository, so we move all the files up
	// with the below renames.
	tmpDir := workDir + ".tmp"

	// mv $workDir $tmpDir
	err = os.Rename(workDir, tmpDir)
	if err != nil {
		return err
	}

	// mv $tmpDir/$(basename $prefix) $workDir
	prefix := fmt.Sprintf("%s@%s/", mod.Path, mod.Version)
	return os.Rename(path.Join(tmpDir, prefix), workDir)
}
