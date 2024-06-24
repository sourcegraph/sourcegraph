package dbconn

import (
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/grafana/regexp"

	"github.com/sourcegraph/sourcegraph/dev/linters/nolint"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var Analyzer = nolint.Wrap(&analysis.Analyzer{
	Name:      "dbconn",
	Doc:       "Disallow dbconn package from appearing in certain dependency trees. Adapted from ./dev/check/go-dbconn-import.sh",
	Run:       run,
	FactTypes: []analysis.Fact{new(importsDbConn)},
})

type importsDbConn bool

func (*importsDbConn) AFact() {}

var allowedToImport = []string{
	"github.com/sourcegraph/sourcegraph/cmd/frontend",
	"github.com/sourcegraph/sourcegraph/cmd/gitserver",
	"github.com/sourcegraph/sourcegraph/cmd/migrator",
	// Transitively depends on updatecheck package which imports but does not use DB
	"github.com/sourcegraph/sourcegraph/cmd/pings",
	"github.com/sourcegraph/sourcegraph/cmd/precise-code-intel-worker",
	"github.com/sourcegraph/sourcegraph/cmd/syntactic-code-intel-worker",
	"github.com/sourcegraph/sourcegraph/cmd/repo-updater",
	// Doesn't connect but uses db internals for use with sqlite
	"github.com/sourcegraph/sourcegraph/cmd/symbols",
	"github.com/sourcegraph/sourcegraph/cmd/worker",
}

const dbconnPath = "github.com/sourcegraph/sourcegraph/internal/database/dbconn"

var cmdPkgRegex = regexp.MustCompile(`^github\.com/sourcegraph/sourcegraph/cmd/[\w-_]+$`)

func run(pass *analysis.Pass) (interface{}, error) {
	// for packages in the github.com/sourcegraph/sourcegraph module, we check whether they directly import
	// dbconn, or whether any of their direct dependencies export a fact[1] stating that they either directly
	// import dbconn or whether any of _their_ direct dependencies export a fact etc etc recursively.
	// In other words, a bool is bubbled up stating either dbconn is imported in this dependency tree or not.
	//
	// [1] https://pkg.go.dev/golang.org/x/tools/go/analysis#hdr-Modular_analysis_with_Facts
	if strings.HasPrefix(pass.Pkg.Path(), "github.com/sourcegraph/sourcegraph") {
		for _, i := range pass.Pkg.Imports() {
			fact := new(importsDbConn)
			// if we directly import dbconn, or any dependencies factually do
			if i.Path() == dbconnPath || (pass.ImportPackageFact(i, fact) && bool(*fact)) {
				fact := importsDbConn(true)
				pass.ExportPackageFact(&fact)
			}
		}
	}

	// skip checking for packages outside, or for non-cmd packages.
	// only raise errors for top-level main packages (which this list should) be composed of.
	if !cmdPkgRegex.MatchString(pass.Pkg.Path()) {
		return nil, nil
	}

	// these packages are allowed to import it.
	if slices.Contains(allowedToImport, pass.Pkg.Path()) {
		return nil, nil
	}

	for _, i := range pass.Pkg.Imports() {
		fact := new(importsDbConn)
		// this correctly reports for any transitive dependency due to this condition when dealing with facts:
		// "The driver program ensures that facts for a pass’s dependencies are generated before analyzing the package"
		// from https://pkg.go.dev/golang.org/x/tools/go/analysis#hdr-Modular_analysis_with_Facts.
		if pass.ImportPackageFact(i, fact) && bool(*fact) {
			return nil, errors.Newf("package %q is not allowed to import %q (directly or transitively)", pass.Pkg.Path(), dbconnPath)
		}
	}

	return nil, nil
}
