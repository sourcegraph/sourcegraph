package cmdlogger

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/sourcegraph/log"

	internalexecutor "github.com/sourcegraph/sourcegraph/internal/executor"
	"github.com/sourcegraph/sourcegraph/internal/executor/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Logger tracks command invocations and stores the command's output and
// error stream values.
type Logger interface {
	// Flush waits until all entries have been written to the store and all
	// background goroutines that watch a log entry and possibly update it have
	// exited.
	Flush() error
	// LogEntry creates a new log entry for the given key and command.
	LogEntry(key string, command []string) LogEntry
}

// LogEntry is returned by Logger.Log and implements the io.WriteCloser
// interface to allow clients to update the Out field of the ExecutionLogEntry.
//
// The Close() method *must* be called once the client is done writing log
// output to flush the entry to the database.
type LogEntry interface {
	io.WriteCloser
	// Finalize completes the log entry with the given exit code.
	Finalize(exitCode int)
	// CurrentLogEntry returns the execution log entry.
	CurrentLogEntry() internalexecutor.ExecutionLogEntry
}

// ExecutionLogEntryStore handle interactions with executor.Job logs.
type ExecutionLogEntryStore interface {
	// AddExecutionLogEntry adds a new log entry to the store.
	AddExecutionLogEntry(ctx context.Context, job types.Job, entry internalexecutor.ExecutionLogEntry) (int, error)
	// UpdateExecutionLogEntry updates the log entry with the given ID.
	UpdateExecutionLogEntry(ctx context.Context, job types.Job, entryID int, entry internalexecutor.ExecutionLogEntry) error
}

type SecretRedactor interface {
	Replace(s string) string
}

// NewLogger creates a new logger instance with the given store, job, record,
// and replacement map.
// When the log messages are serialized, any occurrence of sensitive values are
// replace with a non-sensitive value.
// Each log message is written to the store in a goroutine. The Flush method
// must be called to ensure all entries are written.
func NewLogger(internalLogger log.Logger, store ExecutionLogEntryStore, job types.Job, redactor SecretRedactor) Logger {
	l := &logger{
		internalLogger: internalLogger,
		store:          store,
		job:            job,
		done:           make(chan struct{}),
		handles:        make(chan *entryHandle, logEntryBufSize),
		redactor:       redactor,
		errs:           nil,
	}

	go l.writeEntries()

	return l
}

// logEntryBufSize is the maximum number of log entries that are logged by the
// task execution but not yet written to the database.
const logEntryBufSize = 50

func (l *logger) writeEntries() {
	defer close(l.done)

	var wg sync.WaitGroup
	for handle := range l.handles {
		initialLogEntry := handle.CurrentLogEntry()
		entryID, err := l.store.AddExecutionLogEntry(context.Background(), l.job, initialLogEntry)
		if err != nil {
			// If there is a timeout or cancellation error we don't want to skip
			// writing these logs as users will often want to see how far something
			// progressed prior to a timeout.
			l.internalLogger.Warn(
				"Failed to upload executor log entry for job",
				log.Int("jobID", l.job.ID),
				log.String("repositoryName", l.job.RepositoryName),
				log.String("commit", l.job.Commit),
				log.Error(err),
			)

			l.appendError(err)

			continue
		}
		l.internalLogger.Debug(
			"Writing log entry",
			log.Int("jobID", l.job.ID),
			log.Int("entryID", entryID),
			log.String("entryKey", initialLogEntry.Key),
			log.String("repositoryName", l.job.RepositoryName),
			log.String("commit", l.job.Commit),
		)

		wg.Add(1)
		go func() {
			defer wg.Done()
			l.syncLogEntry(handle, entryID, initialLogEntry)
		}()
	}

	wg.Wait()
}

func (l *logger) appendError(err error) {
	l.errsMu.Lock()
	l.errs = errors.Append(l.errs, err)
	l.errsMu.Unlock()
}

func (l *logger) syncLogEntry(handle *entryHandle, entryID int, old internalexecutor.ExecutionLogEntry) {
	lastWrite := false

	for !lastWrite {
		select {
		case <-handle.done:
			lastWrite = true
		case <-time.After(syncLogEntryInterval):
		}

		current := handle.CurrentLogEntry()
		if !entryWasUpdated(old, current) {
			continue
		}

		l.internalLogger.Debug(
			"Updating executor log entry",
			log.Int("jobID", l.job.ID),
			log.Int("entryID", entryID),
			log.String("repositoryName", l.job.RepositoryName),
			log.String("commit", l.job.Commit),
			log.String("key", current.Key),
			// Since the command may contain sensitive information, redact the
			// command before logging it.
			log.String("command", l.redactor.Replace(strings.Join(current.Command, " "))),
			log.Int("outLen", len(current.Out)),
			log.Intp("exitCode", current.ExitCode),
			log.String("startTime", current.StartTime.String()),
			log.Intp("durationMs", current.DurationMs),
		)

		if err := l.store.UpdateExecutionLogEntry(context.Background(), l.job, entryID, current); err != nil {
			logMethod := l.internalLogger.Warn
			if lastWrite {
				logMethod = l.internalLogger.Error
				// If lastWrite, this MUST complete for the job to be considered successful,
				// so we want to hard-fail otherwise. We store away the error.
				l.appendError(err)
			}

			logMethod(
				"Failed to update executor log entry for job",
				log.Int("jobID", l.job.ID),
				log.Int("entryID", entryID),
				log.String("repositoryName", l.job.RepositoryName),
				log.String("commit", l.job.Commit),
				log.Bool("lastWrite", lastWrite),
				log.Error(err),
			)
		} else {
			old = current
		}
	}
}

const syncLogEntryInterval = 1 * time.Second

// If old didn't have exit code or duration and current does, update; we're finished.
// Otherwise, update if the log text has changed since the last write to the API.
func entryWasUpdated(old, current internalexecutor.ExecutionLogEntry) bool {
	return (current.ExitCode != nil && old.ExitCode == nil) || (current.DurationMs != nil && old.DurationMs == nil) || current.Out != old.Out
}

type logger struct {
	internalLogger log.Logger
	store          ExecutionLogEntryStore
	done           chan struct{}
	handles        chan *entryHandle

	job types.Job

	redactor SecretRedactor

	errs   error
	errsMu sync.Mutex
}

func (l *logger) Flush() error {
	close(l.handles)
	<-l.done

	l.errsMu.Lock()
	defer l.errsMu.Unlock()

	return l.errs
}

func (l *logger) LogEntry(key string, command []string) LogEntry {
	handle := &entryHandle{
		logEntry: internalexecutor.ExecutionLogEntry{
			Key:       key,
			Command:   command,
			StartTime: time.Now(),
		},
		redactor: l.redactor,
		buf:      &bytes.Buffer{},
		done:     make(chan struct{}),
	}

	l.handles <- handle
	return handle
}

type entryHandle struct {
	logEntry internalexecutor.ExecutionLogEntry
	redactor SecretRedactor

	done chan struct{}

	mu         sync.Mutex
	buf        *bytes.Buffer
	exitCode   *int
	durationMs *int
}

func (h *entryHandle) Write(p []byte) (n int, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.buf.Write(p)
}

func (h *entryHandle) Finalize(exitCode int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	durationMs := int(time.Since(h.logEntry.StartTime) / time.Millisecond)
	h.exitCode = &exitCode
	h.durationMs = &durationMs
}

func (h *entryHandle) Close() error {
	close(h.done)
	return nil
}

func (h *entryHandle) CurrentLogEntry() internalexecutor.ExecutionLogEntry {
	logEntry := h.currentLogEntry()
	redact(&logEntry, h.redactor)
	return logEntry
}

func (h *entryHandle) currentLogEntry() internalexecutor.ExecutionLogEntry {
	h.mu.Lock()
	defer h.mu.Unlock()

	logEntry := h.logEntry
	logEntry.ExitCode = h.exitCode
	logEntry.Out = h.buf.String()
	logEntry.DurationMs = h.durationMs
	return logEntry
}

func redact(entry *internalexecutor.ExecutionLogEntry, replacer SecretRedactor) {
	for i, arg := range entry.Command {
		entry.Command[i] = replacer.Replace(arg)
	}
	entry.Out = replacer.Replace(entry.Out)
}
