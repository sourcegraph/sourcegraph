package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/go-github/v55/github"
	"github.com/sourcegraph/conc/pool"

	"github.com/sourcegraph/sourcegraph/lib/output"
)

// create executes a number of steps:
// 1. Creates the amount of users and teams as defined in the flags.
// 2. Assigns the users to the teams in equal shares.
// 3. Assigns the externally created repositories to the orgs, teams, and users to replicate different scale variations.
func create(ctx context.Context, orgs []*org, cfg config) {
	var err error

	// load or generate users
	var users []*user
	if users, err = store.loadUsers(); err != nil {
		log.Fatalf("Failed to load users from state: %s", err)
	}

	if len(users) == 0 {
		if users, err = store.generateUsers(cfg); err != nil {
			log.Fatalf("Failed to generate users: %s", err)
		}
		writeSuccess(out, "generated user jobs in %s", cfg.resume)
	} else {
		writeSuccess(out, "resuming user jobs from %s", cfg.resume)
	}

	// load or generate teams
	var teams []*team
	if teams, err = store.loadTeams(); err != nil {
		log.Fatalf("Failed to load teams from state: %s", err)
	}

	if len(teams) == 0 {
		if teams, err = store.generateTeams(cfg); err != nil {
			log.Fatalf("Failed to generate teams: %s", err)
		}
		writeSuccess(out, "generated team jobs in %s", cfg.resume)
	} else {
		writeSuccess(out, "resuming team jobs from %s", cfg.resume)
	}

	var repos []*repo
	if repos, err = store.loadRepos(); err != nil {
		log.Fatalf("Failed to load repos from state: %s", err)
	}

	if len(repos) == 0 {
		remoteRepos := getGitHubRepos(ctx, cfg.reposSourceOrg)

		if repos, err = store.insertRepos(remoteRepos); err != nil {
			log.Fatalf("Failed to insert repos in state: %s", err)
		}
		writeSuccess(out, "Fetched %d private repos and stored in state", len(remoteRepos))
	} else {
		writeSuccess(out, "resuming repo jobs from %s", cfg.resume)
	}

	bars := []output.ProgressBar{
		{Label: "Creating orgs", Max: float64(cfg.subOrgCount + 1)},
		{Label: "Creating teams", Max: float64(cfg.teamCount)},
		{Label: "Creating users", Max: float64(cfg.userCount)},
		{Label: "Adding users to teams", Max: float64(cfg.userCount)},
		{Label: "Assigning repos", Max: float64(len(repos))},
	}
	if cfg.generateTokens {
		bars = append(bars, output.ProgressBar{Label: "Generating OAuth tokens", Max: float64(cfg.userCount)})
	}
	progress = out.Progress(bars, nil)
	var usersDone int64
	var orgsDone int64
	var teamsDone int64
	var tokensDone int64
	var membershipsDone int64
	var reposDone int64

	p := pool.New().WithMaxGoroutines(1000)

	for _, o := range orgs {
		currentOrg := o
		p.Go(func() {
			currentOrg.executeCreate(ctx, cfg.orgAdmin, &orgsDone)
		})
	}
	p.Wait()

	// Default permission is "read"; we need members to not have access by default on the main organisation.
	defaultRepoPermission := "none"
	var res *github.Response
	_, res, err = gh.Organizations.Edit(ctx, "main-org", &github.Organization{DefaultRepoPermission: &defaultRepoPermission})
	if err != nil && res.StatusCode != 409 {
		// 409 means the base repo permissions are currently being updated already due to a previous run
		log.Fatalf("Failed to make main-org private by default: %s", err)
	}

	for _, t := range teams {
		currentTeam := t
		p.Go(func() {
			currentTeam.executeCreate(ctx, &teamsDone)
		})
	}
	p.Wait()

	for _, u := range users {
		currentUser := u
		p.Go(func() {
			currentUser.executeCreate(ctx, &usersDone)
		})
	}
	p.Wait()

	membershipsPerTeam := int(math.Ceil(float64(cfg.userCount) / float64(cfg.teamCount)))
	p2 := pool.New().WithMaxGoroutines(100)

	for i, t := range teams {
		currentTeam := t
		currentIter := i
		var usersToAssign []*user

		for j := currentIter * membershipsPerTeam; j < ((currentIter + 1) * membershipsPerTeam); j++ {
			usersToAssign = append(usersToAssign, users[j])
		}

		p2.Go(func() {
			currentTeam.executeCreateMemberships(ctx, usersToAssign, &membershipsDone)
		})
	}
	p2.Wait()

	mainOrg, orgRepos := categorizeOrgRepos(cfg, repos, orgs)
	executeAssignOrgRepos(ctx, orgRepos, users, &reposDone, p2)
	p2.Wait()

	// 0.5% repos with only users attached
	amountReposWithOnlyUsers := int(math.Ceil(float64(len(repos)) * 0.005))
	reposWithOnlyUsers := orgRepos[mainOrg][:amountReposWithOnlyUsers]
	// slice out the user repos
	orgRepos[mainOrg] = orgRepos[mainOrg][amountReposWithOnlyUsers:]

	teamRepos := categorizeTeamRepos(cfg, orgRepos[mainOrg], teams)
	userRepos := categorizeUserRepos(reposWithOnlyUsers, users)

	executeAssignTeamRepos(ctx, teamRepos, &reposDone, p2)
	p2.Wait()

	executeAssignUserRepos(ctx, userRepos, &reposDone, p2)
	p2.Wait()

	if cfg.generateTokens {
		generateUserOAuthCsv(ctx, users, tokensDone)
	}
}

// executeCreate checks whether the org already exists. If it does not, it is created.
// The result is stored in the local state.
func (o *org) executeCreate(ctx context.Context, orgAdmin string, orgsDone *int64) {
	if o.Created && o.Failed == "" {
		atomic.AddInt64(orgsDone, 1)
		progress.SetValue(0, float64(*orgsDone))
		return
	}

	existingOrg, resp, oErr := gh.Organizations.Get(ctx, o.Login)
	if oErr != nil && resp.StatusCode != 404 {
		writeFailure(out, "Failed to get org %s, reason: %s\n", o.Login, oErr)
		return
	}

	oErr = nil
	if existingOrg != nil {
		o.Created = true
		o.Failed = ""
		atomic.AddInt64(orgsDone, 1)
		progress.SetValue(0, float64(*orgsDone))

		if oErr = store.saveOrg(o); oErr != nil {
			log.Fatal(oErr)
		}
		return
	}

	_, _, oErr = gh.Admin.CreateOrg(ctx, &github.Organization{Login: &o.Login}, orgAdmin)

	if oErr != nil {
		writeFailure(out, "Failed to create org with login %s, reason: %s\n", o.Login, oErr)
		o.Failed = oErr.Error()
		if oErr = store.saveOrg(o); oErr != nil {
			log.Fatal(oErr)
		}
		return
	}

	atomic.AddInt64(orgsDone, 1)
	progress.SetValue(0, float64(*orgsDone))

	o.Created = true
	o.Failed = ""
	if oErr = store.saveOrg(o); oErr != nil {
		log.Fatal(oErr)
	}

	// writeSuccess(out, "Created org with login %s", o.Login)
}

// executeCreate checks whether the team already exists. If it does not, it is created.
// The result is stored in the local state.
func (t *team) executeCreate(ctx context.Context, teamsDone *int64) {
	if t.Created && t.Failed == "" {
		atomic.AddInt64(teamsDone, 1)
		progress.SetValue(1, float64(*teamsDone))
		return
	}

	existingTeam, resp, tErr := gh.Teams.GetTeamBySlug(ctx, t.Org, t.Name)

	if tErr != nil && resp.StatusCode != 404 {
		writeFailure(out, "failed to get team with name %s, reason: %s\n", t.Name, tErr)
		return
	}

	if existingTeam != nil {
		t.Created = true
		t.Failed = ""
		atomic.AddInt64(teamsDone, 1)
		progress.SetValue(1, float64(*teamsDone))

		if tErr = store.saveTeam(t); tErr != nil {
			log.Fatal(tErr)
		}
	} else {
		// Create the team if not exists
		var res *github.Response
		var err error
	retryCreateTeam:
		if _, res, err = gh.Teams.CreateTeam(ctx, t.Org, github.NewTeam{Name: t.Name}); err != nil {
			if err = t.setFailedAndSave(err); err != nil {
				log.Fatalf("Failed saving to state: %s", err)
			}
		}
		if res != nil && (res.StatusCode == 502 || res.StatusCode == 504) {
			// give some breathing room
			time.Sleep(30 * time.Second)
			goto retryCreateTeam
		}

		t.Created = true
		t.Failed = ""
		atomic.AddInt64(teamsDone, 1)
		progress.SetValue(1, float64(*teamsDone))

		if tErr = store.saveTeam(t); tErr != nil {
			log.Fatal(tErr)
		}
	}
}

// executeCreate checks whether the user already exists. If it does not, it is created.
// The result is stored in the local state.
func (u *user) executeCreate(ctx context.Context, usersDone *int64) {
	if u.Created && u.Failed == "" {
		atomic.AddInt64(usersDone, 1)
		progress.SetValue(2, float64(*usersDone))
		return
	}

	existingUser, resp, uErr := gh.Users.Get(ctx, u.Login)
	if uErr != nil && resp.StatusCode != 404 {
		writeFailure(out, "Failed to get user %s, reason: %s\n", u.Login, uErr)
		return
	}

	uErr = nil
	if existingUser != nil {
		u.Created = true
		u.Failed = ""
		if uErr = store.saveUser(u); uErr != nil {
			log.Fatal(uErr)
		}
		// writeInfo(out, "user with login %s already exists", u.Login)
		atomic.AddInt64(usersDone, 1)
		progress.SetValue(2, float64(*usersDone))
		return
	}

	_, _, uErr = gh.Admin.CreateUser(ctx, u.Login, u.Email)
	if uErr != nil {
		writeFailure(out, "Failed to create user with login %s, reason: %s\n", u.Login, uErr)
		u.Failed = uErr.Error()
		if uErr = store.saveUser(u); uErr != nil {
			log.Fatal(uErr)
		}
		return
	}

	u.Created = true
	u.Failed = ""
	atomic.AddInt64(usersDone, 1)
	progress.SetValue(2, float64(*usersDone))
	if uErr = store.saveUser(u); uErr != nil {
		log.Fatal(uErr)
	}

	// writeSuccess(out, "Created user with login %s", u.Login)
}

// executeCreateMemberships does the following per user:
// 1. It sets the user as a member of the team's parent org. This is an idempotent operation.
// 2. It adds the user to the team. This is an idempotent operation.
// 3. The result is stored in the local state.
func (t *team) executeCreateMemberships(ctx context.Context, users []*user, membershipsDone *int64) {
	// users need to be member of the team's parent org to join the team
	userState := "active"
	userRole := "member"

	for _, u := range users {
		// add user to team's parent org first
		var res *github.Response
		var err error
	retryEditOrgMembership:
		if _, res, err = gh.Organizations.EditOrgMembership(ctx, u.Login, t.Org, &github.Membership{
			State:        &userState,
			Role:         &userRole,
			Organization: &github.Organization{Login: &t.Org},
			User:         &github.User{Login: &u.Login},
		}); res != nil {
			if err = t.setFailedAndSave(err); err != nil {
				log.Fatal(err)
			}
			continue
		}
		if res != nil && (res.StatusCode == 502 || res.StatusCode == 504) {
			time.Sleep(30 * time.Second)
			goto retryEditOrgMembership
		}

	retryAddTeamMembership:
		if _, res, err = gh.Teams.AddTeamMembershipBySlug(ctx, t.Org, t.Name, u.Login, nil); err != nil {
			if err = t.setFailedAndSave(err); err != nil {
				log.Fatal(err)
			}
			continue
		}
		if res != nil && (res.StatusCode == 502 || res.StatusCode == 504) {
			time.Sleep(30 * time.Second)
			goto retryAddTeamMembership
		}

		t.TotalMembers += 1
		atomic.AddInt64(membershipsDone, 1)
		progress.SetValue(3, float64(*membershipsDone))

		if err = store.saveTeam(t); err != nil {
			log.Fatal(err)
		}
	}
}

// categorizeOrgRepos takes the complete list of repos and assigns 1% of it to the specified amount of sub-orgs.
// The remainder is assigned to the main org.
func categorizeOrgRepos(cfg config, repos []*repo, orgs []*org) (*org, map[*org][]*repo) {
	repoCategories := make(map[*org][]*repo)

	// 1% of repos divided equally over sub-orgs
	var mainOrg *org
	var subOrgs []*org
	for _, o := range orgs {
		if strings.HasPrefix(o.Login, "sub-org") {
			subOrgs = append(subOrgs, o)
		} else {
			mainOrg = o
		}
	}

	if cfg.subOrgCount != 0 {
		reposPerSubOrg := (len(repos) / 100) / cfg.subOrgCount
		for i, o := range subOrgs {
			subOrgRepos := repos[i*reposPerSubOrg : (i+1)*reposPerSubOrg]
			repoCategories[o] = subOrgRepos
		}

		// rest assigned to main org
		repoCategories[mainOrg] = repos[len(subOrgs)*reposPerSubOrg:]
	} else {
		// no sub-orgs defined, so everything can be assigned to the main org
		repoCategories[mainOrg] = repos
	}

	return mainOrg, repoCategories
}

// executeAssignOrgRepos transfers the repos categorised per org from the import org to the new owner.
// If sub-orgs are defined, they immediately get assigned 2000 users. The sub-orgs are used for org-level permission syncing.
func executeAssignOrgRepos(ctx context.Context, reposPerOrg map[*org][]*repo, users []*user, reposDone *int64, p *pool.Pool) {
	for o, repos := range reposPerOrg {
		currentOrg := o
		currentRepos := repos

		var res *github.Response
		var err error
		for _, r := range currentRepos {
			currentRepo := r
			p.Go(func() {
				if currentOrg.Login == currentRepo.Owner {
					// writeInfo(out, "Repository %s already owned by %s", r.Name, r.Owner)
					// The repository is already transferred
					atomic.AddInt64(reposDone, 1)
					progress.SetValue(4, float64(*reposDone))
					return
				}

			retryTransfer:
				if _, res, err = gh.Repositories.Transfer(ctx, "blank200k", currentRepo.Name, github.TransferRequest{NewOwner: currentOrg.Login}); err != nil {
					if _, ok := err.(*github.AcceptedError); ok {
						// writeInfo(out, "Repository %s scheduled for transfer as a background job", r.Name)
						// AcceptedError means the transfer is scheduled as a background job
					} else {
						log.Fatalf("Failed to transfer repository %s from %s to %s: %s", currentRepo.Name, currentRepo.Owner, currentOrg.Login, err)
					}
				}

				if res != nil && (res.StatusCode == 502 || res.StatusCode == 504) {
					time.Sleep(30 * time.Second)
					goto retryTransfer
				}

				if res.StatusCode == 422 {
					body, err := io.ReadAll(res.Body)
					if err != nil {
						log.Fatalf("Failed reading response body: %s", err)
					}
					// Usually this means the repository is already transferred but not yet saved in the state, but otherwise:
					if !strings.Contains(string(body), "Repositories cannot be transferred to the original owner") {
						log.Fatalf("Status 422, body: %s", body)
					}
				}

				// writeInfo(out, "Repository %s transferred to %s", r.Name, r.Owner)
				atomic.AddInt64(reposDone, 1)
				progress.SetValue(4, float64(*reposDone))
				currentRepo.Owner = currentOrg.Login
				if err = store.saveRepo(currentRepo); err != nil {
					log.Fatalf("Failed to save repository %s: %s", currentRepo.Name, err)
				}
			})
		}

		p.Wait()

		if strings.HasPrefix(currentOrg.Login, "sub-org") {
			// add 2000 users to sub-orgs
			index, err := strconv.ParseInt(strings.TrimPrefix(currentOrg.Login, "sub-org-"), 10, 32)
			if err != nil {
				log.Fatalf("Failed to parse index from sub-org id: %s", err)
			}
			usersToAdd := users[index*2000 : (index+1)*2000]

			for _, u := range usersToAdd {
				currentUser := u
				var uRes *github.Response
				var uErr error
				p.Go(func() {
				retryEditOrgMembership:
					memberState := "active"
					memberRole := "member"

					if _, uRes, uErr = gh.Organizations.EditOrgMembership(ctx, currentUser.Login, currentOrg.Login, &github.Membership{
						State: &memberState,
						Role:  &memberRole,
					}); uErr != nil {
						log.Fatalf("Failed edit membership of user %s in org %s: %s", currentUser.Login, currentOrg.Login, uErr)
					}

					if uRes != nil && (uRes.StatusCode == 502 || uRes.StatusCode == 504) {
						time.Sleep(30 * time.Second)
						goto retryEditOrgMembership
					}
				})
			}
		}
	}
}

// categorizeTeamRepos divides the provided repos over the teams as follows:
// 1. 95% of teams get a 'small' (remainder of total) amount of repos
// 2. 4% of teams get a 'medium' (0.04% of total) amount of repos
// 3. 1% of teams get a 'large' (0.5% of total) amount of repos
func categorizeTeamRepos(cfg config, mainOrgRepos []*repo, teams []*team) map[*team][]*repo {
	// 1% of teams
	teamsLarge := int(math.Ceil(float64(cfg.teamCount) * 0.01))
	// 0.5% of repos per team
	reposLarge := int(math.Floor(float64(len(mainOrgRepos)) * 0.005))

	// 4% of teams
	teamsMedium := int(math.Ceil(float64(cfg.teamCount) * 0.04))
	// 0.04% of repos per team
	reposMedium := int(math.Floor(float64(len(mainOrgRepos)) * 0.0004))

	// 95% of teams
	teamsSmall := int(math.Ceil(float64(cfg.teamCount) * 0.95))
	// remainder of repos divided over remainder of teams
	reposSmall := int(math.Floor(float64(len(mainOrgRepos)-(reposMedium*teamsMedium)-(reposLarge*teamsLarge)) / float64(teamsSmall)))

	teamCategories := make(map[*team][]*repo)

	for i := range teamsSmall {
		currentTeam := teams[i]
		teamRepos := mainOrgRepos[i*reposSmall : (i+1)*reposSmall]
		teamCategories[currentTeam] = teamRepos
	}

	for i := range teamsMedium {
		currentTeam := teams[teamsSmall+i]
		startIndex := (teamsSmall * reposSmall) + (i * reposMedium)
		endIndex := (teamsSmall * reposSmall) + ((i + 1) * reposMedium)
		teamRepos := mainOrgRepos[startIndex:endIndex]
		teamCategories[currentTeam] = teamRepos
	}

	for i := range teamsLarge {
		currentTeam := teams[teamsSmall+teamsMedium+i]
		startIndex := (teamsSmall * reposSmall) + (teamsMedium * reposMedium) + (i * reposLarge)
		endIndex := (teamsSmall * reposSmall) + (teamsMedium * reposMedium) + ((i + 1) * reposLarge)
		teamRepos := mainOrgRepos[startIndex:endIndex]
		teamCategories[currentTeam] = teamRepos
	}

	remainderIndex := (teamsSmall * reposSmall) + (teamsMedium * reposMedium) + (teamsLarge * reposLarge)
	remainingRepos := mainOrgRepos[remainderIndex:]
	for i, r := range remainingRepos {
		t := teams[i%teamsSmall]
		teamCategories[t] = append(teamCategories[t], r)
	}

	teamsWithNils := make(map[*team][]*repo)
	for t, rr := range teamCategories {
		for _, r := range rr {
			if r == nil {
				teamsWithNils[t] = rr
				break
			}
		}
	}

	return teamCategories
}

// executeAssignTeamRepos adds the provided teams as members of the categorised repos.
func executeAssignTeamRepos(ctx context.Context, reposPerTeam map[*team][]*repo, reposDone *int64, p *pool.Pool) {
	for t, repos := range reposPerTeam {
		currentTeam := t
		currentRepos := repos

		p.Go(func() {
			for _, r := range currentRepos {
				currentRepo := r
				if r.Owner == fmt.Sprintf("%s/%s", currentTeam.Org, currentTeam.Name) {
					// team is already owner
					// writeInfo(out, "Repository %s already owned by %s", r.Name, currentTeam.Name)
					atomic.AddInt64(reposDone, 1)
					progress.SetValue(4, float64(*reposDone))
					continue
				}

				var res *github.Response
				var err error

			retryAddTeamRepo:
				if res, err = gh.Teams.AddTeamRepoBySlug(ctx, currentTeam.Org, currentTeam.Name, currentTeam.Org, currentRepo.Name, &github.TeamAddTeamRepoOptions{Permission: "push"}); err != nil {
					log.Fatalf("Failed to transfer repository %s from %s to %s: %s", currentRepo.Name, currentRepo.Owner, currentTeam.Name, err)
				}

				if res != nil && (res.StatusCode == 502 || res.StatusCode == 504) {
					time.Sleep(30 * time.Second)
					goto retryAddTeamRepo
				}

				if res.StatusCode == 422 {
					body, err := io.ReadAll(res.Body)
					if err != nil {
						log.Fatalf("Failed reading response body: %s", err)
					}
					log.Fatalf("Failed to assign repo %s to team %s: %s", currentRepo.Name, currentTeam.Name, string(body))
				}

				atomic.AddInt64(reposDone, 1)
				progress.SetValue(4, float64(*reposDone))
				currentRepo.Owner = fmt.Sprintf("%s/%s", currentTeam.Org, currentTeam.Name)
				if err = store.saveRepo(r); err != nil {
					log.Fatalf("Failed to save repository %s: %s", currentRepo.Name, err)
				}
				// writeInfo(out, "Repository %s transferred to %s", r.Name, currentTeam.Name)
			}
		})
	}
}

// categorizeUserRepos matches 3 unique users to the provided repos.
func categorizeUserRepos(mainOrgRepos []*repo, users []*user) map[*repo][]*user {
	repoUsers := make(map[*repo][]*user)
	usersPerRepo := 3
	for i, r := range mainOrgRepos {
		usersForRepo := users[i*usersPerRepo : (i+1)*usersPerRepo]
		repoUsers[r] = usersForRepo
	}

	return repoUsers
}

// executeAssignUserRepos adds the categorised users as collaborators to the matched repos.
func executeAssignUserRepos(ctx context.Context, usersPerRepo map[*repo][]*user, reposDone *int64, p *pool.Pool) {
	for r, users := range usersPerRepo {
		currentRepo := r
		currentUsers := users

		p.Go(func() {
			for _, u := range currentUsers {
				var res *github.Response
				var err error

			retryAddCollaborator:
				var invitation *github.CollaboratorInvitation
				if invitation, res, err = gh.Repositories.AddCollaborator(ctx, currentRepo.Owner, currentRepo.Name, u.Login, &github.RepositoryAddCollaboratorOptions{Permission: "push"}); err != nil {
					log.Fatalf("Failed to add user %s as a collaborator to repo %s: %s", u.Login, currentRepo.Name, err)
				}

				if res != nil && (res.StatusCode == 502 || res.StatusCode == 504) {
					time.Sleep(30 * time.Second)
					goto retryAddCollaborator
				}

				// AddCollaborator returns a 201 when an invitation is created.
				//
				// A 204 is returned when:
				// * an existing collaborator is added as a collaborator
				// * an organization member is added as an individual collaborator
				// * an existing team member (whose team is also a repository collaborator) is added as an individual collaborator
				if res.StatusCode == 201 {
				retryAcceptInvitation:
					if res, err = gh.Users.AcceptInvitation(ctx, invitation.GetID()); err != nil {
						log.Fatalf("Failed to accept collaborator invitation for user %s on repo %s: %s", u.Login, currentRepo.Name, err)
					}
					if res != nil && (res.StatusCode == 502 || res.StatusCode == 504) {
						time.Sleep(30 * time.Second)
						goto retryAcceptInvitation
					}
				}

			}

			atomic.AddInt64(reposDone, 1)
			progress.SetValue(4, float64(*reposDone))
			// writeInfo(out, "Repository %s transferred to users", r.Name)
		})
	}
}
