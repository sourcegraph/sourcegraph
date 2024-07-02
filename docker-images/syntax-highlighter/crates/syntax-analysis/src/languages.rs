use std::collections::HashMap;

use once_cell::sync::OnceCell;
use regex::Regex;
use scip::types::Descriptor;
use tree_sitter::{Language, Parser, Query};
use tree_sitter_all_languages::ParserId;

use crate::highlighting::tree_sitter::include_scip_query;

#[derive(Debug)]
pub struct Transform {
    pattern: Regex,
    replace: String,
}

#[derive(Debug)]
pub struct NodeFilter {
    capture: u32,
    names: Vec<String>,
}

pub struct TagConfiguration {
    language: Language,
    query_text: String,
    pub query: Query,

    // Handles #transform! predicates in queries
    transforms: HashMap<usize, Vec<Transform>>,

    // Handles #filter! predicates in queries
    filters: HashMap<usize, Vec<NodeFilter>>,
}

impl TagConfiguration {
    fn new(language: Language, query_text: &str) -> Self {
        let first_line = query_text.lines().next();
        let query_text = match first_line {
            Some(line) if line.starts_with(";;include") => {
                let (_, included_lang) =
                    line.split_once(";;include").expect("must have ;; include");
                let included_lang = included_lang.trim();

                let parser = ParserId::from_name(included_lang).expect("valid language");
                let configuration = get_tag_configuration(parser).expect("valid config");

                format!("{}\n{}", configuration.query_text, query_text)
            }
            _ => query_text.to_string(),
        };

        let query = Query::new(language, &query_text).expect("to parse query");

        let mut transforms = HashMap::new();
        let mut filters = HashMap::new();

        for index in 0..query.pattern_count() {
            let predicate = query.general_predicates(index);

            if !predicate.is_empty() {
                // Collect #transform! predicates
                transforms.insert(
                    index,
                    predicate
                        .iter()
                        .filter_map(|pred| match pred.operator.as_ref() {
                            "transform!" => {
                                let args = &pred.args;
                                if args.len() != 2 {
                                    panic!("bad transform!??!");
                                }

                                let pattern = {
                                    let replace_str = match &args[0] {
                                        tree_sitter::QueryPredicateArg::String(str) => str,
                                        _ => panic!("pattern for #transform! should be a string"),
                                    };

                                    Regex::new(replace_str)
                                        .expect("pattern for #transform! should be a valid regex")
                                };

                                let replace = match &args[1] {
                                    tree_sitter::QueryPredicateArg::String(str) => str.to_string(),
                                    _ => panic!("replace to #transform! should be a string"),
                                };

                                Some(Transform { pattern, replace })
                            }
                            _ => None,
                        })
                        .collect::<Vec<_>>(),
                );

                // Collect #filter! predicates
                filters.insert(
                    index,
                    predicate
                        .iter()
                        .filter_map(|pred| match pred.operator.as_ref() {
                            "filter!" => {
                                let args = &pred.args;

                                if args.len() < 2 {
                                    panic!("should have at least two arguments for filter");
                                }

                                let capture = match &args[0] {
                                    tree_sitter::QueryPredicateArg::Capture(capture) => *capture,
                                    _ => panic!("filter! first arg should be a  capture"),
                                };

                                let names = args[1..]
                                    .iter()
                                    .map(|arg| {
                                        let name = match arg {
                                            tree_sitter::QueryPredicateArg::String(name) => name,
                                            _ => panic!("filter! 1.. args should be string names"),
                                        };

                                        name.to_string()
                                    })
                                    .collect();

                                Some(NodeFilter { capture, names })
                            }
                            _ => None,
                        })
                        .collect::<Vec<_>>(),
                );
            }
        }

        Self {
            language,
            query_text,
            query,
            transforms,
            filters,
        }
    }

    pub fn get_parser(&self) -> Parser {
        let mut parser = Parser::new();
        parser.set_language(self.language).expect("to get a parser");
        parser
    }

    pub fn transform(&self, index: usize, captured: &Descriptor) -> Option<Vec<Descriptor>> {
        match self.transforms.get(&index) {
            Some(transforms) if !transforms.is_empty() => Some(
                transforms
                    .iter()
                    .map(|t| Descriptor {
                        name: t
                            .pattern
                            .replace_all(&captured.name, &t.replace)
                            .to_string(),
                        suffix: captured.suffix,
                        disambiguator: captured.disambiguator.clone(),
                        ..Default::default()
                    })
                    .collect::<Vec<_>>(),
            ),
            _ => None,
        }
    }

    pub fn is_filtered(&self, m: &tree_sitter::QueryMatch) -> bool {
        match self.filters.get(&m.pattern_index) {
            Some(filters) if !filters.is_empty() => filters.iter().any(|filter| {
                m.nodes_for_capture_index(filter.capture)
                    .any(|node| filter.names.iter().any(|name| name == node.kind()))
            }),
            _ => false,
        }
    }
}

pub struct LocalConfiguration {
    language: Language,
    pub query: Query,
}

impl LocalConfiguration {
    pub fn get_parser(&self) -> Parser {
        let mut parser = Parser::new();
        parser.set_language(self.language).expect("to get a parser");
        parser
    }
}

mod tags {
    use super::*;

    macro_rules! create_tags_configuration {
        ($name:tt, $parser_id:path, $file:tt) => {
            pub fn $name() -> &'static TagConfiguration {
                static INSTANCE: OnceCell<TagConfiguration> = OnceCell::new();

                INSTANCE.get_or_init(|| {
                    let language = $parser_id.language();
                    let query = include_scip_query!($file, "scip-tags");
                    TagConfiguration::new(language, query)
                })
            }
        };
    }

    create_tags_configuration!(c, ParserId::C, "c");
    create_tags_configuration!(cpp, ParserId::Cpp, "cpp");
    create_tags_configuration!(c_sharp, ParserId::C_Sharp, "c_sharp");
    create_tags_configuration!(go, ParserId::Go, "go");
    create_tags_configuration!(java, ParserId::Java, "java");
    create_tags_configuration!(javascript, ParserId::Javascript, "javascript");
    create_tags_configuration!(kotlin, ParserId::Kotlin, "kotlin");
    create_tags_configuration!(magik, ParserId::Magik, "magik");
    create_tags_configuration!(python, ParserId::Python, "python");
    create_tags_configuration!(ruby, ParserId::Ruby, "ruby");
    create_tags_configuration!(rust, ParserId::Rust, "rust");
    create_tags_configuration!(scala, ParserId::Scala, "scala");
    create_tags_configuration!(typescript, ParserId::Typescript, "typescript");
    create_tags_configuration!(zig, ParserId::Zig, "zig");

    pub fn get_tag_configuration(parser: ParserId) -> Option<&'static TagConfiguration> {
        match parser {
            ParserId::C => Some(c()),
            ParserId::Cpp => Some(cpp()),
            ParserId::C_Sharp => Some(c_sharp()),
            ParserId::Go => Some(go()),
            ParserId::Java => Some(java()),
            ParserId::Javascript => Some(javascript()),
            ParserId::Kotlin => Some(kotlin()),
            ParserId::Magik => Some(magik()),
            ParserId::Python => Some(python()),
            ParserId::Ruby => Some(ruby()),
            ParserId::Rust => Some(rust()),
            ParserId::Scala => Some(scala()),
            ParserId::Typescript => Some(typescript()),
            ParserId::Zig => Some(zig()),
            _ => None,
        }
    }
}

mod locals {
    use super::*;

    macro_rules! create_locals_configuration {
        ($name:tt, $parser_id:path, $file:tt) => {
            pub fn $name() -> &'static LocalConfiguration {
                static INSTANCE: OnceCell<LocalConfiguration> = OnceCell::new();

                INSTANCE.get_or_init(|| {
                    let language = $parser_id.language();
                    let query = include_scip_query!($file, "scip-locals");

                    LocalConfiguration {
                        language,
                        query: Query::new(language, query).unwrap(),
                    }
                })
            }
        };
    }

    create_locals_configuration!(go, ParserId::Go, "go");
    create_locals_configuration!(perl, ParserId::Perl, "perl");
    create_locals_configuration!(matlab, ParserId::Matlab, "matlab");
    create_locals_configuration!(java, ParserId::Java, "java");

    pub fn get_local_configuration(parser: ParserId) -> Option<&'static LocalConfiguration> {
        match parser {
            ParserId::Go => Some(go()),
            ParserId::Perl => Some(perl()),
            ParserId::Matlab => Some(matlab()),
            ParserId::Java => Some(java()),
            _ => None,
        }
    }
}

pub use locals::get_local_configuration;
pub use tags::get_tag_configuration;
