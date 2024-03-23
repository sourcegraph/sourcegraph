package main

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/validation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

var updateInterval = time.Second / 4

func validate(indexFile *os.File) error {
	ctx := validation.NewValidationContext()
	validator := &validation.Validator{Context: ctx}
	errs := make(chan error, 1)

	go func() {
		defer close(errs)

		if err := validator.Validate(indexFile); err != nil {
			errs <- err
		}
	}()

	if err := printProgress(ctx, errs); err != nil {
		return err
	}

	for i, err := range ctx.Errors {
		fmt.Printf("%d) %s\n", i+1, err)
	}

	if len(ctx.Errors) > 0 {
		return errors.Newf("Detected %d errors", len(ctx.Errors))
	}

	return nil
}

func printProgress(ctx *validation.ValidationContext, errs <-chan error) error {
	out := output.NewOutput(os.Stdout, output.OutputOpts{})
	pending := out.Pending(output.Linef("", output.StylePending, "%d vertices, %d edges", atomic.LoadUint64(&ctx.NumVertices), atomic.LoadUint64(&ctx.NumEdges)))
	defer func() {
		pending.Complete(output.Line(output.EmojiSuccess, output.StyleSuccess, "Done!"))
	}()

	for {
		ctx.ErrorsLock.RLock()
		numErrors := len(ctx.Errors)
		ctx.ErrorsLock.RUnlock()

		pending.Updatef(
			"%d vertices, %d edges, %d errors",
			atomic.LoadUint64(&ctx.NumVertices),
			atomic.LoadUint64(&ctx.NumEdges),
			numErrors,
		)

		select {
		case err := <-errs:
			return err
		case <-time.After(updateInterval):
		}
	}
}
