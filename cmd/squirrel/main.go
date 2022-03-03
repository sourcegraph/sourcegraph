// Command symbols is a service that serves code symbols (functions, variables, etc.) from a repository at a
// specific commit.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/debugserver"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/httpserver"
	"github.com/sourcegraph/sourcegraph/internal/logging"
	"github.com/sourcegraph/sourcegraph/internal/profiler"
	"github.com/sourcegraph/sourcegraph/internal/sentry"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
	"github.com/sourcegraph/sourcegraph/internal/tracer"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func main() {
	routines := []goroutine.BackgroundRoutine{}

	// Set up Google Cloud Profiler when running in Cloud
	if err := profiler.Init(); err != nil {
		log.Fatalf("Failed to start profiler: %v", err)
	}

	// Initialization
	env.HandleHelpFlag()
	conf.Init()
	logging.Init()
	tracer.Init(conf.DefaultClient())
	sentry.Init(conf.DefaultClient())
	trace.Init()

	// Start debug server
	ready := make(chan struct{})
	go debugserver.NewServerRoutine(ready).Start()

	// Create HTTP server
	server := httpserver.NewFromAddr(":8984", &http.Server{
		ReadTimeout:  75 * time.Second,
		WriteTimeout: 10 * time.Minute,
		Handler:      actor.HTTPMiddleware(ot.HTTPMiddleware(trace.HTTPMiddleware(NewHandler(), conf.DefaultClient()))),
	})
	routines = append(routines, server)

	// Mark health server as ready and go!
	close(ready)
	goroutine.MonitorBackgroundRoutines(context.Background(), routines...)
}

func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/definition", definitionHandler)
	mux.HandleFunc("/healthz", handleHealthCheck)
	return mux
}

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte("OK")); err != nil {
		log15.Error("failed to write response to health check, err: %s", err)
	}
}

func definitionHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	repo := q.Get("repo")
	if repo == "" {
		http.Error(w, "missing repo", http.StatusBadRequest)
		return
	}
	commit := q.Get("commit")
	if commit == "" {
		http.Error(w, "missing commit", http.StatusBadRequest)
		return
	}
	path := q.Get("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	row64, err := strconv.ParseInt(q.Get("row"), 10, 32)
	if err != nil {
		http.Error(w, "missing or invalid int row", http.StatusBadRequest)
		return
	}
	row := uint32(row64)
	column64, err := strconv.ParseInt(q.Get("column"), 10, 32)
	if err != nil {
		http.Error(w, "missing or invalid int column", http.StatusBadRequest)
		return
	}
	column := uint32(column64)
	fmt.Println("repo:", repo, "commit:", commit, "path:", path, "row:", row, "column:", column)

	// get file extension
	ext := filepath.Ext(path)
	if ext != ".go" {
		http.Error(w, "only .go files are supported", http.StatusBadRequest)
		return
	}

	readFile := func(RepoCommitPath) ([]byte, error) {
		cmd := gitserver.DefaultClient.Command("git", "cat-file", "blob", commit+":"+path)
		cmd.Repo = api.RepoName(repo)
		stdout, stderr, err := cmd.DividedOutput(r.Context())
		if err != nil {
			return nil, fmt.Errorf("failed to get file contents: %s\n\nstdout:\n\n%s\n\nstderr:\n\n%s", err, stdout, stderr)
		}
		return stdout, nil
	}

	squirrel := NewSquirrel(readFile)

	result, _, err := squirrel.definition(Location{
		RepoCommitPath: RepoCommitPath{
			Repo:   repo,
			Commit: commit,
			Path:   path},
		Row:    row,
		Column: column,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get definition: %s", err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, result)
}

type RepoCommitPath struct {
	Repo   string `json:"repo"`
	Commit string `json:"commit"`
	Path   string `json:"path"`
}

type Location struct {
	RepoCommitPath
	Row    uint32 `json:"row"`
	Column uint32 `json:"column"`
}

type ReadFileFunc func(RepoCommitPath) ([]byte, error)

type Squirrel struct {
	readFile ReadFileFunc
}

func NewSquirrel(readFile ReadFileFunc) *Squirrel {
	return &Squirrel{readFile: readFile}
}

func (s *Squirrel) definition(location Location) (*Location, []Breadcrumb, error) {
	parser := sitter.NewParser()

	var queryString string
	var lang *sitter.Language
	ext := filepath.Ext(location.Path)
	switch ext {
	case ".go":
		queryString = goQueries
		lang = golang.GetLanguage()
	default:
		return nil, nil, fmt.Errorf("unrecognized file extension %s", ext)
	}

	parser.SetLanguage(lang)

	input, err := s.readFile(location.RepoCommitPath)
	if err != nil {
		return nil, nil, err
	}

	tree, err := parser.ParseCtx(context.Background(), nil, input)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse file contents: %s", err)
	}
	root := tree.RootNode()

	startNode := root.NamedDescendantForPointRange(
		sitter.Point{Row: location.Row, Column: location.Column},
		sitter.Point{Row: location.Row, Column: location.Column},
	)

	if startNode == nil {
		return nil, nil, errors.New("node is nil")
	}

	typeOk := false
	for _, identifier := range goIdentifiers {
		if startNode.Type() == identifier {
			typeOk = true
			break
		}
	}
	if !typeOk {
		return nil, nil, errors.Newf("can't find definition of %s", startNode.Type())
	}

	breadcrumbs := []Breadcrumb{{
		Location: location,
		length:   1,
		message:  "start",
	}}

	// Execute the query
	query, err := sitter.NewQuery([]byte(queryString), lang)
	if err != nil {
		return nil, breadcrumbs, errors.Newf("failed to parse query: %s\n%s", err, queryString)
	}
	cursor := sitter.NewQueryCursor()
	cursor.Exec(query, root)

	// Collect all definitions into scopes
	scopes := map[string][]*sitter.Node{}
	match, _, hasCapture := cursor.NextCapture()
	for hasCapture {
		for _, capture := range match.Captures {
			name := query.CaptureNameForId(capture.Index)

			// Add to breadcrumbs
			length := 1
			if capture.Node.EndPoint().Row == capture.Node.StartPoint().Row {
				length = int(capture.Node.EndPoint().Column - capture.Node.StartPoint().Column)
			}
			breadcrumbs = append(breadcrumbs, Breadcrumb{
				Location: Location{
					RepoCommitPath: location.RepoCommitPath,
					Row:            capture.Node.StartPoint().Row,
					Column:         capture.Node.StartPoint().Column,
				},
				length:  length,
				message: fmt.Sprintf("%s: %s", name, capture.Node.Type()),
			})

			// Add definition to nearest scope
			if strings.Contains(name, "definition") {
				for cur := capture.Node; cur != nil; cur = cur.Parent() {
					id := getId(cur)
					_, ok := scopes[id]
					if !ok {
						continue
					}
					scopes[id] = append(scopes[id], capture.Node)
					break
				}

				continue
			}

			// Record the scope
			if strings.Contains(name, "scope") {
				scopes[getId(capture.Node)] = []*sitter.Node{}
				continue
			}
		}

		// Next capture
		match, _, hasCapture = cursor.NextCapture()
	}

	// Walk up the tree to find the nearest definition
	for currentNode := startNode; currentNode != nil; currentNode = currentNode.Parent() {
		scope, ok := scopes[getId(currentNode)]
		if !ok {
			// This node isn't a scope, continue.
			continue
		}

		// Check if the scope contains the definition
		for _, def := range scope {
			if def.Content(input) == startNode.Content(input) {
				return &Location{
					RepoCommitPath: location.RepoCommitPath,
					Row:            def.StartPoint().Row,
					Column:         def.StartPoint().Column,
				}, breadcrumbs, nil
			}
		}
	}

	return nil, breadcrumbs, errors.New("could not find definition")
}

var goIdentifiers = []string{"identifier", "type_identifier"}

var goQueries = `
(
    (function_declaration
        name: (identifier) @definition.function) ;@function
)

(
    (method_declaration
        name: (field_identifier) @definition.method); @method
)

(short_var_declaration
  left: (expression_list
          (identifier) @definition.var))

(var_spec
  name: (identifier) @definition.var)

(parameter_declaration (identifier) @definition.var)
(variadic_parameter_declaration (identifier) @definition.var)

(for_statement
 (range_clause
   left: (expression_list
           (identifier) @definition.var)))

(const_declaration
 (const_spec
  name: (identifier) @definition.var))

(type_declaration
  (type_spec
    name: (type_identifier) @definition.type))

;; reference
(identifier) @reference
(type_identifier) @reference
(field_identifier) @reference
((package_identifier) @reference
  (set! reference.kind "namespace"))

(package_clause
   (package_identifier) @definition.namespace)

(import_spec_list
  (import_spec
    name: (package_identifier) @definition.namespace))

;; Call references
((call_expression
   function: (identifier) @reference)
 (set! reference.kind "call" ))

((call_expression
    function: (selector_expression
                field: (field_identifier) @reference))
 (set! reference.kind "call" ))


((call_expression
    function: (parenthesized_expression
                (identifier) @reference))
 (set! reference.kind "call" ))

((call_expression
   function: (parenthesized_expression
               (selector_expression
                 field: (field_identifier) @reference)))
 (set! reference.kind "call" ))

;; Scopes

(func_literal) @scope
(source_file) @scope
(function_declaration) @scope
(if_statement) @scope
(block) @scope
(expression_switch_statement) @scope
(for_statement) @scope
(method_declaration) @scope
`

type Breadcrumb struct {
	Location
	length  int
	message string
}

// IDs are <startByte>-<endByte> as a proxy for node ID
func getId(node *sitter.Node) string {
	return fmt.Sprintf("%d-%d", node.StartByte(), node.EndByte())
}
