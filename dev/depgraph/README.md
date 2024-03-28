# Sourcegraph dependency graph development utilities

A development utility specific to Sourcegraph package import conventions.

## Building

Run `go build` in this directory.

## Running

Run `depgraph {subcommand}` somewhere inside the sourcegraph/sourcegraph repository. This will analyze the Sourcegraph package dependency graph then perform some subcommand-specific action over it.

### Commands

The following commands are available. Across different commands, `{package}` should be specified as a relative path without any leading `./`. For example, `./dev/depgraph/depgraph summary internal/repos` from the root of the `sourcegraph` directory should work.

#### summary

Usage: `./dev/depgraph/depgraph summary {package}`

This command outputs dependency and dependent information for a particular package.

#### trace

Usage: `./dev/depgraph/depgraph trace {package} [-dependency-max-depth=1] [-dependent-max-depth=1]`

This command outputs a dot-formatted graph encoding the (transitive) dependencies of dependencies and (transitive) dependents of dependents rooted at the given package. Saved to a file `trace.dot`, you can convert this to SVG (or another format) via `dot -Tsvg trace.dot -o trace.svg`.

#### trace-internal

Usage: `./dev/depgraph/depgraph trace-internal {package}

This command outputs a dot-formatted graph encoding the internal dependencies within the given package. Saved to a file `trace.dot`, you can convert this to SVG (or another format) via `dot -Tsvg trace.dot -o trace.svg`.

#### lint

Usage: `./dev/depgraph/depgraph lint [pass...]`

This command ensures the following lint passes. Violations of the lint rules will be displayed on standard out, but the utility does not currently exit with a non-zero status.

- **NoBinarySpecificSharedCode**: Report shared packages that are imported only by a single command
- **NoDeadPackages**: Report unused packages (except for library code and main packages)
- **NoLooseCommands**: Report main packages outside of known command roots
- **NoReachingIntoCommands**: Report packages that import code from an unrelated command
- **NoUnusedSharedCommandCode**: Report packages that could be moved into an internal package
Hello World
