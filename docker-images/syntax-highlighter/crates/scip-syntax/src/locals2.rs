use crate::languages::LocalConfiguration;
use core::cmp::Ordering;
use core::ops::Range;
use id_arena::{Arena, Id};
use itertools::Itertools;
use protobuf::Enum;
use scip::{
    symbol::format_symbol,
    types::{Occurrence, Symbol},
};
use scip_treesitter::prelude::*;
use std::fmt;
use std::slice::Iter;
use tree_sitter::Node;

// What needs to be documented?
//
// 1. How to use the hoisting DSL
// 2. Missing features at this point
//   a) Python's definition vs reference
//   b) Namespacing (Need to figure out what the DSL should be)
//   c) Marking globals to avoid emitting them into occurrences

// What needs to be documented for a PR
//
// 1. Differences to the old implementation (Feature wise)
// 2. Performance characteristics (do some benchmarks)

#[derive(Debug, Clone)]
struct Definition<'a> {
    ty: String,
    node: Node<'a>,
    id: usize,
    text: &'a str,
}

impl fmt::Display for Definition<'_> {
    fn fmt(&self, fmt: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            fmt,
            "def {} {}-{}",
            self.ty,
            self.node.start_position(),
            self.node.end_position()
        )
    }
}

/// We use id_arena to allocate our scopes.
type ScopeRef<'a> = Id<Scope<'a>>;

#[derive(Debug)]
struct Scope<'a> {
    ty: String,
    node: Node<'a>,
    // TODO: (perf) we could also remember how many definitions
    // precede us in the parent, for efficient slicing when searching
    // up the tree
    parent: Option<ScopeRef<'a>>,

    /// Definitions that have been hoisted to the top of this scope
    hoisted_definitions: Vec<Definition<'a>>,
    definitions: Vec<Definition<'a>>,
    children: Vec<ScopeRef<'a>>,
}

impl<'a> Scope<'a> {
    fn new(ty: String, node: Node<'a>, parent: Option<ScopeRef<'a>>) -> Self {
        Scope {
            ty,
            node,
            parent,
            hoisted_definitions: vec![],
            definitions: vec![],
            children: vec![],
        }
    }

    // TODO: Namespacing
    fn find_def(&self, name: &str, start_byte: usize) -> Option<&Definition<'a>> {
        if let Some(def) = self.hoisted_definitions.iter().find(|def| def.text == name) {
            return Some(def);
        };

        // TODO: (perf) Binary search
        if let Some(def) = self.definitions.iter().find(|def| {
            def.text == name &&
            // For non-hoisted definitions we're only looking for
            // definitions that lexically precede the reference
                def.node.start_byte() < start_byte
        }) {
            return Some(def);
        };

        None
    }
}

// We compare ranges in a particular way to ensure a pre-order
// traversal:
// A = 3..9
// B = 10..22
// C = 10..12
// B.cmp(C) = Less
// Because C is contained within B we want to make sure to visit B first.
fn compare_range(a: Range<usize>, b: Range<usize>) -> Ordering {
    let result = (a.start, b.end).cmp(&(b.start, a.end));
    assert!(
        result != Ordering::Equal,
        "Two scopes must never span the exact same range: {a:?}"
    );
    result
}

#[derive(Debug)]
struct CaptureDef<'a> {
    ty: String,
    hoist: Option<String>,
    node: Node<'a>,
}

#[derive(Debug)]
struct LocalResolver<'a> {
    arena: Arena<Scope<'a>>,
    source_bytes: &'a [u8],
    definition_id_supply: usize,
    occurrences: Vec<Occurrence>,
}

impl<'a> LocalResolver<'a> {
    fn new(source_bytes: &'a [u8]) -> Self {
        LocalResolver {
            arena: Arena::new(),
            source_bytes,
            definition_id_supply: 0,
            occurrences: vec![],
        }
    }

    fn start_byte(&self, id: ScopeRef<'a>) -> usize {
        self.arena.get(id).unwrap().node.start_byte()
    }

    fn end_byte(&self, id: ScopeRef<'a>) -> usize {
        self.arena.get(id).unwrap().node.end_byte()
    }

    fn add_definition(
        &mut self,
        id: ScopeRef<'a>,
        definition: Definition<'a>,
        hoist: &Option<String>,
    ) {
        let symbol = format_symbol(Symbol::new_local(definition.id));
        let symbol_roles = scip::types::SymbolRole::Definition.value();

        self.occurrences.push(scip::types::Occurrence {
            range: definition.node.to_scip_range(),
            symbol: symbol.clone(),
            symbol_roles,
            ..Default::default()
        });

        match hoist {
            Some(hoist_scope) => {
                let mut target_scope = id;
                // If we don't find any matching scope we hoist all
                // the way to the top_scope
                for ancestor in self.ancestors(id) {
                    target_scope = ancestor;
                    if self.get_scope(ancestor).ty == *hoist_scope {
                        break;
                    }
                }
                self.get_scope_mut(target_scope)
                    .hoisted_definitions
                    .push(definition)
            }
            None => self.get_scope_mut(id).definitions.push(definition),
        };
    }

    fn get_scope(&self, id: ScopeRef<'a>) -> &Scope<'a> {
        self.arena.get(id).unwrap()
    }

    fn get_scope_mut(&mut self, id: ScopeRef<'a>) -> &mut Scope<'a> {
        self.arena.get_mut(id).unwrap()
    }

    // TODO: This should be an Iterator
    fn ancestors(&self, id: ScopeRef<'a>) -> Vec<ScopeRef<'a>> {
        let mut current = id;
        let mut results = vec![];
        while let Some(next) = self.arena.get(current).unwrap().parent {
            current = next;
            results.push(current)
        }
        results
    }

    fn parent(&self, id: ScopeRef<'a>) -> ScopeRef<'a> {
        self.arena
            .get(id)
            .unwrap()
            .parent
            .expect("Tried to get the root node's parent")
    }

    fn print_scope(&self, id: ScopeRef<'a>, depth: usize) {
        let scope = self.arena.get(id).unwrap();
        println!(
            "{}scope {} {}-{}",
            str::repeat(" ", depth),
            scope.ty,
            scope.node.start_position(),
            scope.node.end_position()
        );

        let mut definitions_iter = scope.definitions.iter().peekable();
        let mut children_iter = scope.children.iter().peekable();
        for definition in scope.hoisted_definitions.iter() {
            println!("{}h:{}", str::repeat(" ", depth + 2), definition);
        }
        loop {
            // TODO: Can we deduplicate this nicely? Pattern Guards
            // are not a thing in Rust
            match (definitions_iter.peek(), children_iter.peek()) {
                (Some(d), Some(c)) => {
                    if d.node.start_byte() < self.start_byte(**c) {
                        let definition = definitions_iter.next().unwrap();
                        println!("{}{}", str::repeat(" ", depth + 2), definition);
                    } else {
                        let child = children_iter.next().unwrap();
                        self.print_scope(*child, depth + 2)
                    }
                }
                (Some(_), None) => {
                    let definition = definitions_iter.next().unwrap();
                    println!("{}{}", str::repeat(" ", depth + 2), definition);
                }
                (None, Some(_)) => {
                    let child = children_iter.next().unwrap();
                    self.print_scope(*child, depth + 2)
                }
                (None, None) => break,
            };
        }
    }

    fn add_defs_while<'b, F>(
        &mut self,
        scope: ScopeRef<'a>,
        definitions_iter: &mut Iter<'b, CaptureDef<'a>>,
        f: F,
    ) where
        F: Fn(&CaptureDef<'a>) -> bool,
        'a: 'b,
    {
        for def_capture in definitions_iter.take_while_ref(|def_capture| f(def_capture)) {
            self.definition_id_supply += 1;
            let definition = Definition {
                id: self.definition_id_supply,
                ty: def_capture.ty.to_string(),
                node: def_capture.node,
                text: def_capture
                    .node
                    .utf8_text(self.source_bytes)
                    .expect("non utf-8 variable name"),
            };
            self.add_definition(scope, definition, &def_capture.hoist)
        }
    }

    fn find_enclosing_scope(&self, top_scope: ScopeRef<'a>, node: Node<'a>) -> ScopeRef<'a> {
        let mut current_scope = top_scope;

        while let Some(next) = self.get_scope(current_scope).children.iter().find(|child| {
            let child_scope = self.get_scope(**child);
            child_scope.node.byte_range().contains(&node.start_byte())
        }) {
            current_scope = *next
        }

        current_scope
    }

    fn collect_captures(
        config: &'a LocalConfiguration,
        tree: &'a tree_sitter::Tree,
        source_bytes: &'a [u8],
    ) -> (
        Vec<(&'a str, Node<'a>)>,
        Vec<CaptureDef<'a>>,
        Vec<(&'a str, Node<'a>)>,
    ) {
        let mut cursor = tree_sitter::QueryCursor::new();
        let capture_names = config.query.capture_names();

        let mut scopes: Vec<(&str, Node<'a>)> = vec![];
        let mut definitions: Vec<CaptureDef> = vec![];
        let mut references: Vec<(&str, Node<'a>)> = vec![];

        for match_ in cursor.matches(&config.query, tree.root_node(), source_bytes) {
            let properties = config.query.property_settings(match_.pattern_index);
            for capture in match_.captures {
                let Some(capture_name) = capture_names.get(capture.index as usize) else {
                    continue;
                };
                if capture_name.starts_with("scope") {
                    let ty = capture_name.strip_prefix("scope.").unwrap_or(capture_name);
                    scopes.push((ty, capture.node))
                } else if capture_name.starts_with("definition") {
                    let mut hoist_scope = None;
                    if let Some(prop) = properties.iter().find(|p| p.key == "hoist".into()) {
                        hoist_scope = Some(prop.value.as_ref().unwrap().to_string());
                    }
                    let ty = capture_name
                        .strip_prefix("definition.")
                        .unwrap_or(capture_name);
                    definitions.push(CaptureDef {
                        ty: ty.to_string(),
                        hoist: hoist_scope,
                        node: capture.node,
                    })
                } else if capture_name.starts_with("reference") {
                    references.push((capture_name, capture.node))
                } else {
                    eprintln!("Discarded capture: {}", capture_name)
                }
            }
        }

        (scopes, definitions, references)
    }

    /// This function is probably the most complicated bit in here.
    /// scopes and definitions are sorted to allow us to build a tree
    /// of scope in pre-traversal order here. We make sure to add all
    /// definitions to their narrowest enclosing scope, or to hoist
    /// them to the closest matching scope.
    fn build_tree(
        &mut self,
        top_scope: ScopeRef<'a>,
        mut scopes: Vec<(&'a str, Node<'a>)>,
        mut definitions: Vec<CaptureDef<'a>>,
    ) {

        // In order to do a pre-order traversal we need to sort scopes and definitions
        // TODO: (perf) Do a pass to check if they're already sorted first?
        scopes.sort_by(|(_, a), (_, b)| compare_range(a.byte_range(), b.byte_range()));
        definitions.sort_by_key(|a| a.node.start_byte());

        let mut definitions_iter = definitions.iter();

        let mut current_scope = top_scope;
        for (scope_ty, scope) in scopes {
            let new_scope_end = scope.end_byte();
            while new_scope_end > self.end_byte(current_scope) {
                // Add all remaining definitions before end of current
                // scope before traversing to parent
                let scope_end_byte = self.end_byte(current_scope);
                self.add_defs_while(current_scope, &mut definitions_iter, |def_capture| {
                    def_capture.node.start_byte() < scope_end_byte
                });

                current_scope = self.parent(current_scope)
            }
            // Before adding the new scope we first attach all
            // definitions that belong to the current scope
            self.add_defs_while(current_scope, &mut definitions_iter, |def_capture| {
                def_capture.node.start_byte() < scope.start_byte()
            });

            let new_scope =
                self.arena
                    .alloc(Scope::new(scope_ty.to_string(), scope, Some(current_scope)));
            self.get_scope_mut(current_scope).children.push(new_scope);
            current_scope = new_scope
        }

        // We need to climb back to the top level scope and add
        // all remaining definitions
        loop {
            let scope_end_byte = self.end_byte(current_scope);
            self.add_defs_while(current_scope, &mut definitions_iter, |def_capture| {
                def_capture.node.start_byte() < scope_end_byte
            });

            if current_scope == top_scope {
                break;
            }

            current_scope = self.parent(current_scope)
        }

        assert!(
            definitions_iter.next().is_none(),
            "Should've entered all definitions into the tree"
        );
    }

    fn resolve_references(
        &mut self,
        top_scope: ScopeRef<'a>,
        references: Vec<(&'a str, Node<'a>)>,
    ) {
        let mut ref_occurrences = vec![];

        // TODO: (perf) Add refs in the pre-order traversal
        for (_ty, node) in references {
            let reference_string = node
                .utf8_text(self.source_bytes)
                .expect("non utf8 reference");
            let mut current_scope = self.find_enclosing_scope(top_scope, node);
            loop {
                let scope = self.get_scope(current_scope);

                // TODO: Need to filter all refs that overlap with definitions
                if let Some(def) = scope.find_def(reference_string, node.start_byte()) {
                    let symbol = format_symbol(Symbol::new_local(def.id));
                    ref_occurrences.push(scip::types::Occurrence {
                        range: node.to_scip_range(),
                        symbol: symbol.clone(),
                        ..Default::default()
                    });
                    break;
                } else if let Some(parent_scope) = scope.parent {
                    current_scope = parent_scope
                } else {
                    break;
                }
            }
        }
        self.occurrences.extend(ref_occurrences);
    }

    // The entry point to locals resolution
    fn process(
        mut self,
        config: &'a LocalConfiguration,
        tree: &'a tree_sitter::Tree,
    ) -> Vec<Occurrence> {
        // First we collect all captures from the tree-sitter locals query
        let (scopes, definitions, references) =
            Self::collect_captures(config, tree, self.source_bytes);

        // Next we build a tree structure of scopes and definitions
        let top_scope = self
            .arena
            .alloc(Scope::new("root".to_string(), tree.root_node(), None));
        self.build_tree(top_scope, scopes, definitions);
        self.print_scope(top_scope, 0); // Just for debugging

        // Finally we resolve all references against that tree structure
        self.resolve_references(top_scope, references);

        self.occurrences
    }
}

pub fn parse_tree<'a>(
    config: &LocalConfiguration,
    tree: &'a tree_sitter::Tree,
    source_bytes: &'a [u8],
) -> Vec<Occurrence> {
    let resolver = LocalResolver::new(source_bytes);
    resolver.process(config, tree)
}

#[cfg(test)]
mod test {
    use scip::types::Document;
    use scip_treesitter::snapshot::{dump_document_with_config, EmitSymbol, SnapshotOptions};
    use scip_treesitter_languages::parsers::BundledParser;

    use super::*;
    use crate::languages::LocalConfiguration;

    fn snapshot_syntax_document(doc: &Document, source: &str) -> String {
        dump_document_with_config(
            doc,
            source,
            SnapshotOptions {
                emit_symbol: EmitSymbol::All,
                ..Default::default()
            },
        )
        .expect("dump document")
    }

    fn parse_file_for_lang(config: &LocalConfiguration, source_code: &str) -> Document {
        let source_bytes = source_code.as_bytes();
        let mut parser = config.get_parser();
        let tree = parser.parse(source_bytes, None).unwrap();

        let occ = parse_tree(config, &tree, source_bytes);
        let mut doc = Document::new();
        doc.occurrences = occ;
        doc.symbols = doc
            .occurrences
            .iter()
            .map(|o| scip::types::SymbolInformation {
                symbol: o.symbol.clone(),
                ..Default::default()
            })
            .collect();

        doc
    }

    #[test]
    fn test_can_do_go() {
        let config = crate::languages::get_local_configuration(BundledParser::Go).unwrap();
        let source_code = include_str!("../testdata/locals.go");
        let doc = parse_file_for_lang(config, source_code);

        let dumped = snapshot_syntax_document(&doc, source_code);
        insta::assert_snapshot!(dumped);
    }

    #[test]
    fn test_can_do_nested_locals() {
        let config = crate::languages::get_local_configuration(BundledParser::Go).unwrap();
        let source_code = include_str!("../testdata/locals-nested.go");
        let doc = parse_file_for_lang(config, source_code);

        let dumped = snapshot_syntax_document(&doc, source_code);
        insta::assert_snapshot!(dumped);
    }

    #[test]
    fn test_can_do_functions(){
        let config = crate::languages::get_local_configuration(BundledParser::Go).unwrap();
        let source_code = include_str!("../testdata/funcs.go");
        let doc = parse_file_for_lang(config, source_code);

        let dumped = snapshot_syntax_document(&doc, source_code);
        insta::assert_snapshot!(dumped);
    }

    #[test]
    fn test_can_do_perl() {
        let config = crate::languages::get_local_configuration(BundledParser::Perl).unwrap();
        let source_code = include_str!("../testdata/perl.pm");
        let doc = parse_file_for_lang(config, source_code);

        let dumped = snapshot_syntax_document(&doc, source_code);
        insta::assert_snapshot!(dumped);
    }

    #[test]
    fn test_can_do_matlab() {
        let config = crate::languages::get_local_configuration(BundledParser::Matlab).unwrap();
        let source_code = include_str!("../testdata/locals.m");
        let doc = parse_file_for_lang(config, source_code);

        let dumped = snapshot_syntax_document(&doc, source_code);
        insta::assert_snapshot!(dumped);
    }

    #[test]
    fn test_can_do_java() {
        let config = crate::languages::get_local_configuration(BundledParser::Java).unwrap();
        let source_code = include_str!("../testdata/locals.java");
        let doc = parse_file_for_lang(config, source_code);

        let dumped = snapshot_syntax_document(&doc, source_code);
        insta::assert_snapshot!(dumped);
    }
}
