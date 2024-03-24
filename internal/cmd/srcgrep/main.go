package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/grafana/regexp"
	"github.com/grafana/regexp/syntax"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"golang.org/x/sync/errgroup"
)

func run(ctx context.Context, rgBaseArgs []string, node query.Node, parameters []query.Parameter) error {
	plan, err := plan(node, parameters)
	if err != nil {
		return err
	}

	var paths []string
	// look for git directories matching repoParams
	if len(plan.RepoInclude) > 0 || len(plan.RepoExclude) > 0 {
		repos, err := walkRepos()
		if err != nil {
			return errors.Errorf("failed to find repositories: %w", err)
		}
		paths = filterRepos(repos, plan.RepoInclude, plan.RepoExclude)
	}

	if len(plan.RipGrepArgs) == 0 {
		for _, name := range paths {
			fmt.Println(name)
		}
		return nil
	}

	var args []string
	args = append(args, rgBaseArgs...)
	args = append(args, plan.RipGrepArgs...)
	args = append(args, paths...)

	cmd := exec.CommandContext(ctx, "rg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type Plan struct {
	RepoInclude []*regexp.Regexp
	RepoExclude []*regexp.Regexp

	RipGrepArgs []string
}

func plan(node query.Node, parameters []query.Parameter) (*Plan, error) {
	pattern := ""
	if p, ok := node.(query.Pattern); ok {
		pattern = p.Value
	} else if node != nil {
		return nil, errors.Errorf("only supports pattern queries, got %T: %s", node, node)
	}

	var args []string
	var repoInclude, repoExclude []*regexp.Regexp

	for _, p := range parameters {
		switch p.Field {
		case query.FieldRepo:
			re, err := regexp.Compile(p.Value)
			if err != nil {
				return nil, errors.Errorf("failed to compile repo regexp %q: %w", p.Value, err)
			}
			if p.Negated {
				repoExclude = append(repoExclude, re)
			} else {
				repoInclude = append(repoInclude, re)
			}

		case query.FieldFile:
			// TODO would be nice if we could change our query parser to
			// instead take in globs.
			glob, err := regexpToGlob(p)
			if err != nil {
				return nil, errors.Errorf("failed to convert file regex %q to ripgrep glob: %w", p.Value, err)
			}
			args = append(args, "--glob", glob)

		case query.FieldCase:
			switch p.Value {
			case "yes":
				args = append(args, "--case-sensitive")
			case "no":
				args = append(args, "--ignore-case")
			default:
				return nil, errors.Errorf("unknown case value: %s", p)
			}

		case query.FieldLang:
			if p.Negated {
				args = append(args, "--type-not", p.Value)
			} else {
				args = append(args, "--type", p.Value)
			}

		default:
			return nil, errors.Errorf("unsupported field: %s", p)
		}
	}

	if pattern != "" {
		args = append(args, "--", pattern)
	} else if len(args) > 0 {
		// if args is empty we are doing a repo search.
		args = append(args, "--files")
	}

	return &Plan{
		RepoInclude: repoInclude,
		RepoExclude: repoExclude,
		RipGrepArgs: args,
	}, nil
}

// regexpToGlob attempts to convert the regex to a glob for ripgrep. However,
// there is a mismatch between the two, so this is done more as a heuristic to
// try and match user intention.
func regexpToGlob(p query.Parameter) (string, error) {
	concat, err := syntax.Parse(p.Value, syntax.Perl)
	if err != nil {
		return "", err
	}

	// We have a very naive implementation. It basically just looks for
	// ^literal$ where the anchors are optional.

	// Pretend we always have a concat to simplify later code.
	if concat.Op != syntax.OpConcat {
		concat = &syntax.Regexp{
			Op:  syntax.OpConcat,
			Sub: []*syntax.Regexp{concat},
		}
	}

	literal := ""
	hasBegin := false
	hasEnd := false
	for i, re := range concat.Sub {
		switch {
		case i == 0 && re.Op == syntax.OpBeginLine:
			hasBegin = true
		case re.Op == syntax.OpLiteral:
			if literal != "" {
				return "", errors.Errorf("only expected one literal")
			}
			literal = string(re.Rune)
		case i == len(concat.Sub)-1 && (re.Op == syntax.OpEndLine || re.Op == syntax.OpEndText):
			hasEnd = true
		default:
			return "", errors.Errorf("do not know how to convert %v into glob", re)
		}
	}

	if literal == "" {
		return "", errors.New("missing literal")
	}

	e := func(b bool, s string) string {
		if b {
			return s
		}
		return ""
	}

	return e(p.Negated, "!") + e(!hasBegin, "**") + globLiteralEscape(literal) + e(!hasEnd, "**"), nil
}

// globLiteralEscape escapes s based on the rules described in the manpage
// GITIGNORE(5) and RG(1)'s --glob section.
func globLiteralEscape(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '!', '[', ']', '{', '}', '*', '?':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func parse(q string) (query.Plan, error) {
	return query.Pipeline(
		query.Init(q, query.SearchTypeRegex),
	)
}

func do(rgArgs []string, q string) error {
	plan, err := parse(q)
	if err != nil {
		return err
	}

	wg, ctx := errgroup.WithContext(context.Background())

	for _, p := range plan {
		p := p
		wg.Go(func() error {
			return run(ctx, rgArgs, p.Pattern, p.Parameters)
		})
	}

	return wg.Wait()
}

func main() {
	if len(os.Args) <= 1 {
		fmt.Fprintf(os.Stderr, "USAGE: %s [RIPGREP_OPTIONS] SOURCEGRAPH_QUERY\n", filepath.Base(os.Args[0]))
		os.Exit(2)
	}

	queryIdx := len(os.Args) - 1
	err := do(os.Args[1:queryIdx], os.Args[queryIdx])

	// rg has well defined exit codes, so pass them on
	var execErr *exec.ExitError
	if errors.As(err, &execErr) {
		os.Exit(execErr.ExitCode())
	} else if err != nil {
		log.Fatal(err)
	}
}
