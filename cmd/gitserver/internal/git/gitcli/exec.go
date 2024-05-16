package gitcli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/grafana/regexp"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (g *gitCLIBackend) Exec(ctx context.Context, args ...string) (io.ReadCloser, error) {
	arguments := []Argument{}
	cmd := args[0]
	// Verify that the command sent is part of our allow list.
	if _, ok := gitCmdAllowlist[cmd]; !ok {
		return nil, errors.Errorf("git command %q not allowed", cmd)
	}
	for _, arg := range args[1:] {
		// All arguments from the exec endpoint are potentially risky.
		arguments = append(arguments, SpecSafeValueArgument{arg})
	}
	return g.NewCommand(ctx, cmd, WithArguments(arguments...))
}

var (
	// gitCmdAllowlist are commands and arguments that are allowed to execute and are
	// checked by IsAllowedGitCmd
	gitCmdAllowlist = map[string][]string{
		"log":       append([]string{}, gitCommonAllowlist...),
		"show":      append([]string{}, gitCommonAllowlist...),
		"remote":    {"-v"},
		"diff":      append([]string{}, gitCommonAllowlist...),
		"diff-tree": append([]string{"--root"}, gitCommonAllowlist...),
		"blame":     {},
		"branch":    {"-r", "-a", "--contains", "--merged", "--format"},

		"rev-parse":    {"--abbrev-ref", "--symbolic-full-name", "--glob", "--exclude"},
		"archive":      {"--worktree-attributes", "--format", "-0", "HEAD", "--"},
		"ls-tree":      {"--name-only", "HEAD", "--long", "--full-name", "--object-only", "--", "-z", "-r", "-t"},
		"ls-files":     {"--with-tree", "-z"},
		"for-each-ref": {"--points-at", "--contains"},
		"tag":          {"--list", "--sort", "-creatordate", "--format", "--points-at"},
		"merge-base":   {"--"},
		"show-ref":     {"--heads"},
		"shortlog":     {"--summary", "--numbered", "--email", "--no-merges", "--after", "--before"},
		"cat-file":     {"-p", "-t"},
	}

	// `git log`, `git show`, `git diff`, etc., share a large common set of allowed args.
	gitCommonAllowlist = []string{
		"--name-only", "--name-status", "--full-history", "-M", "--date", "--format", "-i", "-n", "-n1", "-m", "--", "-n200", "-n2", "--follow", "--author", "--grep", "--date-order", "--decorate", "--skip", "--max-count", "--numstat", "--pretty", "--parents", "--topo-order", "--raw", "--follow", "--all", "--before", "--no-merges", "--fixed-strings",
		"--patch", "--unified", "-S", "-G", "--pickaxe-all", "--pickaxe-regex", "--function-context", "--branches", "--source", "--src-prefix", "--dst-prefix", "--no-prefix",
		"--regexp-ignore-case", "--glob", "--cherry", "-z", "--reverse", "--ignore-submodules",
		"--until", "--since", "--author", "--committer",
		"--no-color", "--decorate", "--no-patch", "--exclude",
		"--no-merges",
		"--no-renames",
		"--full-index",
		"--find-copies",
		"--find-renames",
		"--first-parent",
		"--no-abbrev",
		"--inter-hunk-context",
		"--after",
		"-s",
		"-100",
	}
)

var gitObjectHashRegex = regexp.MustCompile(`^[a-fA-F\d]*$`)

// common revs used with diff
var knownRevs = map[string]struct{}{
	"master":     {},
	"main":       {},
	"head":       {},
	"fetch_head": {},
	"orig_head":  {},
	"@":          {},
}

// isAllowedDiffArg checks if diff arg exists as a file. We do some preliminary checks
// as well as OS calls are more expensive. The function checks for object hashes and
// common revision names.
func isAllowedDiffArg(arg string) bool {
	// a hash is probably not a local file
	if gitObjectHashRegex.MatchString(arg) {
		return true
	}

	// check for parent and copy branch notations
	for _, c := range []string{" ", "^", "~"} {
		if _, ok := knownRevs[strings.ToLower(strings.Split(arg, c)[0])]; ok {
			return true
		}
	}
	// make sure that arg is not a local file
	_, err := os.Stat(arg)

	return os.IsNotExist(err)
}

// isAllowedGitArg checks if the arg is allowed.
func isAllowedGitArg(allowedArgs []string, arg string) bool {
	// Split the arg at the first equal sign and check the LHS against the allowlist args.
	splitArg := strings.Split(arg, "=")[0]

	// We use -- to specify the end of command options.
	// See: https://unix.stackexchange.com/a/11382/214756.
	if splitArg == "--" {
		return true
	}

	for _, allowedArg := range allowedArgs {
		if splitArg == allowedArg {
			return true
		}
	}
	return false
}

// isAllowedDiffPathArg checks if the diff path arg is allowed.
func isAllowedDiffPathArg(arg string, repoDir common.GitDir) bool {
	// allows diff command path that requires (dot) as path
	// example: diff --find-renames ... --no-prefix commit -- .
	if arg == "." {
		return true
	}

	arg = filepath.Clean(arg)
	if !filepath.IsAbs(arg) {
		arg = repoDir.Path(arg)
	}

	filePath, err := filepath.Abs(arg)
	if err != nil {
		return false
	}

	// Check if absolute path is a sub path of the repo dir
	repoRoot, err := filepath.Abs(repoDir.Path())
	if err != nil {
		return false
	}

	return strings.HasPrefix(filePath, repoRoot)
}

// IsAllowedGitCmd checks if the cmd and arguments are allowed.
//
// TODO: This should be unexported and solely be a concern of the CLI package,
// as other backends should do their own validation passes.
func IsAllowedGitCmd(logger log.Logger, cmd string, args []string, dir common.GitDir) bool {
	if len(args) == 0 || len(gitCmdAllowlist) == 0 {
		return false
	}

	allowedArgs, ok := gitCmdAllowlist[cmd]
	if !ok {
		// Command not allowed
		logger.Warn("command not allowed", log.String("cmd", cmd))
		return false
	}

	// I hate state machines, but I hate them less than complicated multi-argument checking
	checkFileInput := false
	for i, arg := range args[1:] {
		if checkFileInput {
			if arg == "-" {
				checkFileInput = false
				continue
			}
			logger.Warn("isAllowedGitCmd: unallowed file input for `git commit`", log.String("cmd", cmd), log.String("arg", arg))
			return false
		}
		if strings.HasPrefix(arg, "-") {
			// Special-case `git log -S` and `git log -G`, which interpret any characters
			// after their 'S' or 'G' as part of the query. There is no long form of this
			// flags (such as --something=query), so if we did not special-case these, there
			// would be no way to safely express a query that began with a '-' character.
			// (Same for `git show`, where the flag has the same meaning.)
			if (cmd == "log" || cmd == "show") && (strings.HasPrefix(arg, "-S") || strings.HasPrefix(arg, "-G")) {
				continue // this arg is OK
			}

			// Special case numeric arguments like `git log -20`.
			if _, err := strconv.Atoi(arg[1:]); err == nil {
				continue // this arg is OK
			}

			// For `git commit`, allow reading the commit message from stdin
			// but don't just blindly accept the `--file` or `-F` args
			// because they could be used to read arbitrary files.
			// Instead, accept only the forms that read from stdin.
			if cmd == "commit" {
				if arg == "--file=-" {
					continue
				}
				// checking `-F` requires a second check for `-` in the next argument
				// Instead of an obtuse check of next and previous arguments, set state and check it the next time around
				// Here's the alternative obtuse check of previous and next arguments:
				// (arg == "-F" && len(args) > i+2 && args[i+2] == "-") || (arg == "-" && args[i] == "-F")
				if arg == "-F" {
					checkFileInput = true
					continue
				}
			}

			if cmd == "diff-tree" {
				if arg == "-r" {
					// Using -r tells diff-tree to recurse into subdirectories, this is allowed
					//
					// See https://git-scm.com/docs/git-diff-tree
					continue
				}
			}

			if !isAllowedGitArg(allowedArgs, arg) {
				logger.Warn("IsAllowedGitCmd.isAllowedGitArgcmd", log.String("cmd", cmd), log.String("arg", arg))
				return false
			}
		}
		// diff argument may contains file path and isAllowedDiffArg and isAllowedDiffPathArg
		// helps verifying the file existence in disk
		if cmd == "diff" {
			dashIndex := slices.Index(args[1:], "--")
			if (dashIndex < 0 || i < dashIndex) && !isAllowedDiffArg(arg) {
				// verifies arguments before --
				logger.Warn("IsAllowedGitCmd.isAllowedDiffArg", log.String("cmd", cmd), log.String("arg", arg))
				return false
			} else if (i > dashIndex && dashIndex >= 0) && !isAllowedDiffPathArg(arg, dir) {
				// verifies arguments after --
				logger.Warn("IsAllowedGitCmd.isAllowedDiffPathArg", log.String("cmd", cmd), log.String("arg", arg))
				return false
			}
		}
	}
	return true
}
