package sgconf

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/run"
)

func TestParseConfig(t *testing.T) {
	input := `
env:
  SRC_REPOS_DIR: $HOME/.sourcegraph/repos

commands:
  frontend:
    cmd: ulimit -n 10000 && .bin/frontend
    install: go build -o .bin/frontend github.com/sourcegraph/sourcegraph/cmd/frontend
    checkBinary: .bin/frontend
    env:
      CONFIGURATION_MODE: server
    watch:
      - lib

checks:
  docker:
    cmd: docker version
    failMessage: "Failed to run 'docker version'. Please make sure Docker is running."

commandsets:
  web:
    - frontend
    - caddy
  enterprise:
    checks:
      - docker
    commands:
      - frontend
      - gitserver
`

	have, err := parseConfig([]byte(input))
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	want := &Config{
		Env: map[string]string{"SRC_REPOS_DIR": "$HOME/.sourcegraph/repos"},
		Commands: map[string]*run.Command{
			"frontend": {
				Config: run.SGConfigCommandOptions{
					Name:           "frontend",
					Env:            map[string]string{"CONFIGURATION_MODE": "server"},
					RepositoryRoot: os.Getenv("SG_FORCE_REPO_ROOT"),
				},
				Cmd:         "ulimit -n 10000 && .bin/frontend",
				Install:     "go build -o .bin/frontend github.com/sourcegraph/sourcegraph/cmd/frontend",
				CheckBinary: ".bin/frontend",
				Watch:       []string{"lib"},
			},
		},
		Commandsets: map[string]*Commandset{
			"web": {
				Name:     "web",
				Commands: []string{"frontend", "caddy"},
			},
			"enterprise": {
				Name:     "enterprise",
				Commands: []string{"frontend", "gitserver"},
				Checks:   []string{"docker"},
			},
		},
	}

	if diff := cmp.Diff(want, have); diff != "" {
		t.Fatalf("wrong config. (-want +got):\n%s", diff)
	}
}

func TestParseAndMerge(t *testing.T) {
	a := `
commands:
  frontend:
    cmd: .bin/frontend
    install: go build .bin/frontend github.com/sourcegraph/sourcegraph/cmd/frontend
    checkBinary: .bin/frontend
    env:
      EXTSVC_CONFIG_FILE: '../dev-private/enterprise/dev/external-services-config.json'
    watch:
      - lib
      - internal
      - cmd/frontend
`
	config, err := parseConfig([]byte(a))
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	b := `
commands:
  frontend:
    env:
      EXTSVC_CONFIG_FILE: ''
`

	overwrite, err := parseConfig([]byte(b))
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	config.Merge(overwrite)

	cmd, ok := config.Commands["frontend"]
	if !ok {
		t.Fatalf("command not found")
	}

	want := &run.Command{
		Config: run.SGConfigCommandOptions{
			Name:           "frontend",
			Env:            map[string]string{"EXTSVC_CONFIG_FILE": ""},
			RepositoryRoot: os.Getenv("SG_FORCE_REPO_ROOT"),
		},
		Cmd:         ".bin/frontend",
		Install:     "go build .bin/frontend github.com/sourcegraph/sourcegraph/cmd/frontend",
		CheckBinary: ".bin/frontend",
		Watch: []string{
			"lib",
			"internal",
			"cmd/frontend",
		},
	}

	if diff := cmp.Diff(cmd, want); diff != "" {
		t.Fatalf("wrong cmd. (-want +got):\n%s", diff)
	}
}
