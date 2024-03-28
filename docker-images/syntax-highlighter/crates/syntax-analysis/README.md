# syntax-analysis

## Locals

[`src/locals.rs`](src/locals.rs) implements an evaluator for our [tree-sitter query] based DSL to label local definitions and references for various programming languages using purely syntactic information.
This data is used to enable fast and lightweight file-local navigation in the blob view.

Queries describing the local binding structure of various programming languages are maintained in `queries/*/scip-locals.scm`.

### Usage example

```rust
use syntax_analysis::languages;
use syntax_analysis::locals;
use scip::types::Document;
use tree_sitter_all_languages::ParserId;

const SOURCE: &[u8] = b"
package main

var y = 4

func my_func(x int) {
  x + y
}";

fn main() {
  let config = languages::get_local_configuration(ParserId::Go).unwrap();
  let tree = config.get_parser().parse(SOURCE, None).unwrap();
  let occurrences = locals::parse_tree(config, &tree, SOURCE);
  let mut document = Document::new();
  document.occurrences = occurrences;
  print!("{:#?}", document);
}
```

### Further documentation
- Read about our [locals query DSL] if you're looking to edit, add to, or just understand our locals queries
- In [locals scoping] we describe how we arrived at the current design of the DSL by analyzing scoping behaviour for a variety of languges

[locals query DSL]: docs/locals-query-dsl.md
[locals scoping]: docs/locals-scoping.md
[tree-sitter query]: https://tree-sitter.github.io/tree-sitter/using-parsers#pattern-matching-with-queries
Hello World
