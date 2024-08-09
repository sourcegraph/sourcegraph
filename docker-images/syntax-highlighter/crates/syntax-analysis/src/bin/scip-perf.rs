// use std::{path::Path, time::Instant};

// use clap::Parser;
// use syntax_analysis::locals::{self};
// use tree_sitter_all_languages::ParserId;
// use walkdir::WalkDir;

// #[derive(Parser)]
// #[command(author, version, about, long_about = None)]
// struct Arguments {
//     /// What language to parse.
//     language: Language,

//     /// Root directory to run local navigation over
//     root_dir: String,
// }

// #[derive(clap::ValueEnum, Clone, Copy)]
// enum Language {
//     Go,
//     Java,
//     Matlab,
// }

// struct ParseTiming {
//     pub file_path: String,
//     pub file_size: usize,
//     pub duration: std::time::Duration,
// }

// fn parse_files(dir: &Path, language: Language) -> Vec<ParseTiming> {
//     let (config, extension) = match language {
//         Language::Go => (
//             syntax_analysis::languages::get_local_configuration(ParserId::Go).unwrap(),
//             "go",
//         ),
//         Language::Java => (
//             syntax_analysis::languages::get_local_configuration(ParserId::Java).unwrap(),
//             "java",
//         ),
//         Language::Matlab => (
//             syntax_analysis::languages::get_local_configuration(ParserId::Matlab).unwrap(),
//             "m",
//         ),
//     };

//     let mut timings = vec![];

//     for entry in WalkDir::new(dir) {
//         let entry = entry.unwrap();
//         let entry = entry.path();

//         match entry.extension() {
//             Some(ext) if extension == ext => {}
//             _ => continue,
//         }

//         let start = Instant::now();
//         let source = match std::fs::read_to_string(entry) {
//             Ok(source) => source,
//             Err(err) => {
//                 eprintln!(
//                     "Skipping '{}', because '{}'",
//                     entry.strip_prefix(dir).unwrap().display(),
//                     err
//                 );
//                 continue;
//             }
//         };
//         let mut parser = config.get_parser();
//         let tree = parser.parse(source.as_bytes(), None).unwrap();

//         locals::find_locals(config, &tree, &source, Default::default()).unwrap();
//         let finish = Instant::now();

//         timings.push(ParseTiming {
//             file_path: entry.file_stem().unwrap().to_string_lossy().to_string(),
//             file_size: source.len(),
//             duration: finish - start,
//         });
//     }

//     timings
// }

// fn measure_parsing() {
//     let args = Arguments::parse();
//     println!("Measuring parsing");
//     let start = Instant::now();

//     let root = Path::new(&args.root_dir);

//     let mut timings = parse_files(root, args.language);
//     timings.sort_by(|a, b| a.duration.cmp(&b.duration));
//     println!("Slowest files:");
//     for timing in timings.iter().rev().take(10) {
//         println!(
//             "{} ({}kb): {:?}",
//             timing.file_path,
//             timing.file_size / 1000,
//             timing.duration
//         );
//     }

//     let finish = Instant::now();

//     println!("Done {:?}", finish - start);
// }

// fn main() {
//     // TODO: parameterize
//     let measure = "parsing";

//     match measure {
//         "parsing" => measure_parsing(),
//         _ => panic!("Unknown measure: {}", measure),
//     }
// }
//
fn main() {}
