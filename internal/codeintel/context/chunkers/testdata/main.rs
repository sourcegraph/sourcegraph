#![allow(macro_expanded_macro_exports_accessed_by_absolute_paths)]

#[macro_use]
extern crate rocket;

use std::path;

use protobuf::Message;
use rocket::serde::json::{json, Json, Value as JsonValue};
use scip_treesitter_languages::parsers::BundledParser;
use serde::Deserialize;
use sg_syntax::{ScipHighlightQuery, SourcegraphQuery};

#[post("/", format = "application/json", data = "<q>")]
fn syntect(q: Json<SourcegraphQuery>) -> JsonValue {
    // TODO(slimsag): In an ideal world we wouldn't be relying on catch_unwind
    // and instead Syntect would return Result types when failures occur. This
    // will require some non-trivial work upstream:
    // https://github.com/trishume/syntect/issues/98
    let result = std::panic::catch_unwind(|| sg_syntax::syntect_highlight(q.into_inner()));
    match result {
        Ok(v) => v,
        Err(_) => json!({"error": "panic while highlighting code", "code": "panic"}),
    }
}

// TODO: Once we're confident we don't need this anymore, we can remove this entirely
// and just have the `scip` endpoint. But I figured I would make it available at least
// for now, since I'm working on doing that.
#[post("/lsif", format = "application/json", data = "<q>")]
fn lsif(q: Json<SourcegraphQuery>) -> JsonValue {
    match sg_syntax::lsif_highlight(q.into_inner()) {
        Ok(v) => v,
        Err(err) => err,
    }
}

#[post("/scip", format = "application/json", data = "<q>")]
fn scip(q: Json<ScipHighlightQuery>) -> JsonValue {
    match sg_syntax::scip_highlight(q.into_inner()) {
        Ok(v) => v,
        Err(err) => err,
    }
}

#[derive(Deserialize, Default, Debug)]
pub struct SymbolQuery {
    filename: String,
    content: String,
}

pub fn jsonify_err(e: impl ToString) -> JsonValue {
    json!({"error": e.to_string()})
}

#[post("/symbols", format = "application/json", data = "<q>")]
fn symbols(q: Json<SymbolQuery>) -> JsonValue {
    let path = path::Path::new(&q.filename);
    let extension = match match path.extension() {
        Some(vals) => vals,
        None => {
            return json!({"error": "Extensionless file"});
        }
    }
    .to_str()
    {
        Some(vals) => vals,
        None => {
            return json!({"error": "Invalid codepoint"});
        }
    };
    let parser = match BundledParser::get_parser_from_extension(extension) {
        Some(parser) => parser,
        None => return json!({"error": "Could not infer parser from extension"}),
    };

    let document = match scip_syntax::get_symbols(&parser, q.content.as_bytes()) {
        Ok(vals) => vals,
        Err(err) => {
            return jsonify_err(err);
        }
    };

    let encoded = match document.write_to_bytes() {
        Ok(vals) => vals,
        Err(err) => {
            return jsonify_err(err);
        }
    };

    json!({"scip": base64::encode(encoded), "plaintext": false})
}

#[get("/health")]
fn health() -> &'static str {
    "OK"
}

#[catch(404)]
fn not_found() -> JsonValue {
    json!({"error": "resource not found", "code": "resource_not_found"})
}

#[launch]
fn rocket() -> _ {
    // Exits with a code zero if the environment variable SANITY_CHECK equals
    // to "true". This enables testing that the current program is in a runnable
    // state against the platform it's being executed on.
    //
    // See https://github.com/GoogleContainerTools/container-structure-test
    match std::env::var("SANITY_CHECK") {
        Ok(v) if v == "true" => {
            println!("Sanity check passed, exiting without error");
            std::process::exit(0)
        }
        _ => {}
    };

    // load configurations on-startup instead of on-first-request.
    // TODO: load individual languages lazily on-request instead, currently
    // CONFIGURATIONS.get will load every configured configuration together.
    scip_treesitter_languages::highlights::CONFIGURATIONS
        .get(&scip_treesitter_languages::parsers::BundledParser::Go);

    // Only list features if QUIET != "true"
    match std::env::var("QUIET") {
        Ok(v) if v == "true" => {}
        _ => sg_syntax::list_features(),
    };

    rocket::build()
        .mount("/", routes![syntect, lsif, scip, symbols, health])
        .register("/", catchers![not_found])
}
