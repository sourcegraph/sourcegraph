package main

import (
	"database/sql"
	"fmt"
	"log"
	"sync"

	"github.com/google/go-github/v55/github"
)

type state struct {
	// sqlite is not thread-safe, this mutex protects access to it
	sync.Mutex
	// where the DB file is
	path string
	// the opened DB
	db *sql.DB
}

var createUserTableStmt = `CREATE TABLE IF NOT EXISTS users (
  login STRING PRIMARY KEY,
  email STRING,
  failed STRING DEFAULT '',
  created BOOLEAN DEFAULT FALSE
)`

var createOrgTableStmt = `CREATE TABLE IF NOT EXISTS orgs (
  login STRING PRIMARY KEY,
  adminLogin STRING,
  failed STRING DEFAULT '',
  created BOOLEAN DEFAULT FALSE
)`

var createTeamTableStmt = `CREATE TABLE IF NOT EXISTS teams (
  name STRING PRIMARY KEY,
  org STRING,
  failed STRING DEFAULT '',
  created BOOLEAN DEFAULT FALSE,
  totalMembers INTEGER DEFAULT 0
)`

var createRepoTableStmt = `CREATE TABLE IF NOT EXISTS repos (
    owner STRING,
    name STRING PRIMARY KEY,
    assignedTeams INTEGER DEFAULT 0,
    assignedUsers INTEGER DEFAULT 0,
    assignedOrgs INTEGER DEFAULT 0,
    complete BOOLEAN DEFAULT FALSE
)`

func newState(path string) (*state, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	for _, statement := range []string{createUserTableStmt, createOrgTableStmt, createTeamTableStmt, createRepoTableStmt} {
		stmt, err := db.Prepare(statement)
		if err != nil {
			return nil, err
		}
		_, err = stmt.Exec()
		if err != nil {
			return nil, err
		}
	}

	return &state{
		path: path,
		db:   db,
	}, nil
}

type user struct {
	Login   string
	Email   string
	Failed  string
	Created bool
}

type team struct {
	Name         string
	Org          string
	Failed       string
	Created      bool
	TotalMembers int
}

func (t *team) setFailedAndSave(e error) error {
	t.Failed = e.Error()
	if err := store.saveTeam(t); err != nil {
		return err
	}
	return nil
}

type org struct {
	Login   string
	Admin   string
	Failed  string
	Created bool
}

type repo struct {
	Owner         string
	Name          string
	AssignedTeams int
	AssignedUsers int
	AssignedOrgs  int
	Complete      bool
}

func (s *state) loadUsers() ([]*user, error) {
	s.Lock()
	defer s.Unlock()
	rows, err := s.db.Query(`SELECT login, email, failed, created FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*user
	for rows.Next() {
		var u user
		err = rows.Scan(&u.Login, &u.Email, &u.Failed, &u.Created)
		if err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, nil
}

func (s *state) loadTeams() ([]*team, error) {
	s.Lock()
	defer s.Unlock()
	rows, err := s.db.Query(`SELECT name, org, failed, created, totalMembers FROM teams`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []*team
	for rows.Next() {
		var t team
		err = rows.Scan(&t.Name, &t.Org, &t.Failed, &t.Created, &t.TotalMembers)
		if err != nil {
			return nil, err
		}
		teams = append(teams, &t)
	}
	return teams, nil
}

func (s *state) loadOrgs() ([]*org, error) {
	s.Lock()
	defer s.Unlock()
	rows, err := s.db.Query(`SELECT login, adminLogin, failed, created FROM orgs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []*org
	for rows.Next() {
		var o org
		err = rows.Scan(&o.Login, &o.Admin, &o.Failed, &o.Created)
		if err != nil {
			return nil, err
		}
		orgs = append(orgs, &o)
	}
	return orgs, nil
}

func (s *state) loadRepos() ([]*repo, error) {
	s.Lock()
	defer s.Unlock()
	rows, err := s.db.Query(`SELECT owner, name, assignedUsers, assignedTeams, assignedOrgs, complete FROM repos`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []*repo
	for rows.Next() {
		var r repo
		err = rows.Scan(&r.Owner, &r.Name, &r.AssignedUsers, &r.AssignedTeams, &r.AssignedOrgs, &r.Complete)
		if err != nil {
			return nil, err
		}
		repos = append(repos, &r)
	}
	return repos, nil
}

func generateNames(prefix string, count int) []string {
	names := make([]string, count)
	for i := range count {
		names[i] = fmt.Sprintf("%s-%09d", prefix, i)
	}
	return names
}

func (s *state) generateUsers(cfg config) ([]*user, error) {
	names := generateNames("user", cfg.userCount)
	if err := s.insertUsers(names); err != nil {
		return nil, err
	}
	return s.loadUsers()
}

func (s *state) generateTeams(cfg config) ([]*team, error) {
	names := generateNames("team", cfg.teamCount)
	orgs, err := s.loadOrgs()
	if err != nil {
		return nil, err
	}
	if len(orgs) == 0 {
		log.Fatal("Organisations must be generated before teams")
	}
	var mainOrg *org
	for _, o := range orgs {
		if o.Login == "main-org" {
			mainOrg = o
			break
		}
	}
	if mainOrg == nil {
		log.Fatal("Unable to locate main-org")
	}

	if err = s.insertTeams(names, mainOrg); err != nil {
		return nil, err
	}
	return s.loadTeams()
}

func (s *state) generateOrgs(cfg config) ([]*org, error) {
	names := []string{"main-org"}
	names = append(names, generateNames("sub-org", cfg.subOrgCount)...)
	if err := s.insertOrgs(names, cfg.orgAdmin); err != nil {
		return nil, err
	}
	return s.loadOrgs()
}

var saveUserStmt = `UPDATE users SET
failed = ?,
created = ?
WHERE login = ?`

func (s *state) saveUser(u *user) error {
	err := s.insertUsers([]string{u.Login})
	if err != nil {
		return err
	}

	s.Lock()
	defer s.Unlock()

	_, err = s.db.Exec(
		saveUserStmt,
		u.Failed,
		u.Created,
		u.Login)

	if err != nil {
		return err
	}
	return nil
}

var saveTeamStmt = `UPDATE teams SET
failed = ?,
created = ?,
totalMembers = ?
WHERE name = ?`

func (s *state) saveTeam(t *team) error {
	err := s.insertTeams([]string{t.Name}, &org{Login: t.Org})
	if err != nil {
		return err
	}
	s.Lock()
	defer s.Unlock()

	_, err = s.db.Exec(
		saveTeamStmt,
		t.Failed,
		t.Created,
		t.TotalMembers,
		t.Name)

	if err != nil {
		return err
	}
	return nil
}

var saveOrgStmt = `UPDATE orgs SET
failed = ?,
created = ?
WHERE login = ?`

func (s *state) saveOrg(o *org) error {
	s.Lock()
	defer s.Unlock()

	_, err := s.db.Exec(
		saveOrgStmt,
		o.Failed,
		o.Created,
		o.Login)
	if err != nil {
		return err
	}
	return nil
}

var saveRepoStmt = `UPDATE repos SET
owner = ?,
assignedTeams = ?,
assignedUsers = ?,
assignedOrgs = ?,
complete = ?
WHERE name = ?`

func (s *state) saveRepo(r *repo) error {
	s.Lock()
	defer s.Unlock()

	_, err := s.db.Exec(
		saveRepoStmt,
		r.Owner,
		r.AssignedTeams,
		r.AssignedUsers,
		r.AssignedOrgs,
		r.Complete,
		r.Name)
	if err != nil {
		return err
	}
	return nil
}

func (s *state) insertUsers(logins []string) error {
	s.Lock()
	defer s.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, login := range logins {
		if _, err = tx.Exec(`INSERT OR IGNORE INTO users(login, email) VALUES (?, ?)`, login, fmt.Sprintf("%s@%s", login, emailDomain)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *state) insertTeams(names []string, org *org) error {
	s.Lock()
	defer s.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, name := range names {
		if _, err = tx.Exec(`INSERT OR IGNORE INTO teams(name, org) VALUES (?, ?)`, name, org.Login); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *state) insertOrgs(logins []string, admin string) error {
	s.Lock()
	defer s.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, login := range logins {
		if _, err = tx.Exec(`INSERT OR IGNORE INTO orgs(login, adminLogin) VALUES (?, ?)`, login, admin); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *state) insertRepos(repos []*github.Repository) ([]*repo, error) {
	s.Lock()
	defer s.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	for _, repo := range repos {
		if _, err = tx.Exec(`INSERT OR IGNORE INTO repos(owner, name) VALUES (?, ?)`, *repo.Owner.Login, *repo.Name); err != nil {
			return nil, err
		}
	}
	err = tx.Commit()
	if err != nil {
		return nil, err
	}
	return s.loadRepos()
}

var deleteUserStmt = `DELETE FROM users
WHERE login = ?`

func (s *state) deleteUser(u *user) error {
	s.Lock()
	defer s.Unlock()

	_, err := s.db.Exec(deleteUserStmt, u.Login)
	if err != nil {
		return err
	}
	return nil
}

var deleteTeamStmt = `DELETE FROM teams
WHERE name = ?`

func (s *state) deleteTeam(t *team) error {
	s.Lock()
	defer s.Unlock()

	_, err := s.db.Exec(deleteTeamStmt, t.Name)
	if err != nil {
		return err
	}
	return nil
}
