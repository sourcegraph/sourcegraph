package background

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

type key int

var (
	jobsKey key
	hasRun  bool
)

type backgroundJobs struct {
	wg                sync.WaitGroup
	stillRunningCount atomic.Int32

	verbose bool
	output  chan string
}

// Context creates a context that can have background jobs added to it with background.Run
func Context(ctx context.Context, verbose bool) context.Context {
	return context.WithValue(ctx, jobsKey, &backgroundJobs{
		verbose: verbose,
		output:  make(chan string, 10), // reasonable default
	})
}

func loadFromContext(ctx context.Context) *backgroundJobs {
	return ctx.Value(jobsKey).(*backgroundJobs)
}

// Run starts the given job and registers it so that background.Wait does not exit until
// this job is complete.
//
// Jobs get a context timeout of 30 seconds, and must respect the cancellation.
// In long-running jobs, output should be sent to the provided backgroundOutput
// to be rendered to the user only on command exit - otherwise, dev/sg/internal/std.Out
// can be used instead to render output immediately.
func Run(ctx context.Context, job func(ctx context.Context, backgroundOutput *std.Output)) {
	jobs := loadFromContext(ctx)
	jobs.wg.Add(1)
	jobs.stillRunningCount.Add(1)

	b := new(bytes.Buffer)
	out := std.NewOutput(b, jobs.verbose)
	go func() {
		// Do not let the job run forever
		jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		// Execute job
		job(jobCtx, out)
		// Signal the completion of this job
		jobs.stillRunningCount.Dec()
		// If the job provides background output, collect it to be rendered on
		// command exit.
		jobs.output <- strings.TrimSpace(b.String())
	}()
}

// Wait blocks until jobs registered in context are complete, rendering their results as
// they complete. If the jobs are all completed when Wait gets called, it will simply flush out
// outputs from completed jobs. This should only be called when user command execution is
// complete, and we are now waiting for background tasks to complete.
func Wait(ctx context.Context, out *std.Output) {
	if hasRun {
		return
	}
	hasRun = true
	jobs := loadFromContext(ctx)
	pendingCount := int(jobs.stillRunningCount.Load())

	firstResultWithOutput := true
	if jobs.verbose && pendingCount > 0 {
		out.WriteLine(output.Styledf(output.StylePending, "Waiting for %d remaining background %s to complete...",
			pendingCount, pluralize("job", "jobs", pendingCount)))
	}
	go func() {
		// Stream job output as they complete
		for jobOutput := range jobs.output {
			if jobOutput != "" {
				if firstResultWithOutput {
					out.Write("") // separator for background output
					firstResultWithOutput = false
				}
				out.Write(jobOutput)
			}
			jobs.wg.Done()
		}
	}()
	jobs.wg.Wait()

	// Done!
	close(jobs.output)
	if jobs.verbose && pendingCount > 0 {
		out.WriteLine(output.Line(output.EmojiSuccess, output.StyleSuccess, "Background jobs done!"))
	}
}

func pluralize(single, plural string, count int) string {
	if count != 1 {
		return plural
	}
	return single
}
