# tree-sitter-all-languages

This is the only location that we should load / compile tree-sitter grammars into their languages.

If you're loading a grammar elsewhere, that is a bug. We should only have one, centralized place that
we are building and loading grammars and sync them all on the same tree-sitter versions.

## Adding a language

- Add the tree-sitter grammar as a dependency
- Add a new entry in the `ParserId`
- Fix associated type errors (since there are a few match statements using the enum).
- Add `highlights.scm`, `locals.scm`, and `injections.scm` to the queries folder.
- Enable the highlights in `src/highlight.rs`
- Add a snapshot test
- Done!
Hello World
