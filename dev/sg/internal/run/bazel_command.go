package run

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/rjeczalik/notify"
)

// A BazelCommand is a command definition for sg run/start that uses
// bazel under the hood. It will handle restarting itself autonomously,
// as long as iBazel is running and watch that specific target.
// Note: if you add a field here be sure to add it to the `Merge` method
type BazelCommand struct {
	Config SGConfigCommandOptions
	Target string `yaml:"target"`
	// RunTarget specifies a target that should be run via `bazel run $RunTarget` instead of directly executing the binary.
	RunTarget string `yaml:"runTarget"`
}

// UnmarshalYAML implements the Unmarshaler interface for BazelCommand.
// This allows us to parse the flat YAML configuration into nested struct.
func (bc *BazelCommand) UnmarshalYAML(unmarshal func(any) error) error {
	// In order to not recurse infinitely (calling UnmarshalYAML over and over) we create a
	// temporary type alias.
	// First parse the BazelCommand specific options
	type rawBazel BazelCommand
	if err := unmarshal((*rawBazel)(bc)); err != nil {
		return err
	}

	// Then parse the common options from the same list into a nested struct
	return unmarshal(&bc.Config)
}

func (bc BazelCommand) GetBinaryLocation() (string, error) {
	return binaryLocation(bc.Target)
}

func (bc BazelCommand) GetConfig() SGConfigCommandOptions {
	return bc.Config
}

func (bc BazelCommand) UpdateConfig(f func(*SGConfigCommandOptions)) SGConfigCommand {
	f(&bc.Config)
	return bc
}

func (bc BazelCommand) GetBazelTarget() string {
	return bc.Target
}
func (bc BazelCommand) watchPaths() ([]string, error) {
	// If no target is defined, there is nothing to be built and watched
	if bc.Target == "" {
		return nil, nil
	}
	// Grab the location of the binary in bazel-out.
	binLocation, err := bc.GetBinaryLocation()
	if err != nil {
		return nil, err
	}
	return []string{binLocation}, nil

}

func (bc BazelCommand) StartWatch(ctx context.Context) (<-chan struct{}, error) {
	if watchPaths, err := bc.watchPaths(); err != nil {
		return nil, err
	} else {
		// skip remove events as we don't care about files being removed, we only
		// want to know when the binary has been rebuilt
		return WatchPaths(ctx, watchPaths, notify.Remove)
	}
}

func (bc BazelCommand) GetExecCmd(ctx context.Context) (*exec.Cmd, error) {
	var cmd string
	var err error
	if bc.RunTarget != "" {
		cmd = "bazel run " + bc.RunTarget
	} else {
		if cmd, err = bc.GetBinaryLocation(); err != nil {
			return nil, err
		}
	}

	return exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf("%s\n%s", bc.Config.PreCmd, cmd)), nil
}

// Merge overrides the behavior of this command with other command.
// This is used for the sg.config.overwrite.yaml functionality
func (bc BazelCommand) Merge(other BazelCommand) BazelCommand {
	merged := bc

	merged.Config = bc.Config.Merge(other.Config)
	merged.Target = mergeStrings(merged.Target, other.Target)
	merged.RunTarget = mergeStrings(merged.RunTarget, other.RunTarget)

	return merged
}

func binaryLocation(target string) (string, error) {
	// Get the output directory from Bazel, which varies depending on which OS
	// we're running against.
	baseOutput, err := exec.Command("bazel", "info", "output_path").Output()
	if err != nil {
		return "", err
	}
	// Trim "bazel-out" because the next bazel query will include it.
	outputPath := strings.TrimSuffix(strings.TrimSpace(string(baseOutput)), "bazel-out")

	// Get the binary from the specific target.
	bin, err := exec.Command("bazel", "cquery", target, "--output=files").Output()
	if err != nil {
		return "", err
	}
	binPath := strings.TrimSpace(string(bin))

	return fmt.Sprintf("%s%s", outputPath, binPath), nil
}
