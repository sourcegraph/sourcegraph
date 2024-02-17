package repos

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/crates"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// NewRustPackagesSource returns a new RustPackagesSource from the given external service.
func NewRustPackagesSource(ctx context.Context, svc *types.ExternalService, cf *httpcli.Factory) (*PackagesSource, error) {
	rawConfig, err := svc.Config.Decrypt(ctx)
	if err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	var c schema.RustPackagesConnection
	if err := jsonc.Unmarshal(rawConfig, &c); err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}

	client := crates.NewClient(svc.URN(), cf)

	return &PackagesSource{
		svc:        svc,
		configDeps: c.Dependencies,
		scheme:     dependencies.RustPackagesScheme,
		src:        &rustPackagesSource{client},
	}, nil
}

type rustPackagesSource struct {
	client *crates.Client
}

var _ packagesSource = &rustPackagesSource{}

func (rustPackagesSource) ParseVersionedPackageFromConfiguration(dep string) (reposource.VersionedPackage, error) {
	return reposource.ParseRustVersionedPackage(dep), nil
}

func (rustPackagesSource) ParsePackageFromName(name reposource.PackageName) (reposource.Package, error) {
	return reposource.ParseRustPackageFromName(name), nil
}

func (rustPackagesSource) ParsePackageFromRepoName(repoName api.RepoName) (reposource.Package, error) {
	return reposource.ParseRustPackageFromRepoName(repoName)
}
