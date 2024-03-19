package main

import (
	"fmt"
	"strings"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func sameLine(a, b Location) bool {
	return a.Range.Start.Line == b.Range.Start.Line &&
		a.Range.End.Line == b.Range.End.Line &&
		a.Range.Start.Line == a.Range.End.Line
}

func header(l Location) string {
	return fmt.Sprintf("%s:%d", l.URI, l.Range.Start.Line)
}

func lineCarets(r Range, name string) string {
	return fmt.Sprintf("%s%s %s",
		strings.Repeat(" ", r.Start.Character),
		strings.Repeat("^", r.End.Character-r.Start.Character),
		name,
	)
}

func fmtLine(line int, prefixWidth int, text string) string {
	var prefix string
	if line == -1 {
		prefix = strings.Repeat(" ", prefixWidth)
	} else {
		prefix = fmt.Sprintf("%"+fmt.Sprint(prefixWidth)+"d", line)
	}

	return fmt.Sprintf("|%s| %s", prefix, text)
}

// src/header.c:5
// |4| /// Some documentation
// |5| void exported_funct() {
// | |      ^^^^^^^^^^^^^^^ expected
// | |     ^^^^^^^^^^^^^^^^ actual
// |6|   return;
//
// Only operates on locations with the same URI. It doesn't make sense to diff
// anything here when we don't have that.
func DrawLocations(contents string, expected, actual Location, context int) (string, error) {
	if expected.URI != actual.URI {
		return "", errors.New("Must pass in two locations with the same URI")
	}

	if expected == actual {
		return "", errors.New("You can't pass in two locations that are the same")
	}

	splitLines := strings.Split(contents, "\n")
	if sameLine(expected, actual) {
		line := expected.Range.End.Line

		if line > len(splitLines) {
			return "", errors.New("Line does not exist in contents")
		}

		text := header(expected) + "\n"

		prefixWidth := len(fmt.Sprintf("%d", line+1+context))

		for offset := context; offset > 0; offset-- {
			newLine := line - offset
			if newLine >= 0 {
				text += fmtLine(newLine, prefixWidth, splitLines[newLine]) + "\n"
			}
		}

		text += fmt.Sprintf("%s\n%s\n%s\n",
			fmtLine(line, prefixWidth, splitLines[line]),
			fmtLine(-1, prefixWidth, lineCarets(expected.Range, "expected")),
			fmtLine(-1, prefixWidth, lineCarets(actual.Range, "actual")),
		)

		for offset := range context {
			newLine := line + offset + 1
			if newLine < len(splitLines) {
				text += fmtLine(newLine, prefixWidth, splitLines[newLine]) + "\n"
			}
		}

		return strings.Trim(text, " \n"), nil
	}

	return "failed: tell TJ to implement this.", nil
}
