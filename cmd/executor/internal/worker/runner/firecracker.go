package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/config"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/cmdlogger"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/internal/executor/types"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type firecrackerRunner struct {
	cmd              command.Command
	vmName           string
	workspaceDevice  string
	internalLogger   log.Logger
	cmdLogger        cmdlogger.Logger
	options          FirecrackerOptions
	dockerAuthConfig types.DockerAuthConfig
	// tmpDir is used to store temporary files used for firecracker execution.
	tmpDir     string
	operations *command.Operations
}

type FirecrackerOptions struct {
	// Enabled determines if commands will be run in Firecracker virtual machines.
	Enabled bool
	// Image is the base image used for all Firecracker virtual machines.
	Image string
	// KernelImage is the base image containing the kernel binary for all Firecracker
	// virtual machines.
	KernelImage string
	// SandboxImage is the docker image used by ignite for isolation of the Firecracker
	// process.
	SandboxImage string
	// VMStartupScriptPath is a path to a file on the host that is loaded into a fresh
	// virtual machine and executed on startup.
	VMStartupScriptPath string
	// DockerRegistryMirrorURLs is an optional parameter to configure docker
	// registry mirrors for the VMs docker daemon on startup. When set, /etc/docker/daemon.json
	// will be mounted into the VM.
	DockerRegistryMirrorURLs []string
	// DockerOptions
	DockerOptions command.DockerOptions
	// KeepWorkspaces prevents deletion of a workspace after a job completes. Setting
	// this value to true will continually use more and more disk, so it should only
	// be used as a debugging mechanism.
	KeepWorkspaces bool
}

var _ Runner = &firecrackerRunner{}

func NewFirecrackerRunner(
	cmd command.Command,
	logger cmdlogger.Logger,
	workspaceDevice string,
	vmName string,
	options FirecrackerOptions,
	dockerAuthConfig types.DockerAuthConfig,
	operations *command.Operations,
) Runner {
	// Use the option configuration unless the user has provided a custom configuration.
	actualDockerAuthConfig := options.DockerOptions.DockerAuthConfig
	if len(dockerAuthConfig.Auths) > 0 {
		actualDockerAuthConfig = dockerAuthConfig
	}

	return &firecrackerRunner{
		cmd:              cmd,
		vmName:           vmName,
		workspaceDevice:  workspaceDevice,
		internalLogger:   log.Scoped("firecracker-runner"),
		cmdLogger:        logger,
		options:          options,
		dockerAuthConfig: actualDockerAuthConfig,
		operations:       operations,
	}
}

func (r *firecrackerRunner) TempDir() string {
	return r.tmpDir
}

func (r *firecrackerRunner) Setup(ctx context.Context) error {
	dir, err := os.MkdirTemp("", "executor-firecracker-runner")
	if err != nil {
		return errors.Wrap(err, "failed to create tmp dir for firecracker runner")
	}
	r.tmpDir = dir

	dockerConfigPath, err := r.setupFirecracker(ctx)
	r.options.DockerOptions.ConfigPath = dockerConfigPath
	return err
}

// setupFirecracker invokes a set of commands to provision and prepare a Firecracker virtual
// machine instance. If a startup script path (an executable file on the host) is supplied,
// it will be mounted into the new virtual machine instance and executed.
func (r *firecrackerRunner) setupFirecracker(ctx context.Context) (string, error) {
	var daemonConfigFile string
	if len(r.options.DockerRegistryMirrorURLs) > 0 {
		var err error
		daemonConfigFile, err = newDockerDaemonConfig(r.tmpDir, r.options.DockerRegistryMirrorURLs)
		if err != nil {
			return "", err
		}
	}

	// If docker auth config is present, write it.
	var dockerConfigPath string
	if len(r.dockerAuthConfig.Auths) > 0 {
		d, err := json.Marshal(r.dockerAuthConfig)
		if err != nil {
			return "", err
		}
		dockerConfigPath, err = os.MkdirTemp(r.tmpDir, "docker_auth")
		if err != nil {
			return "", err
		}
		if err = os.WriteFile(filepath.Join(dockerConfigPath, "config.json"), d, os.ModePerm); err != nil {
			return "", err
		}
	}

	// Make subdirectory called "cni" to store CNI config in. All files from a directory
	// will be considered so this has to be it's own directory with just our config file.
	cniConfigDir := path.Join(r.tmpDir, "cni")
	err := os.Mkdir(cniConfigDir, os.ModePerm)
	if err != nil {
		return "", err
	}
	cniConfigFile := path.Join(cniConfigDir, "10-sourcegraph-executors.conflist")
	err = os.WriteFile(cniConfigFile, []byte(cniConfig(r.options.DockerOptions.Resources.MaxIngressBandwidth, r.options.DockerOptions.Resources.MaxEgressBandwidth)), os.ModePerm)
	if err != nil {
		return "", err
	}

	// Start the VM and wait for the SSH server to become available.
	startCommandSpec := command.Spec{
		Key: "setup.firecracker.start",
		// Tell ignite to use our temporary config file for maximum isolation of
		// envs.
		Env: []string{fmt.Sprintf("CNI_CONF_DIR=%s", cniConfigDir)},
		Command: command.Flatten(
			"ignite", "run",
			"--runtime", "docker",
			"--network-plugin", "cni",
			firecrackerResourceFlags(r.options.DockerOptions.Resources),
			firecrackerCopyfileFlags(r.options.VMStartupScriptPath, daemonConfigFile, dockerConfigPath),
			firecrackerVolumeFlags(r.workspaceDevice),
			"--ssh",
			"--name", r.vmName,
			"--kernel-image", sanitizeImage(r.options.KernelImage),
			"--kernel-args", config.FirecrackerKernelArgs,
			"--sandbox-image", sanitizeImage(r.options.SandboxImage),
			sanitizeImage(r.options.Image),
		),
		Operation: r.operations.SetupFirecrackerStart,
	}

	if err = r.cmd.Run(ctx, r.cmdLogger, startCommandSpec); err != nil {
		return "", errors.Wrap(err, "failed to start firecracker vm")
	}

	if r.options.VMStartupScriptPath != "" {
		startupScriptCommandSpec := command.Spec{
			Key:       "setup.startup-script",
			Command:   command.Flatten("ignite", "exec", r.vmName, "--", r.options.VMStartupScriptPath),
			Operation: r.operations.SetupStartupScript,
		}
		if err = r.cmd.Run(ctx, r.cmdLogger, startupScriptCommandSpec); err != nil {
			return "", errors.Wrap(err, "failed to run startup script")
		}
	}

	if dockerConfigPath != "" {
		return command.FirecrackerDockerConfDir, nil
	}

	return "", nil
}

func newDockerDaemonConfig(tmpDir string, mirrorAddresses []string) (_ string, err error) {
	c, err := json.Marshal(&dockerDaemonConfig{RegistryMirrors: mirrorAddresses})
	if err != nil {
		return "", errors.Wrap(err, "marshalling docker daemon config")
	}

	tmpFilePath := path.Join(tmpDir, dockerDaemonConfigFilename)
	err = os.WriteFile(tmpFilePath, c, os.ModePerm)
	return tmpFilePath, errors.Wrap(err, "writing docker daemon config file")
}

// dockerDaemonConfig is a struct that marshals into a valid docker daemon config.
type dockerDaemonConfig struct {
	RegistryMirrors []string `json:"registry-mirrors"`
}

// dockerDaemonConfigFilename is the filename in the firecracker state tmp directory
// for the optional docker daemon config file.
const dockerDaemonConfigFilename = "docker-daemon.json"

// cniConfig generates a config file that configures the CNI explicitly and adds
// the isolation plugin to the chain.
// This is used to prevent cross-network communication (which currently doesn't
// happen as we only have 1 bridge).
// We also set the maximum bandwidth usable per VM to the configured value to avoid
// abuse and to make sure multiple VMs on the same host won't starve others.
func cniConfig(maxIngressBandwidth, maxEgressBandwidth int) string {
	return fmt.Sprintf(
		defaultCNIConfig,
		config.CNISubnetCIDR,
		maxIngressBandwidth,
		2*maxIngressBandwidth,
		maxEgressBandwidth,
		2*maxEgressBandwidth,
	)
}

// defaultCNIConfig is the CNI config used for our firecracker VMs.
// TODO: Can we remove the portmap completely?
const defaultCNIConfig = `
{
  "cniVersion": "0.4.0",
  "name": "ignite-cni-bridge",
  "plugins": [
    {
  	  "type": "bridge",
  	  "bridge": "ignite0",
  	  "isGateway": true,
  	  "isDefaultGateway": true,
  	  "promiscMode": false,
  	  "ipMasq": true,
  	  "ipam": {
  	    "type": "host-local",
  	    "subnet": %q
  	  }
    },
    {
  	  "type": "portmap",
  	  "capabilities": {
  	    "portMappings": true
  	  }
    },
    {
  	  "type": "firewall"
    },
    {
  	  "type": "isolation"
    },
    {
  	  "name": "slowdown",
  	  "type": "bandwidth",
  	  "ingressRate": %d,
  	  "ingressBurst": %d,
  	  "egressRate": %d,
  	  "egressBurst": %d
    }
  ]
}
`

func firecrackerResourceFlags(options command.ResourceOptions) []string {
	return []string{
		"--cpus", strconv.Itoa(options.NumCPUs),
		"--memory", options.Memory,
		"--size", options.DiskSpace,
	}
}

func firecrackerCopyfileFlags(vmStartupScriptPath, daemonConfigFile, dockerConfigPath string) []string {
	copyfiles := make([]string, 0, 3)
	if vmStartupScriptPath != "" {
		copyfiles = append(copyfiles, fmt.Sprintf("%s:%s", vmStartupScriptPath, vmStartupScriptPath))
	}

	if daemonConfigFile != "" {
		copyfiles = append(copyfiles, fmt.Sprintf("%s:%s", daemonConfigFile, "/etc/docker/daemon.json"))
	}

	if dockerConfigPath != "" {
		copyfiles = append(copyfiles, fmt.Sprintf("%s:%s", dockerConfigPath, command.FirecrackerDockerConfDir))
	}

	sort.Strings(copyfiles)
	return command.Intersperse("--copy-files", copyfiles)
}

func firecrackerVolumeFlags(workspaceDevice string) []string {
	return []string{"--volumes", fmt.Sprintf("%s:%s", workspaceDevice, command.FirecrackerContainerDir)}
}

// sanitizeImage sanitizes the given docker image for use by ignite. The ignite utility
// has some issue parsing docker tags that include a sha256 hash, so we try to remove it
// from any of the image references before passing it to the ignite command.
func sanitizeImage(image string) string {
	if matches := imagePattern.FindStringSubmatch(image); len(matches) == 4 {
		if matches[2] == "" {
			return matches[1]
		}

		return fmt.Sprintf("%s:%s", matches[1], matches[2])
	}

	return image
}

var imagePattern = lazyregexp.New(`([^:@]+)(?::([^@]+))?(?:@sha256:([a-z0-9]{64}))?`)

func (r *firecrackerRunner) Teardown(ctx context.Context) error {
	removeCommandSpec := r.newCommandSpec(
		"teardown.firecracker.remove",
		command.Flatten("ignite", "rm", "-f", r.vmName),
		nil,
		r.operations.TeardownFirecrackerRemove,
	)
	if err := r.cmd.Run(ctx, r.cmdLogger, removeCommandSpec); err != nil {
		r.internalLogger.Error("Failed to remove firecracker vm", log.String("name", r.vmName), log.Error(err))
	}

	if err := os.RemoveAll(r.tmpDir); err != nil {
		r.internalLogger.Error(
			"Failed to remove firecracker vm",
			log.String("name", r.vmName),
			log.String("tmpDir", r.tmpDir),
			log.Error(err),
		)
	}

	return nil
}

func (r *firecrackerRunner) newCommandSpec(key string, cmd []string, env []string, operations *observation.Operation) command.Spec {
	return command.Spec{
		Key:       key,
		Command:   cmd,
		Env:       env,
		Operation: operations,
	}
}

func (r *firecrackerRunner) Run(ctx context.Context, spec Spec) error {
	firecrackerSpec := command.NewFirecrackerSpec(r.vmName, spec.ScriptPath, spec.CommandSpecs[0], r.options.DockerOptions)
	return r.cmd.Run(ctx, r.cmdLogger, firecrackerSpec)
}
