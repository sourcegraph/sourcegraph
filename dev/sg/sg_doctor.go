package main

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/category"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/run"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

var doctorCommand = &cli.Command{
	Name:  "doctor",
	Usage: "performs diagnostics of the local environment and prints out a report",
	Description: `Runs a series of commands defined in sg-doctor.yaml.

	The output of the commands are stored in a report, which can then be given to a dev-infra team memeber for
	further diagnosis.
	`,
	Category: category.Util,
	Action:   runDoctorDiagnostics,
}

//go:embed sg.doctor.yaml
var doctorYaml []byte

type Diagnostic struct {
	Name string `yaml:"name"`
	Cmd  string `yaml:"cmd"`
}

type Diagnostics struct {
	Diagnostic map[string][]Diagnostic `yaml:"diagnostics"`
}

type diagnosticRunner struct {
	diagnostics *Diagnostics
	reporter    *std.Output
}

type DiagnosticResult struct {
	Diagnostic *Diagnostic
	Output     string
	Err        error
}
type DiagnosticReport map[string][]*DiagnosticResult

func (r DiagnosticReport) Add(group string, result *DiagnosticResult) {
	if v, ok := r[group]; !ok {
		r[group] = []*DiagnosticResult{result}
	} else {
		r[group] = append(v, result)
	}
}

func runDoctorDiagnostics(cmd *cli.Context) error {
	diagnostics, err := readDiagnosticDefinitions(doctorYaml)
	if err != nil {
		return errors.Newf("failed to load diagnostics from embedded yaml:", err)
	}

	// We do not want our progress messages to land on std out so we set output to os.Stderr
	diagOut := std.NewOutput(os.Stderr, false)

	runner := &diagnosticRunner{
		diagnostics,
		diagOut,
	}

	diagOut.WriteLine(output.Emoji("🥼", "Gathering diagnostics"))
	report := runner.Run(cmd.Context)
	diagOut.WriteLine(output.Emoji("💉", "Gathering of diagnostics complete!"))
	markdown := buildMarkdownReport(report)

	// check if we're rendering to the terminal or to another program
	o, _ := os.Stdout.Stat()
	if o.Mode()&os.ModeCharDevice != os.ModeCharDevice {
		// our output has been redirected to another program, so lets just render it raw
		fmt.Println(markdown)
	} else {
		// rendering to a terminal! so lets make it nice
		diagOut.WriteMarkdown(markdown)
	}
	return nil
}

func (d *diagnosticRunner) Run(ctx context.Context) DiagnosticReport {
	env := os.Environ()
	report := make(DiagnosticReport)

	for group, diagnostics := range d.diagnostics.Diagnostic {
		d.reporter.WriteLine(output.Emojif("💊", "Running %s diagnostics", group))
		for _, diagnostic := range diagnostics {
			out, err := run.BashInRoot(ctx, diagnostic.Cmd, env)
			diag := diagnostic
			report.Add(group, &DiagnosticResult{
				&diag,
				out,
				err,
			})
		}
	}

	return report
}

func buildMarkdownReport(report DiagnosticReport) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# Diagnostic Report\n\n")
	// General information
	fmt.Fprintf(&sb, "sg commit: `%s`\n\n", BuildCommit)
	fmt.Fprintf(&sb, "generated on: `%s`\n\n", time.Now())
	titleCaser := cases.Title(language.English)

	// map key order isn't stable so we extract them and sort them
	groupKeys := []string{}
	for k := range report {
		groupKeys = append(groupKeys, k)
	}
	slices.Sort(groupKeys)

	// Write out the report
	for _, group := range groupKeys {
		fmt.Fprintf(&sb, "## %s diagnostics\n\n", titleCaser.String(group))
		result := report[group]
		for _, item := range result {
			cmdLine := fmt.Sprintf("Command: `%s`", item.Diagnostic.Cmd)
			outputSection := fmt.Sprintf("Output: \n```\n%s\n```\n", item.Output)
			errSection := fmt.Sprintf("Error: \n```\n%v\n```\n", item.Err)
			fmt.Fprintf(&sb, "### %s\n\n%s\n\n%s\n%s", titleCaser.String(item.Diagnostic.Name), cmdLine, outputSection, errSection)
		}
	}

	return sb.String()
}

func readDiagnosticDefinitions(content []byte) (*Diagnostics, error) {
	var diags Diagnostics
	dec := yaml.NewDecoder(bytes.NewReader(content))

	err := dec.Decode(&diags)
	if err != nil {
		return nil, err
	}

	return &diags, nil
}
