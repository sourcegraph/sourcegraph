package golang

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sourcegraph/run"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/generate"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/dev/sg/root"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

type OutputVerbosityType int

const (
	VerboseOutput OutputVerbosityType = iota
	NormalOutput
	QuietOutput
)

func Generate(ctx context.Context, args []string, progressBar bool, verbosity OutputVerbosityType) *generate.Report {
	start := time.Now()
	var sb strings.Builder
	reportOut := std.NewOutput(&sb, false)

	// Save working directory
	cwd, err := os.Getwd()
	if err != nil {
		return &generate.Report{Err: err}
	}
	defer func() {
		os.Chdir(cwd)
	}()
	rootDir, err := root.RepositoryRoot()
	if err != nil {
		return &generate.Report{Err: err}
	}

	// Determine which goimports we can use
	var goimportsBinary string
	if _, err := exec.LookPath("goimports"); err == nil {
		goimportsBinary = "goimports"
	} else {
		// Installed (unconditionally) below
		goimportsBinary = "./.bin/goimports"
	}

	// Install a local version of goimports - we do this whether we have a
	// version of goimports or not because we need to feed it into the go-mockgen
	// configuration file (which we don't yet template).
	err = run.Cmd(ctx, "go", "install", "golang.org/x/tools/cmd/goimports").
		Environ(os.Environ()).
		Env(map[string]string{
			// Install to local bin
			"GOBIN": filepath.Join(rootDir, ".bin"),
		}).
		Run().
		Stream(&sb)
	if err != nil {
		return &generate.Report{
			Output: sb.String(),
			Err:    errors.Wrap(err, "go install golang.org/x/tools/cmd/goimports returned an error"),
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		return &generate.Report{Err: err}
	}

	// Run go generate on the packages list
	if len(args) == 0 {
		// Grab the packages list
		pkgPaths, err := findPackagesWithGenerate(wd, wd)
		if err != nil {
			return &generate.Report{Err: err}
		}

		// If no packages are given, go for everything but the exception.
		filtered := make([]string, 0, len(pkgPaths))
		for _, pkgPath := range pkgPaths {
			if !strings.Contains(pkgPath, "doc/cli/references") {
				filtered = append(filtered, pkgPath)
			}
		}
		if verbosity != QuietOutput {
			reportOut.WriteLine(output.Linef(output.EmojiInfo, output.StyleBold, "go generate ./... (excluding doc/cli/references)"))
		}
		err = runGoGenerate(ctx, filtered, progressBar, verbosity, &sb)
		if err != nil {
			return &generate.Report{Output: sb.String(), Err: errors.Wrap(err, "go generate")}
		}
	} else {
		// Use the given packages.
		if verbosity != QuietOutput {
			reportOut.WriteLine(output.Linef(output.EmojiInfo, output.StyleBold, "go generate %s", strings.Join(args, " ")))
		}
		err = runGoGenerate(ctx, args, progressBar, verbosity, &sb)
		if err != nil {
			return &generate.Report{Output: sb.String(), Err: errors.Wrap(err, "go generate")}
		}
	}

	// Run goimports -w
	if verbosity != QuietOutput {
		reportOut.WriteLine(output.Linef(output.EmojiInfo, output.StyleBold, "goimports -w"))
	}

	err = root.Run(run.Cmd(ctx, goimportsBinary, "-w")).Stream(&sb)
	if err != nil {
		return &generate.Report{
			Output: sb.String(),
			Err:    errors.Wrap(err, "goimports -w"),
		}
	}

	// Run go mod tidy
	if verbosity != QuietOutput {
		reportOut.WriteLine(output.Linef(output.EmojiInfo, output.StyleBold, "go mod tidy"))
	}

	err = root.Run(run.Cmd(ctx, "go", "mod", "tidy")).Stream(&sb)
	if err != nil {
		return &generate.Report{Output: sb.String(), Err: errors.Wrap(err, "go mod tidy")}
	}

	return &generate.Report{
		Output:   sb.String(),
		Duration: time.Since(start),
	}
}

var goGeneratePattern = regexp.MustCompile(`^//go:generate (.+)$`)

func findPackagesWithGenerate(root, dir string) (packages []string, _ error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			pkgs, err := findPackagesWithGenerate(root, path)
			if err != nil {
				return nil, err
			}

			packages = append(packages, pkgs...)
		} else if filepath.Ext(entry.Name()) == ".go" {
			contents, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}

			for _, line := range bytes.Split(contents, []byte{'\n'}) {
				if goGeneratePattern.Match(line) {
					packages = append(packages, "github.com/sourcegraph/sourcegraph"+dir[len(root):])
				}
			}
		}
	}

	return packages, nil
}

func runGoGenerate(ctx context.Context, pkgPaths []string, progressBar bool, verbosity OutputVerbosityType, out io.Writer) (err error) {
	args := []string{"go", "generate"}
	if verbosity == VerboseOutput {
		args = append(args, "-x")
	}
	if progressBar {
		// If we want to display a progress bar we want the verbose output of `go
		// generate` so we can check which package has been generated.
		args = append(args, "-v")
	}
	args = append(args, pkgPaths...)

	if !progressBar {
		// If we don't want to display a progress bar we stream output to `out`
		// and are done.
		return root.Run(run.Cmd(ctx, args...)).Stream(out)
	}

	done := 0.0
	total := float64(len(pkgPaths))
	progress := std.Out.Progress([]output.ProgressBar{
		{Label: fmt.Sprintf("go generating %d packages", len(pkgPaths)), Max: total},
	}, nil)
	defer func() {
		if err != nil {
			progress.Destroy()
		} else {
			progress.Complete()
		}
	}()

	pkgMap := make(map[string]bool, len(pkgPaths))
	for _, pkg := range pkgPaths {
		pkgMap[strings.TrimPrefix(pkg, "github.com/sourcegraph/sourcegraph/")] = false
	}

	return root.Run(run.Cmd(ctx, args...)).StreamLines(func(line []byte) {
		if !bytes.HasSuffix(line, []byte(".go")) {
			return
		}

		dir := filepath.Dir(string(line))

		if current, ok := pkgMap[dir]; ok && !current {
			pkgMap[dir] = true

			if verbosity == VerboseOutput {
				progress.Writef("Generating %s...", dir)
			}

			done += 1.0
			progress.SetValue(0, done)
			progress.SetLabelAndRecalc(0, fmt.Sprintf("%d/%d packages generated", int(done), int(total)))
		}
	})
}
