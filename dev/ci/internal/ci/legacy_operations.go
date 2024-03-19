package ci

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/sourcegraph/dev/ci/images"
	bk "github.com/sourcegraph/sourcegraph/dev/ci/internal/buildkite"
	"github.com/sourcegraph/sourcegraph/dev/ci/internal/ci/operations"
	"github.com/sourcegraph/sourcegraph/dev/ci/runtype"
)

// Keep: allows building an array of images on one agent. Useful for streamlining and rules_oci in the future.
func legacyBuildCandidateDockerImages(apps []string, version string, tag string, rt runtype.RunType) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		cmds := []bk.StepOpt{}

		cmds = append(cmds,
			bk.Key(candidateImageStepKey(apps[0])),
			bk.Env("DOCKER_BAZEL", "true"),
			bk.Env("DOCKER_BUILDKIT", "1"),
			bk.Env("VERSION", version),
			bk.Agent("queue", AspectWorkflows.QueueDefault),
		)

		// Allow all build scripts to emit info annotations
		// TODO(JH) probably remove
		buildAnnotationOptions := bk.AnnotatedCmdOpts{
			Annotations: &bk.AnnotationOpts{
				Type:         bk.AnnotationTypeInfo,
				IncludeNames: true,
			},
		}

		for _, app := range apps {
			image := strings.ReplaceAll(app, "/", "-")
			localImage := "sourcegraph/" + image + ":" + version

			// Add Sentry environment variables if we are building off main branch
			// to enable building the webapp with source maps enabled
			if rt.Is(runtype.MainDryRun, runtype.CloudEphemeral) && app == "frontend" {
				cmds = append(cmds,
					bk.Env("SENTRY_UPLOAD_SOURCE_MAPS", "1"),
					bk.Env("SENTRY_ORGANIZATION", "sourcegraph"),
					bk.Env("SENTRY_PROJECT", "sourcegraph-dot-com"),
				)
			}

			cmds = append(cmds,
				bk.Cmd(fmt.Sprintf(`echo "--- Building candidate %s image..."`, app)),
				bk.Cmd("export IMAGE='"+localImage+"'"),
			)

			if _, err := os.Stat(filepath.Join("docker-images", app)); err == nil {
				// Building Docker image located under $REPO_ROOT/docker-images/
				buildScriptPath := filepath.Join("docker-images", app, "build.sh")
				cmds = append(cmds,
					bk.Cmd("ls -lah "+buildScriptPath),
					bk.Cmd(buildScriptPath),
				)
			} else if _, err := os.Stat(filepath.Join("client", app)); err == nil {
				// Building Docker image located under $REPO_ROOT/client/
				cmds = append(cmds, bk.AnnotatedCmd("client/"+app+"/build.sh", buildAnnotationOptions))
			} else {
				// Building Docker images located under $REPO_ROOT/cmd/
				cmdDir := func() string {
					folder := app
					if app == "blobstore2" {
						// experiment: cmd/blobstore is a Go rewrite of docker-images/blobstore. While
						// it is incomplete, we do not want cmd/blobstore/Dockerfile to get published
						// under the same name.
						// https://github.com/sourcegraph/sourcegraph/issues/45594
						// TODO(blobstore): remove this when making Go blobstore the default
						folder = "blobstore"
					}

					return "cmd/" + folder
				}()
				buildScriptPath := filepath.Join(cmdDir, "build.sh")
				cmds = append(cmds, bk.AnnotatedCmd(buildScriptPath, buildAnnotationOptions))
			}

			devImage := images.DevRegistryImage(app, tag)
			cmds = append(cmds,
				bk.Cmd(fmt.Sprintf(`echo "--- Tagging and Pushing candidate %s image..."`, app)),
				// Retag the local image for dev registry
				bk.Cmd(fmt.Sprintf("docker tag %s %s", localImage, devImage)),
				// Publish tagged image
				bk.Cmd(fmt.Sprintf("docker push %s || exit 10", devImage)),
				// Retry in case of flakes when pushing
				// bk.AutomaticRetryStatus(3, 10),
				// Retry in case of flakes when pushing
				// bk.AutomaticRetryStatus(3, 222),
			)
		}
		pipeline.AddStep(":bazel::docker: :construction: Build Docker images", cmds...)
	}
}

func legacyBuildCandidateDockerImage(app string, version string, tag string, rt runtype.RunType) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		cmds := []bk.StepOpt{}
		cmds = append(cmds,
			bk.Key(candidateImageStepKey(app)),
			bk.Env("VERSION", version),
			bk.Agent("queue", AspectWorkflows.QueueDefault),
		)

		// Allow all build scripts to emit info annotations
		// TODO(JH) probably remove
		buildAnnotationOptions := bk.AnnotatedCmdOpts{
			Annotations: &bk.AnnotationOpts{
				Type:         bk.AnnotationTypeInfo,
				IncludeNames: true,
			},
		}

		image := strings.ReplaceAll(app, "/", "-")
		localImage := "sourcegraph/" + image + ":" + version

		// Add Sentry environment variables if we are building off main branch
		// to enable building the webapp with source maps enabled
		if rt.Is(runtype.MainDryRun, runtype.CloudEphemeral) && app == "frontend" {
			cmds = append(cmds,
				bk.Env("SENTRY_UPLOAD_SOURCE_MAPS", "1"),
				bk.Env("SENTRY_ORGANIZATION", "sourcegraph"),
				bk.Env("SENTRY_PROJECT", "sourcegraph-dot-com"),
			)
		}

		cmds = append(cmds,
			bk.Cmd(fmt.Sprintf(`echo "--- Building candidate %s image..."`, app)),
			bk.Cmd("export IMAGE='"+localImage+"'"),
		)

		if _, err := os.Stat(filepath.Join("docker-images", app)); err == nil {
			// Building Docker image located under $REPO_ROOT/docker-images/
			buildScriptPath := filepath.Join("docker-images", app, "build.sh")

			cmds = append(cmds, bk.Cmd(buildScriptPath))
		} else if _, err := os.Stat(filepath.Join("client", app)); err == nil {
			// Building Docker image located under $REPO_ROOT/client/
			cmds = append(cmds, bk.AnnotatedCmd("client/"+app+"/build.sh", buildAnnotationOptions))
		} else {
			// Building Docker images located under $REPO_ROOT/cmd/
			cmdDir := func() string {
				folder := app
				if app == "blobstore2" {
					// experiment: cmd/blobstore is a Go rewrite of docker-images/blobstore. While
					// it is incomplete, we do not want cmd/blobstore/Dockerfile to get published
					// under the same name.
					// https://github.com/sourcegraph/sourcegraph/issues/45594
					// TODO(blobstore): remove this when making Go blobstore the default
					folder = "blobstore"
				}

				return "cmd/" + folder
			}()
			buildScriptPath := filepath.Join(cmdDir, "build.sh")
			cmds = append(cmds, bk.AnnotatedCmd(buildScriptPath, buildAnnotationOptions))
		}

		devImage := images.DevRegistryImage(app, tag)
		cmds = append(cmds,
			bk.Cmd(fmt.Sprintf(`echo "--- Tagging and Pushing candidate %s image..."`, app)),
			// Retag the local image for dev registry
			bk.Cmd(fmt.Sprintf("docker tag %s %s", localImage, devImage)),
			// Publish tagged image
			bk.Cmd(fmt.Sprintf("docker push %s || exit 10", devImage)),
			// Retry in case of flakes when pushing
			// bk.AutomaticRetryStatus(3, 10),
			// Retry in case of flakes when pushing
			// bk.AutomaticRetryStatus(3, 222),
		)
		pipeline.AddStep(fmt.Sprintf(":older_man::docker: :construction: Build %s", app), cmds...)
	}
}
