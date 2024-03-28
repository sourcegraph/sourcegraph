pub mod syntect_html;
pub mod syntect_scip;

use anyhow::anyhow;
use protobuf::Message;
use std::fmt::{Debug, Display, Formatter};
use std::path::Path;
use syntect::html::ClassStyle;

use syntect::parsing::{SyntaxReference, SyntaxSet};

pub mod tree_sitter;
use crate::highlighting::syntect_html::ClassedTableGenerator;
use tree_sitter_all_languages::ParserId;

#[derive(Default)]
pub struct FileInfo<'a> {
    // This is kept as a String instead of a PathBuf because:
    // 1. Sourcegraph doesn't support non-UTF-8 values in paths.
    // 2. The input received from the network is UTF-8.
    // 3. We're not using this path to call any OS APIs
    //    which would require Path.
    // 4. It avoids extra conversions when interacting with
    //    syntect or in the ad-hoc language detection code that
    //    we currently have.
    path: String,
    pub contents: &'a str,
    pub language: Option<&'a str>,
}

impl<'a> FileInfo<'a> {
    pub fn new(path: &str, contents: &'a str, language: Option<&'a str>) -> FileInfo<'a> {
        FileInfo {
            path: path.to_string(),
            contents,
            language,
        }
    }

    pub fn new_from_extension(
        extension: &str,
        contents: &'a str,
        language: Option<&'a str>,
    ) -> FileInfo<'a> {
        FileInfo {
            path: format!(
                "__highlighter_synthetic__{}{}",
                if extension.starts_with('.') { "" } else { "." },
                extension
            ),
            contents,
            language,
        }
    }

    fn extension(&self) -> Option<&str> {
        Path::new(&self.path)
            .extension()
            .and_then(|osstr| osstr.to_str())
    }

    pub fn determine_language(
        &self,
        syntax_set: &SyntaxSet,
    ) -> Result<TreeSitterLanguageName, LanguageDetectionError> {
        let name = SublimeLanguageName {
            raw: match self.find_matching_syntax_reference(syntax_set) {
                Ok(language) => language.name.clone(),
                Err(e) => return Err(e),
            },
        }
        .into_tree_sitter_name(self);
        Ok(name)
    }

    pub(crate) fn find_matching_syntax_reference<'s>(
        &self,
        syntax_set: &'s SyntaxSet,
    ) -> Result<&'s SyntaxReference, LanguageDetectionError> {
        // If filetype is passed, we should choose that if possible.
        if let Some(filetype) = self.language {
            // This is `find_syntax_by_name` except that it doesn't care about
            // case sensitivity or anything like that.
            //
            // This makes it just a lost simpler to move between frontend and backend.
            // At some point, we need a definitive list for this.
            if let Some(language) = syntax_set
                .syntaxes()
                .iter()
                .rev()
                .find(|&s| filetype == s.name.to_lowercase())
            {
                return Ok(language);
            }
        }

        if self.path.is_empty() {
            // Legacy codepath, kept for backwards-compatability with old clients.
            return match syntax_set.find_syntax_by_extension(self.extension().unwrap_or("")) {
                Some(v) => Ok(v),
                // Fall back: Determine syntax definition by first line.
                None => match syntax_set.find_syntax_by_first_line(self.contents) {
                    Some(v) => Ok(v),
                    None => Err(LanguageDetectionError::InvalidExtension),
                },
            };
        }

        let file_name = Path::new(&self.path).file_name().and_then(|n| n.to_str());
        let extension = self.extension();

        // Override syntect's language detection for conflicting file extensions because
        // it's impossible to express this logic in a syntax definition.
        struct Override {
            extension: &'static str,
            prefix_langs: Vec<(&'static str, &'static str)>,
            default: &'static str,
        }
        let overrides = vec![
            Override {
                extension: "cls",
                prefix_langs: vec![("%", "TeX"), ("\\", "TeX")],
                default: "Apex",
            },
            Override {
                extension: "xlsg",
                prefix_langs: vec![],
                default: "xlsg",
            },
        ];

        if let Some(extension) = extension {
            for override_ in overrides.iter() {
                if override_.extension != extension {
                    continue;
                }
                let name = match override_
                    .prefix_langs
                    .iter()
                    .find(|(prefix, _)| self.contents.starts_with(prefix))
                {
                    Some((_, lang)) => lang,
                    None => override_.default,
                };
                return Ok(syntax_set
                    .find_syntax_by_name(name)
                    .unwrap_or_else(|| syntax_set.find_syntax_plain_text()));
            }
        }

        Ok(syntax_set
            // First try to find a syntax whose "extension" matches our file
            // name. This is done due to some syntaxes matching an "extension"
            // that is actually a whole file name (e.g. "Dockerfile" or "CMakeLists.txt")
            // see https://github.com/trishume/syntect/pull/170
            .find_syntax_by_extension(file_name.unwrap_or(""))
            .or_else(|| syntax_set.find_syntax_by_extension(extension.unwrap_or("")))
            .or_else(|| syntax_set.find_syntax_by_first_line(self.contents))
            .unwrap_or_else(|| syntax_set.find_syntax_plain_text()))
    }
}

// Language names as used by syntect & Sublime grammars
struct SublimeLanguageName {
    raw: String,
}

impl SublimeLanguageName {
    fn into_tree_sitter_name(self, file_info: &FileInfo<'_>) -> TreeSitterLanguageName {
        if self.raw.is_empty() || self.raw.to_lowercase() == "plain text" {
            let extension = file_info.extension().unwrap_or("");
            if extension != "" {
                match ParserId::from_file_extension(extension) {
                    Some(parser_id) => return TreeSitterLanguageName::new(parser_id.name()),
                    _ => {}
                }
            }
        }

        // Written in an unusual style so that we can:
        // 1. Avoid case-sensitive comparison
        // 2. We can look up corresponding Sublime Grammars easily
        //    when using case-sensitive text search
        let normalized_name = self.raw.as_str().to_lowercase();
        let normalized_name = match normalized_name {
            x if x == "Rust Enhanced".to_lowercase() => "rust",
            x if x == "JS Custom - React".to_lowercase() => "javascript",
            x if x == "TypeScriptReact".to_lowercase() => {
                if file_info.path.ends_with(".tsx") {
                    "tsx"
                } else {
                    "typescript"
                }
            }
            _ => &normalized_name,
        };
        // TODO: When https://github.com/sourcegraph/sourcegraph/issues/56376
        // is fixed, we should be able to detect the language without
        // using syntect. We can directly then map Linguist/enry names
        // to the data that we have instead of separately maintaining
        // another set of names for Tree-sitter.
        TreeSitterLanguageName::new(normalized_name)
    }
}

// Somewhat ad-hoc set of names based on existing Tree-sitter grammars.
pub struct TreeSitterLanguageName {
    raw: String,
}

impl Display for TreeSitterLanguageName {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.raw)
    }
}

impl TreeSitterLanguageName {
    pub fn new(s: &str) -> TreeSitterLanguageName {
        let mut s = s.to_lowercase();
        if &s == "c++" {
            s = "cpp".to_string();
        } else if &s == "c#" {
            s = "c_sharp".to_string();
        }
        TreeSitterLanguageName { raw: s }
    }
}

pub enum HighlightingBackend<'a> {
    SyntectHtml {
        syntax_set: &'a SyntaxSet,
        line_length_limit: Option<usize>,
    },
    SyntectScip {
        syntax_set: &'a SyntaxSet,
        line_length_limit: Option<usize>,
    },
    TreeSitter {
        include_locals: bool,
    },
}

pub struct HighlightedText {
    pub payload: String,
    pub kind: PayloadKind,
    pub grammar: String,
}

#[derive(PartialEq, Eq)]
pub enum PayloadKind {
    Html,
    Base64EncodedScip,
}

impl<'b> HighlightingBackend<'b> {
    /// The result is HTML for SyntectHtml and base64-encoded SCIP data for
    /// other backends.
    pub fn highlight(&self, file_info: &FileInfo<'_>) -> anyhow::Result<HighlightedText> {
        match self {
            HighlightingBackend::SyntectHtml {
                syntax_set,
                line_length_limit,
            } => {
                let syntax_ref = file_info.find_matching_syntax_reference(syntax_set)?;
                Ok(HighlightedText {
                    payload: ClassedTableGenerator::new(
                        syntax_set,
                        syntax_ref,
                        file_info.contents,
                        *line_length_limit,
                        ClassStyle::SpacedPrefixed { prefix: "hl-" },
                    )
                    .generate(),
                    kind: PayloadKind::Html,
                    grammar: format!("Sublime {}", syntax_ref.name),
                })
            }
            HighlightingBackend::SyntectScip {
                syntax_set,
                line_length_limit,
            } => {
                let syntax_ref = file_info.find_matching_syntax_reference(syntax_set)?;
                let document = syntect_scip::DocumentGenerator::new(
                    syntax_set,
                    syntax_ref,
                    file_info.contents,
                    *line_length_limit,
                )
                .generate();
                Ok(HighlightedText {
                    payload: base64::encode(document.write_to_bytes()?),
                    kind: PayloadKind::Base64EncodedScip,
                    grammar: format!("Sublime {}", syntax_ref.name),
                })
            }
            HighlightingBackend::TreeSitter { include_locals } => {
                let language = match file_info.language {
                    None => {
                        return Err(anyhow!(
                            "Tree-sitter backend requires a language to be specified"
                        ))
                    }
                    Some(s) => SublimeLanguageName { raw: s.to_string() },
                };
                let language = language.into_tree_sitter_name(file_info);
                match language.highlight_document(file_info.contents, *include_locals) {
                    Ok(document) => match document.write_to_bytes() {
                        Err(e) => Err(anyhow!("failed to serialize document {:?}", e)),
                        Ok(doc) => Ok(HighlightedText {
                            payload: base64::encode(doc),
                            kind: PayloadKind::Base64EncodedScip,
                            grammar: format!("Tree-sitter {}", language),
                        }),
                    },
                    Err(tree_sitter_highlight::Error::InvalidLanguage) => Err(anyhow!(
                        "{} is not a valid filetype for Tree-sitter",
                        language
                    )),
                    Err(err) => Err(anyhow!("highlighting failed: {}", err)),
                }
            }
        }
    }
}

pub enum LanguageDetectionError {
    InvalidExtension,
}

impl Debug for LanguageDetectionError {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        (&self as &dyn Display).fmt(f)
    }
}

impl std::error::Error for LanguageDetectionError {}

impl Display for LanguageDetectionError {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            LanguageDetectionError::InvalidExtension => f.write_str("invalid extension"),
        }
    }
}

#[cfg(test)]
mod test {
    use syntect::parsing::SyntaxSet;

    use super::*;

    // Local to tests as this crate takes the syntax set as an input argument.
    thread_local! {
        pub(crate) static SYNTAX_SET: SyntaxSet = SyntaxSet::load_defaults_newlines();
    }

    fn test_syntect_lang_detection_impl(path: &str, content: &str, expected_language: &str) {
        let file_info = FileInfo::new(path, content, None);
        SYNTAX_SET.with(|syntax_set| {
            let result = file_info.find_matching_syntax_reference(syntax_set);
            assert_eq!(&result.unwrap().name, expected_language);
        })
    }

    #[test]
    fn test_syntect_lang_detection() {
        let cases = [("foo.cls", "%", "TeX"), ("foo.cls", "/**", "Apex")];

        for (path, content, lang) in cases {
            test_syntect_lang_detection_impl(path, content, lang);
        }
    }

    #[test]
    fn test_extension() {
        let mapping = [
            ("foo", None),
            ("foo.x", Some("x")),
            ("foo.x.y", Some("y")),
            (".abc", None),
            ("a/.abc", None),
            ("a.c/b.d", Some("d")),
            ("a.c/b", None),
        ];

        for (path, expected_ext) in mapping {
            assert_eq!(FileInfo::new(path, "", None).extension(), expected_ext);
        }
    }
}
