package vcssyncer

import (
	"context"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/pypi"
	"github.com/sourcegraph/sourcegraph/internal/unpack"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

func NewPythonPackagesSyncer(
	connection *schema.PythonPackagesConnection,
	svc *dependencies.Service,
	client *pypi.Client,
	fs gitserverfs.FS,
	getRemoteURLSource func(ctx context.Context, name api.RepoName) (RemoteURLSource, error),
) VCSSyncer {
	return &vcsPackagesSyncer{
		logger:             log.Scoped("PythonPackagesSyncer"),
		typ:                "python_packages",
		scheme:             dependencies.PythonPackagesScheme,
		placeholder:        reposource.ParseVersionedPackage("sourcegraph.com/placeholder@v0.0.0"),
		svc:                svc,
		configDeps:         connection.Dependencies,
		source:             &pythonPackagesSyncer{client: client, fs: fs},
		getRemoteURLSource: getRemoteURLSource,
		fs:                 fs,
	}
}

// pythonPackagesSyncer implements packagesSource
type pythonPackagesSyncer struct {
	client *pypi.Client
	fs     gitserverfs.FS
}

func (pythonPackagesSyncer) ParseVersionedPackageFromNameAndVersion(name reposource.PackageName, version string) (reposource.VersionedPackage, error) {
	return reposource.ParseVersionedPackage(string(name) + "==" + version), nil
}

func (pythonPackagesSyncer) ParseVersionedPackageFromConfiguration(dep string) (reposource.VersionedPackage, error) {
	return reposource.ParseVersionedPackage(dep), nil
}

func (pythonPackagesSyncer) ParsePackageFromName(name reposource.PackageName) (reposource.Package, error) {
	return reposource.ParsePythonPackageFromName(name), nil
}

func (pythonPackagesSyncer) ParsePackageFromRepoName(repoName api.RepoName) (reposource.Package, error) {
	return reposource.ParsePythonPackageFromRepoName(repoName)
}

func (s *pythonPackagesSyncer) Download(ctx context.Context, dir string, dep reposource.VersionedPackage) error {
	pythonDep := dep.(*reposource.PythonVersionedPackage)
	pypiFile, err := s.client.Version(ctx, pythonDep.Name, pythonDep.Version)
	if err != nil {
		return err
	}
	packageURL := pypiFile.URL
	pkgData, err := s.client.Download(ctx, packageURL)
	if err != nil {
		return errors.Wrap(err, "download")
	}
	defer pkgData.Close()

	mkdirTemp := func() (string, error) {
		return s.fs.TempDir("pypi-packages")
	}

	if err = unpackPythonPackage(pkgData, packageURL, mkdirTemp, dir); err != nil {
		return errors.Wrap(err, "failed to unzip python module")
	}

	return nil
}

// unpackPythonPackage unpacks the given python package archive into workDir, skipping any
// files that aren't valid or that are potentially malicious. It detects the kind of archive
// and compression used with the given packageURL.
func unpackPythonPackage(pkg io.Reader, packageURL string, mkdirTemp func() (string, error), workDir string) error {
	logger := log.Scoped("unpackPythonPackage")
	u, err := url.Parse(packageURL)
	if err != nil {
		return errors.Wrap(err, "bad python package URL")
	}

	filename := path.Base(u.Path)

	opts := unpack.Opts{
		SkipInvalid:    true,
		SkipDuplicates: true,
		Filter: func(path string, file fs.FileInfo) bool {
			size := file.Size()

			const sizeLimit = 15 * 1024 * 1024
			if size >= sizeLimit {
				logger.With(
					log.String("path", file.Name()),
					log.Int64("size", size),
					log.Float64("limit", sizeLimit),
				).Warn("skipping large file in python package")
				return false
			}

			malicious := isPotentiallyMaliciousFilepathInArchive(path, workDir)
			return !malicious
		},
	}

	switch {
	case strings.HasSuffix(filename, ".tar.gz"), strings.HasSuffix(filename, ".tgz"):
		err = unpack.Tgz(pkg, workDir, opts)
		if err != nil {
			return err
		}
	case strings.HasSuffix(filename, ".whl"), strings.HasSuffix(filename, ".zip"):
		// We cannot unzip in a streaming fashion, so we write the zip file to
		// a temporary file. Otherwise, we would need to load the entire zip into
		// memory, which isn't great for multi-megabyte+ files.

		// Create a tmpdir that gitserver manages.
		tmpdir, err := mkdirTemp()
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpdir)

		// Write the whole package to a temporary file.
		zip, zipLen, err := writeZipToTemp(tmpdir, pkg)
		if err != nil {
			return err
		}
		defer zip.Close()

		err = unpack.Zip(zip, zipLen, workDir, opts)
		if err != nil {
			return err
		}
	case strings.HasSuffix(filename, ".tar"):
		err = unpack.Tar(pkg, workDir, opts)
		if err != nil {
			return err
		}
	default:
		return errors.Errorf("unsupported python package type %q", filename)
	}

	return stripSingleOutermostDirectory(workDir)
}
