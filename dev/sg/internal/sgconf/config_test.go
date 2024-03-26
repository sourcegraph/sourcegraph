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
env:
  GLOBAL_VAR: 'global var orig'
  OVERRIDE_VAR: 'override var orig'
commands:
  frontend:
    cmd: .bin/frontend
    install: go build .bin/frontend github.com/sourcegraph/sourcegraph/cmd/frontend
    checkBinary: .bin/frontend
    env:
      COMMAND_VAR: 'command local'
      COMMAND_OVERRIDE_VAR: 'command local'
    watch:
      - lib
      - internal
      - cmd/frontend
bazelCommands:
  frontend:
    target: //cmd/frontend
    env:
      BAZEL_VAR: 'bazel command local'
      BAZEL_OVERRIDE_VAR: 'bazel command local'
dockerCommands:
  frontend:
    docker:
      image: grafana:candidate
      ports:
        - 3370
      flags:
        cpus: 1
      volumes:
          - from: src
            to: dest
      linux:
          flags:
            add-host: host.docker.internal:host-gateway
            user: $UID
    env:
      DOCKER_VAR: 'docker command local'
      DOCKER_OVERRIDE_VAR: 'docker command local'
`
	config, err := parseConfig([]byte(a))
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	b := `
env:
  OVERRIDE_VAR: 'override var override'
commands:
  frontend:
    env:
      COMMAND_OVERRIDE_VAR: 'command override'
bazelCommands:
  frontend:
    runTarget: //cmd/frontend-run
    env:
      BAZEL_OVERRIDE_VAR: 'bazel command override'
dockerCommands:
  frontend:
    docker:
      image: grafana:update
      ports:
        - 3370
        - 3371
      flags:
        memory: 1g
      volumes:
          - from: override-src
            to: dst
      linux:
          flags:
            user: root
          env:
            FOO: bar
    env:
      DOCKER_OVERRIDE_VAR: 'docker command override'
`

	overwrite, err := parseConfig([]byte(b))
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	merged := config.Merge(overwrite)

	want := &Config{
		Env: map[string]string{
			"GLOBAL_VAR":   "global var orig",
			"OVERRIDE_VAR": "override var override",
		},
		Commands: map[string]*run.Command{"frontend": {
			Config: run.SGConfigCommandOptions{
				Name: "frontend",
				Env: map[string]string{
					"COMMAND_VAR":          "command local",
					"COMMAND_OVERRIDE_VAR": "command override"},
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
		},
		},
		BazelCommands: map[string]*run.BazelCommand{"frontend": {
			Config: run.SGConfigCommandOptions{
				Name: "frontend",
				Env: map[string]string{
					"BAZEL_VAR":          "bazel command local",
					"BAZEL_OVERRIDE_VAR": "bazel command override",
				},
				RepositoryRoot: os.Getenv("SG_FORCE_REPO_ROOT"),
			},
			Target:    "//cmd/frontend",
			RunTarget: "//cmd/frontend-run",
		},
		},
		DockerCommands: map[string]*run.DockerCommand{"frontend": {
			Config: run.SGConfigCommandOptions{
				Name: "frontend",
				Env: map[string]string{
					"DOCKER_VAR":          "docker command local",
					"DOCKER_OVERRIDE_VAR": "docker command override",
				},
				RepositoryRoot: os.Getenv("SG_FORCE_REPO_ROOT"),
			},
			Docker: run.DockerOptions{
				Image: "grafana:update",
				Volumes: []run.DockerVolume{
					{
						From: "override-src",
						To:   "dst",
					},
				},
				Flags: map[string]string{"cpus": "1", "memory": "1g"},
				Ports: []string{"3370",
					"3371",
				},
				Linux: run.DockerLinuxOptions{
					Flags: map[string]string{
						"add-host": "host.docker.internal:host-gateway",
						"user":     "root"},
					Env: map[string]string{"FOO": "bar"}}},
		},
		},
	}

	if diff := cmp.Diff(want, merged); diff != "" {
		t.Fatalf("wrong config. (-want +got):\n%s", diff)
	}
}
