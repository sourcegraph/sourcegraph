package tst

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-github/v53/github"

	"github.com/sourcegraph/sourcegraph/dev/tst/config"
)

type GitHubScenarioBuilder struct {
	test     *testing.T
	client   *GitHubClient
	store    *ScenarioStore
	actions  *actionRunner
	reporter Reporter
}

type GitHubScenario struct {
	client *GitHubClient
	users  []*github.User
	teams  []*github.Team
	repos  []*github.Repository
	org    *github.Organization
}

func NewGitHubScenario(ctx context.Context, cfg *config.Config, t *testing.T) (*GitHubScenarioBuilder, error) {
	client, err := NewGitHubClient(ctx, cfg.GitHub)
	if err != nil {
		return nil, err
	}
	return &GitHubScenarioBuilder{
		test:     t,
		client:   client,
		store:    NewStore(t),
		actions:  NewActionManager(t),
		reporter: NoopReporter{},
	}, nil
}

func (sb *GitHubScenarioBuilder) T(t *testing.T) *GitHubScenarioBuilder {
	sb.test = t
	sb.actions.T = t
	return sb
}

func (sb *GitHubScenarioBuilder) Verbose() {
	sb.reporter = ConsoleReporter{}
	sb.actions.Reporter = sb.reporter
}

func (sb *GitHubScenarioBuilder) Quiet() {
	sb.reporter = NoopReporter{}
	sb.actions.Reporter = sb.reporter
}

func (sb *GitHubScenarioBuilder) Org(name string) *GitHubScenarioBuilder {
	sb.test.Helper()
	org := NewGitHubScenarioOrg(name)
	sb.actions.AddSetup(org.CreateOrgAction(sb.client), org.UpdateOrgPermissionsAction(sb.client))
	sb.actions.AddTeardown(org.DeleteOrgAction(sb.client))
	return sb
}

func (sb *GitHubScenarioBuilder) Users(users ...GitHubScenarioUser) *GitHubScenarioBuilder {
	sb.test.Helper()
	for _, u := range users {
		if u == Admin {
			sb.actions.AddSetup(u.GetUserAction(sb.client))
		} else {
			sb.actions.AddSetup(u.CreateUserAction(sb.client))
			sb.actions.AddTeardown(u.DeleteUserAction(sb.client))
		}
	}
	return sb
}

func Team(name string, u ...GitHubScenarioUser) *GitHubScenarioTeam {
	return NewGitHubScenarioTeam(name, u...)
}

func (sb *GitHubScenarioBuilder) Teams(teams ...*GitHubScenarioTeam) *GitHubScenarioBuilder {
	sb.test.Helper()
	for _, t := range teams {
		sb.actions.AddSetup(t.CreateTeamAction(sb.client), t.AssignTeamAction(sb.client))
		sb.actions.AddTeardown(t.DeleteTeamAction(sb.client))
	}

	return sb
}

func (sb *GitHubScenarioBuilder) Repos(repos ...*GitHubScenarioRepo) *GitHubScenarioBuilder {
	sb.test.Helper()
	for _, r := range repos {
		if r.fork {
			sb.actions.AddSetup(r.ForkRepoAction(sb.client), r.GetRepoAction(sb.client))
			// Seems like you can't change permissions for a repo fork
			//sb.setupActions = append(sb.setupActions, r.SetPermissionsAction(sb.client))
			sb.actions.AddTeardown(r.DeleteRepoAction(sb.client))
		} else {
			sb.actions.AddSetup(r.NewRepoAction(sb.client),
				r.GetRepoAction(sb.client),
				r.InitLocalRepoAction(sb.client),
				r.SetPermissionsAction(sb.client),
			)

			sb.actions.AddTeardown(r.DeleteRepoAction(sb.client))
		}
		sb.actions.AddSetup(r.AssignTeamAction(sb.client))
	}

	return sb
}

func PublicRepo(name string, team string, fork bool) *GitHubScenarioRepo {
	return NewGitHubScenarioRepo(name, team, fork, false)
}

func PrivateRepo(name string, team string, fork bool) *GitHubScenarioRepo {
	return NewGitHubScenarioRepo(name, team, fork, true)
}

func (sb *GitHubScenarioBuilder) Setup(ctx context.Context) (GitHubScenario, func(context.Context) error, error) {
	sb.test.Helper()
	sb.reporter.Writeln("-- Setup --")
	start := time.Now().UTC()
	err := sb.actions.Apply(ctx, sb.store, sb.actions.setup, false)
	sb.reporter.Writef("Run complete: %s\n", time.Now().UTC().Sub(start))
	return GitHubScenario{}, sb.TearDown, err
}

func (sb *GitHubScenarioBuilder) TearDown(ctx context.Context) error {
	sb.test.Helper()
	sb.reporter.Writeln("-- Teardown --")
	start := time.Now().UTC()
	err := sb.actions.Apply(ctx, sb.store, reverse(sb.actions.teardown), false)
	sb.reporter.Writef("Run complete: %s\n", time.Now().UTC().Sub(start))
	return err
}

func (sb *GitHubScenarioBuilder) String() string {
	return sb.actions.String()
}
