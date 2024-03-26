def _schema_migrations(rctx):
    """
    This repository is used to download the schema migrations from GCS.

    We use the GCS JSON API directly instead of gsutil or gcloud because:
        - gsutil may spend up to a ~1m20s trying to contact metadata.google.internal
            without a discovered way to disable that
        - gcloud disallows unauthed access to an even public bucket
    """
    jq_path = rctx.path(Label("@jq//:jq"))

    rctx.file("BUILD.bazel", content = """
package(default_visibility = ["//visibility:public"])

exports_files(["archives"])

filegroup(
    name = "srcs",
    srcs = glob(["**"]),
)
""")

    rctx.report_progress("Listing GCS bucket contents")

    rctx.download("https://storage.googleapis.com/storage/v1/b/schemas-migrations/o?prefix=migrations/migrations-", "bucket_contents.json")

    result = rctx.execute([
        jq_path,
        ".items | map({name, mediaLink, generation})",
        "bucket_contents.json",
    ])
    if result.return_code != 0:
        fail("Failed to extract bucket data from GCS API: {}".format(result.stderr))

    rctx.delete("bucket_contents.json")

    output = json.decode(result.stdout)

    rctx.execute(["mkdir", "archives"])

    rctx.report_progress("Downloading schema migrations from GCS")

    download_tokens = []
    for file in output:
        download_tokens.append(rctx.download(
            file["mediaLink"],
            "archives/" + file["name"].split("/")[-1],
            canonical_id = file["generation"],
            block = False,
        ))

    for token in download_tokens:
        token.wait()

schema_migrations = repository_rule(
    implementation = _schema_migrations,
)
