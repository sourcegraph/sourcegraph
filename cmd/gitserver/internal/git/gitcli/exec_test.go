package gitcli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
)

func TestIsAllowedGitCmd(t *testing.T) {
	isAllowed := [][]string{
		// Required for code monitors
		{"rev-parse", "HEAD"},
		{"rev-parse", "83838383"},
		{"rev-parse", "--glob=refs/heads/*"},
		{"rev-parse", "--glob=refs/heads/*", "--exclude=refs/heads/cc/*"},
	}
	notAllowed := [][]string{
		{"commit", "-F", "/etc/passwd"},
		{"commit", "--file=/absolute/path"},
		{"commit", "-F", "relative/passwd"},
		{"commit", "--file=relative/path"},
	}

	logger := logtest.NoOp(t)
	for _, args := range isAllowed {
		t.Run("", func(t *testing.T) {
			if !IsAllowedGitCmd(logger, args[0], args[1:], "/fake/path") {
				t.Fatalf("expected args to be allowed: %q", args)
			}
		})
	}
	for _, args := range notAllowed {
		t.Run("", func(t *testing.T) {
			if IsAllowedGitCmd(logger, args[0], args[1:], "/fake/path") {
				t.Fatalf("expected args to NOT be allowed: %q", args)
			}
		})
	}
}

func TestIsAllowedDiffGitCmd(t *testing.T) {
	allowed := []struct {
		args []string
		pass bool
	}{
		{args: []string{"diff", "HEAD", "83838383"}, pass: true},
		{args: []string{"diff", "HEAD", "HEAD~10"}, pass: true},
		{args: []string{"diff", "HEAD", "HEAD~10", "--", "foo"}, pass: true},
		{args: []string{"diff", "HEAD", "HEAD~10", "--", "foo/baz"}, pass: true},
		{args: []string{"diff", "ORIG_HEAD", "@~10", "--", "foo/baz"}, pass: true},
		{args: []string{"diff", "HEAD~10", "--", "foo"}, pass: true},
		{args: []string{"diff", "HEAD", "HEAD~10", "--", "foo/baz"}, pass: true},
		{args: []string{"diff", "refs/heads/list", "HEAD~10", "--", "/foo/baz"}, pass: true},
		{args: []string{"diff", "/dev/null", "/etc/passwd"}, pass: false},
		{args: []string{"diff", "/etc/hosts", "/etc/passwd"}, pass: false},
		{args: []string{"diff", "/dev/null", "/etc/passwd"}, pass: false},
		{args: []string{"diff", "/etc/hosts", "/etc/passwd"}, pass: false},
		{args: []string{"diff", "--", "/etc/hosts", "/etc/passwd"}, pass: false},
		{args: []string{"diff", "--", "/etc/ees", "/etc/ee"}, pass: false},
		{args: []string{"diff", "--", "../../../etc/ees", "/etc/ee"}, pass: false},
		{args: []string{"diff", "--", "a/test.txt", "b/test.txt"}, pass: true},
		{args: []string{"diff", "--find-renames"}, pass: true},
		{args: []string{"diff", "a1c0f7d19f6e9eb76facc67c1c22c07bb2ad39c4...c70f79c26526ba74f38ecff2e1e686fc3e2bdcdd"}, pass: true},
	}

	logger := logtest.NoOp(t)
	for _, cmd := range allowed {
		t.Run(fmt.Sprintf("%s returns %t", strings.Join(cmd.args, " "), cmd.pass), func(t *testing.T) {
			assert.Equal(t, cmd.pass, IsAllowedGitCmd(logger, cmd.args[0], cmd.args[1:], "/foo/baz"))
		})
	}
}

func TestStdErrIndicatesCorruption(t *testing.T) {
	bad := []string{
		"error: packfile .git/objects/pack/pack-a.pack does not match index",
		"error: Could not read d24d09b8bc5d1ea2c3aa24455f4578db6aa3afda\n",
		`error: short SHA1 1325 is ambiguous
error: Could not read d24d09b8bc5d1ea2c3aa24455f4578db6aa3afda`,
		`unrelated
error: Could not read d24d09b8bc5d1ea2c3aa24455f4578db6aa3afda`,
		"\n\nerror: Could not read d24d09b8bc5d1ea2c3aa24455f4578db6aa3afda",
		"fatal: commit-graph requires overflow generation data but has none\n",
		"\rResolving deltas: 100% (21750/21750), completed with 565 local objects.\nfatal: commit-graph requires overflow generation data but has none\nerror: https://github.com/sgtest/megarepo did not send all necessary objects\n\n\": exit status 1",
	}
	good := []string{
		"",
		"error: short SHA1 1325 is ambiguous",
		"error: object 156639577dd2ea91cdd53b25352648387d985743 is a blob, not a commit",
		"error: object 45043b3ff0440f4d7937f8c68f8fb2881759edef is a tree, not a commit",
	}
	for _, stderr := range bad {
		if !stdErrIndicatesCorruption(stderr) {
			t.Errorf("should contain corrupt line:\n%s", stderr)
		}
	}
	for _, stderr := range good {
		if stdErrIndicatesCorruption(stderr) {
			t.Errorf("should not contain corrupt line:\n%s", stderr)
		}
	}
}
