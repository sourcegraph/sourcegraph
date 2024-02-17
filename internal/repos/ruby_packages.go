package repos

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/rubygems"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// NewRubyPackagesSource returns a new rubyPackagesSource from the given external service.
func NewRubyPackagesSource(ctx context.Context, svc *types.ExternalService, cf *httpcli.Factory) (*PackagesSource, error) {
	rawConfig, err := svc.Config.Decrypt(ctx)
	if err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	var c schema.RubyPackagesConnection
	if err := jsonc.Unmarshal(rawConfig, &c); err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}

	client := rubygems.NewClient(svc.URN(), c.Repository, cf)

	return &PackagesSource{
		svc:        svc,
		configDeps: c.Dependencies,
		scheme:     dependencies.RubyPackagesScheme,
		src:        &rubyPackagesSource{client},
	}, nil
}

type rubyPackagesSource struct {
	client *rubygems.Client
}

var _ packagesSource = &rubyPackagesSource{}

func (rubyPackagesSource) ParseVersionedPackageFromConfiguration(dep string) (reposource.VersionedPackage, error) {
	return reposource.ParseRubyVersionedPackage(dep), nil
}

func (rubyPackagesSource) ParsePackageFromName(name reposource.PackageName) (reposource.Package, error) {
	return reposource.ParseRubyPackageFromName(name), nil
}

func (rubyPackagesSource) ParsePackageFromRepoName(repoName api.RepoName) (reposource.Package, error) {
	return reposource.ParseRubyPackageFromRepoName(repoName)
}
