package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/sourcegraph/conc/pool"
)

// delete removes users and teams (and team memberships as a side effect) from the GitHub instance.
// Organisations and repositories are left intact.
// The provided CLI flags define how many users and teams have to be deleted, enabling partial deletions.
func delete(ctx context.Context, cfg config) {
	localOrgs, err := store.loadOrgs()
	if err != nil {
		log.Fatalf("Failed to load orgs from state: %s", err)
	}

	if len(localOrgs) == 0 {
		// Fetch orgs currently on instance due to lost state
		remoteOrgs := getGitHubOrgs(ctx)

		writeInfo(out, "Storing %d orgs in state", len(remoteOrgs))
		for _, o := range remoteOrgs {
			if strings.HasPrefix(*o.Name, "org-") {
				o := &org{
					Login:   *o.Login,
					Admin:   cfg.orgAdmin,
					Failed:  "",
					Created: true,
				}
				if err := store.saveOrg(o); err != nil {
					log.Fatalf("Failed to store orgs in state: %s", err)
				}
				localOrgs = append(localOrgs, o)
			}
		}
	}

	localUsers, err := store.loadUsers()
	if err != nil {
		log.Fatalf("Failed to load users from state: %s", err)
	}

	if len(localUsers) == 0 {
		remoteUsers := getGitHubUsers(ctx)

		writeInfo(out, "Storing %d users in state", len(remoteUsers))
		for _, u := range remoteUsers {
			if strings.HasPrefix(*u.Login, "user-") {
				u := &user{
					// Fetch users currently on instance due to lost state
					Login:   *u.Login,
					Email:   fmt.Sprintf("%s@%s", *u.Login, emailDomain),
					Failed:  "",
					Created: true,
				}
				if err := store.saveUser(u); err != nil {
					log.Fatalf("Failed to store users in state: %s", err)
				}
				localUsers = append(localUsers, u)
			}
		}
	}

	localTeams, err := store.loadTeams()
	if err != nil {
		log.Fatalf("Failed to load teams from state: %s", err)
	}

	if len(localTeams) == 0 {
		// Fetch teams currently on instance due to lost state
		remoteTeams := getGitHubTeams(ctx, localOrgs)

		writeInfo(out, "Storing %d teams in state", len(remoteTeams))
		for _, t := range remoteTeams {
			if strings.HasPrefix(*t.Name, "team-") {
				t := &team{
					Name:         *t.Name,
					Org:          *t.Organization.Login,
					Failed:       "",
					Created:      true,
					TotalMembers: 0, //not important for deleting but subsequent use of state will be problematic
				}
				if err := store.saveTeam(t); err != nil {
					log.Fatalf("Failed to store teams in state: %s", err)
				}
				localTeams = append(localTeams, t)
			}
		}
	}

	p := pool.New().WithMaxGoroutines(1000)

	// delete users from instance
	usersToDelete := len(localUsers) - cfg.userCount
	for i := range usersToDelete {
		currentUser := localUsers[i]
		if i%100 == 0 {
			writeInfo(out, "Deleted %d out of %d users", i, usersToDelete)
		}
		p.Go(func() {
			currentUser.executeDelete(ctx)
		})
	}

	teamsToDelete := len(localTeams) - cfg.teamCount
	for i := range teamsToDelete {
		currentTeam := localTeams[i]
		if i%100 == 0 {
			writeInfo(out, "Deleted %d out of %d teams", i, teamsToDelete)
		}
		p.Go(func() {
			currentTeam.executeDelete(ctx)
		})
	}
	p.Wait()

	//for _, t := range localTeams {
	//	currentTeam := t
	//	g.Go(func() {
	//		executeDeleteTeamMembershipsForTeam(ctx, currentTeam.Org, currentTeam.Name)
	//	})
	//}
	//g.Wait()
}

// executeDelete deletes the team from the GitHub instance.
func (t *team) executeDelete(ctx context.Context) {
	existingTeam, resp, grErr := gh.Teams.GetTeamBySlug(ctx, t.Org, t.Name)

	if grErr != nil && resp.StatusCode != 404 {
		writeFailure(out, "Failed to get team %s, reason: %s\n", t.Name, grErr)
	}

	grErr = nil
	if existingTeam != nil {
		_, grErr = gh.Teams.DeleteTeamBySlug(ctx, t.Org, t.Name)
		if grErr != nil {
			writeFailure(out, "Failed to delete team %s, reason: %s\n", t.Name, grErr)
			t.Failed = grErr.Error()
			if grErr = store.saveTeam(t); grErr != nil {
				log.Fatal(grErr)
			}
			return
		}
	}

	if grErr = store.deleteTeam(t); grErr != nil {
		log.Fatal(grErr)
	}

	writeSuccess(out, "Deleted team %s", t.Name)
}

// executeDelete deletes the user from the instance.
func (u *user) executeDelete(ctx context.Context) {
	existingUser, resp, grErr := gh.Users.Get(ctx, u.Login)

	if grErr != nil && resp.StatusCode != 404 {
		writeFailure(out, "Failed to get user %s, reason: %s\n", u.Login, grErr)
		return
	}

	grErr = nil
	if existingUser != nil {
		_, grErr = gh.Admin.DeleteUser(ctx, u.Login)

		if grErr != nil {
			writeFailure(out, "Failed to delete user with login %s, reason: %s\n", u.Login, grErr)
			u.Failed = grErr.Error()
			if grErr = store.saveUser(u); grErr != nil {
				log.Fatal(grErr)
			}
			return
		}
	}

	u.Created = false
	u.Failed = ""
	if grErr = store.deleteUser(u); grErr != nil {
		log.Fatal(grErr)
	}

	writeSuccess(out, "Deleted user %s", u.Login)
}
