package run_test

import (
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/run"
)

func parse(t *testing.T, input string) run.DockerCommand {
	t.Helper()

	got := run.DockerCommand{}
	if err := yaml.Unmarshal([]byte(input), &got); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	return got
}
func TestParseDockerCommand(t *testing.T) {
	want := run.DockerCommand{
		Config: run.SGConfigCommandOptions{
			Name:         "grafana",
			Description:  "Runs Grafana",
			PreCmd:       "echo hello",
			Args:         "--config /sg_config_grafana",
			Env:          map[string]string{"CACHE": "false", "GRAFANA_DISK": "$HOME/.sourcegraph-dev/data/grafana"},
			IgnoreStdout: true,
			IgnoreStderr: false,
			Logfile:      "$HOME/.sourcegraph-dev/logs/grafana/grafana.log",
		},
		Docker: run.DockerOptions{
			Image: "grafana:candidate",
			Volumes: []run.DockerVolume{
				{
					From: "$HOME/.sourcegraph-dev/data/grafana",
					To:   "/var/lib/grafana",
				},
				{
					From: "$(pwd)/dev/grafana/all",
					To:   "/sg_config_grafana/provisioning/datasources",
				},
			},
			Flags: map[string]string{"cpus": "1", "memory": "1g"},
			Ports: []string{"3370",
				"5168",
				"9128:9128",
				"5432:5678",
			},
			Linux: run.DockerLinuxOptions{
				Flags: map[string]string{
					"add-host": "host.docker.internal:host-gateway",
					"user":     "$UID"},
				Env: map[string]string{"FOO": "bar"}}},
		Target: "//docker-images/grafana:image_tarball",
	}

	got := parse(t, grafana)

	if diff := cmp.Diff(got, want); diff != "" {
		t.Fatalf("wrong cmd. (-want +got):\n%s", diff)
	}
}

func TestCompileGrafanaCommand(t *testing.T) {
	want := `docker inspect grafana > /dev/null 2>&1 && docker rm -f grafana
docker load -i ./fake_img.tar

mkdir -p $HOME/.sourcegraph-dev/data/grafana
mkdir -p $(pwd)/dev/grafana/all

echo hello
docker run --rm --name grafana ` +
		`-v $HOME/.sourcegraph-dev/data/grafana:/var/lib/grafana ` +
		`-v $(pwd)/dev/grafana/all:/sg_config_grafana/provisioning/datasources ` +
		`-p 3370:3370 -p 5168:5168 -p 9128:9128 -p 5432:5678 ` +
		`--cpus=1 --memory=1g ` +
		`-e CACHE="false" -e GRAFANA_DISK="$HOME/.sourcegraph-dev/data/grafana" ` +
		`grafana:candidate --config /sg_config_grafana`
	got := parse(t, grafana).GetCmd("./fake_img.tar", false)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("wrong cmd. (-want +got):\n%s", diff)
	}
}

func TestCompileGrafanaCommand_Linux(t *testing.T) {
	want := `docker inspect grafana > /dev/null 2>&1 && docker rm -f grafana
docker load -i ./fake_img.tar

mkdir -p $HOME/.sourcegraph-dev/data/grafana
mkdir -p $(pwd)/dev/grafana/all

echo hello
docker run --rm --name grafana ` +
		`-v $HOME/.sourcegraph-dev/data/grafana:/var/lib/grafana ` +
		`-v $(pwd)/dev/grafana/all:/sg_config_grafana/provisioning/datasources ` +
		`-p 3370:3370 -p 5168:5168 -p 9128:9128 -p 5432:5678 ` +
		`--add-host=host.docker.internal:host-gateway --cpus=1 --memory=1g --user=$UID ` +
		`-e CACHE="false" -e FOO="bar" -e GRAFANA_DISK="$HOME/.sourcegraph-dev/data/grafana" ` +
		`grafana:candidate --config /sg_config_grafana`

	got := parse(t, grafana).GetCmd("./fake_img.tar", true)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("wrong cmd. (-want +got):\n%s", diff)
	}
}

var grafana = `
name: grafana
target: //docker-images/grafana:image_tarball
description: Runs Grafana
precmd: echo hello
ignoreStdout: true
args: "--config /sg_config_grafana"
logfile: "$HOME/.sourcegraph-dev/logs/grafana/grafana.log"
env:
  GRAFANA_DISK: "$HOME/.sourcegraph-dev/data/grafana"
  CACHE: false
docker:
  image: grafana:candidate
  ports:
    - 3370
    - 5168
    - 9128:9128
    - 5432:5678
  flags:
    cpus: 1
    memory: 1g
  volumes:
    - from: $HOME/.sourcegraph-dev/data/grafana
      to: /var/lib/grafana
    - from: $(pwd)/dev/grafana/all
      to: /sg_config_grafana/provisioning/datasources
  linux:
    flags:
      add-host: host.docker.internal:host-gateway
      user: $UID
    env:
      FOO: bar`
