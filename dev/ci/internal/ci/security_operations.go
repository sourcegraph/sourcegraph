package ci

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/dev/ci/images"
	bk "github.com/sourcegraph/sourcegraph/dev/ci/internal/buildkite"
	"github.com/sourcegraph/sourcegraph/dev/ci/internal/ci/operations"
)

func semgrepScan() operations.Operation {
	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep(
			"Semgrep SAST Scan",
			bk.Cmd("dev/ci/semgrep-scan.sh"),
		)
	}

}

// Ask trivy, a security scanning tool, to scan the candidate image
// specified by "app" and "tag".
func trivyScanCandidateImage(app, tag string) operations.Operation {
	// hack to prevent trivy scanes of blobstore and server images due to timeouts,
	// even with extended deadlines
	if app == "blobstore" || app == "server" {
		return func(pipeline *bk.Pipeline) {
			// no-op
		}
	}

	image := images.DevRegistryImage(app, tag)

	// This is the special exit code that we tell trivy to use
	// if it finds a vulnerability. This is also used to soft-fail
	// this step.
	vulnerabilityExitCode := 27

	// For most images, waiting on the server is fine. But with the recent migration to Bazel,
	// this can lead to confusing failures. This will be completely refactored soon.
	//
	// See https://github.com/sourcegraph/sourcegraph/issues/52833 for the ticket tracking
	// the cleanup once we're out of the dual building process.
	dependsOnImage := candidateImageStepKey("server")
	if app == "syntax-highlighter" {
		dependsOnImage = candidateImageStepKey("syntax-highlighter")
	}
	if app == "symbols" {
		dependsOnImage = candidateImageStepKey("symbols")
	}

	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep(fmt.Sprintf(":trivy: :docker: :mag: Scan %s", app),
			// These are the first images in the arrays we use to build images
			bk.DependsOn(dependsOnImage),
			bk.Cmd(fmt.Sprintf("docker pull %s", image)),

			// have trivy use a shorter name in its output
			bk.Cmd(fmt.Sprintf("docker tag %s %s", image, app)),

			bk.Env("IMAGE", app),
			bk.Env("VULNERABILITY_EXIT_CODE", fmt.Sprintf("%d", vulnerabilityExitCode)),
			bk.ArtifactPaths("./*-security-report.html"),
			bk.SoftFail(vulnerabilityExitCode),
			bk.AutomaticRetryStatus(1, 1), // exit status 1 is what happens this flakes on container pulling

			bk.AnnotatedCmd("./dev/ci/trivy/trivy-scan-high-critical.sh", bk.AnnotatedCmdOpts{
				Annotations: &bk.AnnotationOpts{
					Type:            bk.AnnotationTypeWarning,
					MultiJobContext: "docker-security-scans",
				},
			}))
	}
}
