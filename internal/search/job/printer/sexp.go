package printer

import (
	"bytes"
	"io"

	"github.com/sourcegraph/sourcegraph/internal/search/job"
)

// Sexp outputs the s-expression on a single line.
func Sexp(j job.Describer) string {
	return SexpVerbose(j, job.VerbosityNone, false)
}

// Sexp outputs a pretty-printed s-expression with basic verbosity
func SexpPretty(j job.Describer) string {
	return SexpVerbose(j, job.VerbosityBasic, true)
}

// SexpVerbose outputs a formatted s-expression with two spaces of indentation, potentially spanning multiple lines.
func SexpVerbose(j job.Describer, verbosity job.Verbosity, pretty bool) string {
	if pretty {
		return SexpFormat(j, verbosity, "\n", "  ")
	} else {
		return SexpFormat(j, verbosity, " ", "")
	}
}

func SexpFormat(j job.Describer, verbosity job.Verbosity, sep, indent string) string {
	b := new(bytes.Buffer)
	depth := 0

	var writeSexp func(job.Describer)
	writeSexp = func(j job.Describer) {
		if j == nil {
			return
		}
		tags := j.Attributes(verbosity)
		children := j.Children()
		if len(tags) == 0 && len(children) == 0 {
			b.WriteString(trimmedUpperName(j.Name()))
			return
		}

		b.WriteByte('(')
		b.WriteString(trimmedUpperName(j.Name()))
		depth++
		if len(tags) > 0 {
			enc := sexpKeyValueWriter{b}
			for _, field := range tags {
				writeSep(b, sep, indent, depth)
				enc.Write(string(field.Key), field.Value.Emit())
			}
		}
		if len(children) > 0 {
			for _, child := range children {
				writeSep(b, sep, indent, depth)
				writeSexp(child)
			}
		}
		b.WriteByte(')')
		depth--
	}
	writeSexp(j)
	return b.String()
}

func writeSep(b *bytes.Buffer, sep, indent string, depth int) {
	b.WriteString(sep)
	if indent == "" {
		return
	}
	for range depth {
		b.WriteString(indent)
	}
}

type sexpKeyValueWriter struct{ io.StringWriter }

func (w sexpKeyValueWriter) Write(key, value string) {
	w.WriteString("(")
	w.WriteString(key)
	w.WriteString(" . ")
	w.WriteString(value)
	w.WriteString(")")
}
