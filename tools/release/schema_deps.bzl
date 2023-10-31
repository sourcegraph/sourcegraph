"""
This module defines the third party dependency containing all database schemas that the
migrator use to handle migrations. See the README.md in this folder for reference.
"""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_file")

def schema_deps():
    http_file(
        name = "schemas_archive",
        urls = ["https://storage.googleapis.com/schemas-migrations/dist/schemas-v5.2.123456.tar.gz"],
        sha256 = "a18114d5df31fd0f22c77b572601c834039353f66c8107c9364139f6a2d24571",
    )
