package errors

import (
	"context"
)

func Ignore(err error, pred ErrorPredicate) error {
	// If the error (or any wrapped error) is a multierror,
	// filter its children.
	var multi *MultiError
	if As(err, &multi) {
		filtered := multi.Errors[:0]
		for _, childErr := range multi.Errors {
			if ignored := Ignore(childErr, pred); ignored != nil {
				filtered = append(filtered, ignored)
			}
		}
		if len(filtered) == 0 {
			return nil
		}
		multi.Errors = filtered
		return err
	}

	if pred(err) {
		return nil
	}
	return err
}

type ErrorPredicate func(error) bool

func IsContextCanceled(err error) bool {
	return Is(err, context.Canceled)
}
