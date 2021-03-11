package cloneurls

import (
	"context"
	neturl "net/url"
	"strings"

	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/schema"
)

// ReposourceCloneURLToRepoName maps a Git clone URL (format documented here:
// https://git-scm.com/docs/git-clone#_git_urls_a_id_urls_a) to the corresponding repo name if there
// exists a code host configuration that matches the clone URL. Implicitly, it includes a code host
// configuration for github.com, even if one is not explicitly specified. Returns the empty string and nil
// error if a matching code host could not be found. This function does not actually check the code
// host to see if the repository actually exists.
func ReposourceCloneURLToRepoName(ctx context.Context, cloneURL string) (repoName api.RepoName, err error) {
	if repoName := reposource.CustomCloneURLToRepoName(cloneURL); repoName != "" {
		return repoName, nil
	}

	opt := database.ExternalServicesListOptions{
		Kinds: []string{
			extsvc.KindGitHub,
			extsvc.KindGitLab,
			extsvc.KindBitbucketServer,
			extsvc.KindAWSCodeCommit,
			extsvc.KindGitolite,
			extsvc.KindPhabricator,
			extsvc.KindOther,
		},
		LimitOffset: &database.LimitOffset{
			Limit: 500, // The number is randomly chosen
		},
	}
	for {
		svcs, err := database.GlobalExternalServices.List(ctx, opt)
		if err != nil {
			return "", errors.Wrap(err, "list")
		}
		if len(svcs) == 0 {
			break // No more results, exiting
		}
		opt.AfterID = svcs[len(svcs)-1].ID // Advance the cursor

		for _, svc := range svcs {
			cfg, err := extsvc.ParseConfig(svc.Kind, svc.Config)
			if err != nil {
				return "", errors.Wrap(err, "parse config")
			}

			var host string
			var rs reposource.RepoSource
			switch c := cfg.(type) {
			case *schema.GitHubConnection:
				rs = reposource.GitHub{GitHubConnection: c}
				host = c.Url
			case *schema.GitLabConnection:
				rs = reposource.GitLab{GitLabConnection: c}
				host = c.Url
			case *schema.BitbucketServerConnection:
				rs = reposource.BitbucketServer{BitbucketServerConnection: c}
				host = c.Url
			case *schema.AWSCodeCommitConnection:
				rs = reposource.AWS{AWSCodeCommitConnection: c}
				// AWS type does not have URL
			case *schema.GitoliteConnection:
				rs = reposource.Gitolite{GitoliteConnection: c}
				// Gitolite type does not have URL
			case *schema.PhabricatorConnection:
				// If this repository is mirrored by Phabricator, its clone URL should be
				// handled by a supported code host or an OtherExternalServiceConnection.
				// If this repository is hosted by Phabricator, it should be handled by
				// an OtherExternalServiceConnection.
				continue
			case *schema.OtherExternalServiceConnection:
				rs = reposource.Other{OtherExternalServiceConnection: c}
				host = c.Url
			default:
				return "", errors.Errorf("unexpected connection type: %T", cfg)
			}

			// Submodules are allowed to have relative paths for their .gitmodules URL.
			// In that case, we default to stripping any relative prefix and crafting
			// a new URL based on the reposource's host, if available.
			if strings.HasPrefix(cloneURL, "../") && host != "" {
				u, err := neturl.Parse(cloneURL)
				if err != nil {
					return "", err
				}
				base, err := neturl.Parse(host)
				if err != nil {
					return "", err
				}
				cloneURL = base.ResolveReference(u).String()
			}

			repoName, err := rs.CloneURLToRepoName(cloneURL)
			if err != nil {
				return "", err
			}
			if repoName != "" {
				return repoName, nil
			}
		}

		if len(svcs) < opt.Limit {
			break // Less results than limit means we've reached end
		}
	}

	// Fallback for github.com
	rs := reposource.GitHub{
		GitHubConnection: &schema.GitHubConnection{
			Url: "https://github.com",
		},
	}
	return rs.CloneURLToRepoName(cloneURL)
}
