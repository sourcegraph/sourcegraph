package externallink

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/inconshreveable/log15"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
)

// Repository returns the external links for a repository.
//
// For example, a repository might have 2 external links, one to its origin repository on GitHub.com
// and one to the repository on Phabricator.
func Repository(ctx context.Context, db dbutil.DB, repo *types.Repo) (links []*Resolver, err error) {
	phabRepo, link, serviceType := linksForRepository(ctx, db, repo)
	if phabRepo != nil {
		links = append(links, NewResolver(
			strings.TrimSuffix(phabRepo.URL, "/")+"/diffusion/"+phabRepo.Callsign,
			extsvc.TypePhabricator,
		))
	}
	if link != nil && link.Root != "" {
		links = append(links, NewResolver(link.Root, serviceType))
	}
	return links, nil
}

// FileOrDir returns the external links for a file or directory in a repository.
func FileOrDir(ctx context.Context, db dbutil.DB, repo *types.Repo, rev, path string, isDir bool) (links []*Resolver, err error) {
	rev = url.PathEscape(rev)

	phabRepo, link, serviceType := linksForRepository(ctx, db, repo)
	if phabRepo != nil {
		// We need a branch name to construct the Phabricator URL.
		branchName, _, _, err := git.ExecSafe(ctx, repo.Name, []string{"symbolic-ref", "--short", "HEAD"})
		branchName = bytes.TrimSpace(branchName)
		if err == nil && string(branchName) != "" {
			links = append(links, NewResolver(
				fmt.Sprintf("%s/source/%s/browse/%s/%s;%s", strings.TrimSuffix(phabRepo.URL, "/"), phabRepo.Callsign, url.PathEscape(string(branchName)), path, rev),
				extsvc.TypePhabricator,
			))
		}
	}

	if link != nil {
		var url string
		if isDir {
			url = link.Tree
		} else {
			url = link.Blob
		}
		if url != "" {
			url = strings.NewReplacer("{rev}", rev, "{path}", path).Replace(url)
			links = append(links, NewResolver(url, serviceType))
		}
	}

	return links, nil
}

// Commit returns the external links for a commit in a repository.
func Commit(ctx context.Context, db dbutil.DB, repo *types.Repo, commitID api.CommitID) (links []*Resolver, err error) {
	commitStr := url.PathEscape(string(commitID))

	phabRepo, link, serviceType := linksForRepository(ctx, db, repo)
	if phabRepo != nil {
		links = append(links, NewResolver(
			fmt.Sprintf("%s/r%s%s", strings.TrimSuffix(phabRepo.URL, "/"), phabRepo.Callsign, commitStr),
			extsvc.TypePhabricator,
		))
	}

	if link != nil && link.Commit != "" {
		links = append(links, NewResolver(
			strings.Replace(link.Commit, "{commit}", commitStr, -1),
			serviceType,
		))
	}

	return links, nil
}

// linksForRepository gets the information necessary to construct links to resources within this
// repository.
//
// It logs errors to the trace but does not return errors, because external links are not worth
// failing any request for.
func linksForRepository(ctx context.Context, db dbutil.DB, repo *types.Repo) (phabRepo *types.PhabricatorRepo, link *protocol.RepoLinks, serviceType string) {
	span, ctx := ot.StartSpanFromContext(ctx, "externallink.linksForRepository")
	defer span.Finish()
	span.SetTag("Repo", repo.Name)
	span.SetTag("ExternalRepo", repo.ExternalRepo)

	var err error
	phabRepo, err = database.Phabricator(db).GetByName(ctx, repo.Name)
	if err != nil && !errcode.IsNotFound(err) {
		ext.Error.Set(span, true)
		span.SetTag("phabErr", err.Error())
	}

	// Look up repo links in the repo-updater. This supplies links from code host APIs.
	info, err := repoupdater.DefaultClient.RepoLookup(ctx, protocol.RepoLookupArgs{
		Repo: repo.Name,
	})
	if err != nil {
		ext.Error.Set(span, true)
		span.SetTag("repoUpdaterErr", err.Error())
		log15.Warn("linksForRepository failed to RepoLookup", "repo", repo.Name, "error", err)
		linksForRepositoryFailed.Inc()
	}
	if info != nil && info.Repo != nil {
		link = info.Repo.Links
		serviceType = info.Repo.ExternalRepo.ServiceType
	}

	return phabRepo, link, serviceType
}

var linksForRepositoryFailed = promauto.NewCounter(prometheus.CounterOpts{
	Name: "src_graphql_links_for_repository_failed_total",
	Help: "The total number of times the GraphQL field LinksForRepository failed.",
})
