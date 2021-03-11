package command

import (
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/workerutil"
)

// Logger tracks command invocations and stores the command's output and
// error stream values.
type Logger struct {
	replacer *strings.Replacer
	entries  []workerutil.ExecutionLogEntry
}

// NewLogger creates a new logger instance with the given replacement map.
// When the log messages are serialized, any occurrence of sensitive values
// are replace with a non-sensitive value.
func NewLogger(replacements map[string]string) *Logger {
	oldnew := make([]string, 0, len(replacements)*2)
	for k, v := range replacements {
		oldnew = append(oldnew, k, v)
	}

	return &Logger{
		replacer: strings.NewReplacer(oldnew...),
	}
}

// Log redacts secrets from the given log entry and stores it.
func (l *Logger) Log(entry workerutil.ExecutionLogEntry) {
	l.entries = append(l.entries, redact(entry, l.replacer))
}

// Entries returns a copy of the stored log entries.
func (l *Logger) Entries() (entries []workerutil.ExecutionLogEntry) {
	entries = append(entries, l.entries...)
	return entries
}

func redact(entry workerutil.ExecutionLogEntry, replacer *strings.Replacer) workerutil.ExecutionLogEntry {
	for i, arg := range entry.Command {
		entry.Command[i] = replacer.Replace(arg)
	}
	entry.Out = replacer.Replace(entry.Out)
	return entry
}
