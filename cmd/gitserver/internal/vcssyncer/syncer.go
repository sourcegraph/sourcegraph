package vcssyncer

import (
	"context"
	"io"
	"os"
	"os/exec"

	jsoniter "github.com/json-iterator/go"

	"github.com/sourcegraph/sourcegraph/internal/vcs"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/executil"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/perforce"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/urlredactor"
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

			return NewGeneratingSyncer(
				opts.Logger,
				opts.RecordingCommandFactory,
				opts.FS,
				func(ctx context.Context, repoName api.RepoName, dir common.GitDir, progressWriter io.Writer) error {
					fusionConfig := configureFusionClient(&c)

					source, err := opts.GetRemoteURLSource(ctx, repoName)
					if err != nil {
						return errors.Wrap(err, "getting remote URL source")
					}

					remoteURL, err := source.RemoteURL(ctx)
					if err != nil {
						return errors.Wrap(err, "getting remote URL") // This should never happen for Perforce
					}

					p4user, p4passwd, p4port, depot, err := perforce.DecomposePerforceRemoteURL(remoteURL)
					if err != nil {
						return errors.Wrap(err, "invalid perforce remote URL")
					}

					// First, do a quick check if we can reach the Perforce server.
					tryWrite(opts.Logger, progressWriter, "Checking Perforce server connection\n")
					err = perforce.P4TestWithTrust(ctx, opts.FS, perforce.P4TestWithTrustArguments{
						P4Port:   p4port,
						P4User:   p4user,
						P4Passwd: p4passwd,
					})
					if err != nil {
						return errors.Wrap(err, "verifying connection to perforce server")
					}
					tryWrite(opts.Logger, progressWriter, "Perforce server connection succeeded\n")

					var cmd *exec.Cmd
					if fusionConfig.Enabled {
						tryWrite(opts.Logger, progressWriter, "Converting depot using p4-fusion\n")
						// Example: p4-fusion --path //depot/... --user $P4USER --src clones/ --networkThreads 64 --printBatch 10 --port $P4PORT --lookAhead 2000 --retries 10 --refresh 100
						cmd = buildP4FusionCmd(ctx, fusionConfig, depot, p4user, string(dir), p4port)
					} else {
						// TODO: This used to call the following for clone:
						// tryWrite(opts.Logger, progressWriter, "Converting depot using git-p4\n")
						// // Example: git p4 clone --bare --max-changes 1000 //Sourcegraph/@all /tmp/clone-584194180/.git
						// args := append([]string{"p4", "clone", "--bare"}, s.p4CommandOptions()...)
						// args = append(args, depot+"@all", tmpPath)
						// cmd = exec.CommandContext(ctx, "git", args...)
						tryWrite(opts.Logger, progressWriter, "Converting depot using git-p4\n")
						// Example: git p4 sync --max-changes 1000
						args := append([]string{"p4", "sync"}, p4CommandOptions(&c)...)
						cmd = exec.CommandContext(ctx, "git", args...)
					}
					cmd.Env, err = p4CommandEnv(opts.FS, string(dir), p4port, p4user, p4passwd, c.P4Client)
					if err != nil {
						return errors.Wrap(err, "failed to build p4 command env")
					}
					dir.Set(cmd)

					// TODO(keegancsmith)(indradhanush) This is running a remote command and
					// we have runRemoteGitCommand which sets TLS settings/etc. Do we need
					// something for p4?
					redactor := urlredactor.New(remoteURL)
					wrCmd := opts.RecordingCommandFactory.WrapWithRepoName(ctx, opts.Logger, repoName, cmd).WithRedactorFunc(redactor.Redact)
					// Note: Using RunCommandWriteOutput here does NOT store the output of the
					// command as the command output of the wrexec command, because the pipes are
					// already used.
					exitCode, err := executil.RunCommandWriteOutput(ctx, wrCmd, progressWriter, redactor.Redact)
					if err != nil {
						return errors.Wrapf(err, "failed to run p4->git conversion: exit code %d", exitCode)
					}

					if !fusionConfig.Enabled {
						p4home, err := opts.FS.P4HomeDir()
						if err != nil {
							return errors.Wrap(err, "failed to create p4home")
						}

						// Force update "master" to "refs/remotes/p4/master" where changes are synced into
						cmd := wrexec.CommandContext(ctx, nil, "git", "branch", "-f", "master", "refs/remotes/p4/master")
						cmd.Cmd.Env = append(os.Environ(),
							"P4PORT="+p4port,
							"P4USER="+p4user,
							"P4PASSWD="+p4passwd,
							"HOME="+p4home,
						)
						dir.Set(cmd.Cmd)
						if output, err := cmd.CombinedOutput(); err != nil {
							return errors.Wrapf(err, "failed to force update branch with output %q", string(output))
						}
					}

					return nil
				},
				func(ctx context.Context, repoName api.RepoName) error {
					source, err := opts.GetRemoteURLSource(ctx, repoName)
					if err != nil {
						return errors.Wrap(err, "getting remote URL source")
					}

					remoteURL, err := source.RemoteURL(ctx)
					if err != nil {
						return errors.Wrap(err, "getting remote URL") // This should never happen for Perforce
					}

					username, password, host, path, err := perforce.DecomposePerforceRemoteURL(remoteURL)
					if err != nil {
						return errors.Wrap(err, "invalid perforce remote URL")
					}

					return perforce.IsDepotPathCloneable(ctx, opts.FS, perforce.IsDepotPathCloneableArguments{
						P4Port:   host,
						P4User:   username,
						P4Passwd: password,

						DepotPath: path,
					})
				},
				"perforce",
			), nil
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
