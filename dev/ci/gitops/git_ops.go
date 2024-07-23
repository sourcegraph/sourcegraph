package gitops

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var ErrNoTags = errors.New("no tags found")

// HandleGitCommandExec There's a weird behavior that occurs where an error isn't accessible in the err variable
// from a *Cmd executing a git command after calling CombinedOutput().
// This occurs due to how Git handles errors and how the exec package in Go interprets the command's output.
// Git often writes error messages to stderr, but it might still exit with a status code of 0 (indicating success).
// In this case, CombinedOutput() won't return an error, but the error message will be in the out variable.
func HandleGitCommandExec(cmd *exec.Cmd) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		maybeErrMessage := strings.Trim(stderr.String(), "\n")
		if strings.HasPrefix(maybeErrMessage, "fatal:") || strings.HasPrefix(maybeErrMessage, "error:") {
			return nil, errors.New(maybeErrMessage)
		}
		return nil, err
	}

	return stdout.Bytes(), nil
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
		return fmt.Sprintf("origin/%s...%s", baseBranch, commit), nil
	}
}

func GetHEADChangedFiles() ([]string, error) {
	output, err := HandleGitCommandExec(exec.Command("git", "diff", "--name-only", "@^"))
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

	output, err := HandleGitCommandExec(exec.Command("git", "diff", "--name-only", diffArgs))
	if err != nil {
		return nil, err
	}
	changedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	return changedFiles, nil
}

func GetLatestTag() (string, error) {
	output, err := HandleGitCommandExec(exec.Command("git", "tag", "--list", "v*"))
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
		output, err := HandleGitCommandExec(exec.Command("git", "merge-base", "--is-ancestor", mustIncludeCommit, "HEAD"))
		if err == nil {
			found = true
			break
		}
		errs = errors.Append(errs, errors.Errorf("%v | Output: %q", err, string(output)))
	}

	return found, errs
}
