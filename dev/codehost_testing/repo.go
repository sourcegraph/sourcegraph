package codehost_testing

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v55/github"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Repo represents a GitHub repository in the scenario
type Repo struct {
	// s is the GithubScenario instance this repo is part of. Actions it creates will be added to this scenario
	s *GitHubScenario
	// team is the team this repo belongs to
	team *Team
	// org is the Org that owns this repo
	org *Org
	// name is the name of the repo
	name string
}

// Get returns the corresponding GitHub Repository object that was created by the `CreateOrg`
//
// This method will only return a Repository if the Scenario that created it has been applied otherwise
// it will panic.
func (r *Repo) Get(ctx context.Context) (*github.Repository, error) {
	if r.s.IsApplied() {
		return r.get(ctx)
	}
	r.s.t.Fatal("cannot retrieve repo before scenario is applied")
	return nil, nil
}

// get retrieves the GitHub repository without panicking if not applied. It is meant as an
// internal helper method while actions are getting applied.
func (r *Repo) get(ctx context.Context) (*github.Repository, error) {
	return r.s.client.GetRepo(ctx, r.org.name, r.name)
}

// AddTeam creats an action that will update the repo permissions so that the given team
// has access to this repo.
func (r *Repo) AddTeam(team *Team) {
	r.team = team
	action := &Action{
		Name: fmt.Sprintf("repo:team:%s:membership:%s", team.name, r.name),
		Apply: func(ctx context.Context) error {
			org, err := r.org.get(ctx)
			if err != nil {
				return err
			}

			repo, err := r.get(ctx)
			if err != nil {
				return err
			}

			team, err := r.team.get(ctx)
			if err != nil {
				return err
			}

			err = r.s.client.UpdateTeamRepoPermissions(ctx, org, team, repo)
			if err != nil {
				return err
			}
			return nil
		},
		Teardown: nil,
	}

	r.s.Append(action)
}

// SetPermissions adds an action that will set the permissions (public or private) for the repository
func (r *Repo) SetPermissions(private bool) {
	permissionKey := "private"
	if !private {
		permissionKey = "public"
	}
	action := &Action{
		Name: fmt.Sprintf("repo:permissions:%s:%s", r.name, permissionKey),
		Apply: func(ctx context.Context) error {
			repo, err := r.get(ctx)
			if err != nil {
				return err
			}
			repo.Private = &private

			org, err := r.org.get(ctx)
			if err != nil {
				return err
			}

			_, err = r.s.client.UpdateRepo(ctx, org, repo)
			if err != nil {
				return err
			}
			return err
		},
	}

	r.s.Append(action)
}

// WaitTillExists creates an action that waits for the repository to exist on GitHub. This action is especially
// useful for when a repo is forked since a forked repo doesn't immediately exist when requested on GitHub.
func (r *Repo) WaitTillExists() {
	action := &Action{
		Name: fmt.Sprintf("repo:exists:%s", r.name),
		Apply: func(ctx context.Context) error {
			var err error
			for range 5 {
				time.Sleep(1 * time.Second)
				_, err = r.get(ctx)
				if err == nil {
					return nil
				}
			}
			return errors.Newf("repo %q did not exist after waiting: %v", r.name, err)
		},
	}

	r.s.Append(action)
}
