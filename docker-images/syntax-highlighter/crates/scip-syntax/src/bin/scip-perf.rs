use std::{path::Path, time::Instant};

use clap::Parser;
use scip_syntax::locals;
use scip_treesitter_languages::parsers::BundledParser;
use walkdir::WalkDir;

#[derive(Parser)]
#[command(author, version, about, long_about = None)]
struct Arguments {
    /// Root directory to run local navigation over
    root_dir: String,
}

struct ParseTiming {
    pub file_path: String,
    pub file_size: usize,
    pub duration: std::time::Duration,
}

fn parse_files(dir: &Path) -> Vec<ParseTiming> {
    let config = scip_syntax::languages::get_local_configuration(BundledParser::Go).unwrap();
    let extension = "go";

    let mut timings = vec![];

    for entry in WalkDir::new(dir) {
        let entry = entry.unwrap();
        let entry = entry.path();

        match entry.extension() {
            Some(ext) if extension == ext => {}
            _ => continue,
        }

        let start = Instant::now();
        let source = match std::fs::read_to_string(entry) {
            Ok(source) => source,
            Err(err) => {
                eprintln!(
                    "Skipping '{}', because '{}'",
                    entry.strip_prefix(dir).unwrap().display(),
                    err
                );
                continue;
            }
        };
        let source_bytes = source.as_bytes();
        let mut parser = config.get_parser();
        let tree = parser.parse(source_bytes, None).unwrap();

        locals::parse_tree(config, &tree, source_bytes);
        let finish = Instant::now();

        timings.push(ParseTiming {
            file_path: entry.file_stem().unwrap().to_string_lossy().to_string(),
            file_size: source_bytes.len(),
            duration: finish - start,
        });
    }

    timings
}

fn measure_parsing() {
    let args = Arguments::parse();
    println!("Measuring parsing");
    let start = Instant::now();

    let root = Path::new(&args.root_dir);

    let mut timings = parse_files(root);
    timings.sort_by(|a, b| a.duration.cmp(&b.duration));
    println!("Slowest files:");
    for timing in timings.iter().rev().take(10) {
        println!(
            "{} ({}kb): {:?} ",
            timing.file_path,
            timing.file_size / 1000,
            timing.duration
        );
    }

    let finish = Instant::now();

    println!("Done {:?}", finish - start);
}

fn main() {
    // TODO: parameterize
    let measure = "parsing";

    match measure {
        "parsing" => measure_parsing(),
        _ => panic!("Unknown measure: {}", measure),
    }
}
