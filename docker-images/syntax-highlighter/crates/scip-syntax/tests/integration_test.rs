use std::{
    collections::{HashMap, HashSet},
    io::Write,
    process::{Command, Stdio},
};

use anyhow::{anyhow, bail, Context, Result};
use assert_cmd::{cargo::cargo_bin, prelude::*};
use camino::{Utf8Path, Utf8PathBuf};
use scip::types::Document;
use scip_syntax::{
    evaluate::Evaluator,
    index::{index_command, AnalysisFeatures, IndexMode, IndexOptions},
    io::read_index_from_file,
};

fn current_dir() -> Utf8PathBuf {
    Utf8PathBuf::from_path_buf(std::env::current_dir().unwrap()).unwrap()
}

lazy_static::lazy_static! {
    static ref BINARY_LOCATION: Utf8PathBuf = {
        match std::env::var("SCIP_SYNTAX_PATH") {
            Ok(va) => current_dir().join(va),
            _ => Utf8PathBuf::from_path_buf(cargo_bin("scip-syntax")).unwrap(),
        }
    };

    static ref BASE: Utf8PathBuf = {
        match std::env::var("CARGO_MANIFEST_DIR") {
            Ok(va) => current_dir().join(va),
            _ => current_dir()
        }
    };

    static ref JAVA_SCIP_INDEX: Utf8PathBuf = {
        match std::env::var("JAVA_SCIP_INDEX") {
            Ok(va) => current_dir().join(va),
            _ => BASE.join("testdata/java/index.scip")
        }
    };
}

use syntax_analysis::snapshot::{dump_document_with_config, EmitSymbol, SnapshotOptions};
use tar::{Builder, Header};

fn snapshot_syntax_document(doc: &scip::types::Document, source: &str) -> String {
    dump_document_with_config(
        doc,
        source,
        SnapshotOptions {
            emit_symbol: EmitSymbol::All,
            ..Default::default()
        },
    )
    .expect("dump document")
}

#[ignore]
#[test]
fn java_e2e_evaluation() {
    let dir = BASE.join("testdata/java");

    let out_dir = tempdir();

    let candidate = out_dir.join("index-tree-sitter.scip");

    index_command(
        "java".to_string(),
        IndexMode::Workspace {
            location: dir.clone(),
        },
        &candidate,
        &dir,
        None,
        None,
        IndexOptions {
            analysis_features: AnalysisFeatures::default(),
            fail_fast: true,
        },
    )
    .unwrap();

    let mut str = vec![];

    Evaluator::default()
        .evaluate_files(&candidate, &JAVA_SCIP_INDEX)
        .unwrap()
        .write_summary(
            &mut str,
            scip_syntax::evaluate::EvaluationOutputOptions {
                print_false_negatives: true,
                print_true_positives: true,
                print_false_positives: true,
                print_mapping: true,
                disable_colors: true,
            },
        )
        .unwrap();

    insta::assert_snapshot!("java_evaluation", String::from_utf8(str).unwrap());
}

#[ignore]
#[test]
fn java_files_indexing() {
    let out_dir = tempdir();
    let setup = indexing_data();

    let mut cmd = command("index");
    let output_location = out_dir.join("index.scip");
    let paths = extract_paths(&setup);

    prepare(&out_dir, &setup).unwrap();

    cmd.args(vec![
        "files",
        "--language",
        "java",
        "--out",
        output_location.as_str(),
    ])
    .current_dir(&out_dir)
    .args(paths)
    .assert()
    .success();

    let index = read_index_from_file(&output_location).unwrap();

    assert_eq!(extract_paths(&setup), extract_indexed_paths(&index));

    let index_snapshot = snapshot_from_files(&index.documents, &out_dir);

    insta::assert_snapshot!(index_snapshot);
}

#[ignore]
#[test]
fn java_workspace_indexing() {
    let out_dir = tempdir();
    let setup = indexing_data();

    let mut cmd = command("index");
    let output_location = out_dir.join("index.scip");

    prepare(&out_dir, &setup).unwrap();

    cmd.args(vec![
        "workspace",
        out_dir.as_str(),
        "--language",
        "java",
        "--out",
        output_location.as_str(),
    ])
    .assert()
    .success();

    let index = read_index_from_file(&output_location).unwrap();

    assert_eq!(extract_paths(&setup), extract_indexed_paths(&index));

    let index_snapshot = snapshot_from_files(&index.documents, &out_dir);

    insta::assert_snapshot!(index_snapshot);
}

#[ignore]
#[test]
fn java_tar_file_indexing() {
    let out_dir = tempdir();
    let setup = indexing_data();
    let tar_data = create_tar(&setup);

    let data = tar_data.unwrap();

    let mut cmd = command("index");
    let tar_file = out_dir.join("test.tar");
    let output_location = out_dir.join("index.scip");

    write_file_bytes(&tar_file, &data).unwrap();

    cmd.args(vec![
        "tar",
        tar_file.as_str(),
        "--language",
        "java",
        "--out",
        output_location.as_str(),
    ])
    .assert()
    .success();

    let index = read_index_from_file(&output_location).unwrap();

    assert_eq!(extract_paths(&setup), extract_indexed_paths(&index));

    let index_snapshot = snapshot_from_data(&index.documents, &setup);

    insta::assert_snapshot!(index_snapshot);
}

#[ignore]
#[test]
fn java_tar_stream_indexing() {
    let out_dir = tempdir();
    let setup = indexing_data();
    let tar_data = create_tar(&setup);

    let data = tar_data.unwrap();

    let mut cmd = command("index");
    let tar_file = out_dir.join("test.tar");
    let output_location = out_dir.join("index.scip");

    write_file_bytes(&tar_file, &data)
        .context("Failed to write tar data")
        .unwrap();

    let mut spawned = cmd
        .args(vec![
            "tar",
            "-",
            "--language",
            "java",
            "--out",
            output_location.as_str(),
        ])
        .stdin(Stdio::piped())
        .spawn()
        .unwrap();

    spawned.stdin.take().unwrap().write_all(&data).unwrap();

    let exit_status = spawned.wait().unwrap();

    assert_eq!(exit_status.code(), Some(0));

    let index = read_index_from_file(&output_location).unwrap();

    assert_eq!(extract_paths(&setup), extract_indexed_paths(&index));

    let index_snapshot = snapshot_from_data(&index.documents, &setup);

    insta::assert_snapshot!(index_snapshot);
}

fn prepare(temp: &Utf8Path, files: &HashMap<Utf8PathBuf, String>) -> Result<()> {
    for (path, contents) in files.iter() {
        let file_path = temp.join(path);
        write_file_string(&file_path, contents)?;
    }

    Ok(())
}

fn command(sub: &str) -> Command {
    let mut cmd = Command::new(BINARY_LOCATION.as_str());

    cmd.arg(sub);

    cmd
}

fn write_file_string(path: &Utf8Path, contents: &String) -> Result<()> {
    write_file_bytes(path, contents.as_bytes())
}

fn write_file_bytes(path: &Utf8Path, contents: &[u8]) -> Result<()> {
    use std::io::Write;

    let Some(parent) = path.parent() else {
        bail!("failed to find parent dir for {path}")
    };

    std::fs::create_dir_all(parent)
        .with_context(|| anyhow!("Failed to create all parent folders for {path}"))?;

    let output = std::fs::File::create(path)
        .with_context(|| anyhow!("Failed to open file {path} for writing"))?;
    let mut writer = std::io::BufWriter::new(output);
    writer.write_all(contents)?;

    Ok(())
}

fn create_tar(files: &HashMap<Utf8PathBuf, String>) -> Result<Vec<u8>, std::io::Error> {
    let mut ar = Builder::new(Vec::new());

    for (path, text) in files.iter() {
        let mut header = Header::new_gnu();
        let bytes = text.as_bytes();

        header
            .set_path(path.as_str())
            .expect("Failed to set path for archive entry");
        header.set_size(bytes.len() as u64);
        header.set_cksum();
        ar.append(&header, bytes).unwrap();
    }

    ar.into_inner()
}

fn indexing_data() -> HashMap<Utf8PathBuf, String> {
    HashMap::from([
        (
            Utf8PathBuf::from("src/main/java/globals.java"),
            include_str!("../testdata/globals.java").to_string(),
        ),
        (
            Utf8PathBuf::from("package-info.java"),
            include_str!("../testdata/package-info.java").to_string(),
        ),
    ])
}

fn extract_paths(setup: &HashMap<Utf8PathBuf, String>) -> HashSet<String> {
    setup.keys().map(|pb| pb.to_string()).collect()
}

fn extract_indexed_paths(index: &scip::types::Index) -> HashSet<String> {
    index
        .documents
        .iter()
        .map(|pb| pb.relative_path.clone())
        .collect()
}

fn snapshot_from_files(docs: &[Document], project_root: &Utf8Path) -> String {
    let mut str = String::new();
    let mut docs = docs.to_owned();
    docs.sort_by_key(|doc| doc.relative_path.clone());

    for doc in docs {
        let path = project_root.join(doc.relative_path.clone());
        let contents = std::fs::read_to_string(path.clone())
            .with_context(|| anyhow!("Failed to read path {path}"))
            .unwrap();

        str.push_str(&format_snapshot_document(&doc, &contents));
    }

    str
}

fn format_snapshot_document(doc: &scip::types::Document, contents: &str) -> String {
    let mut str = String::new();
    str.push_str(format!("//----FILE={}\n", doc.relative_path).as_str());
    str.push_str(&snapshot_syntax_document(doc, contents));
    str.push_str("\n\n");

    str
}

fn snapshot_from_data(docs: &[Document], data: &HashMap<Utf8PathBuf, String>) -> String {
    let mut str = String::new();
    let mut docs = docs.to_owned();
    docs.sort_by_key(|doc| doc.relative_path.clone());

    for doc in docs {
        let contents = data
            .get(&Utf8PathBuf::from(&doc.relative_path))
            .context(format!("Failed to find {} in data", &doc.relative_path))
            .unwrap();

        str.push_str(&format_snapshot_document(&doc, contents));
    }

    str
}

fn tempdir() -> Utf8PathBuf {
    Utf8PathBuf::from_path_buf(tempfile::tempdir().unwrap().into_path()).expect("non-utf8 tempdir")
}
