package repos

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/pypi"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// NewPythonPackagesSource returns a new PythonPackagesSource from the given external service.
func NewPythonPackagesSource(ctx context.Context, svc *types.ExternalService, cf *httpcli.Factory) (*PackagesSource, error) {
	rawConfig, err := svc.Config.Decrypt(ctx)
	if err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	var c schema.PythonPackagesConnection
	if err := jsonc.Unmarshal(rawConfig, &c); err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}

	client := pypi.NewClient(svc.URN(), c.Urls, cf)

	return &PackagesSource{
		svc:        svc,
		configDeps: c.Dependencies,
		scheme:     dependencies.PythonPackagesScheme,
		src:        &pythonPackagesSource{client},
	}, nil
}

type pythonPackagesSource struct {
	client *pypi.Client
}

var _ packagesSource = &pythonPackagesSource{}

func (s *pythonPackagesSource) Get(ctx context.Context, name reposource.PackageName, version string) (reposource.VersionedPackage, error) {
	_, err := s.client.Version(ctx, name, version)
	if err != nil {
		return nil, err
	}
	return reposource.NewPythonVersionedPackage(name, version), nil
}

func (pythonPackagesSource) ParseVersionedPackageFromConfiguration(dep string) (reposource.VersionedPackage, error) {
	return reposource.ParseVersionedPackage(dep), nil
}

func (pythonPackagesSource) ParsePackageFromName(name reposource.PackageName) (reposource.Package, error) {
	return reposource.ParsePythonPackageFromName(name), nil
}

func (pythonPackagesSource) ParsePackageFromRepoName(repoName api.RepoName) (reposource.Package, error) {
	return reposource.ParsePythonPackageFromRepoName(repoName)
}
