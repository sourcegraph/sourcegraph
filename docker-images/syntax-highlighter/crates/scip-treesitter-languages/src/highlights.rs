use std::collections::HashMap;

use paste::paste;
use scip::types::SyntaxKind;
use scip_macros::include_scip_query;
use tree_sitter_highlight::{Highlight, HighlightConfiguration};

use crate::parsers::BundledParser;

#[rustfmt::skip]
// Table of (@CaptureGroup, SyntaxKind) mapping.
//
// Any capture defined in a query will be mapped to the following SyntaxKind via the highlighter.
//
// To extend what types of captures are included, simply add a line below that takes a particular
// match group that you're interested in and map it to a new SyntaxKind.
//
// We can also define our own new capture types that we want to use and add to queries to provide
// particular highlights if necessary.
//
// (I can also add per-language mappings for these if we want, but you could also just do that with
//  unique match groups. For example `@rust-bracket`, or similar. That doesn't need any
//  particularly new rust code to be written. You can just modify queries for that)
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
    ( $($name: tt),* ) => {{
        use crate::parsers::BundledParser;

        let mut m = HashMap::new();
        let highlight_names = MATCHES_TO_SYNTAX_KINDS.iter().map(|hl| hl.0).collect::<Vec<&str>>();

        $(
            {
                // Create HighlightConfiguration language
                let mut lang = HighlightConfiguration::new(
                    paste! { BundledParser::$name.get_language() },
                    include_scip_query!($name, "highlights"),
                    include_scip_query!($name, "injections"),
                    include_scip_query!($name, "locals"),
                ).expect(stringify!("parser for '{}' must be compiled", $name));

                // Associate highlights with configuration
                lang.configure(&highlight_names);

                // Insert into configurations, so we only create once at startup.
                m.insert(BundledParser::$name, lang);
            }
        )*

        // Manually insert the typescript and tsx languages because the
        // tree-sitter-typescript crate doesn't have a language() function.
        {
            let highlights = vec![
                include_scip_query!("typescript", "highlights"),
                include_scip_query!("javascript", "highlights"),
            ];
            let mut lang = HighlightConfiguration::new(
                BundledParser::Typescript.get_language(),
                &highlights.join("\n"),
                include_scip_query!("typescript", "injections"),
                include_scip_query!("typescript", "locals"),
            ).expect("parser for 'typescript' must be compiled");
            lang.configure(&highlight_names);
            m.insert(BundledParser::Typescript, lang);
        }
        {
            let highlights = vec![
                include_scip_query!("tsx", "highlights"),
                include_scip_query!("typescript", "highlights"),
                include_scip_query!("javascript", "highlights"),
            ];
            let mut lang = HighlightConfiguration::new(
                BundledParser::Tsx.get_language(),
                &highlights.join("\n"),
                include_scip_query!("tsx", "injections"),
                include_scip_query!("tsx", "locals"),
            ).expect("parser for 'tsx' must be compiled");
            lang.configure(&highlight_names);
            m.insert(BundledParser::Tsx, lang);
        }

        m
    }}
}

lazy_static::lazy_static! {
    pub static ref CONFIGURATIONS: HashMap<BundledParser, HighlightConfiguration> = {
        // NOTE: typescript/tsx crates are included, even though not listed below.

        // You can add any new crate::parsers::Parser variants here.
        create_configurations!(
            C,
            Cpp,
            C_Sharp,
            Dart,
            Go,
            Java,
            Javascript,
            Jsonnet,
            Kotlin,
            Matlab,
            Nickel,
            Perl,
            Pod,
            Python,
            Ruby,
            Rust,
            Scala,
            Sql,
            Xlsg,
            Zig
        )
    };
}

pub fn get_highlighting_configuration(filetype: &str) -> Option<&'static HighlightConfiguration> {
    BundledParser::get_parser(filetype).and_then(|parser| CONFIGURATIONS.get(&parser))
}

pub fn get_syntax_kind_for_hl(hl: Highlight) -> SyntaxKind {
    MATCHES_TO_SYNTAX_KINDS[hl.0].1
}
