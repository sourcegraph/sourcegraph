# LSIF development and testing utilities

## Install

Assumes a working Go installation:

```
# lsif-index-tester
go install github.com/sourcegraph/sourcegraph/lib/codeintel/tools/lsif-index-tester

# lsif-semantic-diff
go install github.com/sourcegraph/sourcegraph/lib/codeintel/tools/lsif-semantic-diff

# lsif-validate
go install github.com/sourcegraph/sourcegraph/lib/codeintel/tools/lsif-validate

# lsif-visualize
go install github.com/sourcegraph/sourcegraph/lib/codeintel/tools/lsif-visualize
```

Resulting binary should then be in your `$GOPATH/bin` (conventionally `$HOME/go/bin`), so make sure thats in your `$PATH` or else invoke using absolute/relative location.

Binary releases coming soon™️

## lsif-index-tester

This command tests the relationships of an LSIF index against a set of known golden relationships.

### Project Setup

You should have a folder full of separate projects. Each project should have an `lsif_tests/` folder inside of it.

For example:

```
$ ls lsif-clang/functionaltests
project_1
project_2
```

Inside of each project, they should have:

- `setup_indexer.sh`

  - Bash script that setups up the project.
  - Can install dependencies, build project, etc.
  - This will be run every time before the indexer is run (so it can be worthwhile to make sure you cache the build if nothing changes)

- `lsif_tests/`
  - Within `lsif_tests/` you can have as many `.json` files as you'd like
  - Each `.json` file is a map that details different tests. See below for different support test tags.

#### Test: `textDocument/definition`

```json
{
  "textDocument/definition": [
    {
      "name": "other_simple",
      "request": {
        "textDocument": "src/uses_header.c",
        "position": {
          "line": 6,
          "character": 8
        }
      },
      "response": {
        "textDocument": "src/uses_header.c",
        "range": {
          "start": { "line": 10, "character": 5 },
          "end": { "line": 10, "character": 22 }
        }
      }
    }
  ]
}
```

- `name`: String
  - The name of the test. Used when printing output
- `request`: `textDocument/request` request.
- `reseponse`: expected `textDocument/response` response.

See: [textDocument definition](https://microsoft.github.io/language-server-protocol/specification#textDocument_definition)

### Project Testing

```
go run . --indexer "lsif-clang compile_commands.json" --dir "/path/to/test_directory/"
```

- `--indexer` is the set of commands to actually run the indexer
- `--dir` is the root directory that contains an `lsif_tests` directory.

## lsif-repl

Documentation coming soon.

## lsif-semantic-diff

Documentation coming soon.

## lsif-validate

This command validates the output of an LSIF indexer. The following properties are validated:

- Element IDs are unique
- All references of element occur after its definition
- A single metadata vertex exists and is the firsts element in the dump
- The project root is a valid URL
- Each document URI is a URL relative to the project root
- Each range vertex has sane bounds (non-negative line/character values and the ending position occurs strictly after the starting position)
- 1-to-n edges have a non-empty `inVs` array
- Edges refer to identifiers attached to the correct element type, as follows:

  | label                     | inV(s)               | outV                | condition                      |
  | ------------------------- | -------------------- | ------------------- | ------------------------------ |
  | `contains`                | `range`              |                     | if outV is a `document`        |
  | `item`                    | `range`              |                     |                                |
  | `item`                    | `referenceResult`    |                     | if outV is a `referenceResult` |
  | `next`                    | `resultSet`          | `range`/`resultSet` |                                |
  | `textDocument/definition` | `definitionResult`   | `range`/`resultSet` |                                |
  | `textDocument/references` | `referenceResult`    | `range`/`resultSet` |                                |
  | `textDocument/hover`      | `hoverResult`        | `range`/`resultSet` |                                |
  | `moniker`                 | `moniker`            | `range`/`resultSet` |                                |
  | `nextMoniker`             | `moniker`            | `moniker`           |                                |
  | `packageInformation`      | `packageInformation` | `moniker`           |                                |

- Each vertex is reachable from a range or document vertex (_ignored: metadata, project, document, and event vertices_)
- Each range belongs to a unique document
- No two ranges belonging to the same document improperly overlap
- The inVs of each `item` edge belong to that document referred to by the edge's `document` field
- Each range and result set has at most one result set attached to it

## lsif-visualize

Documentation coming soon.
Hello World
