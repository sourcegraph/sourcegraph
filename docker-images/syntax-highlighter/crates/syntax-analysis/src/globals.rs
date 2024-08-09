use anyhow::{bail, ensure, Context, Result};
use bitvec::prelude::*;
use protobuf::Enum;
use scip::types::{symbol_information, Descriptor, Document, Occurrence, SymbolInformation};

use crate::{languages::TagConfiguration, range::Range};

#[derive(Debug)]
pub struct Scope {
    pub ident_range: Range,
    pub scope_range: Range,
    pub globals: Vec<Global>,
    pub children: Vec<Scope>,
    pub descriptors: Vec<Descriptor>,
    pub kind: symbol_information::Kind,
}

#[derive(Debug)]
pub struct Global {
    pub range: Range,
    pub enclosing: Option<Range>,
    pub descriptors: Vec<Descriptor>,
    pub kind: symbol_information::Kind,
}

impl Scope {
    pub fn insert_scope(&mut self, scope: Scope) {
        if let Some(child) = self
            .children
            .iter_mut()
            .find(|child| child.scope_range.contains(&scope.scope_range))
        {
            child.insert_scope(scope);
        } else {
            self.children.push(scope);
        }
    }

    pub fn insert_global(&mut self, global: Global) {
        if let Some(child) = self
            .children
            .iter_mut()
            .find(|child| child.scope_range.contains(&global.range))
        {
            child.insert_global(global)
        } else {
            self.globals.push(global);
        }
    }

    pub fn into_document(
        &mut self,
        hint: usize,
        scheme: &str,
        base_descriptors: Vec<Descriptor>,
    ) -> Document {
        let mut descriptor_stack = base_descriptors;

        let mut occurrences = Vec::with_capacity(hint);
        let mut symbols = Vec::with_capacity(hint);
        self.traverse(
            true,
            scheme,
            &mut occurrences,
            &mut descriptor_stack,
            &mut symbols,
        );

        Document {
            occurrences,
            symbols,
            ..Default::default()
        }
    }

    fn traverse(
        &self,
        is_root: bool,
        scheme: &str,
        occurrences: &mut Vec<Occurrence>,
        descriptor_stack: &mut Vec<Descriptor>,
        symbols: &mut Vec<SymbolInformation>,
    ) {
        descriptor_stack.extend(self.descriptors.clone());

        if !is_root {
            let symbol = scip::symbol::format_symbol(scip::types::Symbol {
                scheme: scheme.into(),
                package: None.into(),
                descriptors: descriptor_stack.clone(),
                ..Default::default()
            });

            occurrences.push(scip::types::Occurrence {
                range: self.ident_range.to_vec(),
                symbol: symbol.clone(),
                symbol_roles: scip::types::SymbolRole::Definition.value(),
                enclosing_range: self.scope_range.to_vec(),
                ..Default::default()
            });

            symbols.push(SymbolInformation {
                symbol,
                kind: self.kind.into(),
                ..Default::default()
            })
        }

        for global in &self.globals {
            let mut global_descriptors = descriptor_stack.clone();
            global_descriptors.extend(global.descriptors.clone());

            let symbol = scip::symbol::format_symbol(scip::types::Symbol {
                scheme: scheme.into(),
                package: None.into(),
                descriptors: global_descriptors,
                ..Default::default()
            });

            let symbol_roles = scip::types::SymbolRole::Definition.value();
            occurrences.push(scip::types::Occurrence {
                range: global.range.to_vec(),
                symbol: symbol.clone(),
                symbol_roles,
                enclosing_range: match &global.enclosing {
                    Some(enclosing) => enclosing.to_vec(),
                    None => vec![],
                },
                ..Default::default()
            });

            symbols.push(SymbolInformation {
                symbol,
                kind: global.kind.into(),
                ..Default::default()
            });
        }

        self.children
            .iter()
            .for_each(|c| c.traverse(false, scheme, occurrences, descriptor_stack, symbols));

        self.descriptors.iter().for_each(|_| {
            descriptor_stack.pop();
        });
    }
}

pub fn parse_tree<'a>(
    config: &TagConfiguration,
    tree: &'a tree_sitter::Tree,
    source: &'a str,
) -> Result<(Scope, usize)> {
    let source_bytes = source.as_bytes();
    let mut cursor = tree_sitter::QueryCursor::new();

    let root_node = tree.root_node();
    let capture_names = config.query.capture_names();

    let mut scopes = vec![];
    let mut globals = vec![];

    let mut local_ranges = BitVec::<u8, Msb0>::repeat(false, source_bytes.len());

    let matches = cursor.matches(&config.query, root_node, source_bytes);
    for m in matches {
        if config.is_filtered(&m) {
            continue;
        }

        let mut node = None;
        let mut enclosing_node = None;
        let mut scope = None;
        let mut local_range = None;
        let mut descriptors = vec![];
        let mut kind = None;

        for capture in m.captures {
            let capture_name = *capture_names
                .get(capture.index as usize)
                .context("capture indexes should always work")?;

            if capture_name.starts_with("descriptor") {
                descriptors.push((
                    capture_name,
                    capture
                        .node
                        .utf8_text(source_bytes)
                        .context("Unexpected non-utf-8 content. This is a tree-sitter bug")?,
                ));
                node = Some(capture.node);
            }

            if capture_name.starts_with("scope") {
                ensure!(scope.is_none(), "declare only one scope per match");
                scope = Some(capture);
            }

            if capture_name.starts_with("enclosing") {
                ensure!(enclosing_node.is_none(), "declare only one scope per match");
                enclosing_node = Some(capture.node);
            }

            if capture_name.starts_with("local") {
                local_range = Some(capture.node.byte_range());
            }

            if capture_name.starts_with("kind") {
                ensure!(kind.is_none(), "declare only one kind per match");
                kind = Some(capture_name)
            }
        }

        match node {
            Some(node) => {
                // TODO: I think we may need to consider something like this at some point
                // but for now it's fine. Just something I was thinking of while debugging
                // an issue with go locals
                //
                // match scope {
                //     Some(scope) if local_ranges[scope.node.start_byte()] => continue,
                //     None if local_ranges[node.start_byte()] => continue,
                //     _ => {}
                // };

                if local_ranges[node.start_byte()] {
                    continue;
                }

                let kind = crate::ts_scip::captures_to_kind(&kind);

                let descriptors = descriptors
                    .iter()
                    .map(|(capture, name)| {
                        crate::ts_scip::capture_name_to_descriptor(capture, name.to_string())
                    })
                    .collect();

                match scope {
                    Some(scope_ident) => scopes.push(Scope {
                        ident_range: node.into(),
                        scope_range: scope_ident.node.into(),
                        globals: vec![],
                        children: vec![],
                        descriptors,
                        kind,
                    }),
                    None => {
                        let (last, rest) = match descriptors.split_last() {
                            Some(res) => res,
                            None => continue,
                        };

                        match config.transform(m.pattern_index, last) {
                            Some(transformed) => {
                                for transform in transformed {
                                    globals.push(Global {
                                        range: node.into(),
                                        enclosing: enclosing_node.map(|n| n.into()),
                                        descriptors: {
                                            let mut descriptors = rest
                                                .iter()
                                                .map(|d| (*d).clone())
                                                .collect::<Vec<_>>();
                                            descriptors.push(transform);
                                            descriptors
                                        },
                                        kind,
                                    });
                                }
                            }
                            None => globals.push(Global {
                                range: node.into(),
                                enclosing: enclosing_node.map(|n| n.into()),
                                descriptors,
                                kind,
                            }),
                        }
                    }
                }
            }
            None => {
                if local_range.is_none() {
                    bail!("there must always be at least one descriptor (except for @local)");
                }
            }
        }

        if let Some(local_range) = local_range {
            local_ranges.get_mut(local_range).unwrap().fill(true);
        }
    }

    let mut root = Scope {
        ident_range: root_node.into(),
        scope_range: root_node.into(),
        globals: vec![],
        children: vec![],
        descriptors: vec![],
        kind: symbol_information::Kind::UnspecifiedKind,
    };

    scopes.sort_by_key(|m| {
        std::cmp::Reverse((
            m.scope_range.start_line,
            m.scope_range.end_line,
            m.scope_range.start_col,
        ))
    });

    // Add all the scopes to our tree
    while let Some(m) = scopes.pop() {
        root.insert_scope(m);
    }

    while let Some(m) = globals.pop() {
        root.insert_global(m);
    }

    Ok((root, globals.len()))
}

#[cfg(test)]
pub mod test {
    use scip::types::Document;
    use tree_sitter_all_languages::ParserId;

    use super::*;
    use crate::snapshot::{self, dump_document_with_config, SnapshotOptions};

    pub fn parse_file_for_lang(config: &TagConfiguration, source_code: &str) -> Document {
        let mut parser = config.get_parser();
        let tree = parser.parse(source_code.as_bytes(), None).unwrap();

        let (mut scope, hint) = parse_tree(config, &tree, source_code).unwrap();
        scope.into_document(hint, "scip-ctags", vec![])
    }

    // #[test]
    // fn test_enclosing_range() {
    //     let config = crate::languages::get_tag_configuration(ParserId::Go).expect("to have parser");
    //     let source_code = include_str!("../testdata/scopes_of_go.go");
    //     let doc = parse_file_for_lang(config, source_code);

    //     // let dumped = dump_document(&doc, source_code)?;
    //     let dumped = dump_document_with_config(
    //         &doc,
    //         source_code,
    //         SnapshotOptions {
    //             snapshot_range: None,
    //             emit_syntax: snapshot::EmitSyntax::None,
    //             emit_symbol: snapshot::EmitSymbol::Enclosing,
    //         },
    //     )
    //     .unwrap();

    //     insta::assert_snapshot!(dumped);
    // }
}
