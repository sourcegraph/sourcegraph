package main

import (
	"context"
	"log"
	"time"

	"github.com/google/go-github/v55/github"
	"github.com/sourcegraph/conc/pool"
)

// getGitHubRepos fetches the current repos on the GitHub instance for the given org name.
func getGitHubRepos(ctx context.Context, orgName string) []*github.Repository {
	p := pool.NewWithResults[[]*github.Repository]().WithMaxGoroutines(250)
	// 200k repos + some buffer space returning empty pages
	for i := range 2050 {
		writeInfo(out, "Fetching repo page %d", i)
		page := i
		p.Go(func() []*github.Repository {
			var resp *github.Response
			var reposPage []*github.Repository
			var err error

		retryListByOrg:
			if reposPage, resp, err = gh.Repositories.ListByOrg(ctx, orgName, &github.RepositoryListByOrgOptions{
				Type: "private",
				ListOptions: github.ListOptions{
					Page:    page,
					PerPage: 100,
				},
			}); err != nil {
				log.Printf("Failed getting repo page %d for org %s: %s", page, orgName, err)
			}
			if resp != nil && (resp.StatusCode == 502 || resp.StatusCode == 504) {
				time.Sleep(30 * time.Second)
				goto retryListByOrg
			}

			return reposPage
		})
	}
	var repos []*github.Repository
	for _, rr := range p.Wait() {
		repos = append(repos, rr...)
	}
	return repos
}

// getGitHubUsers fetches the existing users on the GitHub instance.
func getGitHubUsers(ctx context.Context) []*github.User {
	var users []*github.User
	var since int64
	for {
		// writeInfo(out, "Fetching user page, last ID seen is %d", since)
		usersPage, _, err := gh.Users.ListAll(ctx, &github.UserListOptions{
			Since:       since,
			ListOptions: github.ListOptions{PerPage: 100},
		})
		if err != nil {
			log.Fatal(err)
		}
		if len(usersPage) != 0 {
			since = *usersPage[len(usersPage)-1].ID
			users = append(users, usersPage...)
		} else {
			break
		}
	}

	return users
}

// getGitHubTeams fetches the current teams on the GitHub instance for the given orgs.
func getGitHubTeams(ctx context.Context, orgs []*org) []*github.Team {
	var teams []*github.Team
	var currentPage int
	for _, o := range orgs {
		for {
			// writeInfo(out, "Fetching team page %d for org %s", currentPage, o.Login)
			teamsPage, _, err := gh.Teams.ListTeams(ctx, o.Login, &github.ListOptions{
				Page:    currentPage,
				PerPage: 100,
			})
			// not returned in API response but necessary
			for _, t := range teamsPage {
				t.Organization = &github.Organization{Login: &o.Login}
			}
			if err != nil {
				log.Fatal(err)
			}
			if len(teamsPage) != 0 {
				currentPage++
				teams = append(teams, teamsPage...)
			} else {
				break
			}
		}
		currentPage = 0
	}

	return teams
}

// getGitHubOrgs fetches the current orgs on the GitHub instance.
func getGitHubOrgs(ctx context.Context) []*github.Organization {
	var orgs []*github.Organization
	var since int64
	for {
		// writeInfo(out, "Fetching org page, last ID seen is %d", since)
		orgsPage, _, err := gh.Organizations.ListAll(ctx, &github.OrganizationsListOptions{
			Since:       since,
			ListOptions: github.ListOptions{PerPage: 100},
		})
		if err != nil {
			log.Fatal(err)
		}
		if len(orgsPage) != 0 {
			since = *orgsPage[len(orgsPage)-1].ID
			orgs = append(orgs, orgsPage...)
		} else {
			break
		}
	}

	return orgs
}
