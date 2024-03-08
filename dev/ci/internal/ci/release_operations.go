package ci

import (
	bk "github.com/sourcegraph/sourcegraph/dev/ci/internal/buildkite"
	"github.com/sourcegraph/sourcegraph/dev/ci/internal/ci/operations"
)

func promoteRFC795Images(c Config) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep("Promote release to public",
			bk.Agent("queue", "bazel"),
			bk.Cmd("./tools/release/promote.sh"),
		)
	}
}
