# Switching from Oracle OpenGrok to Sourcegraph

> NOTE: This guide helps Sourcegraph users switch from using Oracle OpenGrok's search syntax to Sourcegraph's. See our [Oracle OpenGrok admin migration guide](../../../admin/migration/opengrok.md) for instructions on switching from an OpenGrok deployment to Sourcegraph.

## Related pages

- [All Sourcegraph search documentation](../index.md)
  - [Sourcegraph query syntax](../reference/queries.md)
- [OpenGrok to Sourcegraph admin migration guide](../../../admin/migration/opengrok.md)

## Switching to Sourcegraph's query syntax

### Wildcards vs. regular expressions

Oracle OpenGrok provides wildcard support for searches. For example, to find all strings beginning with `foo`, you can use the wildcard search `foo*`. Similarly, OpenGrok provides the `?` operator for single character wildcards.

Sourcegraph, [provides full regular expression search](../reference/queries.md#regexp-search), with support for the [RE2 syntax](https://golang.org/s/re2syntax). The same search above would take the form `foo.*` (or in this case, just `foo`, since Sourcegraph automatically supports partial matches). Much more powerful regexp expressions are available.

(Note that Sourcegraph also provides a [literal search mode](../reference/queries.md#Literal-search-default) by default, in which there's no need to escape special characters. This simplifies searches such as `foo(`, which would result in an error in regexp mode.)

### Selecting repositories and branches

Oracle OpenGrok provides a multi-select dropdown box to allow users to select which repositories to include in a search. This scope is stored across sessions, until the user changes it.

Sourcegraph provides a search keyword (`repo:`) that supports regexp and partial matches for selecting repositories. As examples:

- To search for the string "pattern" in all repositories in the github.com/org org, search for `pattern repo:github.com/org`.
- To search in a distinct list of repositories, you can use a `|` character as a regexp OR operator: `pattern repo:github.com/org/repository1|github.com/org/repository2`.
  - Note this query could be simplified further using more advanced regexp matching if the two repos share part of their names, such as: `pattern repo:github.com/org/repository(1|2)`.

Sourcegraph also allows site admins to create pre-defined repository groupings, using [version contexts](../explanations/features.md#version-contexts-experimental).

### Searching in non-master (unindexed) branches, tags, and commits

Oracle OpenGrok can only search in the version of code that is stored on disk. To search across multiple revisions or branches, an OpenGrok administrator must explicitly add each of those copies of the code to OpenGrok.

Sourcegraph allows users to search on any Git revision, even if it is not indexed. Users can append `@<git rev>` to the end of any `repo:` keyword to specify which version to search. For example, to search on `feature-branch`, use `pattern repo:github.com/org/repo@feature-branch`, and to search on a commit `abc123`, use `pattern repo:github.com/org/repo@abc123`.

Sourcegraph also provides the ability to search on multiple Git revisions in a single repository at once, using `:` characters to separate revision names in the `repo:` field. For example, search both `feature-branch` and `abc123` in a single query using `pattern repo:github.com/org/repo@feature-branch:abc123`.

### Special characters

Oracle OpenGrok doesn't index most single-character strings (such as for special characters like `{`, `}`, `[`, `]`, `+`, `-`, and more), and non-alpha-numeric characters generally.

Sourcegraph indexes all characters, and can search for strings of any length. Using the default [literal search mode](../reference/queries.md#Literal-search-default), any search (including those with special characters like `foo.bar`, `try {`, `i++`, `i-=1`, `foo->bar`, and more), will all be searchable without special handling. Using [regexp mode](../reference/queries.md#regexp-search) would require escaping special charactes.

The only exceptions are colon characters, which are by default used for specifying a [search keyword](#search-keywords) on Sourcegraph. Any search containing colons can be done using the `content:` keyword (for example, `content:"foo::bar"`) to explicitly mark it as the search string.

### Boolean operators

Oracle OpenGrok provides three boolean operators — `AND`, `OR`, and `NOT` — for scoping searches to files that contain strings that match multiple patterns.

Sourcegraph also provides [`AND`, `OR`, and `NOT` operators](../reference/queries.md#boolean-operators).

> NOTE: Operators are available as of Sourcegraph 3.17

### Search keywords

Both Sourcegraph and OpenGrok allow users to add keywords for scoping searches. Below is a mapping from OpenGrok syntax to Sourcegraph.

|                                                          | OpenGrok                                  | Sourcegraph                                                                                          |
|----------------------------------------------------------|-------------------------------------------|------------------------------------------------------------------------------------------------------|
| Search text                                              | `full:pattern`                            | `pattern`                                                                                            |
| Search symbol definitions                                | `def:pattern`                             | `pattern type:symbol`                                                                                |
| Search symbol references                                 | `def:pattern`                             | Available through hover tooltips and `Find references` panels on code pages                          |
| Search for repository names                              | Not supported                             | `pattern type:repo`                                                                                  |
| Search for file names                                    | `file:pattern`                            | `file:pattern`                                                                                       |
| Search commit messages                                   | `hist:pattern`                            | `pattern type:commit`                                                                                |
| Search code changes (diff search)                        | Not supported                             | `pattern type:diff`                                                                                  |
| Scope searches to a language                             | `pattern type:c`                          | `pattern lang:c`                                                                                     |
| Case sensitivity                                         | Not supported                             | `case:yes` or `case:no`                                                                              |
| Scope searches to forked repositories                    | Supported through the repository selector | `fork:yes`, `fork:no`, or `fork:only`                                                                |
| Scope searches to archived repositories                  | Supported through the repository selector | `archived:yes`, `archived:no`, or `archived:only`                                                    |
| Scope searches to repositories that contain a file       | Not supported                             | `pattern repo:contains.file(README)`                                                                 |
| Scope searches to repositories that contain file content | Not supported                             | `pattern repo:contains.content(TODO)`                                                                |
| Scope searches to recently updated repositories          | Not supported                             | `pattern repo:contains.commit.after(3 months)` or `pattern repo:contains.commit.after(june 25 2017)` |

Sourcegraph also provides keywords to [scope commit message and diff searches](../reference/queries.md#keywords-diff-and-commit-searches-only) to specific authors or timeframes in which a change was made.

To see an exhaustive list of Sourcegraph's search keywords, see the [search query syntax](../reference/queries.md#keywords-all-searches) page.
