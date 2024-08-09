use core::{cmp::Ordering, ops::Range};
use std::{
    collections::{HashMap, HashSet},
    fmt::Write,
    ops::{Index, IndexMut},
    slice::Iter,
};

use anyhow::{Context, Result};
use id_arena::{Arena, Id};
use if_chain::if_chain;
use itertools::Itertools;
use protobuf::Enum;
use scip::{
    symbol::format_symbol,
    types::{descriptor, Occurrence, Symbol},
};
use string_interner::{symbol::SymbolU32, StringInterner};
use tree_sitter::Node;

use crate::tree_sitter_ext::NodeExt;
/// This module contains logic to understand the binding structure of
/// a given source file. We emit information about references and
/// definitions of _local_ bindings as well as references to non-locals.
/// A local binding is a binding that cannot be accessed from another file.
/// It is important to never mark a non-local as local, because that would mean
/// we'd prevent search-based lookup from finding references to that binding.
///
/// We implement this in a language-agnostic way by relying on
/// tree-sitter and a DSL built on top of its [query syntax].
///
/// [query syntax]: https://tree-sitter.github.io/tree-sitter/using-parsers#query-syntax
use crate::{languages::LocalConfiguration, SCIP_SYNTAX_SCHEME};

// Missing features at this point
// a) Namespacing
//
// The simplest thing I can think of right now is to use
// `@definition.namespace` and `@reference.namespace`. Because
// most of the locals queries just declare all `(identifier)` as
// references we'll probably make it so `@reference` with no
// namespace matches definitions in any namespace and
// `@definition` matches any `@reference.namespace`

#[derive(Clone, Debug, Copy, PartialEq)]
enum ReferenceDescriptor {
    Method,
    Type,
    Term,
    Namespace,
}

impl ReferenceDescriptor {
    fn from_str(str: &str) -> Option<ReferenceDescriptor> {
        match str {
            "method" => Some(ReferenceDescriptor::Method),
            "type" => Some(ReferenceDescriptor::Type),
            "term" => Some(ReferenceDescriptor::Term),
            "namespace" => Some(ReferenceDescriptor::Namespace),
            _ => None,
        }
    }
}

#[derive(Clone, Debug, Copy, PartialEq)]
enum Visibility {
    /// Local references are resolved to a definition within the locals scope tree
    /// and if none is found a global reference is emitted
    Local,

    /// Global references bypass the local scope resolution, and always emit a
    /// a global reference, with the symbol being the node's text content.
    Global,
}

pub fn find_locals(
    config: &LocalConfiguration,
    tree: &tree_sitter::Tree,
    source: &str,
    options: LocalResolutionOptions,
) -> Result<Vec<Occurrence>> {
    let resolver = LocalResolver::new(source, tree.root_node(), options);
    resolver.process(config, tree, None)
}

#[derive(Debug, Clone)]
struct Definition<'a> {
    id: DefId,
    node: Node<'a>,
    name: Name,
}

#[derive(Debug, Clone)]
struct Reference<'a> {
    node: Node<'a>,
    name: Name,
    visibility: Visibility,
    descriptor: Option<ReferenceDescriptor>,
    /// When dealing with def_refs there are references that we've
    /// already resolved to their definitions. Because we don't want
    /// to duplicate that work we store the definition's id here.
    resolves_to: Option<DefId>,
}

/// We use id_arena to allocate our scopes.
type ScopeId<'a> = Id<Scope<'a>>;

/// We use string_interner to intern variable names
type Name = SymbolU32;

/// The id's we create to reference definitions
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
struct DefId(u32);

impl DefId {
    pub fn as_local_symbol(&self) -> Symbol {
        Symbol::new_local(self.0 as usize)
    }
}

#[derive(Debug)]
struct Scope<'a> {
    /// For a query that captures a "@scope.function" this will
    /// contain the string "function"
    kind: Name,
    node: Node<'a>,
    // TODO: (perf) we could also remember how many definitions
    // precede us in the parent, for efficient slicing when searching
    // up the tree
    parent: Option<ScopeId<'a>>,
    /// Scopes that appear nested underneath this scope. Sorted
    /// lexicographically
    children: Vec<ScopeId<'a>>,

    /// Definitions that have been hoisted to the top of this scope
    hoisted_definitions: HashMap<Name, Definition<'a>>,
    /// Definitions that appear in this scope. Sorted lexicographical
    definitions: Vec<Definition<'a>>,
    /// References that appear in this scope. Sorted lexicographically
    references: Vec<Reference<'a>>,
}

impl<'a> Scope<'a> {
    fn new(kind: Name, node: Node<'a>, parent: Option<ScopeId<'a>>) -> Self {
        Scope {
            kind,
            node,
            parent,
            hoisted_definitions: HashMap::new(),
            definitions: vec![],
            references: vec![],
            children: vec![],
        }
    }

    // TODO: Namespacing
    fn find_def(&self, name: Name, start_byte: usize) -> Option<&Definition<'a>> {
        if let Some(def) = self.hoisted_definitions.get(&name) {
            return Some(def);
        };
        // For non-hoisted definitions we're only looking for definitions that lexically
        // precede the reference
        let mut preceding_defs = match self
            .definitions
            .binary_search_by_key(&start_byte, |def| def.node.start_byte())
        {
            Ok(ix) => self.definitions[..=ix].iter(),
            Err(ix) => self.definitions[..ix].iter(),
        };
        preceding_defs.find(|definition| definition.name == name)
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
    debug_assert!(
        result != Ordering::Equal,
        "Two scopes must never span the exact same range: {a:?}",
    );
    result
}

/// Before building the scope tree and resolving references, we first
/// run the tree-sitter query and extract all capture from all matches
/// into this structure
#[derive(Debug)]
struct Captures<'a> {
    scopes: Vec<ScopeCapture<'a>>,
    definitions: Vec<DefCapture<'a>>,
    references: Vec<RefCapture<'a>>,
}

#[derive(Debug)]
struct ScopeCapture<'a> {
    kind: &'a str,
    node: Node<'a>,
}

#[derive(Debug)]
struct DefCapture<'a> {
    hoist: Option<Name>,
    is_def_ref: bool,
    node: Node<'a>,
}

#[derive(Debug)]
struct RefCapture<'a> {
    node: Node<'a>,
    visibility: Visibility,
    descriptor: Option<ReferenceDescriptor>,
}

/// The maximum number of parent scopes we traverse before giving up to
/// prevent infinite loops
const MAX_SCOPE_DEPTH: i32 = 10000;

/// Created by LocalResolver::ancestors()
#[derive(Debug, Clone, Copy)]
struct Ancestors<'arena, 'a> {
    fuel: i32,
    /// A reference to LocalResolver's arena, which holds all scopes
    /// for the entire tree
    arena: &'arena Arena<Scope<'a>>,
    next_scope: Option<ScopeId<'a>>,
}

impl<'arena, 'a> Iterator for Ancestors<'arena, 'a> {
    type Item = ScopeId<'a>;
    fn next(&mut self) -> Option<ScopeId<'a>> {
        match self.next_scope {
            None => None,
            Some(current_scope) => {
                self.fuel -= 1;
                if self.fuel < 0 {
                    eprintln!(
                        "Detected a likely infinite loop in syntax_analysis::locals::LocalResolver"
                    );
                    return None;
                }
                self.next_scope = self.arena.get(current_scope).unwrap().parent;
                Some(current_scope)
            }
        }
    }
}

#[derive(Debug, Clone, Copy)]
enum DefRef {
    PreviousDefinition(DefId),
    NewDefinition(DefId),
}

#[derive(Debug, Clone, Copy)]
pub struct LocalResolutionOptions {
    pub emit_global_references: bool,
}

impl Default for LocalResolutionOptions {
    fn default() -> Self {
        LocalResolutionOptions {
            emit_global_references: true,
        }
    }
}

#[derive(Debug)]
struct LocalResolver<'a> {
    options: LocalResolutionOptions,
    arena: Arena<Scope<'a>>,
    interner: StringInterner,

    source_bytes: &'a [u8],
    definition_id_supply: u32,
    top_scope: ScopeId<'a>,
    occurrences: Vec<Occurrence>,
}

impl<'a> Index<ScopeId<'a>> for LocalResolver<'a> {
    type Output = Scope<'a>;

    fn index(&self, index: ScopeId<'a>) -> &Scope<'a> {
        self.arena.get(index).unwrap()
    }
}

impl<'a> IndexMut<ScopeId<'a>> for LocalResolver<'a> {
    fn index_mut(&mut self, index: ScopeId<'a>) -> &mut Scope<'a> {
        self.arena.get_mut(index).unwrap()
    }
}

impl<'a> LocalResolver<'a> {
    fn new(source: &'a str, root: Node<'a>, options: LocalResolutionOptions) -> Self {
        let mut arena = Arena::new();
        let mut interner = StringInterner::default();
        let top_scope = arena.alloc(Scope::new(
            interner.get_or_intern_static("global"),
            root,
            None,
        ));

        LocalResolver {
            options,
            arena,
            interner,
            source_bytes: source.as_bytes(),
            definition_id_supply: 0,
            top_scope,
            occurrences: vec![],
        }
    }

    fn start_byte(&self, scope_id: ScopeId<'a>) -> usize {
        self[scope_id].node.start_byte()
    }

    fn end_byte(&self, scope_id: ScopeId<'a>) -> usize {
        self[scope_id].node.end_byte()
    }

    fn add_reference(&mut self, scope_id: ScopeId<'a>, reference: Reference<'a>) {
        self[scope_id].references.push(reference)
    }

    fn add_definition(
        &mut self,
        scope_id: ScopeId<'a>,
        name: Name,
        node: Node<'a>,
        hoist: Option<Name>,
        is_def_ref: bool,
    ) {
        // We delay creation of this definition behind a closure, so
        // that we don't generate fresh definition_id's for def_ref's
        // that turn out to be references rather than definitions
        let make_def = |this: &mut Self| {
            this.definition_id_supply += 1;
            let def_id = DefId(this.definition_id_supply);
            let definition = Definition {
                id: def_id,
                name,
                node,
            };
            (def_id, definition)
        };

        let is_new_definition = match hoist {
            Some(hoist_scope) => {
                // If we don't find any matching scope we hoist all
                // the way to the top_scope
                let target_scope = self
                    .ancestors(scope_id)
                    .find(|scope| self[*scope].kind == hoist_scope)
                    .unwrap_or(self.top_scope);
                // Remove me once let-chains are stabilized
                // (https://github.com/rust-lang/rust/issues/53667)
                if_chain! {
                    if is_def_ref;
                    if let Some(previous) = self[target_scope]
                        .hoisted_definitions
                        .get(&name);
                    then {
                        DefRef::PreviousDefinition(previous.id)
                    } else {
                        let (def_id, definition) = make_def(self);
                        self[target_scope].hoisted_definitions.insert(definition.name, definition);
                        DefRef::NewDefinition(def_id)
                    }
                }
            }
            None => {
                if_chain! {
                    if is_def_ref;
                    if let Some(previous) = self.find_def(scope_id, name, node.start_byte());
                    then {
                        DefRef::PreviousDefinition(previous.id)
                    } else {
                        let (def_id, definition) = make_def(self);
                        self[scope_id].definitions.push(definition);
                        DefRef::NewDefinition(def_id)
                    }
                }
            }
        };

        match is_new_definition {
            DefRef::NewDefinition(definition_id) => {
                self.occurrences.push(scip::types::Occurrence {
                    range: node.scip_range(),
                    symbol: format_symbol(definition_id.as_local_symbol()),
                    symbol_roles: scip::types::SymbolRole::Definition.value(),
                    ..Default::default()
                });
            }
            DefRef::PreviousDefinition(definition_id) => {
                self[scope_id].references.push(Reference {
                    name,
                    node,
                    resolves_to: Some(definition_id),
                    visibility: Visibility::Local,
                    descriptor: None,
                })
            }
        };
    }

    fn ancestors(&self, scope_id: ScopeId<'a>) -> Ancestors<'_, 'a> {
        Ancestors {
            fuel: MAX_SCOPE_DEPTH,
            arena: &self.arena,
            next_scope: self[scope_id].parent,
        }
    }

    fn ancestors_with_self(&self, scope_id: ScopeId<'a>) -> Ancestors<'_, 'a> {
        Ancestors {
            fuel: MAX_SCOPE_DEPTH,
            arena: &self.arena,
            next_scope: Some(scope_id),
        }
    }

    fn print_scope(&self, w: &mut dyn Write, scope_id: ScopeId<'a>, depth: usize) {
        let scope = &self[scope_id];
        writeln!(
            w,
            "{}scope {} {}-{}",
            str::repeat(" ", depth),
            self.interner.resolve(scope.kind).unwrap(),
            scope.node.start_position(),
            scope.node.end_position()
        )
        .unwrap();

        let mut definitions_iter = scope.definitions.iter().peekable();
        let mut references_iter = scope.references.iter().peekable();
        let mut children_iter = scope.children.iter().peekable();

        fn is_before(v1: Option<usize>, v2: Option<usize>) -> bool {
            match (v1, v2) {
                (Some(v1), Some(v2)) => v1 <= v2,
                (None, _) => false,
                (_, None) => true,
            }
        }

        // Hoisted definitions always get printed first
        let mut hoisted_defs: Vec<&Definition<'a>> = scope.hoisted_definitions.values().collect();
        hoisted_defs.sort_by_key(|def| def.node.start_byte());
        for definition in hoisted_defs {
            writeln!(
                w,
                "{}hoisted_def {} {}-{}",
                str::repeat(" ", depth + 2),
                self.interner.resolve(definition.name).unwrap(),
                definition.node.start_position(),
                definition.node.end_position()
            )
            .unwrap();
        }
        loop {
            let next_def = definitions_iter.peek().map(|d| d.node.start_byte());
            let next_ref = references_iter.peek().map(|r| r.node.start_byte());
            let next_scope = children_iter.peek().map(|s| self.start_byte(**s));

            if next_def.is_none() && next_ref.is_none() && next_scope.is_none() {
                break;
            }

            if is_before(next_def, next_ref) {
                if is_before(next_def, next_scope) {
                    let definition = definitions_iter.next().unwrap();
                    writeln!(
                        w,
                        "{}def {} {}-{}",
                        str::repeat(" ", depth + 2),
                        self.interner.resolve(definition.name).unwrap(),
                        definition.node.start_position(),
                        definition.node.end_position()
                    )
                    .unwrap();
                    continue;
                }
            } else if is_before(next_ref, next_scope) {
                let reference = references_iter.next().unwrap();
                writeln!(
                    w,
                    "{}ref {} {}-{}",
                    str::repeat(" ", depth + 2),
                    self.interner.resolve(reference.name).unwrap(),
                    reference.node.start_position(),
                    reference.node.end_position()
                )
                .unwrap();
                continue;
            }
            let child = children_iter.next().unwrap();
            self.print_scope(w, *child, depth + 2)
        }
    }

    fn make_name(&mut self, s: &str) -> Name {
        self.interner.get_or_intern(s)
    }

    fn add_refs_while<'b, F>(
        &mut self,
        scope: ScopeId<'a>,
        references_iter: &mut Iter<'b, RefCapture<'a>>,
        f: F,
    ) -> Result<()>
    where
        F: Fn(&RefCapture<'a>) -> bool,
        'a: 'b,
    {
        for ref_capture in references_iter.take_while_ref(|ref_capture| f(ref_capture)) {
            let name = self.make_name(
                ref_capture
                    .node
                    .utf8_text(self.source_bytes)
                    .context("Unexpected non-utf-8 content. This is a tree-sitter bug")?,
            );
            let reference = Reference {
                node: ref_capture.node,
                name,
                resolves_to: None,
                visibility: ref_capture.visibility,
                descriptor: ref_capture.descriptor,
            };
            self.add_reference(scope, reference)
        }
        Ok(())
    }

    fn add_defs_while<'b, F>(
        &mut self,
        scope: ScopeId<'a>,
        definitions_iter: &mut Iter<'b, DefCapture<'a>>,
        f: F,
    ) -> Result<()>
    where
        F: Fn(&DefCapture<'a>) -> bool,
        'a: 'b,
    {
        for def_capture in definitions_iter.take_while_ref(|def_capture| f(def_capture)) {
            let name = self.make_name(
                def_capture
                    .node
                    .utf8_text(self.source_bytes)
                    .context("Unexpected non-utf-8 content. This is a tree-sitter bug")?,
            );
            self.add_definition(
                scope,
                name,
                def_capture.node,
                def_capture.hoist,
                def_capture.is_def_ref,
            )
        }
        Ok(())
    }

    fn collect_captures(
        &mut self,
        config: &'a LocalConfiguration,
        tree: &'a tree_sitter::Tree,
        source_bytes: &'a [u8],
    ) -> Captures<'a> {
        let mut cursor = tree_sitter::QueryCursor::new();
        let capture_names = config.query.capture_names();

        let mut scopes: Vec<ScopeCapture> = vec![];
        let mut definitions: Vec<DefCapture> = vec![];
        let mut references: Vec<RefCapture<'a>> = vec![];

        let mut skip_occurrences_at_offsets: HashSet<usize> = HashSet::new();

        for match_ in cursor.matches(&config.query, tree.root_node(), source_bytes) {
            let properties = config.query.property_settings(match_.pattern_index);
            for capture in match_.captures {
                let offset = capture.node.start_byte();
                if skip_occurrences_at_offsets.contains(&offset) {
                    continue;
                }

                let Some(capture_name) = capture_names.get(capture.index as usize) else {
                    continue;
                };
                let capture_name = *capture_name;
                if capture_name.starts_with("scope") {
                    let kind = capture_name.strip_prefix("scope.").unwrap_or(capture_name);
                    scopes.push(ScopeCapture {
                        kind,
                        node: capture.node,
                    })
                } else if capture_name.starts_with("definition") {
                    let is_def_ref = properties.iter().any(|p| p.key.as_ref() == "def_ref");
                    let mut hoist = None;
                    if let Some(prop) = properties.iter().find(|p| p.key.as_ref() == "hoist") {
                        if let Some(hoist_target) = prop.value.as_ref() {
                            hoist = Some(self.interner.get_or_intern(hoist_target));
                        } else {
                            debug_assert!(false, "hoist _must_ be targeting a scope level");
                        }
                    }
                    definitions.push(DefCapture {
                        hoist,
                        is_def_ref,
                        node: capture.node,
                    })
                } else if capture_name.starts_with("reference") {
                    // Skip duplicate matches for the same reference
                    if let Some(last) = references.last() {
                        if last.node.start_byte() == offset {
                            continue;
                        }
                    }
                    // Skip references at locations we've already recorded a definition
                    if let Some(last) = definitions.last() {
                        if last.node.start_byte() == offset {
                            continue;
                        }
                    }

                    let kind_property = properties
                        .iter()
                        .find(|p| p.key.as_ref() == "kind")
                        .and_then(|p| p.value.as_deref());

                    let (visibility, descriptor) = match kind_property {
                        None => (Visibility::Local, None),
                        Some(input) => {
                            if let Some(input) = input.strip_prefix("global") {
                                (
                                    Visibility::Global,
                                    ReferenceDescriptor::from_str(input.trim_start_matches('.')),
                                )
                            } else {
                                (Visibility::Local, ReferenceDescriptor::from_str(input))
                            }
                        }
                    };

                    references.push(RefCapture {
                        node: capture.node,
                        visibility,
                        descriptor,
                    });
                } else if capture_name == "occurrence.skip" {
                    let offset = capture.node.start_byte();
                    skip_occurrences_at_offsets.insert(offset);
                } else {
                    debug_assert!(false, "Discarded capture: {capture_name}")
                }
            }
        }

        Captures {
            scopes,
            definitions,
            references,
        }
    }

    /// Build a tree of scopes for pre-order traversal by sorting scopes, definitions
    /// and references. Definitions and references are added or hoisted to the
    /// closest enclosing scope as appropriate.
    fn build_tree(&mut self, captures: Captures<'a>) -> Result<()> {
        let Captures {
            mut scopes,
            mut definitions,
            mut references,
        } = captures;
        scopes.sort_by(|a, b| compare_range(a.node.byte_range(), b.node.byte_range()));
        definitions.sort_by_key(|a| a.node.start_byte());
        references.sort_by_key(|a| a.node.start_byte());

        let mut definitions_iter = definitions.iter();
        let mut references_iter = references.iter();

        let mut current_scope = self.top_scope;
        for ScopeCapture {
            kind: scope_kind,
            node: scope_node,
        } in scopes
        {
            // Close all scopes that end before the new scope
            let new_scope_end = scope_node.start_byte();
            let scopes_to_close: Vec<_> = self
                .ancestors_with_self(current_scope)
                .take_while(|scope| new_scope_end > self.end_byte(*scope))
                .collect();
            for scope in scopes_to_close {
                let scope_end_byte = self.end_byte(scope);
                self.add_defs_while(scope, &mut definitions_iter, |def_capture| {
                    def_capture.node.start_byte() < scope_end_byte
                })?;
                self.add_refs_while(scope, &mut references_iter, |ref_capture| {
                    ref_capture.node.start_byte() < scope_end_byte
                })?;
                current_scope = self[scope].parent.unwrap_or(current_scope);
            }

            // Before adding the new scope we first attach all
            // definitions and references that belong to the current
            // scope
            let new_scope_start = scope_node.start_byte();
            self.add_defs_while(current_scope, &mut definitions_iter, |def_capture| {
                def_capture.node.start_byte() < new_scope_start
            })?;
            self.add_refs_while(current_scope, &mut references_iter, |ref_capture| {
                ref_capture.node.start_byte() < new_scope_start
            })?;

            let new_scope = self.arena.alloc(Scope::new(
                self.interner.get_or_intern(scope_kind),
                scope_node,
                Some(current_scope),
            ));
            self[current_scope].children.push(new_scope);
            current_scope = new_scope
        }

        // We need to climb back to the top level scope and add all remaining definitions
        let scopes_to_toplevel: Vec<_> = self.ancestors_with_self(current_scope).collect();
        for scope in scopes_to_toplevel {
            let scope_end_byte = self.end_byte(scope);
            self.add_defs_while(scope, &mut definitions_iter, |def_capture| {
                def_capture.node.start_byte() < scope_end_byte
            })?;
            self.add_refs_while(scope, &mut references_iter, |ref_capture| {
                ref_capture.node.start_byte() < scope_end_byte
            })?;
        }
        debug_assert!(
            definitions_iter.next().is_none(),
            "Should've entered all definitions into the tree"
        );
        debug_assert!(
            references_iter.next().is_none(),
            "Should've entered all references into the tree"
        );
        Ok(())
    }

    /// Walks up the scope tree and tries to find the definition for a given name
    fn find_def(
        &self,
        scope: ScopeId<'a>,
        name: Name,
        start_byte: usize,
    ) -> Option<&Definition<'a>> {
        self.ancestors_with_self(scope)
            .find_map(|scope| self[scope].find_def(name, start_byte))
    }

    fn make_global_reference(&self, reference: &Reference) -> scip::types::Occurrence {
        let referenced_name = self.interner.resolve(reference.name).unwrap().to_string();

        let suffix = match reference.descriptor {
            Some(ReferenceDescriptor::Type) => descriptor::Suffix::Type,
            Some(ReferenceDescriptor::Method) => descriptor::Suffix::Method,
            Some(ReferenceDescriptor::Term) => descriptor::Suffix::Term,
            Some(ReferenceDescriptor::Namespace) => descriptor::Suffix::Namespace,
            None => descriptor::Suffix::Term,
        };

        let symbol = scip::symbol::format_symbol(scip::types::Symbol {
            scheme: SCIP_SYNTAX_SCHEME.into(),
            package: None.into(),
            descriptors: vec![scip::types::Descriptor {
                name: referenced_name,
                suffix: suffix.into(),
                ..Default::default()
            }],
            ..Default::default()
        });

        scip::types::Occurrence {
            range: reference.node.scip_range(),
            symbol,
            ..Default::default()
        }
    }

    fn make_local_reference(
        &self,
        reference: &Reference,
        def_id: DefId,
    ) -> scip::types::Occurrence {
        scip::types::Occurrence {
            range: reference.node.scip_range(),
            symbol: format_symbol(def_id.as_local_symbol()),
            ..Default::default()
        }
    }

    fn resolve_references(&mut self) {
        let mut ref_occurrences = vec![];

        for (scope_ref, scope) in self.arena.iter() {
            for reference in scope.references.iter() {
                if let Some(def_id) = reference.resolves_to {
                    ref_occurrences.push(self.make_local_reference(reference, def_id));
                    continue;
                }
                let offset = reference.node.start_byte();
                if reference.visibility == Visibility::Local {
                    if let Some(def) = self.find_def(scope_ref, reference.name, offset) {
                        ref_occurrences.push(self.make_local_reference(reference, def.id));
                        continue;
                    }
                }
                if self.options.emit_global_references {
                    ref_occurrences.push(self.make_global_reference(reference))
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
        test_writer: Option<&mut dyn Write>,
    ) -> Result<Vec<Occurrence>> {
        // First we collect all captures from the tree-sitter locals query
        let captures = self.collect_captures(config, tree, self.source_bytes);

        // Next we build a tree structure of scopes and definitions
        self.build_tree(captures)?;
        match test_writer {
            None => {}
            Some(w) => self.print_scope(w, self.top_scope, 0),
        }

        // Finally we resolve all references against that tree structure
        self.resolve_references();
        Ok(self.occurrences)
    }
}

#[cfg(test)]
mod test {
    use scip::types::Document;
    use tree_sitter_all_languages::ParserId;

    use super::*;
    use crate::{
        languages::LocalConfiguration,
        snapshot::{dump_document_with_config, EmitSymbol, SnapshotOptions},
    };

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

    fn parse_file_for_lang(
        config: &LocalConfiguration,
        source_code: &str,
        emit_global_references: bool,
    ) -> (Document, String) {
        let source_bytes = source_code.as_bytes();
        let mut parser = config.get_parser();
        let tree = parser.parse(source_bytes, None).unwrap();
        let resolver = LocalResolver::new(
            source_code,
            tree.root_node(),
            LocalResolutionOptions {
                emit_global_references,
            },
        );
        let mut tree_output = String::new();
        let occ = resolver.process(config, &tree, Some(&mut tree_output));

        let mut doc = Document::new();
        doc.occurrences = occ.unwrap();
        doc.symbols = doc
            .occurrences
            .iter()
            .map(|o| scip::types::SymbolInformation {
                symbol: o.symbol.clone(),
                ..Default::default()
            })
            .collect();

        (doc, tree_output)
    }

    // #[test]
    // fn go() {
    //     let config = crate::languages::get_local_configuration(ParserId::Go).unwrap();
    //     let source_code = include_str!("../testdata/locals.go");
    //     let (doc, scope_tree) = parse_file_for_lang(config, source_code, false);
    //     let dumped = snapshot_syntax_document(&doc, source_code);
    //     insta::assert_snapshot!("go_occurrences", dumped);
    //     insta::assert_snapshot!("go_scopes", scope_tree);
    // }

    // #[test]
    // fn perl() {
    //     let config = crate::languages::get_local_configuration(ParserId::Perl).unwrap();
    //     let source_code = include_str!("../testdata/perl.pm");
    //     let (doc, scope_tree) = parse_file_for_lang(config, source_code, false);
    //     let dumped = snapshot_syntax_document(&doc, source_code);
    //     insta::assert_snapshot!("perl_occurrences", dumped);
    //     insta::assert_snapshot!("perl_scopes", scope_tree);
    // }

    // #[test]
    // fn matlab() {
    //     let config = crate::languages::get_local_configuration(ParserId::Matlab).unwrap();
    //     let source_code = include_str!("../testdata/locals.m");
    //     let (doc, scope_tree) = parse_file_for_lang(config, source_code, false);
    //     let dumped = snapshot_syntax_document(&doc, source_code);
    //     insta::assert_snapshot!("matlab_occurrences", dumped);
    //     insta::assert_snapshot!("matlab_scopes", scope_tree);
    // }

    // #[test]
    // fn java() {
    //     let config = crate::languages::get_local_configuration(ParserId::Java).unwrap();
    //     let source_code = include_str!("../testdata/locals.java");
    //     let (doc, scope_tree) = parse_file_for_lang(config, source_code, true);
    //     let dumped = snapshot_syntax_document(&doc, source_code);
    //     insta::assert_snapshot!("java_scopes", scope_tree);
    //     insta::assert_snapshot!("java_occurrences", dumped);
    // }
}
