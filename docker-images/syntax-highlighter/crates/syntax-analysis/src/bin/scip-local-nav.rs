use std::{fs, path::Path};

use clap::Parser;
use scip::{types::Document, write_message_to_file};
use syntax_analysis::{languages::LocalConfiguration, locals::find_locals};
use tree_sitter_all_languages::ParserId;
use walkdir::WalkDir;

// TODO: Could probably add some filters here for managing/enabling/disabling
// certain filetypes.

#[derive(Parser)]
#[command(author, version, about, long_about = None)]
struct Arguments {
    /// Root directory to run local navigation over
    root_dir: String,
}

fn parse_files(config: &LocalConfiguration, root: &Path, dir: &Path) -> Vec<Document> {
    // TODO: Filter

    let extension = "go";

    let mut documents = vec![];
    for entry in WalkDir::new(dir) {
        let entry = entry.unwrap();
        let entry = entry.path();

        match entry.extension() {
            Some(ext) => {
                if ext != extension {
                    continue;
                }
            }
            None => continue,
        }

        let contents = fs::read_to_string(entry).expect("is a valid file");
        let mut parser = config.get_parser();
        let tree = parser
            .parse(contents.as_bytes(), None)
            .expect("to parse the tree");

        let occs = find_locals(config, &tree, &contents, Default::default()).unwrap();

        let mut doc = Document::new();
        doc.language = "go".to_string();
        doc.relative_path = entry
            .strip_prefix(root)
            .unwrap()
            .to_string_lossy()
            .to_string();
        doc.occurrences = occs;
        doc.symbols = vec![];

        // All the symbols are local, so we don't want to do this.
        // doc.symbols = doc
        //     .occurrences
        //     .iter()
        //     .map(|o| scip::types::SymbolInformation {
        //         symbol: o.symbol.clone(),
        //         ..Default::default()
        //     })
        //     .collect();

        documents.push(doc);
    }

    documents
}

fn main() {
    println!("scip-local-nav");

    let args = Arguments::parse();
    let directory = Path::new(&args.root_dir);

    let mut index = scip::types::Index {
        metadata: Some(scip::types::Metadata {
            tool_info: Some(scip::types::ToolInfo {
                name: "scip-local-nav".to_string(),
                version: "0.0.1".to_string(),
                arguments: vec![],
                ..Default::default()
            })
            .into(),
            project_root: "file://".to_string() + directory.to_str().unwrap(),
            ..Default::default()
        })
        .into(),
        ..Default::default()
    };

    // let config = syntax_analysis::languages::get_local_configuration(ParserId::Go).unwrap();
    // index
    //     .documents
    //     .extend(parse_files(config, directory, directory));

    println!("{:?}", index.documents.len());
    write_message_to_file(directory.join("index.scip"), index).expect("to write the file");
}
