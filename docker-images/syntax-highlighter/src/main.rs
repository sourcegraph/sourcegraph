#![allow(macro_expanded_macro_exports_accessed_by_absolute_paths)]

#[macro_use]
extern crate rocket;

use rocket::serde::json::{json, Json, Value as JsonValue};
use syntect_server::{ScipHighlightQuery, SourcegraphQuery};

fn merge_ok_err<A>(r: Result<A, A>) -> A {
    r.unwrap_or_else(|e| e)
}

#[post("/", format = "application/json", data = "<q>")]
fn syntect(q: Json<SourcegraphQuery>) -> JsonValue {
    // TODO(slimsag): In an ideal world we wouldn't be relying on catch_unwind
    // and instead Syntect would return Result types when failures occur. This
    // will require some non-trivial work upstream:
    // https://github.com/trishume/syntect/issues/98
    let result = std::panic::catch_unwind(|| syntect_server::syntect_highlight(q.into_inner()));
    match result {
        Ok(v) => merge_ok_err(v),
        Err(_) => json!({"error": "panic while highlighting code", "code": "panic"}),
    }
}

// TODO: Once we're confident we don't need this anymore, we can remove this entirely
// and just have the `scip` endpoint. But I figured I would make it available at least
// for now, since I'm working on doing that.
#[post("/lsif", format = "application/json", data = "<q>")]
fn lsif(q: Json<SourcegraphQuery>) -> JsonValue {
    merge_ok_err(syntect_server::lsif_highlight(q.into_inner()))
}

#[post("/scip", format = "application/json", data = "<q>")]
fn scip(q: Json<ScipHighlightQuery>) -> JsonValue {
    merge_ok_err(syntect_server::scip_highlight(q.into_inner()))
}

pub fn jsonify_err(e: impl ToString) -> JsonValue {
    json!({"error": e.to_string()})
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
    // syntax_analysis::highlighting::tree_sitter::CONFIGURATIONS
    //     .get(&tree_sitter_all_languages::ParserId::Go);

    // Only list features if QUIET != "true"
    match std::env::var("QUIET") {
        Ok(v) if v == "true" => {}
        _ => syntect_server::list_features(),
    };

    rocket::build()
        .mount("/", routes![syntect, lsif, scip, health])
        .register("/", catchers![not_found])
}
