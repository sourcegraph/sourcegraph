# Dependencies and generated code

## Go dependency management

We use Go modules to manage Go dependencies in this repository.

## Codegen

The Sourcegraph repository relies on code generation triggered by `go generate`. Code generation is used for a variety of tasks:

- generating code for mocking interfaces
- generate wrappers for interfaces (e.g., `./server/internal/middleware/*` packages)
- pack app templates and assets into binaries

To generate everything, just run:

```bash
sg generate
```

Note: Sometimes, there are erroneous diffs. This occurs for a few
reasons, none of which are legitimate (i.e., they are tech debt items
we need to address):

- The codegen tools might emit code that depends on system configuration,
  such as the system timezone or packages you have in your GOPATH. We
  need to submit PRs to the tools to eliminate these issues.
- You might have existing but gitignored files that the codegen tools
  read on your disk that other developers don't have. (This occurs for
  app assets especially.)

If you think a diff is erroneous, don't commit it. Add a tech debt
item to the issue tracker and assign the person who you think is
responsible (or ask).
