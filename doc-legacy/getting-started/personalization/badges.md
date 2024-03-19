# Repository badges (Go open-source projects only)

> NOTE: Right now, this feature is only supported for open-source Go repositories on Sourcegraph.com.

Sourcegraph.com generates badges with usage statistics for open-source Go repositories. The badges shows the total number of Go packages (among all known public repositories) that import the repository's Go packages.

To add a badge to your project's README.md, add this Markdown:

``` markdown
[![Sourcegraph](https://sourcegraph.com/github.com/gorilla/mux/-/badge.svg)](https://sourcegraph.com/github.com/gorilla/mux?badge)
```

(Be sure to replace `github.com/gorilla/mux` with your own repository's name.)

This renders the following badge:

[![Sourcegraph](https://sourcegraph.com/github.com/gorilla/mux/-/badge.svg)](https://sourcegraph.com/github.com/gorilla/mux?badge)

## Known issues

Please report any other issues and feature requests on the [Sourcegraph issue tracker](https://github.com/sourcegraph/sourcegraph/issues).

- The number may be overcounted because it is the sum of counts for all of the repository's subpackages. If another project uses multiple subpackages in this repository, the project is counted multiple times.
- Importers using custom Go import paths (i.e., anything other than import paths prefixed by the repository name, such as `github.com/foo/bar`) will not be counted.
