/// This module contains logic to understand the binding structure of
/// a given source file. We emit information about references and
/// definition of _local_ bindings. A local binding is a binding that
/// cannot be accessed from another file. It is important to never
/// mark a non-local as local, because that would mean we'd prevent
/// search-based lookup from finding references to that binding.
///
/// We implement this in a language-agnostic way by relying on
/// tree-sitter and a DSL built on top of its [query syntax].
///
/// [query syntax]: https://tree-sitter.github.io/tree-sitter/using-parsers#query-syntax
use crate::languages::LocalConfiguration;
use anyhow::Result;
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
use std::collections::HashSet;
use std::fmt;
use std::slice::Iter;
use string_interner::{DefaultSymbol, StringInterner};
use tree_sitter::Node;

// What needs to be documented?
//
// 1. Missing features at this point
//   a) Python's definition vs reference
//   b) Namespacing (Need to figure out what the DSL should be)
//   c) Marking globals to avoid emitting them into occurrences

// What needs to be documented for a PR
//
// 1. Differences to the old implementation (Feature wise)
// 2. Performance characteristics (do some benchmarks)

pub fn parse_tree<'a>(
    config: &LocalConfiguration,
    tree: &'a tree_sitter::Tree,
    source_bytes: &'a [u8],
) -> Result<Vec<Occurrence>> {
    let resolver = LocalResolver::new(source_bytes);
    Ok(resolver.process(config, tree))
}

pub fn parse_tree_test<'a>(
    config: &LocalConfiguration,
    tree: &'a tree_sitter::Tree,
    source_bytes: &'a [u8],
) -> Vec<Occurrence> {
    let resolver = LocalResolver::new(source_bytes);
    resolver.process(config, tree)
}

#[derive(Debug, Clone)]
struct Definition<'a> {
    ty: String,
    node: Node<'a>,
    id: usize,
    name: Name,
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

#[derive(Debug, Clone)]
struct Reference<'a> {
    node: Node<'a>,
    name: Name,
}

/// We use id_arena to allocate our scopes.
type ScopeRef<'a> = Id<Scope<'a>>;

/// We use string_interner to intern variable names
type Name = DefaultSymbol;

#[derive(Debug)]
struct Scope<'a> {
    /// For a query that captures a "@scope.function" this will
    /// contain the string "function"
    ty: String,
    node: Node<'a>,
    // TODO: (perf) we could also remember how many definitions
    // precede us in the parent, for efficient slicing when searching
    // up the tree
    parent: Option<ScopeRef<'a>>,

    /// Definitions that have been hoisted to the top of this scope
    // TODO: (perf) for hoisted definitions the lexicographical order
    // shouldn't matter anymore, so we might want to turn this into a
    // HashMap for faster lookups
    hoisted_definitions: Vec<Definition<'a>>,
    /// Definitions that appear in this scope. Sorted lexicographical
    definitions: Vec<Definition<'a>>,
    /// References that appear in this scope. Sorted lexicographical
    references: Vec<Reference<'a>>,
    /// Scopes that appear nested underneath this scope. Sorted
    /// lexicographically
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
            references: vec![],
            children: vec![],
        }
    }

    // TODO: Namespacing
    fn find_def(&self, name: Name, start_byte: usize) -> Option<&Definition<'a>> {
        if let Some(def) = self.hoisted_definitions.iter().find(|def| def.name == name) {
            return Some(def);
        };

        for definition in self.definitions.iter() {
            // For non-hoisted definitions we're only looking for
            // definitions that lexically precede the reference
            if definition.node.start_byte() > start_byte {
                break;
            }

            if definition.name == name {
                return Some(definition);
            }
        }

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
    // TODO: This assert fails for Java, ideally we'd fix the query to
    // not report duplicate scopes
    // assert!(
    //     result != Ordering::Equal,
    //     "Two scopes must never span the exact same range: {a:?}"
    // );
    result
}

#[derive(Debug)]
struct Captures<'a> {
    scopes: Vec<ScopeCapture<'a>>,
    definitions: Vec<DefCapture<'a>>,
    references: Vec<RefCapture<'a>>,
}

#[derive(Debug)]
struct ScopeCapture<'a> {
    ty: &'a str,
    node: Node<'a>,
}

#[derive(Debug)]
struct DefCapture<'a> {
    ty: String,
    hoist: Option<String>,
    node: Node<'a>,
}

#[derive(Debug)]
struct RefCapture<'a> {
    _ty: String,
    node: Node<'a>,
}

/// Created by LocalResolver::ancestors()
#[derive(Debug)]
struct Ancestors<'arena, 'a> {
    arena: &'arena Arena<Scope<'a>>,
    current_scope: ScopeRef<'a>,
}

impl<'arena, 'a> Iterator for Ancestors<'arena, 'a> {
    type Item = ScopeRef<'a>;
    fn next(&mut self) -> Option<ScopeRef<'a>> {
        let scope = self.arena.get(self.current_scope).unwrap();
        match scope.parent {
            None => return None,
            Some(parent) => {
                self.current_scope = parent;
                return Some(parent);
            }
        }
    }
}

#[derive(Debug)]
struct LocalResolver<'a> {
    arena: Arena<Scope<'a>>,
    interner: StringInterner,

    source_bytes: &'a [u8],
    definition_id_supply: usize,
    // TODO: This is a hack to not record references that overlap with
    // definitions. We should either fix our queries so this doesn't
    // happen, or do it in a more performant manner.
    definition_start_bytes: HashSet<usize>,
    occurrences: Vec<Occurrence>,
}

impl<'a> LocalResolver<'a> {
    fn new(source_bytes: &'a [u8]) -> Self {
        LocalResolver {
            arena: Arena::new(),
            interner: StringInterner::default(),
            source_bytes,
            definition_id_supply: 0,
            definition_start_bytes: HashSet::new(),
            occurrences: vec![],
        }
    }

    fn _start_byte(&self, id: ScopeRef<'a>) -> usize {
        self.get_scope(id).node.start_byte()
    }

    fn end_byte(&self, id: ScopeRef<'a>) -> usize {
        self.get_scope(id).node.end_byte()
    }

    fn add_reference(&mut self, id: ScopeRef<'a>, reference: Reference<'a>) {
        self.get_scope_mut(id).references.push(reference)
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

        self.definition_start_bytes
            .insert(definition.node.start_byte());

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

    fn ancestors(&self, id: ScopeRef<'a>) -> Ancestors<'_, 'a> {
        Ancestors {
            arena: &self.arena,
            current_scope: id,
        }
    }

    fn parent(&self, id: ScopeRef<'a>) -> ScopeRef<'a> {
        self.get_scope(id)
            .parent
            .expect("Tried to get the root node's parent")
    }

    fn _print_scope(&self, id: ScopeRef<'a>, depth: usize) {
        let scope = self.get_scope(id);
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
                    if d.node.start_byte() < self._start_byte(**c) {
                        let definition = definitions_iter.next().unwrap();
                        println!("{}{}", str::repeat(" ", depth + 2), definition);
                    } else {
                        let child = children_iter.next().unwrap();
                        self._print_scope(*child, depth + 2)
                    }
                }
                (Some(_), None) => {
                    let definition = definitions_iter.next().unwrap();
                    println!("{}{}", str::repeat(" ", depth + 2), definition);
                }
                (None, Some(_)) => {
                    let child = children_iter.next().unwrap();
                    self._print_scope(*child, depth + 2)
                }
                (None, None) => break,
            };
        }
    }

    fn mk_name(&mut self, s: &str) -> Name {
        self.interner.get_or_intern(s)
    }

    fn add_refs_while<'b, F>(
        &mut self,
        scope: ScopeRef<'a>,
        references_iter: &mut Iter<'b, RefCapture<'a>>,
        f: F,
    ) where
        F: Fn(&RefCapture<'a>) -> bool,
        'a: 'b,
    {
        for ref_capture in references_iter.take_while_ref(|ref_capture| f(ref_capture)) {
            let name = self.mk_name(
                ref_capture
                    .node
                    .utf8_text(self.source_bytes)
                    .expect("non utf-8 variable name"),
            );
            let reference = Reference {
                node: ref_capture.node,
                name,
            };
            self.add_reference(scope, reference)
        }
    }

    fn add_defs_while<'b, F>(
        &mut self,
        scope: ScopeRef<'a>,
        definitions_iter: &mut Iter<'b, DefCapture<'a>>,
        f: F,
    ) where
        F: Fn(&DefCapture<'a>) -> bool,
        'a: 'b,
    {
        for def_capture in definitions_iter.take_while_ref(|def_capture| f(def_capture)) {
            self.definition_id_supply += 1;
            let name = self.mk_name(
                def_capture
                    .node
                    .utf8_text(self.source_bytes)
                    .expect("non utf-8 variable name"),
            );
            let definition = Definition {
                id: self.definition_id_supply,
                ty: def_capture.ty.to_string(),
                node: def_capture.node,
                name,
            };
            self.add_definition(scope, definition, &def_capture.hoist)
        }
    }

    fn collect_captures(
        config: &'a LocalConfiguration,
        tree: &'a tree_sitter::Tree,
        source_bytes: &'a [u8],
    ) -> Captures<'a> {
        let mut cursor = tree_sitter::QueryCursor::new();
        let capture_names = config.query.capture_names();

        let mut scopes: Vec<ScopeCapture> = vec![];
        let mut definitions: Vec<DefCapture> = vec![];
        let mut references: Vec<RefCapture<'a>> = vec![];

        for match_ in cursor.matches(&config.query, tree.root_node(), source_bytes) {
            let properties = config.query.property_settings(match_.pattern_index);
            for capture in match_.captures {
                let Some(capture_name) = capture_names.get(capture.index as usize) else {
                    continue;
                };
                if capture_name.starts_with("scope") {
                    let ty = capture_name.strip_prefix("scope.").unwrap_or(capture_name);
                    scopes.push(ScopeCapture {
                        ty,
                        node: capture.node,
                    })
                } else if capture_name.starts_with("definition") {
                    let mut hoist_scope = None;
                    if let Some(prop) = properties.iter().find(|p| p.key == "hoist".into()) {
                        hoist_scope = Some(prop.value.as_ref().unwrap().to_string());
                    }
                    let ty = capture_name
                        .strip_prefix("definition.")
                        .unwrap_or(capture_name);
                    definitions.push(DefCapture {
                        ty: ty.to_string(),
                        hoist: hoist_scope,
                        node: capture.node,
                    })
                } else if capture_name.starts_with("reference") {
                    let ty = capture_name
                        .strip_prefix("reference.")
                        .unwrap_or(capture_name);
                    references.push(RefCapture {
                        _ty: ty.to_string(),
                        node: capture.node,
                    })
                } else {
                    eprintln!("Discarded capture: {}", capture_name)
                }
            }
        }

        Captures {
            scopes,
            definitions,
            references,
        }
    }

    /// This function is probably the most complicated bit in here.
    /// scopes, definitions, and references are sorted to allow us to
    /// build a tree of scope in pre-traversal order here. We make
    /// sure to add all definitions and references to their narrowest
    /// enclosing scope, or to hoist them to the closest matching
    /// scope.
    fn build_tree(&mut self, top_scope: ScopeRef<'a>, captures: Captures<'a>) {
        let Captures {
            mut scopes,
            mut definitions,
            mut references,
        } = captures;
        // In order to do a pre-order traversal we need to sort scopes and definitions
        // TODO: (perf) Do a pass to check if they're already sorted first?
        scopes.sort_by(|a, b| compare_range(a.node.byte_range(), b.node.byte_range()));
        definitions.sort_by_key(|a| a.node.start_byte());
        references.sort_by_key(|a| a.node.start_byte());

        let mut definitions_iter = definitions.iter();
        let mut references_iter = references.iter();

        let mut current_scope = top_scope;
        for ScopeCapture {
            ty: scope_ty,
            node: scope,
        } in scopes
        {
            let new_scope_end = scope.end_byte();
            while new_scope_end > self.end_byte(current_scope) {
                // Add all remaining definitions before end of current
                // scope before traversing to parent
                let scope_end_byte = self.end_byte(current_scope);
                self.add_defs_while(current_scope, &mut definitions_iter, |def_capture| {
                    def_capture.node.start_byte() < scope_end_byte
                });
                self.add_refs_while(current_scope, &mut references_iter, |ref_capture| {
                    ref_capture.node.start_byte() < scope_end_byte
                });

                current_scope = self.parent(current_scope)
            }
            // Before adding the new scope we first attach all
            // definitions that belong to the current scope
            self.add_defs_while(current_scope, &mut definitions_iter, |def_capture| {
                def_capture.node.start_byte() < scope.start_byte()
            });
            self.add_refs_while(current_scope, &mut references_iter, |ref_capture| {
                ref_capture.node.start_byte() < scope.start_byte()
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
            self.add_refs_while(current_scope, &mut references_iter, |ref_capture| {
                ref_capture.node.start_byte() < scope_end_byte
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

    fn resolve_references(&mut self) {
        let mut ref_occurrences = vec![];

        for (scope_ref, scope) in self.arena.iter() {
            for Reference { name, node } in scope.references.iter() {
                // See the comment on LocalResolver.definition_start_bytes
                if self.definition_start_bytes.contains(&node.start_byte()) {
                    continue;
                }

                let mut current_scope = scope_ref;
                loop {
                    let scope = self.get_scope(current_scope);

                    if let Some(def) = scope.find_def(*name, node.start_byte()) {
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
        let captures = Self::collect_captures(config, tree, self.source_bytes);

        // Next we build a tree structure of scopes and definitions
        let top_scope = self
            .arena
            .alloc(Scope::new("global".to_string(), tree.root_node(), None));
        self.build_tree(top_scope, captures);
        // TODO: Maybe write a couple snapshot tests that assert on
        // the structure of this tree?
        // self.print_scope(top_scope, 0); // Just for debugging

        // Finally we resolve all references against that tree structure
        self.resolve_references();

        self.occurrences
    }
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

        let occ = parse_tree_test(config, &tree, source_bytes);
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
    fn test_can_do_functions() {
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
