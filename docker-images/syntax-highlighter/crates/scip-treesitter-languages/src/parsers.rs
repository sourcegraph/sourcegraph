use std::collections::HashSet;

use tree_sitter::Language;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum BundledParser {
    C,
    Cpp,
    #[allow(non_camel_case_types)]
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
    Zig,

    // These two are special cases
    Typescript,
    Tsx,
}

impl BundledParser {
    pub fn get_language(self) -> Language {
        match self {
            BundledParser::C => tree_sitter_c::language(),
            BundledParser::Cpp => tree_sitter_cpp::language(),
            BundledParser::C_Sharp => tree_sitter_c_sharp::language(),
            BundledParser::Dart => tree_sitter_dart::language(),
            BundledParser::Go => tree_sitter_go::language(),
            BundledParser::Java => tree_sitter_java::language(),
            BundledParser::Javascript => tree_sitter_javascript::language(),
            BundledParser::Jsonnet => tree_sitter_jsonnet::language(),
            BundledParser::Kotlin => tree_sitter_kotlin::language(),
            BundledParser::Matlab => tree_sitter_matlab::language(),
            BundledParser::Nickel => tree_sitter_nickel::language(),
            BundledParser::Perl => tree_sitter_perl::language(),
            BundledParser::Pod => tree_sitter_pod::language(),
            BundledParser::Python => tree_sitter_python::language(),
            BundledParser::Ruby => tree_sitter_ruby::language(),
            BundledParser::Rust => tree_sitter_rust::language(),
            BundledParser::Scala => tree_sitter_scala::language(),
            BundledParser::Sql => tree_sitter_sql::language(),
            BundledParser::Typescript => tree_sitter_typescript::language_typescript(),
            BundledParser::Tsx => tree_sitter_typescript::language_tsx(),
            BundledParser::Xlsg => tree_sitter_xlsg::language(),
            BundledParser::Zig => tree_sitter_zig::language(),
        }
    }

    pub fn get_parser(name: &str) -> Option<Self> {
        match name {
            "c" => Some(BundledParser::C),
            "cpp" => Some(BundledParser::Cpp),
            "c_sharp" => Some(BundledParser::C_Sharp),
            "dart" => Some(BundledParser::Dart),
            "go" => Some(BundledParser::Go),
            "java" => Some(BundledParser::Java),
            "javascript" => Some(BundledParser::Javascript),
            "jsonnet" => Some(BundledParser::Jsonnet),
            "kotlin" => Some(BundledParser::Kotlin),
            "matlab" => Some(BundledParser::Matlab),
            "nickel" => Some(BundledParser::Nickel),
            "perl" => Some(BundledParser::Perl),
            "pod" => Some(BundledParser::Pod),
            "python" => Some(BundledParser::Python),
            "ruby" => Some(BundledParser::Ruby),
            "rust" => Some(BundledParser::Rust),
            "scala" => Some(BundledParser::Scala),
            "sql" => Some(BundledParser::Sql),
            "typescript" => Some(BundledParser::Typescript),
            "tsx" => Some(BundledParser::Tsx),
            "xlsg" => Some(BundledParser::Xlsg),
            "zig" => Some(BundledParser::Zig),
            _ => None,
        }
    }

    pub fn get_language_name(&self) -> &str {
        match self {
            BundledParser::C => "c",
            BundledParser::Cpp => "cpp",
            BundledParser::C_Sharp => "c_sharp",
            BundledParser::Dart => "dart",
            BundledParser::Go => "go",
            BundledParser::Java => "java",
            BundledParser::Javascript => "javascript",
            BundledParser::Jsonnet => "jsonnet",
            BundledParser::Kotlin => "kotlin",
            BundledParser::Matlab => "matlab",
            BundledParser::Nickel => "nickel",
            BundledParser::Perl => "perl",
            BundledParser::Pod => "pod",
            BundledParser::Python => "python",
            BundledParser::Ruby => "ruby",
            BundledParser::Rust => "rust",
            BundledParser::Scala => "scala",
            BundledParser::Sql => "sql",
            BundledParser::Typescript => "typescript",
            BundledParser::Tsx => "tsx",
            BundledParser::Xlsg => "xlsg",
            BundledParser::Zig => "zig",
        }
    }

    pub fn get_language_extensions(&self) -> HashSet<&str> {
        let ar = {
            match self {
                BundledParser::Go => vec!["go"],
                BundledParser::Java => vec!["java"],
                BundledParser::Javascript => vec!["js"],
                BundledParser::Typescript => vec!["ts"],
                BundledParser::Python => vec!["py"],
                _ => vec![],
            }
        };

        HashSet::from_iter(ar)
    }

    // TODO(SuperAuguste): language detection library
    pub fn get_parser_from_extension(name: &str) -> Option<Self> {
        match name {
            "c" => Some(BundledParser::C),
            "cpp" => Some(BundledParser::Cpp),
            "cs" => Some(BundledParser::C_Sharp),
            "dart" => Some(BundledParser::Dart),
            "go" => Some(BundledParser::Go),
            "java" => Some(BundledParser::Java),
            "js" => Some(BundledParser::Javascript),
            "jsonnet" => Some(BundledParser::Jsonnet),
            "m" => Some(BundledParser::Matlab),
            "kt" => Some(BundledParser::Kotlin),
            "ncl" => Some(BundledParser::Nickel),
            "pl" => Some(BundledParser::Perl),
            "pod" => Some(BundledParser::Pod),
            "py" => Some(BundledParser::Python),
            "rb" => Some(BundledParser::Ruby),
            "rs" => Some(BundledParser::Rust),
            "scala" => Some(BundledParser::Scala),
            "sql" => Some(BundledParser::Sql),
            "ts" => Some(BundledParser::Typescript),
            "tsx" => Some(BundledParser::Tsx),
            "xlsg" => Some(BundledParser::Xlsg),
            "zig" => Some(BundledParser::Zig),
            _ => None,
        }
    }
}
