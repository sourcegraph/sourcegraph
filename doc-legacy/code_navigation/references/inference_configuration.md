# Auto-indexing inference configuration reference

<aside class="beta">
<p>
<span class="badge badge-beta">Beta</span> This feature is in beta for self-hosted customers.

<p><b>We're very much looking for input and feedback on this feature.</b> You can either <a href="https://sourcegraph.com/contact">contact us directly</a>, <a href="https://github.com/sourcegraph/sourcegraph">file an issue</a>, or <a href="https://twitter.com/sourcegraph">tweet at us</a>.</p>
</aside>
</p>
</aside>

This document details how a site administrator can supply a Lua script to customize the way [Sourcegraph detects precise code intelligence indexing jobs from repository contents](../explanations/auto_indexing_inference.md).

By default, Sourcegraph will attempt to infer index jobs for the following languages:

- [`Go`](../explanations/auto_indexing_inference.md#go)
- [`Java`/`Scala`/`Kotlin`](../explanations/auto_indexing_inference.md#java)
- `Python`
- `Ruby`
- [`Rust`](../explanations/auto_indexing_inference.md#rust)
- [`TypeScript`/`JavaScript`](../explanations/auto_indexing_inference.md#typescript)

Inference logic can be disabled or altered in the case when the target repositories do not conform to a pattern that the Sourcegraph default inference logic recognizes. Inference logic is controlled by a **Lua override script** that can be supplied in the UI under `Admin > Code graph > Inference`.

> NOTE: While the change is self-service, **Sourcegraph support is more than happy to help write custom behaviors with you**. Do not hesitate to contact us to get the inference logic behaving how you would expect for **your** repositories.

## Example

The **Lua override script** ultimately must return an _auto-indexing config object_. A configuration that neither disables or adds new recognizers does not change the default inference behavior.

```lua
return require("sg.autoindex.config").new({
  -- Empty configuration (see below for usage)
})
```

To **disable** default behaviors, you can re-assign a recognizer value to `false`. Each of the built-in recognizers are prefixed with `sg.` (and are the only ones allowed to be).

```lua
return require("sg.autoindex.config").new({
  -- Disable default Python inference
  ["sg.python"] = false
})
```

To **add** additional behaviors, you can create and register a new **recognizer**. A recognizer is an interface that requests some set of files from a repository, and returns a set of auto-indexing job configurations that could produce a precise code intelligence index.

A _path recognizer_ is a concrete recognizer that advertises a set of path _globs_ it is interested in, then invokes its `generate` function with matching paths from a repository. In the following, all files matching `Snek.module` (`Snek.module`, `proj/Snek.module`, `proj/sub/Snek.module`, etc) are passed to a call to `generate` (if non-empty). The generate function will then return a list of indexing job descriptions. The [guide for auto-indexing jobs configuration](auto_indexing_configuration.md#keys-1) gives detailed descriptions on the fields of this object.

```lua
local path = require("path")
local pattern = require("sg.autoindex.patterns")
local recognizer = require("sg.autoindex.recognizer")

local snek_recognizer = recognizer.new_path_recognizer {
  patterns = {
    -- Look for Snek.module files 
    -- (would match Snek.module; proj/Snek.module, proj/sub/Snek.module, etc)
    pattern.new_path_basename("Snek.module"),

    -- Ignore any files in test or vendor directories
    pattern.new_path_exclude(
      pattern.new_path_segment("test"),
      pattern.new_path_segment("vendor")
    ),
  },

  -- Called with list of matching Snek.module files
  generate = function(_, paths)
    local jobs = {}
    for i = 1, #paths do
      -- Create indexing job description for each matching file
      table.insert(jobs, {
        indexer = "acme/snek:latest",  -- Run this indexer...
        root = path.dirname(paths[i]), -- ...in this directory
        local_steps = {"snekpm install"}, -- Install dependencies
        indexer_args = {"snek", "index", ".", "--output", "index.scip"},
        outfile = "index.scip",
      })
    end

    return jobs
  end
}

return require("sg.autoindex.config").new({
  -- Register new recognizer
  ["acme.snek"] = snek_recognizer,
})
```

## Available libraries

There are a number of specific and general-purpose Lua libraries made accessible via the built-in `require`.

The type signatures for the functions below use the following syntax:

- `(A1, ..., An) -> R`: Function type with arguments of type `A1, ..., An` and return type `R`.
- `array[A]`: Table with indexes 1 to N of elements of type `A`.
- `table[K, V]`: Table with keys of type `K` and values of type `V`.
- `A | B`: Union type (includes values of type `A` and type `B`).
- `A...`: Variadic (0 or more values of A, without being wrapped in a table).
- `"mystring"`: Literal string type with only `"mystring"` as the allowed value.
- `{K1: V1, K2: V2, ...}`: Heterogenous table (object) with a key of type `K1` mapping to a value of type `V1` etc.
- `void`: no value returned from function

### `sg.autoindex.recognizer`

This auto-indexing-specific library defines the following two functions.

<!-- TODO - document paths_for_content;api.register -->
- `new_path_recognizer` creates a `Recognizer` from a config object containing `patterns` and `generate` fields. See the [example](#example) above for basic usage.
  - Type:
    ```
    ({
      "patterns": array[pattern],
      "patterns_for_content": array[pattern],
      "generate": (registration_api, paths: array[string], contents_by_path: table[string, string]) -> array[index_job],
    }) -> recognizer
    ```
    where `index_job` is an object with the following shape:
    ```
    index_job = {
      "indexer": string, -- Docker image for the indexer
      "root": string,    -- working directory for invoking the indexer
      "steps": array[{   -- preparatory steps to run before invoking the indexer (e.g. installing dependencies)
        "root": string,  -- working directory for this step
        "image": string  -- Docker image to use for preparatory step
        "commands": array[string] -- List of commands to run inside the Docker image
      }],
      "local_steps": array[string] -- List of commands to run inside the indexer image at "root" before invoking
                                   -- the indexer (e.g. to install dependencies)
      "indexer_args": array[string], -- command-line invocation for the indexer
      "outfile": string,             -- path to the index generated by the indexer
      "requested_envvars": array[string], -- List of environment variables needed. These are made accessible
                                          -- to steps, local_steps, and the indexer_args command.
    }
    ```
    For installing dependencies, if the indexer image contains the relevant package manager(s),
    then it is simpler to install dependencies using `local_steps`. Otherwise,
    the `steps` field allows more customizability.

- `new_fallback_recognizer` creates a `recognizer` from an ordered list of `recognizer`s. Each `recognizer` is called sequentially, until one of them emits non-empty results.
  - Type: `(array[recognizer]) -> recognizer`

The `registration_api` object has the following API:
- `register` which queues a `recognizer` to be run at a later stage.
  This makes it possible to add more recognizers dynamically,
  such as based on whether specific configuration files were found or not.
  - Type: `(recognizer) -> void`

### `sg.autoindex.patterns`

This auto-indexing-specific library defines the following four path pattern constructors.

- `new_path_literal(fullpath)` creates a `pattern` that matches an exact filepath.
  - Type: `(string) -> pattern`
- `new_path_segment(segment)` creates a `pattern` that matches a directory name.
  - Type: `(string) -> pattern`
- `new_path_basename(basename)` creates a `pattern` that matches a basename exactly.
  - Type: `(string) -> pattern`
- `new_path_extension(ext_no_leading_dot)` creates a `pattern` that matches files with a given extension. 
  - Type: `(string) -> pattern`

This library also defines the following two pattern collection constructors.

- `new_path_combine(patterns)` creates a pattern collection object (to be used with [recognizers](#sg-autoindex-recognizers)) from the given set of path `pattern`s.
  - Type: `((pattern | array[pattern])...) -> pattern`
- `new_path_exclude(patterns)` creates a new _inverted_ pattern collection object. Paths matching these `pattern`s are filtered out from the set of matching filepaths given to a recognizer's `generate` function.
  - Type: `((pattern | array[pattern])...) -> pattern`

### `path`

This library defines the following utility functions:

- `ancestors(path)` returns a list `{dirname(path), dirname(dirname(path)), ...}`. The last element in the list will be an empty string.
  - Type: `(string) -> array[string]`
- `basename(path)` returns the basename of the given path as defined by Go's [filepath.Base](https://pkg.go.dev/path/filepath#Base).
  - Type: `(string) -> string`
- `dirname(path)` returns the dirname of the given path as defined by Go's [filepath.Dir](https://pkg.go.dev/path/filepath#Dir), except that it (1) returns an empty path instead of `"."` if the path is empty and (2) removes a leading `/` if present.
  - Type: `string -> string`
- `join(path1, path2)` returns a filepath created by joining the given path segments via filepath separator.
  - Type: `(string, string) -> string`
- `split(path)` is a convenience function that returns `dirname(path), basename(path)`.
  - Type: `(string) -> string, string`

### `json`

This library defines the following two JSON utility functions:

- `encode(val)` returns a JSON-ified version of the given Lua object.
- `decode(json)` returns a Lua table representation of the given JSON text.

### `fun`

[Lua Functional](https://github.com/luafun/luafun/tree/cb6a7e25d4b55d9578fd371d1474b00e47bd29f3#lua-functional) is a high-performance functional programming library accessible via `local fun = require("fun")`. This library has a number of functional utilities to help make recognizer code a bit more expressive.
