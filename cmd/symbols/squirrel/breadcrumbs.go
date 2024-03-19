package squirrel

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"

	"github.com/sourcegraph/sourcegraph/internal/types"
)

// Breadcrumb is an arbitrary annotation on a token in a file. It's used as a way to log where Squirrel
// has been traversing through trees and files for debugging.
type Breadcrumb struct {
	types.RepoCommitPathRange
	length  int
	message func() string
	number  int
	depth   int
	file    string
	line    int
}

// Breadcrumbs is a slice of Breadcrumb.
type Breadcrumbs []Breadcrumb

// Prints breadcrumbs like this:
//
//	v some breadcrumb
//	  vvv other breadcrumb
//
// 78 | func f(f Foo) {
func (bs *Breadcrumbs) pretty(w *strings.Builder, readFile readFileFunc) {
	// First collect all the breadcrumbs in a map (path -> line -> breadcrumb) for easier printing.
	pathToLineToBreadcrumbs := map[types.RepoCommitPath]map[int][]Breadcrumb{}
	for _, breadcrumb := range *bs {
		path := breadcrumb.RepoCommitPath

		if _, ok := pathToLineToBreadcrumbs[path]; !ok {
			pathToLineToBreadcrumbs[path] = map[int][]Breadcrumb{}
		}

		pathToLineToBreadcrumbs[path][breadcrumb.Row] = append(pathToLineToBreadcrumbs[path][breadcrumb.Row], breadcrumb)
	}

	// Loop over each path, printing the breadcrumbs for each line.
	for repoCommitPath, lineToBreadcrumb := range pathToLineToBreadcrumbs {
		// Print the path header.
		blue := color.New(color.FgBlue).SprintFunc()
		grey := color.New(color.FgBlack).SprintFunc()
		fmt.Fprintf(w, blue("repo %s, commit %s, path %s"), repoCommitPath.Repo, repoCommitPath.Commit, repoCommitPath.Path)
		fmt.Fprintln(w)

		// Read the file.
		contents, err := readFile(context.Background(), repoCommitPath)
		if err != nil {
			fmt.Println("Error reading file: ", err)
			return
		}

		// Print the breadcrumbs for each line.
		for lineNumber, line := range strings.Split(string(contents), "\n") {
			breadcrumbs, ok := lineToBreadcrumb[lineNumber]
			if !ok {
				// No breadcrumbs on this line.
				continue
			}

			fmt.Fprintln(w)

			gutter := fmt.Sprintf("%5d | ", lineNumber)

			columnToMessage := map[int]string{}
			for _, breadcrumb := range breadcrumbs {
				for column := breadcrumb.Column; column < breadcrumb.Column+breadcrumb.length; column++ {
					columnToMessage[lengthInSpaces(line[:column])] = breadcrumb.message()
				}

				gutterPadding := strings.Repeat(" ", len(gutter))

				space := strings.Repeat(" ", lengthInSpaces(line[:breadcrumb.Column]))

				arrows := color.HiMagentaString(strings.Repeat("v", breadcrumb.length))

				fmt.Fprintf(w, "%s%s%s %s %s\n", gutterPadding, space, arrows, color.RedString("%d", breadcrumb.number), breadcrumb.message())
			}

			fmt.Fprint(w, grey(gutter))
			lineWithSpaces := strings.ReplaceAll(line, "\t", "    ")
			for c := range len(lineWithSpaces) {
				if _, ok := columnToMessage[c]; ok {
					fmt.Fprint(w, color.HiMagentaString(string(lineWithSpaces[c])))
				} else {
					fmt.Fprint(w, grey(string(lineWithSpaces[c])))
				}
			}
			fmt.Fprintln(w)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Breadcrumbs by call tree:")
	fmt.Fprintln(w)

	for _, b := range *bs {
		fmt.Fprintf(w, "%s%s%s %s\n", strings.Repeat("| ", b.depth), itermSource(b.file, b.line), color.RedString("%d", b.number), b.message())
	}
}

func itermSource(absPath string, line int) string {
	if os.Getenv("SRC_LOG_SOURCE_LINK") == "true" {
		// Link to open the file:line in VS Code.
		url := fmt.Sprintf("vscode://file%s:%d", absPath, line)

		// Constructs an escape sequence that iTerm recognizes as a link.
		// See https://iterm2.com/documentation-escape-codes.html
		link := fmt.Sprintf("\x1B]8;;%s\x07%s\x1B]8;;\x07", url, "src")

		return fmt.Sprintf(color.New(color.Faint).Sprint(link) + " ")
	}

	return ""
}

func (bs *Breadcrumbs) prettyPrint(readFile readFileFunc) {
	fmt.Println(" ")
	fmt.Println(bracket(bs.prettyString(readFile)))
	fmt.Println(" ")
}

func (bs *Breadcrumbs) prettyString(readFile readFileFunc) string {
	sb := &strings.Builder{}
	bs.pretty(sb, readFile)
	return sb.String()
}
