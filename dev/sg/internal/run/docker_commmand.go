package run

import (
	"cmp"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/rjeczalik/notify"
)

// A DockerCommand is a command definition for sg run/start that uses
// bazel under the hood. It will handle restarting itself autonomously,
// as long as iBazel is running and watch that specific target.
// Note: if you add a field here be sure to add it to the `Merge` method
type DockerCommand struct {
	Config SGConfigCommandOptions
	Docker DockerOptions `yaml:"docker"`
	// Optional bazel target to build and watch which provides a docker image tarball
	// if not provided, the DockerOptions::Image will simply be run directly
	// if Pull=true, it will be pulled first
	Target string `yaml:"target"`
}

type DockerOptions struct {
	Image string `yaml:"image"`
	// If true, the image will be pulled before running the container
	Pull    bool           `yaml:"pull"`
	Volumes []DockerVolume `yaml:"volumes"`
	// Additional flags to pass to the docker run command
	// e.g. cpus: 1 would be converted to --cpus=1
	Flags map[string]string `yaml:"flags"`
	// Ports is a list of ports to expose from the container to the host.
	// If only a single value is given it will be assumed to map that port from
	// the container to the same port on the host
	Ports []string           `yaml:"ports"`
	Linux DockerLinuxOptions `yaml:"linux"`
}

func (do DockerOptions) Merge(other DockerOptions) DockerOptions {
	merged := do
	merged.Image = mergeStrings(merged.Image, other.Image)
	merged.Pull = merged.Pull || other.Pull
	merged.Volumes = mergeSlices(merged.Volumes, other.Volumes)
	merged.Flags = mergeMaps(merged.Flags, other.Flags)
	merged.Ports = mergeSlices(merged.Ports, other.Ports)
	merged.Linux = merged.Linux.Merge(other.Linux)
	return merged
}

// DockerLinuxOptions is a struct that holds linux specific modifications to
// DockerEngine parameters for the DockerCommand
type DockerLinuxOptions struct {
	Flags map[string]string `yaml:"flags"`
	Env   map[string]string `yaml:"env"`
}

func (dlo DockerLinuxOptions) Merge(other DockerLinuxOptions) DockerLinuxOptions {
	merged := dlo
	merged.Flags = mergeMaps(merged.Flags, other.Flags)
	merged.Env = mergeMaps(merged.Env, other.Env)

	return merged
}

// Details for a docker volume to mount into the container
type DockerVolume struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

// UnmarshalYAML implements the Unmarshaler interface for DockerCommand.
// This allows us to parse the flat YAML configuration into nested struct.
func (dc *DockerCommand) UnmarshalYAML(unmarshal func(any) error) error {
	// In order to not recurse infinitely (calling UnmarshalYAML over and over) we create a
	// temporary type alias.
	// First parse the DockerCommand specific options
	type rawDocker DockerCommand
	if err := unmarshal((*rawDocker)(dc)); err != nil {
		return err
	}

	// Then parse the common options from the same list into a nested struct
	return unmarshal(&dc.Config)
}

func (dc DockerCommand) GetConfig() SGConfigCommandOptions {
	config := dc.Config
	// Add a custom preamble for docker listing ports
	config.Preamble += dc.GetDockerPreamble()
	// Add any platform specific environment overrides
	config.Env = makeEnvMap(config.Env, dc.GetDockerEnv(runtime.GOOS == "linux"))
	return config
}

func (dc DockerCommand) GetBinaryLocation() (string, error) {
	if dc.Target == "" {
		return "", nil
	}
	return binaryLocation(dc.Target)
}

func (dc DockerCommand) GetBazelTarget() string {
	return dc.Target
}

func (dc DockerCommand) UpdateConfig(f func(*SGConfigCommandOptions)) SGConfigCommand {
	f(&dc.Config)
	return dc
}

func (dc DockerCommand) StartWatch(ctx context.Context) (<-chan struct{}, error) {
	if watchPaths, err := dc.watchPaths(); err != nil {
		return nil, err
	} else {
		// skip remove events as we don't care about files being removed, we only
		// want to know when the binary has been rebuilt
		return WatchPaths(ctx, watchPaths, notify.Remove)
	}
}

func (dc DockerCommand) watchPaths() ([]string, error) {
	// If no target is defined, there is nothing to be built and watched
	if dc.Target == "" {
		return nil, nil
	}
	// Grab the location of the binary in bazel-out.
	binLocation, err := dc.GetBinaryLocation()
	if err != nil {
		return nil, err
	}
	return []string{binLocation}, nil
}

// GetDockerEnv returns the environment variables to be passed to the docker run command
func (dc DockerCommand) GetDockerEnv(isLinux bool) map[string]string {
	env := dc.Config.Env
	if isLinux {
		merge(env, dc.Docker.Linux.Env)
	}
	return env
}

// GetFlags returns the flags (i.e. --something) to be passed to the docker run command
func (opts DockerOptions) GetFlags(isLinux bool) map[string]string {
	if isLinux {
		merge(opts.Flags, opts.Linux.Flags)
	}
	return opts.Flags
}

// CreateDockerVolumes returns bash commands that will ensure that all of the local volumes
// exist before the docker run command is executed
func (dc DockerCommand) CreateDockerVolumes() string {
	var cmd strings.Builder
	for _, volume := range dc.Docker.Volumes {
		fmt.Fprintf(&cmd, "mkdir -p %s\n", volume.From)
	}
	return cmd.String()
}

func (dc DockerCommand) GetDockerImage(bin string) string {
	if bin != "" {
		return fmt.Sprintf("docker load -i %s\n", bin)
	}
	if dc.Docker.Pull {
		return fmt.Sprintf("docker pull %s\n", dc.Docker.Image)
	}

	return ""
}

func (dc DockerCommand) GetDockerPreamble() string {
	var preamble strings.Builder
	if dc.Config.Logfile != "" {
		fmt.Fprintf(&preamble, "Writing log output to %s\n", dc.Config.Logfile)
	}

	if len(dc.Docker.Ports) > 0 {
		var localports []string
		for _, port := range dc.Docker.Ports {
			localports = append(localports, strings.Split(port, ":")[0])
		}
		fmt.Fprintf(&preamble, "Listening on local ports: %s\n", strings.Join(localports, ", "))
	}
	return preamble.String()
}

// Constructs the actual docker run command to be executed
func (dc DockerCommand) GetDockerCommand(isLinux bool) string {
	var cmd strings.Builder
	fmt.Fprintf(&cmd, "docker run --rm --name %s", dc.Config.Name)
	for _, volume := range dc.Docker.Volumes {
		fmt.Fprintf(&cmd, " -v %s:%s", volume.From, volume.To)
	}
	for _, port := range dc.Docker.Ports {
		if strings.Contains(port, ":") {
			fmt.Fprintf(&cmd, " -p %s", port)
		} else {
			fmt.Fprintf(&cmd, " -p %s:%s", port, port)
		}
	}
	for _, flag := range toSortedPairs(dc.Docker.GetFlags(isLinux)) {
		fmt.Fprintf(&cmd, " --%s=%s", flag.Key, flag.Value)
	}
	for _, env := range toSortedPairs(dc.GetDockerEnv(isLinux)) {
		fmt.Fprintf(&cmd, ` -e %s="%s"`, env.Key, env.Value)
	}
	fmt.Fprintf(&cmd, " %s %s", dc.Docker.Image, dc.Config.Args)
	return cmd.String()

}

func (dc DockerCommand) GetCmd(bin string, isLinux bool) string {
	cleanup := fmt.Sprintf("docker inspect %s > /dev/null 2>&1 && docker rm -f %s", dc.Config.Name, dc.Config.Name)
	load := dc.GetDockerImage(bin)
	docker := dc.GetDockerCommand(isLinux)
	volumes := dc.CreateDockerVolumes()

	return strings.Join([]string{cleanup, load, volumes, dc.Config.PreCmd, docker}, "\n")
}

func (dc DockerCommand) GetExecCmd(ctx context.Context) (*exec.Cmd, error) {
	bin, err := dc.GetBinaryLocation()
	if err != nil {
		return nil, err
	}
	cmd := dc.GetCmd(bin, runtime.GOOS == "linux")
	return exec.CommandContext(ctx, "bash", "-c", cmd), nil
}

// Overrides the behavior of this command with other command.
// This is used for the sg.config.overwrite.yaml functionality
func (bc DockerCommand) Merge(other DockerCommand) DockerCommand {
	merged := bc

	merged.Target = mergeStrings(merged.Target, other.Target)
	merged.Config = merged.Config.Merge(other.Config)
	merged.Docker = merged.Docker.Merge(other.Docker)

	return merged
}

type Entry[K, V any] struct {
	Key   K
	Value V
}

func toSortedPairs[K cmp.Ordered, V any](m map[K]V) []Entry[K, V] {
	keys := make([]K, len(m))
	pairs := make([]Entry[K, V], len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for i, k := range keys {
		pairs[i] = Entry[K, V]{k, m[k]}
	}
	return pairs
}

func merge(base, overrides map[string]string) {
	for k, v := range overrides {
		base[k] = v
	}
}
