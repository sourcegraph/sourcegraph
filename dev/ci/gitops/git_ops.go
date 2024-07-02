package gitops

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var ErrNoTags = errors.New("no tags found")

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
		return fmt.Sprintf("origin/%s...%s", baseBranch, commit), nil
	}
}

func GetHEADChangedFiles() ([]string, error) {
	output, err := exec.Command("git", "diff", "--name-only", "@^").CombinedOutput()
	if err != nil {
		return nil, err
	}
	changedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	return changedFiles, nil
}

func GetBranchChangedFiles(baseBranch, commit string) ([]string, error) {
	diffArgs, err := determineDiffArgs(baseBranch, commit)
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(os.Stderr, "git diff --name-only", diffArgs)
	output, err := exec.Command("git", "diff", "--name-only", diffArgs).CombinedOutput()
	if err != nil {
		return nil, err
	}
	changedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	return changedFiles, nil
}

func GetLatestTag() (string, error) {
	output, err := exec.Command("git", "tag", "--list", "v*").CombinedOutput()
	if err != nil {
		return "", err
	}

	tagMap := map[string]struct{}{}
	for _, tag := range strings.Split(string(output), "\n") {
		if version, ok := oobmigration.NewVersionFromString(tag); ok {
			tagMap[version.String()] = struct{}{}
		}
	}
	if len(tagMap) == 0 {
		return "", ErrNoTags
	}

	versions := make([]oobmigration.Version, 0, len(tagMap))
	for tag := range tagMap {
		version, _ := oobmigration.NewVersionFromString(tag)
		versions = append(versions, version)
	}
	oobmigration.SortVersions(versions)

	return versions[len(versions)-1].String(), nil
}

func HasIncludedCommit(commits ...string) (bool, error) {
	found := false
	var errs error
	for _, mustIncludeCommit := range commits {
		output, err := exec.Command("git", "merge-base", "--is-ancestor", mustIncludeCommit, "HEAD").CombinedOutput()
		if err == nil {
			found = true
			break
		}
		errs = errors.Append(errs, errors.Errorf("%v | Output: %q", err, string(output)))
	}

	return found, errs
}
