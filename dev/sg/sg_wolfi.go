package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/category"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/wolfi"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

var (
	checkLock        bool
	buildLegacy      bool
	keyringAppend    string
	repositoryAppend string
)

var (
	wolfiCommand = &cli.Command{
		Name:        "wolfi",
		Usage:       "Automate Wolfi related tasks",
		Description: `Build Wolfi packages and images locally, and update base image hashes`,
		UsageText: `
# Update base image hashes
sg wolfi update-hashes
sg wolfi update-hashes jaeger-agent

# Build a specific package using a manifest from wolfi-packages/
sg wolfi package jaeger
sg wolfi package jaeger.yaml

# Build a base image using a manifest from wolfi-images/
sg wolfi image gitserver
sg wolfi image gitserver.yaml
`,
		Category: category.Dev,
		Subcommands: []*cli.Command{{
			Name:      "package",
			ArgsUsage: "<package-manifest>",
			Usage:     "Build a package locally using a manifest from sourcegraph/wolfi-packages/",
			UsageText: `
Build a Wolfi package locally by running Melange against a provided Melange manifest file, which can be found in sourcegraph/wolfi-packages.

This is convenient for testing package changes locally before pushing to the Wolfi registry.
Base images containing locally-built packages can then be built using 'sg wolfi image'.
`,
			Action: func(ctx *cli.Context) error {
				args := ctx.Args().Slice()
				if len(args) == 0 {
					return errors.New("no package manifest file provided")
				}
				packageName := args[0]

				c, err := wolfi.InitLocalPackageRepo()
				if err != nil {
					return err
				}

				manifestBaseName, buildDir, err := wolfi.SetupPackageBuild(packageName)
				if err != nil {
					return err
				}

				defer wolfi.RemoveBuildDir(buildDir)

				err = c.DoPackageBuild(manifestBaseName, buildDir)
				if err != nil {
					return err
				}

				return nil
			},
		},
			{
				Name:      "image",
				ArgsUsage: "<base-image-manifest>",
				Usage:     "Build a base image locally using a manifest from sourcegraph/wolfi-images/",
				UsageText: `
Build a base image locally by running apko against a provided apko manifest file, which can be found in sourcegraph/wolfi-images.

Any packages built locally using 'sg wolfi package' can be included in the base image using the 'package@local' syntax in the base image manifest.
This is convenient for testing package changes locally before publishing them.

Once built, the base image is loaded into Docker and can be run locally.
It can also be used for local development by updating its path and hash in the 'dev/oci_deps.bzl' file.
`,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:        "legacy",
						Aliases:     []string{"l"},
						Usage:       "Build using legacy apko binary rather than Bazel",
						Destination: &buildLegacy,
					},
				},
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) == 0 {
						return errors.New("no base image manifest file provided")
					}

					baseImageName := args[0]

					pc, err := wolfi.InitLocalPackageRepo()
					if err != nil {
						return err
					}

					// Additional repos cannot be provided as they will not be used unless lockfile is outdated
					bc, err := wolfi.SetupBaseImageBuild(baseImageName, pc, wolfi.BaseImageOpts{})
					if err != nil {
						return err
					}
					if bc.BazelBuildPath == "" {
						if os.Getenv("BUILDKITE") == "true" {
							std.Out.WriteLine(output.Linef(output.EmojiWarning, output.StyleBold, "No Bazel build path found for %s - no fallback avilable in Buildkite, so soft-failing", baseImageName))
							return cli.Exit("Cannot build base image without Bazel build path on Buildkite (soft-fail)", 222)
						}
						std.Out.WriteLine(output.Linef(output.EmojiWarning, output.StyleBold, "No Bazel build path found for %s - falling back to legacy build method", baseImageName))
						buildLegacy = true
					}

					// WORKAROUND: rules_apko does not support package repos on the local filesystem, so fall back to legacy build
					hasLocalPackage, err := bc.ContainsLocalPackages()
					if err != nil {
						return err
					}
					if hasLocalPackage {
						std.Out.WriteLine(output.Linef(output.EmojiWarning, output.StyleBold, "%s.yaml contains an `@local` package - falling back to legacy build method", baseImageName))
						buildLegacy = true
					}

					if !buildLegacy {
						isMatch, err := bc.CheckApkoLockHash()
						if err != nil {
							return err
						}
						if !isMatch {
							std.Out.WriteLine(output.Linef("🛠️ ", output.StyleBold, "%s.yaml does not match %s.lock.json - regenerating lockfile (run manually with `sg wolfi lock %s`)", baseImageName, baseImageName, baseImageName))
							if err = bc.UpdateImage(ctx); err != nil {
								return err
							}
						}

						fmt.Printf("Using bazel build process\n")
						if err = bc.DoBaseImageBuild(); err != nil {
							return err
						}

						std.Out.WriteLine(output.Linef("🛠️ ", output.StyleBold, "Run base image locally using:\n\n\tdocker run -it --entrypoint /bin/sh %s\n", wolfi.DockerImageName(bc.ImageName)))
					} else {
						if err = bc.DoBaseImageBuildLegacy(); err != nil {
							return err
						}

						if err = bc.LoadBaseImage(); err != nil {
							return err
						}

						if err = bc.CleanupBaseImageBuild(); err != nil {
							return err
						}
					}

					return nil

				},
			}, {
				Name:      "scan-images",
				ArgsUsage: "<base-image-name>",
				Usage:     "Scan Wolfi base images for vulnerabilities",
				UsageText: `
Scans the Wolfi base images in the 'dev/oci_deps.bzl' file.`,
				Action: func(ctx *cli.Context) error {
					wolfi.ScanImages()

					return nil
				},
			}, {
				Name:      "update-hashes",
				ArgsUsage: "<base-image-name>",
				Usage:     "Update Wolfi base images hashes to the latest versions",
				UsageText: `
Update the hash references for Wolfi base images in the 'dev/oci_deps.bzl' file.
By default all hashes will be updated; pass in a base image name to update a specific image.

Hash references are updated by fetching the ':latest' tag for each base image from the registry, and updating the corresponding hash in 'dev/oci_deps.bzl'.
`,
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					var imageName string
					if len(args) == 1 {
						imageName = args[0]
					}

					return wolfi.UpdateHashes(ctx, imageName)
				},
			}, {
				Name:      "lock",
				ArgsUsage: "<base-image-name>",
				Usage:     "Update the lockfile for a Wolfi base image by fetching the latest package versions",
				UsageText: `
# Update lockfile for all base images
sg wolfi lock

# Update lockfile for the Gitserver base image
sg wolfi lock gitserver

Takes a container image YAML file containing a list of packages and generates a lockfile with resolved package versions.
This lockfile ensures reproducible builds by pinning the exact versions of the packages used in the container image.

If no <base-image-name> is provided, the lockfile for all base images will be updated.

Lockfiles can be found at wolfi-images/<image>.lock.json
`,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:        "check",
						Aliases:     []string{"c"},
						Usage:       "Check if the lockfile is up to date",
						Destination: &checkLock,
					},
					&cli.StringFlag{
						Name:        "repository-append",
						Aliases:     []string{"r"},
						Usage:       "Path to additional repositories to include",
						Destination: &repositoryAppend,
					},
					&cli.StringFlag{
						Name:        "keyring-append",
						Aliases:     []string{"k"},
						Usage:       "Path to additional keys to include in the keyring",
						Destination: &keyringAppend,
					},
				},
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					var imageName string
					if len(args) == 1 {
						imageName = args[0]
					}
					fmt.Printf("args are %v\n", args)

					if checkLock {
						var imageNames []string
						if imageName != "" {
							imageNames = append(imageNames, imageName)
						}
						allImagesMatch, mismatchedImages, err := wolfi.CheckApkoLockHashes(imageNames)
						if err != nil {
							return err
						}

						if !allImagesMatch {
							std.Out.WriteLine(
								output.Linef(
									"🛠️ ",
									output.StyleBold, "Lockfiles for the following images need to be updated:\n - "+strings.Join(mismatchedImages, "\n - ")),
							)
							return errors.New("lockfiles are not up to date - run `sg wolfi lock` to update them")
						} else {
							std.Out.WriteLine(output.Linef("🛠️ ", output.StyleBold, "Lockfiles all up to date"))
						}

						return nil
					}

					opts := wolfi.BaseImageOpts{
						RepositoryAppend: repositoryAppend,
						KeyringAppend:    keyringAppend,
					}
					fmt.Printf("Keyring append is %s\n\n", keyringAppend)

					if imageName != "" {
						fmt.Printf("*** In lock section\n\n")
						bc, err := wolfi.SetupBaseImageBuild(imageName, wolfi.PackageRepoConfig{}, opts)
						fmt.Printf("Base image config: %+v\n\n", bc)
						if err != nil {
							return err
						}
						return bc.UpdateImage(ctx)
					} else {
						return wolfi.UpdateAllImages(ctx, opts)
					}
				},
			},
		},
	}
)
