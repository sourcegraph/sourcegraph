package ci

import (
	"fmt"
	"strings"

	bk "github.com/sourcegraph/sourcegraph/dev/ci/internal/buildkite"
	"github.com/sourcegraph/sourcegraph/dev/ci/runtype"

	"github.com/sourcegraph/sourcegraph/dev/ci/images"
	"github.com/sourcegraph/sourcegraph/dev/ci/internal/ci/operations"
)

// releasePromoteImages runs a script that iterates through all defined images that we're producing that has been uploaded
// on the internal registry with a given version and retags them to the public registry.
func releasePromoteImages(c Config) operations.Operation {
	image_args := strings.Join(images.SourcegraphDockerImages, " ")
	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep("Promote release to public",
			bk.Agent("queue", AspectWorkflows.QueueDefault),
			bk.Env("VERSION", c.Version),
			bk.Env("INTERNAL_REGISTRY", images.SourcegraphInternalReleaseRegistry),
			bk.Env("PUBLIC_REGISTRY", images.SourcegraphPublicReleaseRegistry),
			bk.AnnotatedCmd(
				fmt.Sprintf("./tools/release/promote_images.sh %s", image_args),
				bk.AnnotatedCmdOpts{
					Annotations: &bk.AnnotationOpts{
						Type:         bk.AnnotationTypeInfo,
						IncludeNames: false,
					},
				},
			),
		)
	}
}

// releaseTestOperations runs the script defined in release.yaml that tests the release.
func releaseTestOperation(c Config) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep("Release tests",
			bk.Agent("queue", AspectWorkflows.QueueDefault),
			bk.Env("VERSION", c.Version),
			bk.AnnotatedCmd(
				bazelCmd(`run --run_under="cd $$PWD &&" //dev/sg -- release run test --branch $$BUILDKITE_BRANCH`),
				bk.AnnotatedCmdOpts{
					Annotations: &bk.AnnotationOpts{
						Type:         bk.AnnotationTypeInfo,
						IncludeNames: true,
					},
				},
			))
	}
}

// releaseFinalizeOperation runs the script defined in release.yaml that finalizes the release. It picks
// the variant (internal or public) based on the run type.
//
// Important: this helper doesn't inject the `wait` step, it's on the calling side to handle that. This is
// necessary because by definition, you want to call finalize only when everything else succeeded.
func releaseFinalizeOperation(c Config) operations.Operation {
	label := "Finalize internal release"
	command := "internal"
	if c.RunType.Is(runtype.PromoteRelease) {
		label = "Final release promotion"
		command = "promote-to-public"
	}

	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep(label,
			bk.Agent("queue", AspectWorkflows.QueueDefault),
			bk.Env("VERSION", c.Version),
			bk.AnnotatedCmd(
				bazelCmd(fmt.Sprintf(`run --run_under="cd $$PWD &&" //dev/sg -- release run %s finalize --branch $$BUILDKITE_BRANCH`, command)),
				bk.AnnotatedCmdOpts{
					Annotations: &bk.AnnotationOpts{
						Type:         bk.AnnotationTypeInfo,
						IncludeNames: true,
					},
				},
			))
	}
}
