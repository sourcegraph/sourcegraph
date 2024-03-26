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
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

// define a constant wolfi-images
const baseImageDir = "wolfi-images"

var localRepoRegex = lazyregexp.New(`(?m)^\s+-\s+.*@local`)

type BaseImageConfig struct {
	PackageRepoConfig PackageRepoConfig
	ImageConfigDir    string // Directory containing all image configs
	ImageName         string // Name of image e.g. gitserver
	ImageConfigPath   string // Full path e.g. wolfi-images/gitserver.yaml
	LockfilePath      string // Path to lockfile e.g. wolfi-images/gitserver.lock.json
	BazelBuildPath    string // Bazel build path for image e.g. //cmd/gitserver:wolfi_base_tarball
	KeyringAppend     string // Path to additional keys to include in the keyring
	RepositoryAppend  string // Path to additional repositories to include in the keyring
}

type BaseImageOpts struct {
	KeyringAppend    string
	RepositoryAppend string
}

func SetupBaseImageBuild(name string, pc PackageRepoConfig, opts BaseImageOpts) (bc BaseImageConfig, err error) {
	bc.PackageRepoConfig = pc

	// Get root of repo
	repoRoot, err := root.RepositoryRoot()
	if err != nil {
		return bc, errors.Wrap(err, "unable to get repository root")
	}
	bc.ImageConfigDir = filepath.Join(repoRoot, baseImageDir)

	// Strip .yaml suffix if it exists
	bc.ImageName = strings.Replace(name, ".yaml", "", 1)
	bc.LockfilePath = filepath.Join(bc.ImageConfigDir, bc.ImageName+".lock.json")

	// Check manfest exists
	bc.ImageConfigPath = filepath.Join(bc.ImageConfigDir, bc.ImageName+".yaml")
	if _, err = os.Stat(bc.ImageConfigPath); os.IsNotExist(err) {
		return bc, errors.Wrap(err, "manifest file does not exist")
	}

	// Ignore error if no Bazel build path can be found - some images are not built in this repo
	imagePath, err := resolveImagePath(bc.ImageName)
	if err == nil {
		bc.BazelBuildPath = fmt.Sprintf("//%s:wolfi_base_tarball", imagePath)
	}

	bc.KeyringAppend = opts.KeyringAppend
	bc.RepositoryAppend = opts.RepositoryAppend

	return bc, nil
}

// resolveImagePath takes an image name and returns the build path where the image's Bazel config can be found
func resolveImagePath(name string) (string, error) {
	// Handle special case mappings
	specialCase := map[string]string{
		"sourcegraph":       "wolfi-images/sourcegraph",
		"sourcegraph-dev":   "wolfi-images/sourcegraph-dev",
		"postgres-exporter": "docker-images/postgres_exporter",
		"redis-exporter":    "docker-images/redis_exporter",
		"redis":             "docker-images/redis-cache", // Or redis-store
		"blobstore":         "docker-images/blobstore",   // cmd/blobstore is unused
	}
	if val, exists := specialCase[name]; exists {
		fmt.Printf("Special case mapping for image '%s' to '%s'\n", name, val)
		return val, nil
	}

	repoRoot, err := root.RepositoryRoot()
	if err != nil {
		return "", errors.Wrap(err, "unable to get repository root")
	}

	// Search for requested image in standard locations
	imageDirs := []string{"cmd", "docker-images"}
	for _, dir := range imageDirs {
		imagePath := filepath.Join(dir, name)
		fullImagePath := filepath.Join(repoRoot, imagePath)
		if _, err := os.Stat(fullImagePath); !os.IsNotExist(err) {
			return imagePath, nil
		}
	}

	return "", errors.New(fmt.Sprintf("no such image '%s' (searched %+v)", name, imageDirs))
}

func (bc BaseImageConfig) DoBaseImageBuild() error {
	// If we're already running in Bazel we can't run Bazel again inside its own builddir,
	// so ensure we're running in the base repo
	buildDir, err := os.Getwd()
	if err != nil {
		return err
	}
	bwd := os.Getenv("BUILD_WORKING_DIRECTORY")
	if bwd != "" {
		buildDir = bwd
	}

	if bc.BazelBuildPath == "" {
		return errors.Newf("no Bazel build path found for image '%s'", bc.ImageName)
	}

	std.Out.WriteLine(output.Linef("📦", output.StylePending, "Building base image %s using `bazel run %s`", bc.ImageName, bc.BazelBuildPath))
	std.Out.WriteLine(output.Linef("🤖", output.StylePending, "rules_apko build output:\n"))

	cmd := exec.Command(
		"bazel", "run", bc.BazelBuildPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = buildDir
	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to build base image")
	}

	return nil
}

func (bc BaseImageConfig) DoBaseImageBuildLegacy() error {
	std.Out.WriteLine(output.Linef("📦", output.StylePending, "Building base image %s...", bc.ImageName))
	std.Out.WriteLine(output.Linef("🤖", output.StylePending, "Apko build output:\n"))

	fmt.Printf("*** Work dir is %s\n", bc.ImageConfigDir)
	wd, _ := os.Getwd()
	fmt.Printf("*** pwd is %s\n", wd)
	lscmd := exec.Command("ls", "-al", bc.ImageConfigDir+"/")
	lscmd.Stdout = os.Stdout
	lscmd.Stderr = os.Stderr
	lscmd.Run()

	imageName := legacyDockerImageName(bc.ImageName)
	imageFileName := imageFileName(bc.ImageName)

	fmt.Printf("\n\nRunning ls in container\n\n")

	cmd := exec.Command(
		"docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/work", bc.ImageConfigDir+"/"),
		"-v", fmt.Sprintf("%s:/packages", bc.PackageRepoConfig.PackageDir),
		"-v", fmt.Sprintf("%s:/keys", bc.PackageRepoConfig.KeyDir),
		"-v", fmt.Sprintf("%s:/images", bc.PackageRepoConfig.ImageDir),
		"-e", fmt.Sprintf("SOURCE_DATE_EPOCH=%d", time.Now().Unix()),
		"-w", "/work",
		"cgr.dev/chainguard/apko", "build",
		"--arch", "x86_64",
		"--repository-append", "@local /packages",
		"--keyring-append", fmt.Sprintf("/keys/%s.pub", bc.PackageRepoConfig.KeyFilename),
		fmt.Sprintf("/work/%s.yaml", bc.ImageName),
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
	std.Out.WriteSuccessf("Successfully built base image %s\n", bc.ImageName)

	return nil
}

func DockerImageName(name string) string {
	return fmt.Sprintf("%s-base:latest", name)
}

func legacyDockerImageName(name string) string {
	return fmt.Sprintf("sourcegraph-wolfi/%s-base:latest", name)
}

func imageFileName(name string) string {
	return fmt.Sprintf("sourcegraph-wolfi-%s-base.tar", name)
}

func (bc BaseImageConfig) LoadBaseImage() error {
	baseImagePath := filepath.Join(bc.PackageRepoConfig.ImageDir, imageFileName(bc.ImageName))
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
	std.Out.WriteLine(output.Linef("🛠️ ", output.StyleBold, "Run base image locally using:\n\n\tdocker run -it --entrypoint /bin/sh %s-amd64\n", legacyDockerImageName(bc.ImageName)))

	return nil
}

func (bc BaseImageConfig) CleanupBaseImageBuild() error {
	imageDir := bc.PackageRepoConfig.ImageDir
	if !strings.HasSuffix(imageDir, "/wolfi-images/local-images") {
		return errors.New(fmt.Sprintf("directory '%s' does not look like the image output directory - not cleaning up", imageDir))
	}

	if err := os.RemoveAll(imageDir); err != nil {
		return errors.Wrap(err, fmt.Sprintf("unable to remove image output dir '%s'", imageDir))
	}

	return nil
}

// ContainsLocalPackages checks if a BaseImageConfig contains a reference to a
// @local package repository.
func (bc BaseImageConfig) ContainsLocalPackages() (bool, error) {
	imageConfigData, err := os.ReadFile(bc.ImageConfigPath)
	if err != nil {
		return false, err
	}

	imageConfig := string(imageConfigData)
	localRepoMatch := localRepoRegex.MatchString(imageConfig)

	return localRepoMatch, nil
}
