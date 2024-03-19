package ci

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sourcegraph/log"
	"gopkg.in/yaml.v2"

	bk "github.com/sourcegraph/sourcegraph/dev/ci/internal/buildkite"
	"github.com/sourcegraph/sourcegraph/dev/ci/internal/ci/changed"
	"github.com/sourcegraph/sourcegraph/dev/ci/internal/ci/operations"
	"github.com/sourcegraph/sourcegraph/dev/sg/root"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const wolfiImageDir = "wolfi-images"
const wolfiPackageDir = "wolfi-packages"

var baseImageRegex = lazyregexp.New(`wolfi-images\/([\w-]+)[.]yaml`)
var packageRegex = lazyregexp.New(`wolfi-packages\/([\w-]+)[.]yaml`)

// WolfiPackagesOperations rebuilds any packages whose configurations have changed
func WolfiPackagesOperations(changedFiles []string) (*operations.Set, []string) {
	ops := operations.NewNamedSet("Dependency packages")

	var changedPackages []string
	var buildStepKeys []string
	for _, c := range changedFiles {
		match := packageRegex.FindStringSubmatch(c)
		if len(match) == 2 {
			changedPackages = append(changedPackages, match[1])
			buildFunc, key := buildPackage(match[1])
			ops.Append(buildFunc)
			buildStepKeys = append(buildStepKeys, key)
		}
	}

	ops.Append(buildRepoIndex(buildStepKeys))

	return ops, changedPackages
}

// WolfiBaseImagesOperations rebuilds any base images whose configurations have changed
func WolfiBaseImagesOperations(changedFiles []string, tag string, packagesChanged bool) (*operations.Set, int) {
	ops := operations.NewNamedSet("Base image builds")
	logger := log.Scoped("gen-pipeline")

	var buildStepKeys []string
	for _, c := range changedFiles {
		match := baseImageRegex.FindStringSubmatch(c)
		if len(match) == 2 {
			buildFunc, key := buildWolfiBaseImage(match[1], tag, packagesChanged)
			ops.Append(buildFunc)
			buildStepKeys = append(buildStepKeys, key)
		} else {
			logger.Fatal(fmt.Sprintf("Unable to extract base image name from '%s', matches were %+v\n", c, match))
		}
	}

	ops.Append(allBaseImagesBuilt(buildStepKeys))

	return ops, len(buildStepKeys)
}

// Dependency tree between steps:
// (buildPackage[1], buildPackage[2], ...) <-- buildRepoIndex <-- (buildWolfi[1], buildWolfi[2], ...)

func buildPackage(target string) (func(*bk.Pipeline), string) {
	stepKey := sanitizeStepKey(fmt.Sprintf("package-dependency-%s", target))

	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep(fmt.Sprintf(":package: Package dependency '%s'", target),
			bk.Cmd(fmt.Sprintf("./dev/ci/scripts/wolfi/build-package.sh %s", target)),
			// We want to run on the bazel queue, so we have a pretty minimal agent.
			bk.Agent("queue", AspectWorkflows.QueueDefault),
			bk.DependsOn(AspectWorkflows.TestStepKey, AspectWorkflows.IntegrationTestStepKey),
			bk.Key(stepKey),
			bk.SoftFail(222),
		)
	}, stepKey
}

func buildRepoIndex(packageKeys []string) func(*bk.Pipeline) {
	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep(":card_index_dividers: Build and sign repository index",
			bk.Cmd("./dev/ci/scripts/wolfi/build-repo-index.sh"),
			// We want to run on the bazel queue, so we have a pretty minimal agent.
			bk.Agent("queue", AspectWorkflows.QueueSmall),
			// Depend on all previous package building steps
			bk.DependsOn(packageKeys...),
			bk.Key("buildRepoIndex"),
		)
	}
}

func buildWolfiBaseImage(target string, tag string, dependOnPackages bool) (func(*bk.Pipeline), string) {
	stepKey := sanitizeStepKey(fmt.Sprintf("build-base-image-%s", target))

	return func(pipeline *bk.Pipeline) {

		cmd := fmt.Sprintf("./dev/ci/scripts/wolfi/build-base-image.sh %s %s", target, tag)
		opts := []bk.StepOpt{
			bk.AnnotatedCmd(cmd, bk.AnnotatedCmdOpts{
				Annotations: &bk.AnnotationOpts{
					Type:            bk.AnnotationTypeInfo,
					IncludeNames:    false,
					MultiJobContext: "wolfi-images",
				},
			}),
			// We want to run on the bazel queue, so we have a pretty minimal agent.
			bk.Agent("queue", AspectWorkflows.QueueDefault),
			bk.DependsOn(AspectWorkflows.TestStepKey, AspectWorkflows.IntegrationTestStepKey),
			bk.Key(stepKey),
			bk.SoftFail(222),
		}
		// If packages have changed, wait for repo to be re-indexed as base images may depend on new packages
		if dependOnPackages {
			opts = append(opts, bk.DependsOn("buildRepoIndex"))
		}

		pipeline.AddStep(
			fmt.Sprintf(":octopus: Build Wolfi base image '%s'", target),
			opts...,
		)
	}, stepKey
}

// No-op to ensure all base images are updated before building full images
func allBaseImagesBuilt(baseImageKeys []string) func(*bk.Pipeline) {
	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep(":octopus: All base images built",
			bk.Cmd("echo 'All base images built'"),
			// We want to run on the bazel queue, so we have a pretty minimal agent.
			bk.Agent("queue", AspectWorkflows.QueueSmall),
			// Depend on all previous package building steps
			bk.DependsOn(baseImageKeys...),
			bk.Key("buildAllBaseImages"),
		)
	}
}

var reStepKeySanitizer = lazyregexp.New(`[^a-zA-Z0-9_-]+`)

// sanitizeStepKey sanitizes BuildKite StepKeys by removing any invalid characters
func sanitizeStepKey(key string) string {
	return reStepKeySanitizer.ReplaceAllString(key, "")
}

// GetDependenciesOfPackages takes a list of packages and returns the set of base images that depend on these packages
// Returns two slices: the image names, and the paths to the associated config files
func GetDependenciesOfPackages(packageNames []string, repo string) (images []string, imagePaths []string, err error) {
	repoRoot, err := root.RepositoryRoot()
	if err != nil {
		return nil, nil, err
	}
	wolfiImageDirPath := filepath.Join(repoRoot, wolfiImageDir)

	packagesByImage, err := GetAllImageDependencies(wolfiImageDirPath)
	if err != nil {
		return nil, nil, err
	}

	// Create a list of images that depend on packageNames
	for _, packageName := range packageNames {
		i := GetDependenciesOfPackage(packagesByImage, packageName, repo)
		images = append(images, i...)
	}

	// Dedupe image names
	images = sortUniq(images)
	// Append paths to returned image names
	imagePaths = imagesToImagePaths(wolfiImageDir, images)

	return
}

// GetDependenciesOfPackage returns the list of base images that depend on the given package
func GetDependenciesOfPackage(packagesByImage map[string][]string, packageName string, repo string) (images []string) {
	// Use a regex to catch cases like the `jaeger` package which builds `jaeger-agent` and `jaeger-all-in-one`
	var packageNameRegex = lazyregexp.New(fmt.Sprintf(`^%s(?:-[a-z0-9-]+)?$`, packageName))
	if repo != "" {
		packageNameRegex = lazyregexp.New(fmt.Sprintf(`^%s(?:-[a-z0-9-]+)?@%s`, packageName, repo))
	}

	for image, packages := range packagesByImage {
		for _, p := range packages {
			match := packageNameRegex.FindStringSubmatch(p)
			if len(match) > 0 {
				images = append(images, image)
			}
		}
	}

	// Dedupe image names
	images = sortUniq(images)

	return
}

// Add directory path and .yaml extension to each image name
func imagesToImagePaths(path string, images []string) (imagePaths []string) {
	for _, image := range images {
		imagePaths = append(imagePaths, filepath.Join(path, image)+".yaml")
	}

	return
}

func sortUniq(inputs []string) []string {
	unique := make(map[string]bool)
	var dedup []string
	for _, input := range inputs {
		if !unique[input] {
			unique[input] = true
			dedup = append(dedup, input)
		}
	}
	sort.Strings(dedup)
	return dedup
}

// GetAllImageDependencies returns a map of base images to the list of packages they depend upon
func GetAllImageDependencies(wolfiImageDir string) (packagesByImage map[string][]string, err error) {
	packagesByImage = make(map[string][]string)

	files, err := os.ReadDir(wolfiImageDir)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".yaml") {
			continue
		}

		filename := filepath.Join(wolfiImageDir, f.Name())
		imageName := strings.Replace(f.Name(), ".yaml", "", 1)

		packages, err := getPackagesFromBaseImageConfig(filename)
		if err != nil {
			return nil, err
		}

		packagesByImage[imageName] = packages
	}

	return
}

// BaseImageConfig follows a subset of the structure of a Wolfi base image manifests
type BaseImageConfig struct {
	Contents struct {
		Packages []string `yaml:"packages"`
	} `yaml:"contents"`
}

// getPackagesFromBaseImageConfig reads a base image config file and extracts the list of packages it depends on
func getPackagesFromBaseImageConfig(configFile string) ([]string, error) {
	var config BaseImageConfig

	yamlFile, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse YAML file '%s'", configFile)
	}

	return config.Contents.Packages, nil
}

// addWolfiOps adds operations to rebuild modified Wolfi packages and base images.
func addWolfiOps(c Config) (packageOps, baseImageOps *operations.Set) {
	// Rebuild Wolfi packages that have config changes
	var updatedPackages []string
	if c.Diff.Has(changed.WolfiPackages) {
		packageOps, updatedPackages = WolfiPackagesOperations(c.ChangedFiles[changed.WolfiPackages])
	}

	// Rebuild Wolfi base images
	// Inspect package dependencies, and rebuild base images with updated packages
	_, imagesWithChangedPackages, err := GetDependenciesOfPackages(updatedPackages, "sourcegraph")
	if err != nil {
		panic(err)
	}
	// Rebuild base images with package changes AND with config changes
	imagesToRebuild := append(imagesWithChangedPackages, c.ChangedFiles[changed.WolfiBaseImages]...)
	imagesToRebuild = sortUniq(imagesToRebuild)

	if len(imagesToRebuild) > 0 {
		baseImageOps, _ = WolfiBaseImagesOperations(
			imagesToRebuild,
			c.Version,
			(len(updatedPackages) > 0),
		)
	}

	return packageOps, baseImageOps
}

// wolfiRebuildAllBaseImages adds operations to rebuild all Wolfi base images and push to registry
func wolfiRebuildAllBaseImages(c Config) *operations.Set {
	// List all YAML files in wolfi-images/
	dir := "wolfi-images"
	files, err := os.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	var wolfiBaseImages []string
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".yaml" {
			fullPath := filepath.Join(dir, f.Name())
			wolfiBaseImages = append(wolfiBaseImages, fullPath)
		}
	}

	// Rebuild all images
	var baseImageOps *operations.Set
	if len(wolfiBaseImages) > 0 {
		baseImageOps, _ = WolfiBaseImagesOperations(
			wolfiBaseImages,
			c.Version,
			false,
		)
	}

	return baseImageOps
}

// wolfiGenerateBaseImagePR updates base image hashes and creates a PR in GitHub
func wolfiGenerateBaseImagePR() *operations.Set {
	ops := operations.NewNamedSet("Base Image Update PR")

	ops.Append(
		func(pipeline *bk.Pipeline) {
			pipeline.AddStep(":whale::hash: Update Base Image Hashes",
				bk.Cmd("./dev/ci/scripts/wolfi/update-base-image-hashes.sh"),
				bk.Agent("queue", AspectWorkflows.QueueSmall),
				bk.DependsOn("buildAllBaseImages"),
				bk.Key("updateBaseImageHashes"),
			)
		},
	)

	return ops
}
