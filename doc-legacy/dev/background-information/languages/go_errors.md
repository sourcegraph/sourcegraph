# Error handling in Go

We disallow use of error packages — including the stdlib [`errors`](https://golang.org/pkg/errors/) package — (enforced by a lint pass in CI) other than our internal `github.com/sourcegraph/sourcegraph/lib/errors` package.

We also require the use of `errors.Newf` over the use of `fmt.Errorf`, which also constructs an error type. This is to ensure that each error constructed by Sourcegraph is tagged with a stack depth and allows redaction of content within user-visible strings.

## Use of `errors.New`

Use this function to create an error value with a static message.

```go
var ErrSomethingWentWrong = errors.New("something went wrong")
```

Idiomatic error messages in Go should start with a lowercase letter and should contain no trailing punctuation.

Generally, errors of this class should be created as a constant at the highest level possible (e.g., an unexported package constant). Such errors should be exported if direct comparison of error values should be allowed by a user.

Idiomatically, error constant _values_ should always have a name of the format `ErrX` (or `errX` if package-private) and types that can be used as `error` should have a name of the format `XError`.

## Use of `errors.Errorf`

Use this function to create an error value with non-static message.

```go
return errors.Errorf("user %d does not exist", userID)
```

The formatting directives are the same as `fmt.Sprintf`. The [`%w` formatting directive](https://blog.golang.org/go1.13-errors#TOC_3.3.) special cases error values. Prefer to use `errors.Wrap` over this directive.

## Use of `errors.Wrap`

Use this function to add additional data to an existing error value.

```go
if err := myPkg.MyFunc(ctx); err != nil {
	return errors.Wrap(err, "myPkg.MyFunc")
}
```

Wrap all errors being returned from a method if that error did not originate in the same package (except for standard library packages like `os` or `http` which have very obvious and distinct error messages) or the same struct (unless its not used from multiple callers). This produces error messages with a usable stacktrace.

## Use of `errors.Is`

Use this function to determine if any cause of a given error value is equal to a target error value.

```go
for _, filename := range filenames {
	f, err := os.Open(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// skip missing files
			continue
		}
	
		return err
	}
	
	defer f.Close()

	// ...
}
```

> NOTE: The second argument to this function should generally be error constant, or at the least a trivial (message-only) error value. Comparison of error values uses the `==` operator or the first error value's `Is(other error) bool` method (if implemented). Comparing two error values of the same type but with different field values will generally not do what you want; use the `HasType` function instead.

## Use of `errors.IsAny`

Use this function to determine if any cause of a given error is equal to at least one target error value.

```go
if err := Some(ctx); err != nil {
	if errors.IsAny(err, context.DeadlineExceeded, context.Canceled) {
		// context error
	} else {
		// another type of error
	}
}
```

As with the `Is` function, the second argument to this function should generally be an error constant.

## Use of `errors.HasType`

Use this function to determine if a given error has a particular type.

```go
type NotFoundError struct {
	ID string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("Object `%d` not found", e.ID)
}

// Note: We use a pointer here because the receiver of the
// target struct's Error method is a pointer. If the receiver
// was a value type, we'd use a value type here as well.
if errors.HasType(err, &NotFoundError{}) {
	// not found error
} else {
	// another type of error
}
```

This function should be preferred when comparing an error value with an _error struct_ that may tag additional data (e.g., a failed opcode or the identifier of a missing object).

Note that using the `Is` function here will fail to match any error that does not have equivalent fields. In the example above, `Is` will only match error values where the value of the `ID` field is the zero value.

## Use of `errors.As`

Use this function to safely cast a given error value to a particular type. This should be preferred over using Go language-level type casing of an error value (e.g., `err.(*MyType)`), which fails to unwrap errors.

```go
type OpError struct {
	Op string
}

func (e OpError) Error() string {
	return fmt.Sprintf("Oops because of %s", e.Op)
}

// Note: We use a value type here because the receiver of the
// target struct's Error method is a value type. If the receiver
// was a pointer, we'd use a pointer here as well.
var e OpError

if errors.As(err, &e) {
	switch e.Op {
		// ...
	}
} else {
	// ...
}
```

This function can also be used with interface values.

```go
func isRetryable(err error) bool {
	var e interface { Retryable() bool }

	if errors.As(err, &e) {
		return e.Retryable()
	}

	return false
}
```

## Use of `errors.Cause`

Use this function to remove all layers from a given error value and return only the root cause. This function should be used rarely as the `Is`, `HasType`, and `As` functions cover the most common functionality.

```go
fmt.Printf("Root error: %s\n", errors.Cause(err))
```

## Use of `errors.MultiError`

`MultiError` is an error interface that implements a "_bag_ of errors". Typically, errors are _chains_: where error A causes error B, and so on, through the use of [`Wrap`](#use-of-errorswrap). If you have tasks that run in parallel and return errors in tandem, for example, you may want a _bag_ of errors instead.

To create a `MultiError`, you can use `Append` or `CombineErrors`. A common paradigm is:

```go
var err errors.MultiError
for _, fn := range fnsThatReturnError {
  err = errors.Append(err, fn())
}
return err
```

The `MultiError` type:

- will be treated as a `nil` error if an `Append` or `CombineErrors` only merges errors that are `nil`
- exposes errors within to introspection methods like `As`, `Is`, etc.
- prints all errors within in a multi-line list format on `(MultiError).Error()`
- acts like a single error if the bag only contains a single error (notably for printing and introspection behaviours)

Check out the source code for the `MultiError` implementation in [`lib/errors`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/lib/errors).

## Printing errors

Printing errors with most formatting directives like `%s`, `%v`, etc. will render an error by calling its `Error()` implementation.

Errors created from the `lib/errors` library, such as [`New`](#use-of-errorsnew), [`Wrap`](#use-of-errorswrap), etc. also carry additional details such as stack traces. This is exposed to integrations like Sentry, and can be rendered with the `%+v` formatting directive.
