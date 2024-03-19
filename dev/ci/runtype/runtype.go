package runtype

import (
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// RunType indicates the type of this run. Each CI pipeline can only be a single run type.
type RunType int

const (
	// RunTypes should be defined by order of precedence.

	PullRequest       RunType = iota // pull request build
	ManuallyTriggered                // build that is manually triggred - typically used to start CI for external contributions

	BextNightly       // browser extension nightly build
	BextManualNightly // browser extension nightly build, triggered with a branch pattern
	WolfiBaseRebuild  // wolfi base image build

	// Release branches

	TaggedRelease     // semver-tagged release
	ReleaseBranch     // release branch build
	BextReleaseBranch // browser extension release build

	InternalRelease // Internal release
	PromoteRelease  // Public release

	// Main branches

	MainBranch // main branch build
	MainDryRun // run everything main does, except for deploy-related steps

	// Build branches (NOT releases)

	ImagePatch          // build a patched image after testing
	ImagePatchNoTest    // build a patched image without testing
	ExecutorPatchNoTest // build executor image without testing
	CandidatesNoTest    // build one or all candidate images without testing
	CloudEphemeral      // build all images and push to cloud ephemeral registry for use with cloud deployment

	BazelDo // run a specific bazel command

	// None is a no-op, add all run types above this type.
	None
)

// Compute determines what RunType matches the given parameters.
func Compute(tag, branch string, env map[string]string) RunType {
	for runType := PullRequest + 1; runType < None; runType += 1 {
		if runType.Matcher().Matches(tag, branch, env) {
			return runType
		}
	}
	// RunType is PullRequest by default
	return PullRequest
}

// RunTypes returns all runtypes.
func RunTypes() []RunType {
	var results []RunType
	for runType := PullRequest + 1; runType < None; runType += 1 {
		results = append(results, runType)
	}
	return results
}

// Is returns true if this run type is one of the given RunTypes
func (t RunType) Is(oneOfTypes ...RunType) bool {
	for _, rt := range oneOfTypes {
		if t == rt {
			return true
		}
	}
	return false
}

// Matcher returns the requirements for a build to be considered of this RunType.
func (t RunType) Matcher() *RunTypeMatcher {
	switch t {
	case PromoteRelease:
		return &RunTypeMatcher{
			EnvIncludes: map[string]string{
				"RELEASE_PUBLIC": "true",
			},
		}
	case InternalRelease:
		return &RunTypeMatcher{
			EnvIncludes: map[string]string{
				"RELEASE_INTERNAL": "true",
			},
		}
	case BextNightly:
		return &RunTypeMatcher{
			EnvIncludes: map[string]string{
				"BEXT_NIGHTLY": "true",
			},
		}
	case BextManualNightly:
		return &RunTypeMatcher{
			Branch: "bext-nightly/",
		}
	case WolfiBaseRebuild:
		return &RunTypeMatcher{
			EnvIncludes: map[string]string{
				"WOLFI_BASE_REBUILD": "true",
			},
		}
	case TaggedRelease:
		return &RunTypeMatcher{
			TagPrefix: "v",
		}
	case ReleaseBranch:
		return &RunTypeMatcher{
			Branch:       `^[0-9]+\.[0-9]+$`,
			BranchRegexp: true,
		}
	case BextReleaseBranch:
		return &RunTypeMatcher{
			Branch:      "bext/release",
			BranchExact: true,
		}

	case MainBranch:
		return &RunTypeMatcher{
			Branch:      "main",
			BranchExact: true,
		}
	case MainDryRun:
		return &RunTypeMatcher{
			Branch: "main-dry-run/",
		}
	case ManuallyTriggered:
		return &RunTypeMatcher{
			Branch: "_manually_triggered_external/",
		}
	case ImagePatch:
		return &RunTypeMatcher{
			Branch:                 "docker-images-patch/",
			BranchArgumentRequired: true,
		}
	case ImagePatchNoTest:
		return &RunTypeMatcher{
			Branch:                 "docker-images-patch-notest/",
			BranchArgumentRequired: true,
		}
	case ExecutorPatchNoTest:
		return &RunTypeMatcher{
			Branch: "executor-patch-notest/",
		}

	case CandidatesNoTest:
		return &RunTypeMatcher{
			Branch: "docker-images-candidates-notest/",
		}
	case BazelDo:
		return &RunTypeMatcher{
			Branch: "bazel-do/",
		}
	case CloudEphemeral:
		return &RunTypeMatcher{
			Branch: "cloud-ephemeral/",
		}

	}

	return nil
}

func (t RunType) String() string {
	switch t {
	case PullRequest:
		return "Pull request"
	case ManuallyTriggered:
		return "Manually Triggered External Build"
	case BextNightly:
		return "Browser extension nightly release build"
	case BextManualNightly:
		return "Manually triggered browser extension nightly release build"
	case WolfiBaseRebuild:
		return "Wolfi base images rebuild"
	case TaggedRelease:
		return "Tagged release"
	case ReleaseBranch:
		return "Release branch"
	case BextReleaseBranch:
		return "Browser extension release build"
	case MainBranch:
		return "Main branch"
	case MainDryRun:
		return "Main dry run"
	case ImagePatch:
		return "Patch image"
	case ImagePatchNoTest:
		return "Patch image without testing"
	case CandidatesNoTest:
		return "Build all candidates without testing"
	case ExecutorPatchNoTest:
		return "Build executor without testing"
	case BazelDo:
		return "Bazel command"
	case InternalRelease:
		return "Internal release"
	case PromoteRelease:
		return "Public release"
	case CloudEphemeral:
		return "Cloud ephemeral"
	}
	return "None"
}

// RunTypeMatcher defines the requirements for any given build to be considered a build of
// this RunType.
type RunTypeMatcher struct {
	// Branch loosely matches branches that begin with this value, unless a different type
	// of match is indicated (e.g. BranchExact, BranchRegexp)
	Branch       string
	BranchExact  bool
	BranchRegexp bool
	// BranchArgumentRequired indicates the path segment following the branch prefix match is
	// expected to be an argument (does not work in conjunction with BranchExact)
	BranchArgumentRequired bool

	// TagPrefix matches tags that begin with this value.
	TagPrefix string

	// EnvIncludes validates if these key-value pairs are configured in environment.
	EnvIncludes map[string]string
}

// Matches returns true if the given properties and environment match this RunType.
func (m *RunTypeMatcher) Matches(tag, branch string, env map[string]string) bool {
	if m.Branch != "" {
		switch {
		case m.BranchExact:
			return m.Branch == branch
		case m.BranchRegexp:
			return lazyregexp.New(m.Branch).MatchString(branch)
		default:
			return strings.HasPrefix(branch, m.Branch)
		}
	}

	if m.TagPrefix != "" {
		return strings.HasPrefix(tag, m.TagPrefix)
	}

	if len(m.EnvIncludes) > 0 && len(env) > 0 {
		for wantK, wantV := range m.EnvIncludes {
			gotV, exists := env[wantK]
			if !exists || (wantV != gotV) {
				return false
			}
		}
		return true
	}

	return false
}

// IsBranchPrefixMatcher indicates that this matcher matches on branch prefixes.
func (m *RunTypeMatcher) IsBranchPrefixMatcher() bool {
	return m.Branch != "" && !m.BranchExact && !m.BranchRegexp
}

// ExtractBranchArgument extracts the second segment, delimited by '/', of the branch as
// an argument, for example:
//
//	prefix/{argument}
//	prefix/{argument}/something-else
//
// If BranchArgumentRequired, an error is returned if no argument is found.
//
// Only works with Branch matches, and does not work with BranchExact.
func (m *RunTypeMatcher) ExtractBranchArgument(branch string) (string, error) {
	if m.BranchExact || m.Branch == "" {
		return "", errors.New("unsupported matcher type")
	}

	parts := strings.Split(branch, "/")
	if len(parts) < 2 || len(parts[1]) == 0 {
		if m.BranchArgumentRequired {
			return "", errors.New("branch argument expected, but none found")
		}
		return "", nil
	}
	return parts[1], nil
}
