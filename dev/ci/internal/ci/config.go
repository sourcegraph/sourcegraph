package ci

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/dev/ci/images"
	"github.com/sourcegraph/sourcegraph/dev/ci/internal/ci/changed"
	"github.com/sourcegraph/sourcegraph/dev/ci/runtype"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Config is the set of configuration parameters that determine the structure of the CI build. These
// parameters are extracted from the build environment (branch name, commit hash, timestamp, etc.)
type Config struct {
	// RunType indicates what kind of pipeline run should be generated, based on various
	// bits of metadata
	RunType runtype.RunType

	// Build metadata
	Time        time.Time
	Branch      string
	Version     string
	Commit      string
	BuildNumber int

	// Diff denotes what has changed since the merge-base with origin/main.
	Diff changed.Diff
	// ChangedFiles lists files that have changed, group by diff type
	ChangedFiles changed.ChangedFiles

	// MustIncludeCommit, if non-empty, is a list of commits at least one of which must be present
	// in the branch. If empty, then no check is enforced.
	MustIncludeCommit []string

	// MessageFlags contains flags parsed from commit messages.
	MessageFlags MessageFlags

	// Notify declares configuration required to generate notifications.
	Notify SlackNotification
}

type SlackNotification struct {
	// An Buildkite "Notification service" must exist for this channel in order for notify
	// to work. This is configured here: https://buildkite.com/organizations/sourcegraph/services
	//
	// Under "Choose notifications to send", uncheck the option for "Failed" state builds.
	// Failure notifications will be generated by the pipeline generator.
	Channel string
	// This Slack token is used for retrieving Slack user data to generate messages.
	SlackToken string
}

// NewConfig computes configuration for the pipeline generator based on Buildkite environment
// variables.
func NewConfig(now time.Time) Config {
	var (
		commit = os.Getenv("BUILDKITE_COMMIT")
		branch = os.Getenv("BUILDKITE_BRANCH")
		tag    = os.Getenv("BUILDKITE_TAG")
		// evaluates what type of pipeline run this is
		runType = runtype.Compute(tag, branch, map[string]string{
			"BEXT_NIGHTLY":       os.Getenv("BEXT_NIGHTLY"),
			"RELEASE_NIGHTLY":    os.Getenv("RELEASE_NIGHTLY"),
			"VSCE_NIGHTLY":       os.Getenv("VSCE_NIGHTLY"),
			"WOLFI_BASE_REBUILD": os.Getenv("WOLFI_BASE_REBUILD"),
			"RELEASE_INTERNAL":   os.Getenv("RELEASE_INTERNAL"),
			"RELEASE_PUBLIC":     os.Getenv("RELEASE_PUBLIC"),
		})
		// defaults to 0
		buildNumber, _ = strconv.Atoi(os.Getenv("BUILDKITE_BUILD_NUMBER"))
	)

	var mustIncludeCommits []string
	if rawMustIncludeCommit := os.Getenv("MUST_INCLUDE_COMMIT"); rawMustIncludeCommit != "" {
		mustIncludeCommits = strings.Split(rawMustIncludeCommit, ",")
		for i := range mustIncludeCommits {
			mustIncludeCommits[i] = strings.TrimSpace(mustIncludeCommits[i])
		}
	}

	// detect changed files
	var changedFiles []string
	diffCommand := []string{"diff", "--name-only"}
	if commit != "" {
		if runType.Is(runtype.MainBranch) {
			// We run builds on every commit in main, so on main, just look at the diff of the current commit.
			diffCommand = append(diffCommand, "@^")
		} else {
			baseBranch := os.Getenv("BUILDKITE_PULL_REQUEST_BASE_BRANCH")
			if diffArgs, err := determineDiffArgs(baseBranch, commit); err != nil {
				panic(err)
			} else {
				// the base we want to diff against should exist locally now so we can diff!
				diffCommand = append(diffCommand, diffArgs)
			}
		}
	} else {
		diffCommand = append(diffCommand, "origin/main...")
		// for testing
		commit = "1234567890123456789012345678901234567890"
	}
	fmt.Fprintf(os.Stderr, "running diff command: git %v\n", diffCommand)
	if output, err := exec.Command("git", diffCommand...).Output(); err != nil {
		panic(err)
	} else {
		changedFiles = strings.Split(strings.TrimSpace(string(output)), "\n")
	}

	diff, changedFilesByDiffType := changed.ParseDiff(changedFiles)

	fmt.Fprintf(os.Stderr, "Parsed diff:\n\tgit command: %v\n\tchanged files: %v\n\tdiff changes: %q\n",
		append([]string{"git"}, diffCommand...),
		changedFiles,
		diff.String(),
	)
	fmt.Fprint(os.Stderr, "The generated build pipeline will now follow, see you next time!\n")

	return Config{
		RunType: runType,

		Time:              now,
		Branch:            branch,
		Version:           inferVersion(runType, tag, commit, buildNumber, branch, now),
		Commit:            commit,
		MustIncludeCommit: mustIncludeCommits,
		Diff:              diff,
		ChangedFiles:      changedFilesByDiffType,
		BuildNumber:       buildNumber,

		// get flags from commit message
		MessageFlags: parseMessageFlags(os.Getenv("BUILDKITE_MESSAGE")),

		Notify: SlackNotification{
			Channel:    "#buildkite-main",
			SlackToken: os.Getenv("SLACK_INTEGRATION_TOKEN"),
		},
	}
}

func determineDiffArgs(baseBranch, commit string) (string, error) {
	// We have a different base branch (possibily) and on aspect agents we are in a detached state with only 100 commit depth
	// so we might not know about this base branch ... so we first fetch the base and then diff
	//
	// Determine the base branch
	if baseBranch == "" {
		// When the base branch is not set, then this is probably a build where a commit got merged
		// onto the current branch. So we just diff with the current commit
		return "@^", nil
	}

	// fetch the branch to make sure it exists
	refspec := fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", baseBranch, baseBranch)
	if _, err := exec.Command("git", "fetch", "origin", refspec).Output(); err != nil {
		return "", errors.Newf("failed to fetch %s: %s", baseBranch, err)
	} else {
		fmt.Fprintf(os.Stderr, "fetched %s\n", baseBranch)
		return fmt.Sprintf("origin/%s...%s", baseBranch, commit), nil
	}
}

// inferVersion constructs the Sourcegraph version from the given build state.
func inferVersion(runType runtype.RunType, tag string, commit string, buildNumber int, branch string, now time.Time) string {
	// If we're building a release, use the version that is being released regardless of
	// all other build attributes, such as tag, commit, build number, etc ...
	if runType.Is(runtype.InternalRelease, runtype.PromoteRelease) {
		return os.Getenv("VERSION")
	}

	if runType.Is(runtype.TaggedRelease) {
		// This tag is used for publishing versioned releases.
		//
		// The Git tag "v1.2.3" should map to the Docker image "1.2.3" (without v prefix).
		return strings.TrimPrefix(tag, "v")
	}

	// "main" branch is used for continuous deployment and has a special-case format
	version := images.BranchImageTag(now, commit, buildNumber, sanitizeBranchForDockerTag(branch), tryGetLatestTag())

	// Add additional patch suffix
	if runType.Is(runtype.ImagePatch, runtype.ImagePatchNoTest, runtype.ExecutorPatchNoTest) {
		version = version + "_patch"
	}

	return version
}

func tryGetLatestTag() string {
	output, err := exec.Command("git", "tag", "--list", "v*").CombinedOutput()
	if err != nil {
		return ""
	}

	tagMap := map[string]struct{}{}
	for _, tag := range strings.Split(string(output), "\n") {
		if version, ok := oobmigration.NewVersionFromString(tag); ok {
			tagMap[version.String()] = struct{}{}
		}
	}
	if len(tagMap) == 0 {
		return ""
	}

	versions := make([]oobmigration.Version, 0, len(tagMap))
	for tag := range tagMap {
		version, _ := oobmigration.NewVersionFromString(tag)
		versions = append(versions, version)
	}
	oobmigration.SortVersions(versions)

	return versions[len(versions)-1].String()
}

func (c Config) shortCommit() string {
	// http://git-scm.com/book/en/v2/Git-Tools-Revision-Selection#Short-SHA-1
	if len(c.Commit) < 12 {
		return c.Commit
	}

	return c.Commit[:12]
}

func (c Config) ensureCommit() error {
	if len(c.MustIncludeCommit) == 0 {
		return nil
	}

	found := false
	var errs error
	for _, mustIncludeCommit := range c.MustIncludeCommit {
		output, err := exec.Command("git", "merge-base", "--is-ancestor", mustIncludeCommit, "HEAD").CombinedOutput()
		if err == nil {
			found = true
			break
		}
		errs = errors.Append(errs, errors.Errorf("%v | Output: %q", err, string(output)))
	}
	if !found {
		fmt.Printf("This branch %q at commit %s does not include any of these commits: %s.\n", c.Branch, c.Commit, strings.Join(c.MustIncludeCommit, ", "))
		fmt.Println("Rebase onto the latest main to get the latest CI fixes.")
		fmt.Printf("Errors from `git merge-base --is-ancestor $COMMIT HEAD`: %s", errs)
		return errs
	}
	return nil
}

// candidateImageTag provides the tag for a candidate image built for this Buildkite run.
//
// Note that the availability of this image depends on whether a candidate gets built,
// as determined in `addDockerImages()`.
func (c Config) candidateImageTag() string {
	return images.CandidateImageTag(c.Commit, c.BuildNumber)
}

// MessageFlags indicates flags that can be parsed out of commit messages to change
// pipeline behaviour. Use sparingly! If you are generating a new pipeline, please use
// RunType instead.
type MessageFlags struct {
	// ProfilingEnabled, if true, tells buildkite to print timing and resource utilization information
	// for each command
	ProfilingEnabled bool

	// SkipHashCompare, if true, tells buildkite to disable skipping of steps that compare
	// hash output.
	SkipHashCompare bool

	// ForceReadyForReview, if true will skip the draft pull request check and run the Chromatic steps.
	// This allows a user to run the job without marking their PR as ready for review
	ForceReadyForReview bool

	// NoBazel, if true prevents automatic replacement of job with their Bazel equivalents.
	NoBazel bool
}

// parseMessageFlags gets MessageFlags from the given commit message.
func parseMessageFlags(msg string) MessageFlags {
	return MessageFlags{
		ProfilingEnabled:    strings.Contains(msg, "[buildkite-enable-profiling]"),
		SkipHashCompare:     strings.Contains(msg, "[skip-hash-compare]"),
		ForceReadyForReview: strings.Contains(msg, "[review-ready]"),
	}
}

func sanitizeBranchForDockerTag(branch string) string {
	branch = strings.ReplaceAll(branch, "/", "-")
	branch = strings.ReplaceAll(branch, "+", "-")
	return branch
}
