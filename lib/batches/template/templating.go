package template

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/template"

	"github.com/gobwas/glob"
	"github.com/grafana/regexp"

	"github.com/sourcegraph/sourcegraph/lib/batches/execution"
	"github.com/sourcegraph/sourcegraph/lib/batches/git"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const startDelim = "${{"
const endDelim = "}}"

var builtins = template.FuncMap{
	"join":    strings.Join,
	"split":   strings.Split,
	"replace": strings.ReplaceAll,
	"join_if": func(sep string, elems ...string) string {
		var nonBlank []string
		for _, e := range elems {
			if e != "" {
				nonBlank = append(nonBlank, e)
			}
		}
		return strings.Join(nonBlank, sep)
	},
	"matches": func(in, pattern string) (bool, error) {
		g, err := glob.Compile(pattern)
		if err != nil {
			return false, err
		}
		return g.Match(in), nil
	},
}

// ValidateBatchSpecTemplate attempts to perform a dry run replacement of the whole batch
// spec template for any templating variables which are not dependent on execution
// context. It returns a tuple whose first element is whether or not the batch spec is
// valid and whose second element is an error message if the spec is found to be invalid.
func ValidateBatchSpecTemplate(spec string) (bool, error) {
	// We use empty contexts to create "dummy" `template.FuncMap`s -- function mappings
	// with all the right keys, but no actual values. We'll use these `FuncMap`s to do a
	// dry run on the batch spec to determine if it's valid or not, before we actually
	// execute it.
	sc := &StepContext{}
	sfm := sc.ToFuncMap()
	cstc := &ChangesetTemplateContext{}
	cstfm := cstc.ToFuncMap()

	// Strip any use of `outputs` fields from the spec template. Without using real
	// contexts for the `FuncMap`s, they'll fail to `template.Execute`, and it's difficult
	// to statically validate them without deeper inspection of the YAML, so our
	// validation is just a best-effort without them.
	outputRe := regexp.MustCompile(`(?i)\$\{\{\s*[^}]*\s*outputs\.[^}]*\}\}`)
	spec = outputRe.ReplaceAllString(spec, "")

	// Also strip index references. We also can't validate whether or not an index is in
	// range without real context.
	indexRe := regexp.MustCompile(`(?i)\$\{\{\s*index\s*[^}]*\}\}`)
	spec = indexRe.ReplaceAllString(spec, "")

	// By default, text/template will continue even if it encounters a key that is not
	// indexed in any of the provided `FuncMap`s. A missing key is an indication of an
	// unknown or mistyped template variable which would invalidate the batch spec, so we
	// want to fail immediately if we encounter one. We accomplish this by setting the
	// option "missingkey=error". See https://pkg.go.dev/text/template#Template.Option for
	// more.
	t, err := New("validateBatchSpecTemplate", spec, "missingkey=error", sfm, cstfm)

	if err != nil {
		// Attempt to extract the specific template variable field that caused the error
		// to provide a clearer message.
		errorRe := regexp.MustCompile(`(?i)function "(?P<key>[^"]+)" not defined`)
		if matches := errorRe.FindStringSubmatch(err.Error()); len(matches) > 0 {
			return false, errors.Newf("validating batch spec template: unknown templating variable: '%s'", matches[1])
		}
		// If we couldn't give a more specific error, fall back on the one from text/template.
		return false, errors.Wrap(err, "validating batch spec template")
	}

	var out bytes.Buffer
	if err = t.Execute(&out, &StepContext{}); err != nil {
		// Attempt to extract the specific template variable fields that caused the error
		// to provide a clearer message.
		errorRe := regexp.MustCompile(`(?i)at <(?P<outer>[^>]+)>:.*for key "(?P<inner>[^"]+)"`)
		if matches := errorRe.FindStringSubmatch(err.Error()); len(matches) > 0 {
			return false, errors.Newf("validating batch spec template: unknown templating variable: '%s.%s'", matches[1], matches[2])
		}
		// If we couldn't give a more specific error, fall back on the one from text/template.
		return false, errors.Wrap(err, "validating batch spec template")
	}

	return true, nil
}

func isTrueOutput(output interface{ String() string }) bool {
	return strings.TrimSpace(output.String()) == "true"
}

func EvalStepCondition(condition string, stepCtx *StepContext) (bool, error) {
	if condition == "" {
		return true, nil
	}

	var out bytes.Buffer
	if err := RenderStepTemplate("step-condition", condition, &out, stepCtx); err != nil {
		return false, errors.Wrap(err, "parsing step if")
	}

	return isTrueOutput(&out), nil
}

func RenderStepTemplate(name, tmpl string, out io.Writer, stepCtx *StepContext) error {
	// By default, text/template will continue even if it encounters a key that is not
	// indexed in any of the provided `FuncMap`s, replacing the variable with "<no
	// value>". This means that a mis-typed variable such as "${{
	// repository.search_resalt_paths }}" would just be evaluated as "<no value>", which
	// is not a particularly useful substitution and will only indirectly manifest to the
	// user as an error during execution. Instead, we prefer to fail immediately if we
	// encounter an unknown variable. We accomplish this by setting the option
	// "missingkey=error". See https://pkg.go.dev/text/template#Template.Option for more.
	t, err := New(name, tmpl, "missingkey=error", stepCtx.ToFuncMap())
	if err != nil {
		return errors.Wrap(err, "parsing step run")
	}

	return t.Execute(out, stepCtx)
}

func RenderStepMap(m map[string]string, stepCtx *StepContext) (map[string]string, error) {
	rendered := make(map[string]string, len(m))

	for k, v := range m {
		var out bytes.Buffer

		if err := RenderStepTemplate(k, v, &out, stepCtx); err != nil {
			return rendered, err
		}

		rendered[k] = out.String()
	}

	return rendered, nil
}

// TODO(mrnugget): This is bad and should be (a) removed or (b) moved to batches package
type BatchChangeAttributes struct {
	Name        string
	Description string
}

type Repository struct {
	Name        string
	Branch      string
	FileMatches []string
}

func (r Repository) SearchResultPaths() (list fileMatchPathList) {
	sort.Strings(r.FileMatches)
	return r.FileMatches
}

type fileMatchPathList []string

func (f fileMatchPathList) String() string { return strings.Join(f, " ") }

// StepContext represents the contextual information available when rendering a
// step's fields, such as "run" or "outputs", as templates.
type StepContext struct {
	// BatchChange are the attributes in the BatchSpec that are set on the BatchChange.
	BatchChange BatchChangeAttributes
	// Outputs are the outputs set by the current and all previous steps.
	Outputs map[string]any
	// Step is the result of the current step. Empty when evaluating the "run" field
	// but filled when evaluating the "outputs" field.
	Step execution.AfterStepResult
	// Steps contains the path in which the steps are being executed and the
	// changes made by all steps that were executed up until the current step.
	Steps StepsContext
	// PreviousStep is the result of the previous step. Empty when there is no
	// previous step.
	PreviousStep execution.AfterStepResult
	// Repository is the Sourcegraph repository in which the steps are executed.
	Repository Repository
}

// ToFuncMap returns a template.FuncMap to access fields on the StepContext in a
// text/template.
func (stepCtx *StepContext) ToFuncMap() template.FuncMap {
	newStepResult := func(res *execution.AfterStepResult) map[string]any {
		m := map[string]any{
			"modified_files": "",
			"added_files":    "",
			"deleted_files":  "",
			"renamed_files":  "",
			"stdout":         "",
			"stderr":         "",
		}
		if res == nil {
			return m
		}

		m["modified_files"] = res.ChangedFiles.Modified
		m["added_files"] = res.ChangedFiles.Added
		m["deleted_files"] = res.ChangedFiles.Deleted
		m["renamed_files"] = res.ChangedFiles.Renamed
		m["stdout"] = res.Stdout
		m["stderr"] = res.Stderr

		return m
	}

	return template.FuncMap{
		"previous_step": func() map[string]any {
			return newStepResult(&stepCtx.PreviousStep)
		},
		"step": func() map[string]any {
			return newStepResult(&stepCtx.Step)
		},
		"steps": func() map[string]any {
			res := newStepResult(&execution.AfterStepResult{ChangedFiles: stepCtx.Steps.Changes})
			res["path"] = stepCtx.Steps.Path
			return res
		},
		"outputs": func() map[string]any {
			return stepCtx.Outputs
		},
		"repository": func() map[string]any {
			return map[string]any{
				"search_result_paths": stepCtx.Repository.SearchResultPaths(),
				"name":                stepCtx.Repository.Name,
				"branch":              stepCtx.Repository.Branch,
			}
		},
		"batch_change": func() map[string]any {
			return map[string]any{
				"name":        stepCtx.BatchChange.Name,
				"description": stepCtx.BatchChange.Description,
			}
		},
	}
}

type StepsContext struct {
	// Changes that have been made by executing all steps.
	Changes git.Changes
	// Path is the relative-to-root directory in which the steps have been
	// executed. Default is "". No leading "/".
	Path string
}

// ChangesetTemplateContext represents the contextual information available
// when rendering a field of the ChangesetTemplate as a template.
type ChangesetTemplateContext struct {
	// BatchChangeAttributes are the attributes of the BatchChange that will be
	// created/updated.
	BatchChangeAttributes BatchChangeAttributes

	// Steps are the changes made by all steps that were executed.
	Steps StepsContext

	// Outputs are the outputs defined and initialized by the steps.
	Outputs map[string]any

	// Repository is the repository in which the steps were executed.
	Repository Repository
}

// ToFuncMap returns a template.FuncMap to access fields on the StepContext in a
// text/template.
func (tmplCtx *ChangesetTemplateContext) ToFuncMap() template.FuncMap {
	return template.FuncMap{
		"repository": func() map[string]any {
			return map[string]any{
				"search_result_paths": tmplCtx.Repository.SearchResultPaths(),
				"name":                tmplCtx.Repository.Name,
				"branch":              tmplCtx.Repository.Branch,
			}
		},
		"batch_change": func() map[string]any {
			return map[string]any{
				"name":        tmplCtx.BatchChangeAttributes.Name,
				"description": tmplCtx.BatchChangeAttributes.Description,
			}
		},
		"outputs": func() map[string]any {
			return tmplCtx.Outputs
		},
		"steps": func() map[string]any {
			return map[string]any{
				"modified_files": tmplCtx.Steps.Changes.Modified,
				"added_files":    tmplCtx.Steps.Changes.Added,
				"deleted_files":  tmplCtx.Steps.Changes.Deleted,
				"renamed_files":  tmplCtx.Steps.Changes.Renamed,
				"path":           tmplCtx.Steps.Path,
			}
		},
		// Leave batch_change_link alone; it will be rendered during the reconciler phase instead.
		"batch_change_link": func() string {
			return "${{ batch_change_link }}"
		},
	}
}

func RenderChangesetTemplateField(name, tmpl string, tmplCtx *ChangesetTemplateContext) (string, error) {
	var out bytes.Buffer

	// By default, text/template will continue even if it encounters a key that is not
	// indexed in any of the provided `FuncMap`s, replacing the variable with "<no
	// value>". This means that a mis-typed variable such as "${{
	// repository.search_resalt_paths }}" would just be evaluated as "<no value>", which
	// is not a particularly useful substitution and will only indirectly manifest to the
	// user as an error during execution. Instead, we prefer to fail immediately if we
	// encounter an unknown variable. We accomplish this by setting the option
	// "missingkey=error". See https://pkg.go.dev/text/template#Template.Option for more.
	t, err := New(name, tmpl, "missingkey=error", tmplCtx.ToFuncMap())
	if err != nil {
		return "", err
	}

	if err := t.Execute(&out, tmplCtx); err != nil {
		return "", err
	}

	return strings.TrimSpace(out.String()), nil
}
