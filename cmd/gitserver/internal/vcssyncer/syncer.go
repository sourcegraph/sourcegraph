package vcssyncer

import (
	"context"
	"io"

	jsoniter "github.com/json-iterator/go"

	"github.com/sourcegraph/sourcegraph/internal/vcs"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/crates"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gomodproxy"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/npm"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/pypi"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/rubygems"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// VCSSyncer describes whether and how to sync content from a VCS remote to
// local disk.
type VCSSyncer interface {
	// Type returns the type of the syncer.
	Type() string
	// IsCloneable checks to see if the VCS remote URL is cloneable. Any non-nil
	// error indicates there is a problem.
	//
	// All implementations should redact any sensitive information from the
	// error message.
	IsCloneable(ctx context.Context, repoName api.RepoName) error
	// Fetch tries to fetch updates from the remote to given directory.
	// 🚨 SECURITY:
	// Output returned from this function should NEVER contain sensitive information.
	// The VCSSyncer implementation is responsible of redacting potentially
	// sensitive data like secrets.
	// Progress reported through the progressWriter will be streamed line-by-line
	// with both LF and CR being valid line terminators.
	Fetch(ctx context.Context, repoName api.RepoName, dir common.GitDir, progressWriter io.Writer) error
}

type NewVCSSyncerOpts struct {
	ExternalServiceStore    database.ExternalServiceStore
	RepoStore               database.RepoStore
	DepsSvc                 *dependencies.Service
	Repo                    api.RepoName
	CoursierCacheDir        string
	RecordingCommandFactory *wrexec.RecordingCommandFactory
	Logger                  log.Logger
	FS                      gitserverfs.FS
	GetRemoteURLSource      func(ctx context.Context, repo api.RepoName) (RemoteURLSource, error)
}

func NewVCSSyncer(ctx context.Context, opts *NewVCSSyncerOpts) (VCSSyncer, error) {
	// We need an internal actor in case we are trying to access a private repo. We
	// only need access in order to find out the type of code host we're using, so
	// it's safe.
	r, err := opts.RepoStore.GetByName(actor.WithInternalActor(ctx), opts.Repo)
	if err != nil {
		return nil, errors.Wrap(err, "get repository")
	}

	extractOptions := func(connection any) (string, error) {
		for _, info := range r.Sources {
			extSvc, err := opts.ExternalServiceStore.GetByID(ctx, info.ExternalServiceID())
			if err != nil {
				return "", errors.Wrap(err, "get external service")
			}
			rawConfig, err := extSvc.Config.Decrypt(ctx)
			if err != nil {
				return "", err
			}
			normalized, err := jsonc.Parse(rawConfig)
			if err != nil {
				return "", errors.Wrap(err, "normalize JSON")
			}
			if err = jsoniter.Unmarshal(normalized, connection); err != nil {
				return "", errors.Wrap(err, "unmarshal JSON")
			}
			return extSvc.URN(), nil
		}
		return "", errors.Errorf("unexpected empty Sources map in %v", r)
	}

	out, err := func() (VCSSyncer, error) {
		switch r.ExternalRepo.ServiceType {
		case extsvc.TypePerforce:
			var c schema.PerforceConnection
			if _, err := extractOptions(&c); err != nil {
				return nil, err
			}

			return NewPerforceDepotSyncer(opts.Logger, opts.RecordingCommandFactory, opts.FS, &c, opts.GetRemoteURLSource), nil
		case extsvc.TypeJVMPackages:
			var c schema.JVMPackagesConnection
			if _, err := extractOptions(&c); err != nil {
				return nil, err
			}
			return NewJVMPackagesSyncer(&c, opts.DepsSvc, opts.GetRemoteURLSource, opts.CoursierCacheDir, opts.FS), nil
		case extsvc.TypeNpmPackages:
			var c schema.NpmPackagesConnection
			urn, err := extractOptions(&c)
			if err != nil {
				return nil, err
			}
			cli, err := npm.NewHTTPClient(urn, c.Registry, c.Credentials, httpcli.ExternalClientFactory)
			if err != nil {
				return nil, err
			}
			return NewNpmPackagesSyncer(c, opts.DepsSvc, cli, opts.FS, opts.GetRemoteURLSource), nil
		case extsvc.TypeGoModules:
			var c schema.GoModulesConnection
			urn, err := extractOptions(&c)
			if err != nil {
				return nil, err
			}
			cli := gomodproxy.NewClient(urn, c.Urls, httpcli.ExternalClientFactory)
			return NewGoModulesSyncer(&c, opts.DepsSvc, cli, opts.FS, opts.GetRemoteURLSource), nil
		case extsvc.TypePythonPackages:
			var c schema.PythonPackagesConnection
			urn, err := extractOptions(&c)
			if err != nil {
				return nil, err
			}
			cli, err := pypi.NewClient(urn, c.Urls, httpcli.ExternalClientFactory)
			if err != nil {
				return nil, err
			}
			return NewPythonPackagesSyncer(&c, opts.DepsSvc, cli, opts.FS, opts.GetRemoteURLSource), nil
		case extsvc.TypeRustPackages:
			var c schema.RustPackagesConnection
			urn, err := extractOptions(&c)
			if err != nil {
				return nil, err
			}
			cli, err := crates.NewClient(urn, httpcli.ExternalClientFactory)
			if err != nil {
				return nil, err
			}
			return NewRustPackagesSyncer(&c, opts.DepsSvc, cli, opts.FS, opts.GetRemoteURLSource), nil
		case extsvc.TypeRubyPackages:
			var c schema.RubyPackagesConnection
			urn, err := extractOptions(&c)
			if err != nil {
				return nil, err
			}
			cli, err := rubygems.NewClient(urn, c.Repository, httpcli.ExternalClientFactory)
			if err != nil {
				return nil, err
			}
			return NewRubyPackagesSyncer(&c, opts.DepsSvc, cli, opts.FS, opts.GetRemoteURLSource), nil
		}

		return NewGitRepoSyncer(opts.Logger, opts.RecordingCommandFactory, opts.GetRemoteURLSource), nil
	}()

	if err != nil {
		return nil, err
	}

	return newInstrumentedSyncer(out), nil
}

type notFoundError struct{ error }

func (e notFoundError) NotFound() bool { return true }

// RemoteURLSource is a source of a remote URL for a repository.
//
// The remote URL may change over time, so it's important to use this interface
// to get the remote URL every time instead of caching it at the call site.
type RemoteURLSource interface {
	// RemoteURL returns the latest remote URL for a repository.
	RemoteURL(ctx context.Context) (*vcs.URL, error)
}

// RemoteURLSourceFunc is an adapter to allow the use of ordinary functions as
// RemoteURLSource.
type RemoteURLSourceFunc func(ctx context.Context) (*vcs.URL, error)

func (f RemoteURLSourceFunc) RemoteURL(ctx context.Context) (*vcs.URL, error) {
	return f(ctx)
}

var _ RemoteURLSource = RemoteURLSourceFunc(nil)
