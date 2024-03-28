use paste::paste;
use scip::types::{Document, Occurrence, SyntaxKind};
use std::collections::HashMap;
use tree_sitter_all_languages::ParserId;
use tree_sitter_highlight::{
    Highlight, HighlightConfiguration, HighlightEvent, Highlighter as TSHighlighter,
};

use crate::range::Range;

macro_rules! include_scip_query {
    ($lang: expr, $query: literal) => {
        include_str!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/queries/",
            $lang,
            "/",
            $query,
            ".scm"
        ))
    };
}
use crate::highlighting::TreeSitterLanguageName;
pub(crate) use include_scip_query;

#[rustfmt::skip]
// This table serves two purposes.
//
// 1. It serves as the list of all captures that we
//    recognize in highlighting queries specified in
//    highlights.scm files. The list of captures is
//    the union across all languages.
// 2. It describes the capture -> syntax kind mapping.
//
// Client-side code will convert the syntax kinds into actual colors.
const MATCHES_TO_SYNTAX_KINDS: &[(&str, SyntaxKind)] = &[
    ("boolean",                 SyntaxKind::BooleanLiteral),
    ("character",               SyntaxKind::CharacterLiteral),
    ("comment",                 SyntaxKind::Comment),
    ("conditional",             SyntaxKind::IdentifierKeyword),
    ("constant",                SyntaxKind::IdentifierConstant),
    ("identifier.constant",     SyntaxKind::IdentifierConstant),
    ("constant.builtin",        SyntaxKind::IdentifierBuiltin),
    ("constant.null",           SyntaxKind::IdentifierNull),
    ("float",                   SyntaxKind::NumericLiteral),
    ("function",                SyntaxKind::IdentifierFunction),
    ("method",                  SyntaxKind::IdentifierFunction),
    ("identifier.function",     SyntaxKind::IdentifierFunction),
    ("function.builtin",        SyntaxKind::IdentifierBuiltin),
    ("identifier.builtin",      SyntaxKind::IdentifierBuiltin),
    ("identifier",              SyntaxKind::Identifier),
    ("identifier.attribute",    SyntaxKind::IdentifierAttribute),
    ("tag.attribute",           SyntaxKind::TagAttribute),
    ("include",                 SyntaxKind::IdentifierKeyword),
    ("keyword",                 SyntaxKind::IdentifierKeyword),
    ("keyword.function",        SyntaxKind::IdentifierKeyword),
    ("keyword.return",          SyntaxKind::IdentifierKeyword),
    ("number",                  SyntaxKind::NumericLiteral),
    ("operator",                SyntaxKind::IdentifierOperator),
    ("identifier.operator",     SyntaxKind::IdentifierOperator),
    ("property",                SyntaxKind::Identifier),
    ("punctuation",             SyntaxKind::UnspecifiedSyntaxKind),
    ("punctuation.bracket",     SyntaxKind::UnspecifiedSyntaxKind),
    ("punctuation.delimiter",   SyntaxKind::PunctuationDelimiter),
    ("string",                  SyntaxKind::StringLiteral),
    ("string.special",          SyntaxKind::StringLiteral),
    ("string.escape",           SyntaxKind::StringLiteralEscape),
    ("tag",                     SyntaxKind::UnspecifiedSyntaxKind),
    ("type",                    SyntaxKind::IdentifierType),
    ("identifier.type",         SyntaxKind::IdentifierType),
    ("type.builtin",            SyntaxKind::IdentifierBuiltinType),
    ("regex.delimiter",         SyntaxKind::RegexDelimiter),
    ("regex.join",              SyntaxKind::RegexJoin),
    ("regex.escape",            SyntaxKind::RegexEscape),
    ("regex.repeated",          SyntaxKind::RegexRepeated),
    ("regex.wildcard",          SyntaxKind::RegexWildcard),
    ("identifier",              SyntaxKind::Identifier),
    ("variable",                SyntaxKind::Identifier),
    ("identifier.builtin",      SyntaxKind::IdentifierBuiltin),
    ("variable.builtin",        SyntaxKind::IdentifierBuiltin),
    ("identifier.parameter",    SyntaxKind::IdentifierParameter),
    ("variable.parameter",      SyntaxKind::IdentifierParameter),
    ("identifier.module",       SyntaxKind::IdentifierModule),
    ("variable.module",         SyntaxKind::IdentifierModule),
];

/// Add a language highlight configuration to the CONFIGURATIONS global.
///
/// This makes it so you don't have to understand how configurations are added,
/// just add the name of filetype that you want.
macro_rules! create_configurations {
    ( $(($name:tt,$dirname:literal)),* ) => {{
        use tree_sitter_all_languages::ParserId;

        let mut m = HashMap::new();
        let highlight_names = MATCHES_TO_SYNTAX_KINDS.iter().map(|hl| hl.0).collect::<Vec<&str>>();

        $(
            {
                // Create HighlightConfiguration language
                let mut lang = HighlightConfiguration::new(
                    paste! { ParserId::$name.language() },
                    include_scip_query!($dirname, "highlights"),
                    include_scip_query!($dirname, "injections"),
                    include_scip_query!($dirname, "locals"),
                ).expect(stringify!("parser for '{}' must be compiled", $name));

                // Associate highlights with configuration
                lang.configure(&highlight_names);

                // Insert into configurations, so we only create once at startup.
                m.insert(ParserId::$name, lang);
            }
        )*

        {
            let highlights = vec![
                include_scip_query!("typescript", "highlights"),
                include_scip_query!("javascript", "highlights"),
            ];
            let mut lang = HighlightConfiguration::new(
                ParserId::Typescript.language(),
                &highlights.join("\n"),
                include_scip_query!("typescript", "injections"),
                include_scip_query!("typescript", "locals"),
            ).expect("parser for 'typescript' must be compiled");
            lang.configure(&highlight_names);
            m.insert(ParserId::Typescript, lang);
        }
        {
            let highlights = vec![
                include_scip_query!("tsx", "highlights"),
                include_scip_query!("typescript", "highlights"),
                include_scip_query!("javascript", "highlights"),
            ];
            let mut lang = HighlightConfiguration::new(
                ParserId::Tsx.language(),
                &highlights.join("\n"),
                include_scip_query!("tsx", "injections"),
                include_scip_query!("tsx", "locals"),
            ).expect("parser for 'tsx' must be compiled");
            lang.configure(&highlight_names);
            m.insert(ParserId::Tsx, lang);
        }

        m
    }}
}

lazy_static::lazy_static! {
    pub static ref CONFIGURATIONS: HashMap<ParserId, HighlightConfiguration> = {
        create_configurations!(
            (C, "c"),
            (Cpp, "cpp"),
            (C_Sharp, "c_sharp"),
            (Go, "go"),
            (Java, "java"),
            (Javascript, "javascript"),
            (Jsonnet, "jsonnet"),
            (Kotlin, "kotlin"),
            (Matlab, "matlab"),
            (Nickel, "nickel"),
            (Perl, "perl"),
            (Pkl, "pkl"),
            (Pod, "pod"),
            (Python, "python"),
            (Ruby, "ruby"),
            (Rust, "rust"),
            (Scala, "scala"),
            (Sql, "sql"),
            // Skipping TypeScript and TSX here as they're handled
            // specially inside the macro implementation.
            (Xlsg, "xlsg"),
            (Zig, "zig")
        )
    };
}

fn get_syntax_kind_for_hl(hl: Highlight) -> SyntaxKind {
    MATCHES_TO_SYNTAX_KINDS[hl.0].1
}

impl TreeSitterLanguageName {
    pub fn highlight_document(
        &self,
        code: &str,
        include_locals: bool,
    ) -> Result<Document, tree_sitter_highlight::Error> {
        match self.highlighting_configuration() {
            Some(lang_config) => {
                self.highlight_document_with_config(code, include_locals, lang_config)
            }
            None => Err(tree_sitter_highlight::Error::InvalidLanguage),
        }
    }

    fn parser_id(&self) -> Option<ParserId> {
        ParserId::from_name(&self.raw)
    }

    fn highlighting_configuration(&self) -> Option<&'static HighlightConfiguration> {
        CONFIGURATIONS.get(&self.parser_id()?)
    }

    pub fn highlight_document_with_config(
        &self,
        code: &str,
        include_locals: bool,
        lang_config: &HighlightConfiguration,
    ) -> Result<Document, tree_sitter_highlight::Error> {
        // Normalize string to be always only \n endings.
        //  We don't care that the byte offsets are "incorrect" now for this
        //  because we are using a line,col based approach
        let code = code.replace("\r\n", "\n");

        // TODO: We should automatically apply no highlights when we are
        // in an injected piece of code.
        //
        // Unfortunately, that information isn't currently available when
        // we are iterating in the higlighter.
        let mut highlighter = TSHighlighter::new();
        let highlights = highlighter.highlight(lang_config, code.as_bytes(), None, |l| {
            TreeSitterLanguageName::new(l).highlighting_configuration()
        })?;

        let mut emitter = ScipEmitter::new();
        let mut doc = emitter.render(highlights, &code)?;
        doc.occurrences.sort_by_key(|a| (a.range[0], a.range[1]));

        if include_locals {
            let parser = self.parser_id();
            if let Some(parser) = parser {
                // TODO: Could probably write this in a much better way.
                let mut local_occs = crate::get_locals(parser, &code).unwrap_or_default();

                // Get ranges in reverse order, because we're going to pop off the back of the list.
                //  (that's why we're sorting the opposite way of the document occurrences above).
                local_occs.sort_by_key(|a| (-a.range[0], -a.range[1]));

                let mut next_doc_idx = 0;
                while let Some(local) = local_occs.pop() {
                    // We *should* be able to assume that all these ranges are valid ranges
                    // but for now we'll skip if they aren't.
                    //
                    // We can add some observability stuff to this later, and/or make
                    // certain builds fail or something to test this out better (but
                    // not have syntax highlighting completely fall apart from one
                    // bad range)
                    let local_range = match Range::from_vec(&local.range) {
                        Some(range) => range,
                        None => continue,
                    };

                    let (matching_idx, matching_occ) = match doc
                        .occurrences
                        .iter_mut()
                        .enumerate()
                        .skip(next_doc_idx)
                        .find(|(_, occ)| local_range.eq_vec(&occ.range))
                    {
                        Some(found) => found,
                        None => continue,
                    };

                    next_doc_idx = matching_idx;

                    // Update occurrence with new information from locals
                    matching_occ.symbol = local.symbol;
                    matching_occ.symbol_roles = local.symbol_roles;
                }
            }
        }

        Ok(doc)
    }
}

struct OffsetManager {
    source: String,
    offsets: Vec<usize>,
}

impl OffsetManager {
    // Pre-condition: Input string is non-empty
    fn new(s: &str) -> Self {
        debug_assert!(!s.is_empty());
        let source = s.to_string();

        let mut offsets = Vec::new();
        let mut pos = 0;
        for line in s.lines() {
            offsets.push(pos);
            // pos += line.chars().count() + 1;
            //
            // NOTE: This intentionally in bytes. The correct stuff is done in
            // self.line_and_col later
            pos += line.len() + 1;
        }

        Self { source, offsets }
    }

    fn line_and_col(&self, offset_byte: usize) -> (usize, usize) {
        // let offset_char = self.source.bytes
        let mut line = 0;
        for window in self.offsets.windows(2) {
            let curr = window[0];
            let next = window[1];
            if next > offset_byte {
                return (
                    line,
                    // Return the number of characters between the locations (which is the column)
                    self.source[curr..offset_byte].chars().count(),
                );
            }

            line += 1;
        }

        (
            line,
            // Return the number of characters between the locations (which is the column)
            self.source[*self.offsets.last().unwrap()..offset_byte]
                .chars()
                .count(),
        )
    }

    // range takes in start and end offsets and returns start/end line/column.
    fn range(&self, start_byte: usize, end_byte: usize) -> Vec<i32> {
        let start_pos = self.line_and_col(start_byte);
        let end_pos = self.line_and_col(end_byte);

        if start_pos.0 == end_pos.0 {
            vec![start_pos.0 as i32, start_pos.1 as i32, end_pos.1 as i32]
        } else {
            vec![
                start_pos.0 as i32,
                start_pos.1 as i32,
                end_pos.0 as i32,
                end_pos.1 as i32,
            ]
        }
    }
}

pub(crate) struct ScipEmitter {}

/// Our version of `tree_sitter_highlight::HtmlRenderer`, which emits stuff as a table.
///
/// You can see the original version in the tree_sitter_highlight crate.
impl ScipEmitter {
    pub fn new() -> Self {
        ScipEmitter {}
    }

    pub fn render(
        &mut self,
        highlighter: impl Iterator<Item = Result<HighlightEvent, tree_sitter_highlight::Error>>,
        source: &str,
    ) -> Result<Document, tree_sitter_highlight::Error> {
        let mut doc = Document::new();
        if source.is_empty() {
            return Ok(doc);
        }

        let line_manager = OffsetManager::new(source);
        let mut highlights = vec![];
        for event in highlighter {
            match event? {
                HighlightEvent::HighlightStart(s) => highlights.push(s),
                HighlightEvent::HighlightEnd => {
                    highlights.pop();
                }

                // No highlights matched
                HighlightEvent::Source { .. } if highlights.is_empty() => {}

                // When a `start`->`end` has some highlights
                HighlightEvent::Source {
                    start: start_byte,
                    end: end_byte,
                } => {
                    let mut occurrence = Occurrence::new();
                    occurrence.range = line_manager.range(start_byte, end_byte);
                    occurrence.syntax_kind =
                        get_syntax_kind_for_hl(*highlights.last().unwrap()).into();

                    doc.occurrences.push(occurrence);
                }
            }
        }

        Ok(doc)
    }
}

#[cfg(test)]
mod test {
    use std::{
        fs::{read_dir, File},
        io::Read,
    };

    use super::*;
    use crate::highlighting::FileInfo;
    use crate::snapshot::{self, dump_document_with_config};

    fn snapshot_treesitter_syntax_kinds(doc: &Document, source: &str) -> String {
        dump_document_with_config(
            doc,
            source,
            snapshot::SnapshotOptions {
                emit_syntax: snapshot::EmitSyntax::Highlighted,
                emit_symbol: snapshot::EmitSymbol::None,
                ..Default::default()
            },
        )
        .expect("dump document")
    }

    fn snapshot_treesitter_syntax_and_symbols(doc: &Document, source: &str) -> String {
        dump_document_with_config(
            doc,
            source,
            snapshot::SnapshotOptions {
                emit_syntax: snapshot::EmitSyntax::Highlighted,
                emit_symbol: snapshot::EmitSymbol::All,
                ..Default::default()
            },
        )
        .expect("dump document")
    }

    #[test]
    fn test_highlights_one_comment() -> anyhow::Result<()> {
        let src = "// Hello World";
        let document = TreeSitterLanguageName::new("go").highlight_document(src, false)?;
        insta::assert_snapshot!(snapshot_treesitter_syntax_kinds(&document, src));

        Ok(())
    }

    #[test]
    fn test_highlights_a_sql_query_within_go() -> anyhow::Result<()> {
        let src = r#"package main

const MySqlQuery = `
SELECT * FROM my_table
`
"#;

        let document = TreeSitterLanguageName::new("go").highlight_document(src, false)?;
        insta::assert_snapshot!(snapshot_treesitter_syntax_kinds(&document, src));

        Ok(())
    }

    #[test]
    fn test_highlight_csharp_file() -> anyhow::Result<()> {
        let src = "using System;";
        let document = TreeSitterLanguageName::new("c_sharp").highlight_document(src, false)?;
        insta::assert_snapshot!(snapshot_treesitter_syntax_kinds(&document, src));

        Ok(())
    }

    #[test]
    fn test_all_files() -> Result<(), std::io::Error> {
        let crate_root: std::path::PathBuf = std::env::var("CARGO_MANIFEST_DIR").unwrap().into();
        let input_dir = crate_root
            .join("src")
            .join("highlighting")
            .join("snapshots")
            .join("files");
        let dir = read_dir(&input_dir).unwrap();

        let mut failed_tests = vec![];
        let mut count = 0;
        for entry in dir {
            count += 1;
            let entry = entry?;
            let filepath = entry.path();
            let mut file = File::open(&filepath)?;
            let mut contents = String::new();
            file.read_to_string(&mut contents)?;

            let language = &crate::highlighting::test::SYNTAX_SET
                .with(|syntax_set| {
                    FileInfo::new(filepath.to_string_lossy().as_ref(), &contents, None)
                        .determine_language(syntax_set)
                })
                .unwrap();

            let document = language.highlight_document(&contents, true).unwrap();
            // TODO: I'm not sure if there's a better way to run the snapshots without
            // panicing and then catching, but this will do for now.
            match std::panic::catch_unwind(|| {
                insta::assert_snapshot!(
                    filepath.strip_prefix(&input_dir).unwrap().to_str().unwrap(),
                    snapshot_treesitter_syntax_kinds(&document, &contents)
                );
            }) {
                Ok(_) => println!("{}: OK", filepath.to_str().unwrap()),
                Err(err) => failed_tests.push(err),
            }
        }
        assert_ne!(count, 0, "Found at least one file");

        if !failed_tests.is_empty() {
            return Err(std::io::Error::new(
                std::io::ErrorKind::Other,
                format!("{} tests failed", failed_tests.len()),
            ));
        }

        Ok(())
    }

    #[test]
    fn test_files_with_locals() -> Result<(), std::io::Error> {
        let crate_root: std::path::PathBuf = std::env::var("CARGO_MANIFEST_DIR").unwrap().into();
        let input_dir = crate_root
            .join("src")
            .join("highlighting")
            .join("snapshots")
            .join("files-with-locals");
        let dir = read_dir(&input_dir).unwrap();

        let mut failed_tests = vec![];
        let mut count = 0;
        for entry in dir {
            count += 1;
            let entry = entry?;
            let filepath = entry.path();
            let mut file = File::open(&filepath)?;
            let mut contents = String::new();
            file.read_to_string(&mut contents)?;

            let language = crate::highlighting::test::SYNTAX_SET
                .with(|syntax_set| {
                    FileInfo::new(filepath.to_string_lossy().as_ref(), &contents, None)
                        .determine_language(syntax_set)
                })
                .unwrap();

            let document = language.highlight_document(&contents, true).unwrap();

            // TODO: I'm not sure if there's a better way to run the snapshots without
            // panicing and then catching, but this will do for now.
            match std::panic::catch_unwind(|| {
                insta::assert_snapshot!(
                    filepath.strip_prefix(&input_dir).unwrap().to_str().unwrap(),
                    snapshot_treesitter_syntax_and_symbols(&document, &contents)
                );
            }) {
                Ok(_) => println!("{}: OK", filepath.to_str().unwrap()),
                Err(err) => failed_tests.push(err),
            }
        }
        assert_ne!(count, 0, "Found at least one file");

        if !failed_tests.is_empty() {
            return Err(std::io::Error::new(
                std::io::ErrorKind::Other,
                format!("{} tests failed", failed_tests.len()),
            ));
        }

        Ok(())
    }
}
