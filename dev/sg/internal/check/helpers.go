package check

import (
	"context"
	"os"
	"time"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func Any(checks ...CheckFunc) CheckFunc {
	return func(ctx context.Context) (err error) {
		for _, chk := range checks {
			err = chk(ctx)
			if err == nil {
				return nil
			}
		}
		return err
	}
}

func Combine(checks ...CheckFunc) CheckFunc {
	return func(ctx context.Context) (err error) {
		for _, chk := range checks {
			err = chk(ctx)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

func Retry(check CheckFunc, retries int, sleep time.Duration) CheckFunc {
	return func(ctx context.Context) (err error) {
		for range retries {
			err = check(ctx)
			if err == nil {
				return nil
			}
			time.Sleep(sleep)
		}
		return err
	}
}

func WrapErrMessage(check CheckFunc, message string) CheckFunc {
	return func(ctx context.Context) error {
		err := check(ctx)
		if err != nil {
			return errors.Wrap(err, message)
		}
		return nil
	}
}

// SkipOnNix will not run check if running inside of our nix develop
// environment. reason is not read, but is used to document why at the
// callsite.
func SkipOnNix(reason string, check CheckFunc) CheckFunc {
	if os.Getenv("IN_NIX_SHELL") != "" && os.Getenv("name") == "sourcegraph-dev-env" {
		return func(ctx context.Context) error {
			return nil
		}
	}
	return check
}
