# Go style guide

This file documents the style used in Sourcegraph's code. For non-code text, see the overall [content guidelines](https://handbook.sourcegraph.com/communication/content_guidelines).

For all things not covered in this document, defer to
[Go Code Review Comments](https://code.google.com/p/go-wiki/wiki/CodeReviewComments)
and [Effective Go](http://golang.org/doc/effective_go.html).

Related pages: [Testing Go code](testing_go_code.md) | [Error handling in Go](go_errors.md) | [Exposing Services](../exposing-services.md)

## Panics

Panics are used for code paths that should never be reached.

## Options

In the general case, when a pointer to an "options" struct is an argument
to a function (such as `Get(build BuildSpec, opt *BuildGetOptions) (*Build, Response, error)`,
that pointer may be `nil`. When the pointer is nil, the function does its default behavior.
If the options struct should not be nil, either make the argument the value instead of a
pointer or document it.

## Pull requests

Avoid unnecessary formatting changes that are unrelated to your change. If you find code that isn't formatted correctly, fix the formatting in a separate PR by running the appropriate formatter (e.g. `prettier`, `gofmt`).

## Group code blocks logically

Declare your variables close to where they are actually used.

prefer

```go
a, b := Vector{1, 2, 3}, Vector{4, 5, 6}
a, b := swap(a, b)

c := Vector{7, 8, 9}
total := add([]Vector(a, b, c))...)
```

over

```go
a, b, c := Vector{1, 2, 3}, Vector{4, 5, 6}, Vector{7, 8, 9}
a, b := swap(a, b)

total := add([]Vector(a, b, c))...)
```

The `c` `Vector` isn't used until the `add()` function call, so why not declare it immediately beforehand?

By logically grouping components together, you make sure that the context around them isn't lost by the time they come into play. More concretely:

- You have to keep less in your mental buffer -- which is great if you use a screenreader
- You have to navigate around the code base less to find definitions or declarations -- and that’s great if you have difficulties with fine motor control
- You’re minimizing the amount of navigation needed to comprehend a block of code.

This advice also goes for other types of declarations -- interfaces, structs, etc…

## Keep names short

Prefer

```go
var a, b Vector
```

over

```go
var vectorA, vectorB Vector
```

Go [already encourages short variable names](https://github.com/golang/go/wiki/CodeReviewComments#variable-names).

In addition, short names:

- Are faster to listen to (and read)
- Are easier to navigate around
- Are less effort to type

In the above example, you might think that the names `vectorA` and `vectorB` are good because you're putting context inside the name itself. That way, there's no confusion / ambiguity when the variables are referred to elsewhere. However, this is redundant / not necessary if you're following the [group code blocks logically](#group-code-blocks-logically) advice above.

## Make names meaningful

Prefer

```go
var total, scaled Vector
```

over

```go
var tVec, sVec Vector
```

Using meaningful names reduces the amount of work that a person has to do to understand what’s going on in your code. More concretely:

- They don’t have to keep as much context in their head about what that variable does.
- They don’t have to jump around to find definitions, usage, etc…
- It can also help distinguish important variables from temporary placeholders

Whenever possible, prefer meaningful names over explanatory comments. Comments are an extra thing to navigate around, and they don't actually reduce the amount of jumping around the codebase that you'll need to do when the variables are used later on.

## Use pronounceable names

Prefer

```go
var total Vector
func add(...)
```

over

```go
var tVec Vector
var addAllVecs(...)
```

Pronounceable names:

- Screen readers can actually read them
- Takes less time than pronouncing a string of letters

[You should watch this short YouTube video of @juliaferraioli navigating some Go code with a screenreader.](https://www.youtube.com/watch?v=xwjvufcJK-Q)

Now, something you may have noticed during the demo, is how screen readers handle variable names. It’s rough, right?

[@juliaferraioli](https://twitter.com/juliaferraioli) shared an anecdote about how she spent about 15 minutes scratching her head during a code review the other day, wondering what “gee-thub” was, before she realized that it was reading “GitHub”.

So make sure you use pronounceable names. Don’t make up words. Think of how they would be spoken. **Avoid concatenated variable names when possible.** Various screen readers won’t necessarily make it clear that the variable name is one word.

## Use new lines intentionally

```go
a, b := Vector{1, 2, 3}, Vector{4, 5, 6}
a, b := swap(a, b)

c := Vector{7, 8, 9}
total := add([]Vector(a, b, c))...)
```

If we revisit the recommended organization, we can also see the usage of new lines. Newlines are something we kind of pepper in our code without really thinking about it. However, they can be really powerful signals. I recommend that you treat them like paragraph breaks -- if you don’t use any at all, your reader is lost. If you use them too much, your message is fragmented. They can help guide the user to where the logical components are.

Be intentional with them!
