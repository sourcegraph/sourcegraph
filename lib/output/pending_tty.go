package output

import (
	"bytes"
	"fmt"
	"time"

	"github.com/mattn/go-runewidth"
)

type pendingTTY struct {
	o       *Output
	line    FancyLine
	spinner *spinner
}

func (p *pendingTTY) Verbose(s string) {
	if p.o.verbose {
		p.Write(s)
	}
}

func (p *pendingTTY) Verbosef(format string, args ...any) {
	if p.o.verbose {
		p.Writef(format, args...)
	}
}

func (p *pendingTTY) VerboseLine(line FancyLine) {
	if p.o.verbose {
		p.WriteLine(line)
	}
}

func (p *pendingTTY) Write(s string) {
	p.o.Lock()
	defer p.o.Unlock()

	p.o.moveUp(1)
	p.o.clearCurrentLine()
	fmt.Fprintln(p.o.w, s)
	p.write(p.line)
}

func (p *pendingTTY) Writef(format string, args ...any) {
	p.o.Lock()
	defer p.o.Unlock()

	p.o.moveUp(1)
	p.o.clearCurrentLine()
	fmt.Fprintf(p.o.w, format, p.o.caps.formatArgs(args)...)
	fmt.Fprint(p.o.w, "\n")
	p.write(p.line)
}

func (p *pendingTTY) WriteLine(line FancyLine) {
	p.o.Lock()
	defer p.o.Unlock()

	p.o.moveUp(1)
	p.o.clearCurrentLine()
	line.write(p.o.w, p.o.caps)
	p.write(p.line)
}

func (p *pendingTTY) Update(s string) {
	p.o.Lock()
	defer p.o.Unlock()

	p.line.format = "%s"
	p.line.args = []any{s}

	p.o.moveUp(1)
	p.o.clearCurrentLine()
	p.write(p.line)
}

func (p *pendingTTY) Updatef(format string, args ...any) {
	p.o.Lock()
	defer p.o.Unlock()

	p.line.format = format
	p.line.args = args

	p.o.moveUp(1)
	p.o.clearCurrentLine()
	p.write(p.line)
}

func (p *pendingTTY) Complete(message FancyLine) {
	p.spinner.stop()

	p.o.Lock()
	defer p.o.Unlock()

	p.o.moveUp(1)
	p.o.clearCurrentLine()
	p.write(message)
}

func (p *pendingTTY) Close() { p.Destroy() }

func (p *pendingTTY) Destroy() {
	p.spinner.stop()

	p.o.Lock()
	defer p.o.Unlock()

	p.o.moveUp(1)
	p.o.clearCurrentLine()
}

func newPendingTTY(message FancyLine, o *Output) *pendingTTY {
	p := &pendingTTY{
		o:       o,
		line:    message,
		spinner: newSpinner(100 * time.Millisecond),
	}
	p.updateEmoji(spinnerStrings[0])
	fmt.Fprintln(p.o.w, "")

	go func() {
		for s := range p.spinner.C {
			func() {
				p.o.Lock()
				defer p.o.Unlock()

				p.updateEmoji(s)

				p.o.moveUp(1)
				p.o.clearCurrentLine()
				p.write(p.line)
			}()
		}
	}()

	return p
}

func (p *pendingTTY) updateEmoji(emoji string) {
	// We add an extra space because the Braille characters are single width,
	// but emoji are generally double width and that's what will most likely be
	// used in the completion message, if any.
	p.line.emoji = fmt.Sprintf("%s%s ", p.o.caps.formatArgs([]any{
		p.line.style,
		emoji,
	})...)
}

func (p *pendingTTY) write(message FancyLine) {
	var buf bytes.Buffer

	// This appends a newline to buf, so we have to be careful to ensure that
	// we also add a newline if the line is truncated.
	message.write(&buf, p.o.caps)

	// Using runewidth.Truncate ensures that we're only truncating based on
	// visible string width. I.e. escape codes are not counted towards the
	// string width.
	fmt.Fprint(p.o.w, runewidth.Truncate(buf.String(), p.o.caps.Width, "...\n"))
}
