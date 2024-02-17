package repos

import (
	"context"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/npm"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// NewNpmPackagesSource returns a new PackagesSource from the given external
// service.
func NewNpmPackagesSource(ctx context.Context, svc *types.ExternalService, cf *httpcli.Factory) (*PackagesSource, error) {
	rawConfig, err := svc.Config.Decrypt(ctx)
	if err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	var c schema.NpmPackagesConnection
	if err := jsonc.Unmarshal(rawConfig, &c); err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}

	client := npm.NewHTTPClient(svc.URN(), c.Registry, c.Credentials, cf)

	return &PackagesSource{
		svc:        svc,
		configDeps: c.Dependencies,
		scheme:     dependencies.NpmPackagesScheme,
		/* depsSvc initialized in SetDependenciesService */
		src: &npmPackagesSource{client},
	}, nil
}

var _ packagesSource = &npmPackagesSource{}

type npmPackagesSource struct {
	client npm.Client
}

func (s npmPackagesSource) GetPackage(ctx context.Context, name reposource.PackageName) (reposource.Package, error) {
	// By using the empty string "" for the version, the request URL becomes "NPM_REGISTRY_URL/PACKAGE_NAME/",
	// which returns metadata about the package instead a specific version. For example, compare:
	// - https://registry.npmjs.org/react/
	// - https://registry.npmjs.org/react/0.0.1
	return s.Get(ctx, name, "")
}

func (npmPackagesSource) ParseVersionedPackageFromConfiguration(dep string) (reposource.VersionedPackage, error) {
	return reposource.ParseNpmVersionedPackage(dep)
}

func (s *npmPackagesSource) ParsePackageFromName(name reposource.PackageName) (reposource.Package, error) {
	return s.ParsePackageFromRepoName(api.RepoName("npm/" + strings.TrimPrefix(string(name), "@")))
}

func (npmPackagesSource) ParsePackageFromRepoName(repoName api.RepoName) (reposource.Package, error) {
	pkg, err := reposource.ParseNpmPackageFromRepoURL(repoName)
	if err != nil {
		return nil, err
	}
	return &reposource.NpmVersionedPackage{NpmPackageName: pkg}, nil
}

func (s *npmPackagesSource) Get(ctx context.Context, name reposource.PackageName, version string) (reposource.VersionedPackage, error) {
	parsedDbPackage, err := reposource.ParseNpmPackageFromPackageSyntax(name)
	if err != nil {
		return nil, err
	}

	dep := &reposource.NpmVersionedPackage{NpmPackageName: parsedDbPackage, Version: version}

	info, err := s.client.GetDependencyInfo(ctx, dep)
	if err != nil {
		return nil, err
	}

	dep.PackageDescription = info.Description
	dep.TarballURL = info.Dist.TarballURL

	return dep, nil
}
