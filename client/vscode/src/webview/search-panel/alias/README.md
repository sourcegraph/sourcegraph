# Aliased files

Lots of shared client code used in the initial implementation of the VS Code
To expedite the implementation and review processes, we decided to "fork"
code that would require significant refactoring to work in the VS Code
extension context.

Our plan is remove the need for these aliased/forked files (method TBD).
Resolving this divergence will reduce the risk of regressions introduced
by changes in the way that base code interacts with forked code.

- Search result handling
  - We can't use relative links like in the web app, for most search result types (and eventually all),
    we need click handlers that call VS Code extension APIs.
  - Forked components: `FileMatchChildren`, `SearchResult`, `RepoFileLink`
  - What's changed:
    - Create a React context to wrap around `StreamingSearchResultsList` (shared) to pass
      VS Code extension APIs to forked search result components.
    - Change links to buttons, call VS Code file handlers from context on click.
Hello World
