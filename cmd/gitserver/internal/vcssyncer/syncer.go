package vcssyncer

import (
	"context"
	"io"
	"os/exec"

	jsoniter "github.com/json-iterator/go"

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
	"github.com/sourcegraph/sourcegraph/internal/vcs"
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
	IsCloneable(ctx context.Context, repoName api.RepoName, remoteURL *vcs.URL) error
	// Clone should clone the repo onto disk into the given tmpPath.
	//
	// For now, regardless of the VCSSyncer implementation, the result that ends
	// up in tmpPath is expected to be a valid Git repository and should be initially
	// optimized (repacked, commit-graph written, etc).
	//
	// targetDir is passed for reporting purposes, but should not be written to
	// during this process.
	//
	// Progress can be reported by writing to the progressWriter.
	// 🚨 SECURITY:
	// Content written to this writer should NEVER contain sensitive information.
	// The VCSSyncer implementation is responsible of redacting potentially
	// sensitive data like secrets.
	// Progress reported through the progressWriter will be streamed line-by-line
	// with both LF and CR being valid line terminators.
	Clone(ctx context.Context, repo api.RepoName, remoteURL *vcs.URL, targetDir common.GitDir, tmpPath string, progressWriter io.Writer) error
	// Fetch tries to fetch updates from the remote to given directory.
	// The revspec parameter is optional and specifies that the client is specifically
	// interested in fetching the provided revspec (example "v2.3.4^0").
	// For package hosts (vcsPackagesSyncer, npm/pypi/crates.io), the revspec is used
	// to lazily fetch package versions. More details at
	// https://github.com/sourcegraph/sourcegraph/issues/37921#issuecomment-1184301885
	// Beware that the revspec parameter can be any random user-provided string.
	Fetch(ctx context.Context, remoteURL *vcs.URL, repoName api.RepoName, dir common.GitDir, revspec string) ([]byte, error)
	// RemoteShowCommand returns the command to be executed for showing remote.
	RemoteShowCommand(ctx context.Context, remoteURL *vcs.URL) (cmd *exec.Cmd, err error)
}

type NewVCSSyncerOpts struct {
	ExternalServiceStore    database.ExternalServiceStore
	RepoStore               database.RepoStore
	DepsSvc                 *dependencies.Service
	Repo                    api.RepoName
	ReposDir                string
	CoursierCacheDir        string
	RecordingCommandFactory *wrexec.RecordingCommandFactory
	Logger                  log.Logger
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

	switch r.ExternalRepo.ServiceType {
	case extsvc.TypePerforce:
		var c schema.PerforceConnection
		if _, err := extractOptions(&c); err != nil {
			return nil, err
		}

		p4Home, err := gitserverfs.MakeP4HomeDir(opts.ReposDir)
		if err != nil {
			return nil, err
		}

		return NewPerforceDepotSyncer(opts.Logger, opts.RecordingCommandFactory, &c, opts.ReposDir, p4Home), nil
	case extsvc.TypeJVMPackages:
		var c schema.JVMPackagesConnection
		if _, err := extractOptions(&c); err != nil {
			return nil, err
		}
		return NewJVMPackagesSyncer(&c, opts.DepsSvc, opts.CoursierCacheDir, opts.ReposDir), nil
	case extsvc.TypeNpmPackages:
		var c schema.NpmPackagesConnection
		urn, err := extractOptions(&c)
		if err != nil {
			return nil, err
		}
		cli := npm.NewHTTPClient(urn, c.Registry, c.Credentials, httpcli.NewExternalClientFactory())
		if err != nil {
			return nil, err
		}
		return NewNpmPackagesSyncer(c, opts.DepsSvc, cli, opts.ReposDir), nil
	case extsvc.TypeGoModules:
		var c schema.GoModulesConnection
		urn, err := extractOptions(&c)
		if err != nil {
			return nil, err
		}
		cli := gomodproxy.NewClient(urn, c.Urls, httpcli.NewExternalClientFactory())
		return NewGoModulesSyncer(&c, opts.DepsSvc, cli, opts.ReposDir), nil
	case extsvc.TypePythonPackages:
		var c schema.PythonPackagesConnection
		urn, err := extractOptions(&c)
		if err != nil {
			return nil, err
		}
		cli := pypi.NewClient(urn, c.Urls, httpcli.NewExternalClientFactory())
		return NewPythonPackagesSyncer(&c, opts.DepsSvc, cli, opts.ReposDir), nil
	case extsvc.TypeRustPackages:
		var c schema.RustPackagesConnection
		urn, err := extractOptions(&c)
		if err != nil {
			return nil, err
		}
		cli := crates.NewClient(urn, httpcli.NewExternalClientFactory())
		return NewRustPackagesSyncer(&c, opts.DepsSvc, cli, opts.ReposDir), nil
	case extsvc.TypeRubyPackages:
		var c schema.RubyPackagesConnection
		urn, err := extractOptions(&c)
		if err != nil {
			return nil, err
		}
		cli := rubygems.NewClient(urn, c.Repository, httpcli.NewExternalClientFactory())
		return NewRubyPackagesSyncer(&c, opts.DepsSvc, cli, opts.ReposDir), nil
	}

	return NewGitRepoSyncer(opts.Logger, opts.RecordingCommandFactory), nil
}

type notFoundError struct{ error }

func (e notFoundError) NotFound() bool { return true }
