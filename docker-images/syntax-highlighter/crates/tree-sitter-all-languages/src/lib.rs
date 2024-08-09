use std::collections::HashSet;

use tree_sitter::Language;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum ParserId {
    C,
    // Cpp,
    // #[allow(non_camel_case_types)]
    // C_Sharp,
    // Dart,
    // Go,
    // Hack,
    // Java,
    // Javascript,
    // Jsonnet,
    // Kotlin,
    // Magik,
    // Matlab,
    // Nickel,
    // Perl,
    // Pkl,
    // Pod,
    // Python,
    // Ruby,
    // Rust,
    // Scala,
    // Sql,
    // Xlsg,
    // Zig,

    // These two are special cases since we process them
    // in a way where they can inherit tree-sitter queries from others languages
    // Typescript,
    // Tsx,
}

impl ParserId {
    pub fn language(self) -> Language {
        match self {
            ParserId::C => tree_sitter_c::language(),
            // ParserId::Cpp => tree_sitter_cpp::language(),
            // ParserId::C_Sharp => tree_sitter_c_sharp::language(),
            // ParserId::Dart => tree_sitter_dart::language(),
            // ParserId::Go => tree_sitter_go::language(),
            // ParserId::Hack => tree_sitter_hack::language(),
            // ParserId::Java => tree_sitter_java::language(),
            // ParserId::Javascript => tree_sitter_javascript::language(),
            // ParserId::Jsonnet => tree_sitter_jsonnet::language(),
            // ParserId::Kotlin => tree_sitter_kotlin::language(),
            // ParserId::Magik => tree_sitter_magik::language(),
            // ParserId::Matlab => tree_sitter_matlab::language(),
            // ParserId::Nickel => tree_sitter_nickel::language(),
            // ParserId::Perl => tree_sitter_perl::language(),
            // ParserId::Pkl => tree_sitter_pkl::language(),
            // ParserId::Pod => tree_sitter_pod::language(),
            // ParserId::Python => tree_sitter_python::language(),
            // ParserId::Ruby => tree_sitter_ruby::language(),
            // ParserId::Rust => tree_sitter_rust::language(),
            // ParserId::Scala => tree_sitter_scala::language(),
            // ParserId::Sql => tree_sitter_sql::language(),
            // ParserId::Typescript => tree_sitter_typescript::language_typescript(),
            // ParserId::Tsx => tree_sitter_typescript::language_tsx(),
            // ParserId::Xlsg => tree_sitter_xlsg::language(),
            // ParserId::Zig => tree_sitter_zig::language(),
        }
    }

    pub fn get_parser(self) -> tree_sitter::Parser {
        let mut parser = tree_sitter::Parser::new();
        parser.set_language(&self.language()).expect("Error assigning language to parser, likely a version mismatch between compiled grammar and tree-sitter library.");
        parser
    }

    pub fn from_name(name: &str) -> Option<Self> {
        match name {
            "c" => Some(ParserId::C),
            // "cpp" => Some(ParserId::Cpp),
            // "c_sharp" => Some(ParserId::C_Sharp),
            // "dart" => Some(ParserId::Dart),
            // "go" => Some(ParserId::Go),
            // "hack" => Some(ParserId::Hack),
            // "java" => Some(ParserId::Java),
            // "javascript" => Some(ParserId::Javascript),
            // "jsonnet" => Some(ParserId::Jsonnet),
            // "kotlin" => Some(ParserId::Kotlin),
            // "magik" => Some(ParserId::Magik),
            // "matlab" => Some(ParserId::Matlab),
            // "nickel" => Some(ParserId::Nickel),
            // "perl" => Some(ParserId::Perl),
            // "pkl" => Some(ParserId::Pkl),
            // "pod" => Some(ParserId::Pod),
            // "python" => Some(ParserId::Python),
            // "ruby" => Some(ParserId::Ruby),
            // "rust" => Some(ParserId::Rust),
            // "scala" => Some(ParserId::Scala),
            // "sql" => Some(ParserId::Sql),
            // "typescript" => Some(ParserId::Typescript),
            // "tsx" => Some(ParserId::Tsx),
            // "xlsg" => Some(ParserId::Xlsg),
            // "zig" => Some(ParserId::Zig),
            _ => None,
        }
    }

    pub fn name(&self) -> &str {
        match self {
            ParserId::C => "c",
            // ParserId::Cpp => "cpp",
            // ParserId::C_Sharp => "c_sharp",
            // ParserId::Dart => "dart",
            // ParserId::Go => "go",
            // ParserId::Hack => "hack",
            // ParserId::Java => "java",
            // ParserId::Javascript => "javascript",
            // ParserId::Jsonnet => "jsonnet",
            // ParserId::Kotlin => "kotlin",
            // ParserId::Magik => "magik",
            // ParserId::Matlab => "matlab",
            // ParserId::Nickel => "nickel",
            // ParserId::Perl => "perl",
            // ParserId::Pkl => "pkl",
            // ParserId::Pod => "pod",
            // ParserId::Python => "python",
            // ParserId::Ruby => "ruby",
            // ParserId::Rust => "rust",
            // ParserId::Scala => "scala",
            // ParserId::Sql => "sql",
            // ParserId::Typescript => "typescript",
            // ParserId::Tsx => "tsx",
            // ParserId::Xlsg => "xlsg",
            // ParserId::Zig => "zig",
        }
    }

    pub fn language_extensions(&self) -> HashSet<&str> {
        let ar = {
            match self {
                // ParserId::Go => vec!["go"],
                // ParserId::Java => vec!["java"],
                // ParserId::Javascript => vec!["js"],
                // ParserId::Typescript => vec!["ts"],
                // ParserId::Python => vec!["py"],
                _ => vec![],
            }
        };

        HashSet::from_iter(ar)
    }

    // TODO(SuperAuguste): language detection library
    pub fn from_file_extension(extension: &str) -> Option<Self> {
        match extension {
            "c" => Some(ParserId::C),
            // "cpp" => Some(ParserId::Cpp),
            // "cs" => Some(ParserId::C_Sharp),
            // "dart" => Some(ParserId::Dart),
            // "go" => Some(ParserId::Go),
            // "hack" => Some(ParserId::Hack),
            // "java" => Some(ParserId::Java),
            // "js" => Some(ParserId::Javascript),
            // "jsonnet" => Some(ParserId::Jsonnet),
            // "m" => Some(ParserId::Matlab),
            // "magik" => Some(ParserId::Magik),
            // "kt" => Some(ParserId::Kotlin),
            // "ncl" => Some(ParserId::Nickel),
            // "pl" => Some(ParserId::Perl),
            // "pkl" => Some(ParserId::Pkl),
            // "pod" => Some(ParserId::Pod),
            // "py" => Some(ParserId::Python),
            // "rb" => Some(ParserId::Ruby),
            // "rs" => Some(ParserId::Rust),
            // "scala" => Some(ParserId::Scala),
            // "sql" => Some(ParserId::Sql),
            // "ts" => Some(ParserId::Typescript),
            // "tsx" => Some(ParserId::Tsx),
            // "xlsg" => Some(ParserId::Xlsg),
            // "zig" => Some(ParserId::Zig),
            _ => None,
        }
    }
}
