package command

import (
	"fmt"
	"strings"

	"github.com/kballard/go-shellquote"
)

const (
	// FirecrackerContainerDir is the directory where the container is mounted in the firecracker VM.
	FirecrackerContainerDir = "/work"
	// FirecrackerDockerConfDir is the directory where the docker config is mounted in the firecracker VM.
	FirecrackerDockerConfDir = "/etc/docker/cli"
)

// NewFirecrackerSpec returns a spec that will run the given command in a firecracker VM.
func NewFirecrackerSpec(vmName string, scriptPath string, spec Spec, options DockerOptions) Spec {
	dockerSpec := NewDockerSpec(FirecrackerContainerDir, scriptPath, spec, options)
	innerCommand := shellquote.Join(dockerSpec.Command...)

	// Note: src-cli run commands don't receive env vars in firecracker so we
	// have to prepend them inline to the script.
	// TODO: This branch should disappear when we make src-cli a non-special cased
	// thing.
	if spec.Image == "" && len(dockerSpec.Env) > 0 {
		innerCommand = fmt.Sprintf("%s %s", strings.Join(quoteEnv(dockerSpec.Env), " "), innerCommand)
	}
	if dockerSpec.Dir != "" {
		innerCommand = fmt.Sprintf("cd %s && %s", shellquote.Join(dockerSpec.Dir), innerCommand)
	}
	return Spec{
		Key:       spec.Key,
		Command:   []string{"ignite", "exec", vmName, "--", innerCommand},
		Operation: spec.Operation,
	}
}

// quoteEnv returns a slice of env vars in which the values are properly shell quoted.
func quoteEnv(env []string) []string {
	quotedEnv := make([]string, len(env))

	for i, e := range env {
		elems := strings.SplitN(e, "=", 2)
		quotedEnv[i] = fmt.Sprintf("%s=%s", elems[0], shellquote.Join(elems[1]))
	}

	return quotedEnv
}
