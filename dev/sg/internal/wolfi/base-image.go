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

func (c PackageRepoConfig) SetupBaseImageBuild(name string) (manifestBaseName string, buildDir string, err error) {
	// Get root of repo
	repoRoot, err := root.RepositoryRoot()
	if err != nil {
		return "", "", errors.Wrap(err, "unable to get repository root")
	}
	buildDir = filepath.Join(repoRoot, "wolfi-images")

	// Strip .yaml suffix if it exists
	manifestBaseName = strings.Replace(name, ".yaml", "", 1)
	manifestFileName := manifestBaseName + ".yaml"

	// Check manfest exists
	manifestPath := filepath.Join(repoRoot, "wolfi-images", manifestFileName)

	if _, err = os.Stat(manifestPath); os.IsNotExist(err) {
		return "", "", errors.Wrap(err, "manifest file does not exist")
	}

	return
}

func (c PackageRepoConfig) DoBaseImageBuild(name string, buildDir string) error {
	std.Out.WriteLine(output.Linef("📦", output.StylePending, "Building base image %s...", name))
	std.Out.WriteLine(output.Linef("🤖", output.StylePending, "Apko build output:\n"))

	imageName := fmt.Sprintf("sourcegraph-wolfi/%s-base:latest", name)
	imageFileName := fmt.Sprintf("sourcegraph-wolfi-%s-base.tar", name)

	cmd := exec.Command(
		"docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/work", buildDir),
		"-v", fmt.Sprintf("%s:/packages", c.PackageDir),
		"-v", fmt.Sprintf("%s:/keys", c.KeyDir),
		"-v", fmt.Sprintf("%s:/images", c.ImageDir),
		"-e", fmt.Sprintf("SOURCE_DATE_EPOCH=%d", time.Now().Unix()),
		"-w", "/work",
		"cgr.dev/chainguard/apko", "build",
		"--arch", "x86_64",
		"--repository-append", "@local /packages",
		"--keyring-append", fmt.Sprintf("/keys/%s.pub", c.KeyFilename),
		fmt.Sprintf("/work/%s.yaml", name),
		imageName,
		filepath.Join("/images", imageFileName),
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to build base image")
	}

	std.Out.Write("")
	std.Out.WriteSuccessf("Successfully built base image %s\n", name)

	return nil
}

func dockerImageName(name string) string {
	return fmt.Sprintf("sourcegraph-wolfi/%s-base:latest-amd64", name)
}

func imageFileName(name string) string {
	return fmt.Sprintf("sourcegraph-wolfi-%s-base.tar", name)
}

func (c PackageRepoConfig) LoadBaseImage(name string) error {
	baseImagePath := filepath.Join(c.ImageDir, imageFileName(name))
	std.Out.WriteLine(output.Linef("🐳", output.StylePending, "Loading base image into Docker... (%s)", baseImagePath))

	f, err := os.Open(baseImagePath)
	if err != nil {
		return err
	}

	cmd := exec.Command(
		"docker", "load",
	)
	cmd.Stdin = f
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to load base image in Docker")
	}

	std.Out.Write("")
	std.Out.WriteLine(output.Linef("🛠️ ", output.StyleBold, "Run base image locally using:\n\n\tdocker run -it --entrypoint /bin/sh %s\n", dockerImageName(name)))

	return nil
}

func (c PackageRepoConfig) CleanupBaseImageBuild(name string) error {
	imageDir := c.ImageDir
	if !strings.HasSuffix(imageDir, "/wolfi-images/local-images") {
		return errors.Newf("directory '%s' does not look like the image output directory - not cleaning up", imageDir)
	}

	if err := os.RemoveAll(imageDir); err != nil {
		return errors.Wrap(err, fmt.Sprintf("unable to remove image output dir '%s'", imageDir))
	}

	return nil
}
