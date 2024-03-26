package wolfi

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/dev/sg/root"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

// PackageRepoConfig represents config for a local package repo
type PackageRepoConfig struct {
	PackageDir  string
	ImageDir    string
	Arch        string
	KeyDir      string
	KeyFilename string
	KeyFilepath string
}

// InitLocalPackageRepo initializes a local package repository
func InitLocalPackageRepo() (PackageRepoConfig, error) {
	repoRoot, err := root.RepositoryRoot()
	if err != nil {
		return PackageRepoConfig{}, err
	}

	c := PackageRepoConfig{
		PackageDir:  filepath.Join(repoRoot, "wolfi-packages/local-repo/packages"),
		ImageDir:    filepath.Join(repoRoot, "wolfi-images/local-images"),
		Arch:        "x86_64",
		KeyDir:      filepath.Join(repoRoot, "wolfi-packages/local-repo/keys"),
		KeyFilename: "sourcegraph-dev-local.rsa",
	}
	c.KeyFilepath = filepath.Join(c.KeyDir, c.KeyFilename)

	// Make directories
	if err := os.MkdirAll(filepath.Join(c.PackageDir, c.Arch), os.ModePerm); err != nil {
		return c, err
	}
	if err := os.MkdirAll(c.KeyDir, os.ModePerm); err != nil {
		return c, err
	}
	if err := os.MkdirAll(filepath.Join(c.ImageDir, c.Arch), os.ModePerm); err != nil {
		return c, err
	}

	// Generate keys for local repository
	// Skip if we are running on buildkite
	if os.Getenv("BUILDKITE") == "true" {
		return c, nil
	}
	if _, err = os.Stat(c.KeyFilepath); os.IsNotExist(err) {
		if err := c.GenerateKeypair(); err != nil {
			return c, err
		}
	} else if err != nil {
		return c, err
	}

	return c, nil
}

// GenerateKeypair generates a new RSA keypair for signing packages
func (c PackageRepoConfig) GenerateKeypair() error {
	// Run docker command
	std.Out.WriteLine(output.Linef("🗝️ ", output.StylePending, "Initializing keypair for local repo..."))

	cmd := exec.Command(
		"docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/keys", c.KeyDir),
		"cgr.dev/chainguard/melange", "keygen",
		fmt.Sprintf("/keys/%s", c.KeyFilename),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to generate keypair")
	}

	std.Out.WriteLine(output.Linef("🔐", output.StyleSuccess, "Keypair initialized"))

	return nil
}

// SetupPackageBuild sets up the build directory for a package
func SetupPackageBuild(name string) (manifestBaseName string, buildDir string, err error) {
	// Get root of repo
	repoRoot, err := root.RepositoryRoot()
	if err != nil {
		return "", "", errors.Wrap(err, "unable to get repository root")
	}

	// Strip .yaml suffix if it exists
	manifestBaseName = strings.Replace(name, ".yaml", "", 1)
	manifestFileName := manifestBaseName + ".yaml"

	// Check manfest exists
	manifestPath := filepath.Join(repoRoot, "wolfi-packages", manifestFileName)
	manifestDir := filepath.Join(repoRoot, "wolfi-packages", manifestBaseName)

	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return "", "", errors.Wrap(err, "manifest file does not exist")
	}

	// Create a temp dir
	buildDir, err = os.MkdirTemp("", "sg-wolfi-package-tmp")
	if err != nil {
		return "", "", errors.Wrap(err, "unable to create temporary build directory")
	}

	// Copy files
	cmd := exec.Command("cp", "-r", manifestPath, buildDir)
	err = cmd.Run()
	if err != nil {
		return "", "", errors.Wrap(err, "error copying build config to temp dir")
	}
	if _, err := os.Stat(manifestDir); !os.IsNotExist(err) {
		cmd := exec.Command("cp", "-r", manifestDir, buildDir)
		err = cmd.Run()
		if err != nil {
			return "", "", errors.Wrap(err, "error copying build config dir to temp dir")
		}
	}

	return
}

// DoPackageBuild builds a package using the provided build config
func (c PackageRepoConfig) DoPackageBuild(name string, buildDir string) error {
	std.Out.WriteLine(output.Linef("📦", output.StylePending, "Building package %s...", name))
	std.Out.WriteLine(output.Linef("🤖", output.StylePending, "Melange build output:\n"))

	cmd := exec.Command(
		"docker", "run", "--rm", "--privileged",
		"-v", fmt.Sprintf("%s:/work", buildDir),
		"-v", fmt.Sprintf("%s:/work/packages", c.PackageDir),
		"-v", fmt.Sprintf("%s:/keys", c.KeyDir),
		"cgr.dev/chainguard/melange", "build",
		"--arch", "x86_64",
		"--signing-key", filepath.Join("/keys", c.KeyFilename),
		fmt.Sprintf("/work/%s.yaml", name),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmdErr := cmd.Run()

	std.Out.Write("")

	if cmdErr != nil {
		return errors.Wrapf(cmdErr, "failed to build package %s", name)
	}

	std.Out.Write("")

	std.Out.WriteSuccessf("Successfully built package %s\n", name)
	std.Out.WriteLine(output.Linef("🛠️ ", output.StyleBold, "Use this package in local image builds by adding the package '%s@local' to the base image config\n", name))

	return nil
}

// RemoveBuildDir recursively removes the temporary build directory if it is in the OS temp dir.
// If the initial removal fails, it waits 50ms and tries again.
// If all removal attempts fail, it prints a message to stdout.
func RemoveBuildDir(path string) {
	if !strings.HasPrefix(path, os.TempDir()) {
		return
	}

	if err := os.RemoveAll(path); err != nil {
		// wait a bit in case any build processes (I'm looking at you, Docker!) are still using the directory
		time.Sleep(50 * time.Millisecond)
		if err := os.RemoveAll(path); err != nil {
			std.Out.WriteLine(output.Linef(output.EmojiWarningSign, output.StyleWarning, " Could not delete temp build dir %s because %s", path, err))
		}
	}
}
