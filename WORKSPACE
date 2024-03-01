load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "platforms",
    sha256 = "8150406605389ececb6da07cbcb509d5637a3ab9a24bc69b1101531367d89d74",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/platforms/releases/download/0.0.8/platforms-0.0.8.tar.gz",
        "https://github.com/bazelbuild/platforms/releases/download/0.0.8/platforms-0.0.8.tar.gz",
    ],
)

http_archive(
    name = "bazel_skylib",
    sha256 = "66ffd9315665bfaafc96b52278f57c7e2dd09f5ede279ea6d39b2be471e7e3aa",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.4.2/bazel-skylib-1.4.2.tar.gz",
        "https://github.com/bazelbuild/bazel-skylib/releases/download/1.4.2/bazel-skylib-1.4.2.tar.gz",
    ],
)

load("@bazel_skylib//:workspace.bzl", "bazel_skylib_workspace")

bazel_skylib_workspace()

http_archive(
    name = "aspect_bazel_lib",
    patch_args = ["-p1"],
    patches = [
        "//third_party/bazel_lib:use_default_shell_env.patch",
    ],
    sha256 = "4d6010ca5e3bb4d7045b071205afa8db06ec11eb24de3f023d74d77cca765f66",
    strip_prefix = "bazel-lib-1.39.0",
    url = "https://github.com/aspect-build/bazel-lib/releases/download/v1.39.0/bazel-lib-v1.39.0.tar.gz",
)

# rules_js defines an older rules_nodejs, so we override it here
http_archive(
    name = "rules_nodejs",
    sha256 = "162f4adfd719ba42b8a6f16030a20f434dc110c65dc608660ef7b3411c9873f9",
    strip_prefix = "rules_nodejs-6.0.2",
    url = "https://github.com/bazelbuild/rules_nodejs/releases/download/v6.0.2/rules_nodejs-v6.0.2.tar.gz",
)

http_archive(
    name = "aspect_rules_js",
    patch_args = ["-p1"],
    patches = [
        "//third_party/rules_js:use_default_shell_env.patch",
    ],
    sha256 = "76a04ef2120ee00231d85d1ff012ede23963733339ad8db81f590791a031f643",
    strip_prefix = "rules_js-1.34.1",
    url = "https://github.com/aspect-build/rules_js/releases/download/v1.34.1/rules_js-v1.34.1.tar.gz",
)

http_archive(
    name = "aspect_rules_ts",
    sha256 = "bd3e7b17e677d2b8ba1bac3862f0f238ab16edb3e43fb0f0b9308649ea58a2ad",
    strip_prefix = "rules_ts-2.1.0",
    url = "https://github.com/aspect-build/rules_ts/releases/download/v2.1.0/rules_ts-v2.1.0.tar.gz",
)

http_archive(
    name = "aspect_rules_swc",
    sha256 = "8eb9e42ed166f20cacedfdb22d8d5b31156352eac190fc3347db55603745a2d8",
    strip_prefix = "rules_swc-1.1.0",
    url = "https://github.com/aspect-build/rules_swc/releases/download/v1.1.0/rules_swc-v1.1.0.tar.gz",
)

http_archive(
    name = "io_bazel_rules_go",
    patch_args = ["-p1"],
    patches = [
        "//third_party/rules_go:package_main.patch",
    ],
    sha256 = "de7974538c31f76658e0d333086c69efdf6679dbc6a466ac29e65434bf47076d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.45.0/rules_go-v0.45.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.45.0/rules_go-v0.45.0.zip",
    ],
)

http_archive(
    name = "rules_proto",
    sha256 = "dc3fb206a2cb3441b485eb1e423165b231235a1ea9b031b4433cf7bc1fa460dd",
    strip_prefix = "rules_proto-5.3.0-21.7",
    urls = [
        "https://github.com/bazelbuild/rules_proto/archive/refs/tags/5.3.0-21.7.tar.gz",
    ],
)

http_archive(
    name = "rules_proto_grpc",
    sha256 = "9ba7299c5eb6ec45b6b9a0ceb9916d0ab96789ac8218269322f0124c0c0d24e2",
    strip_prefix = "rules_proto_grpc-4.5.0",
    urls = ["https://github.com/rules-proto-grpc/rules_proto_grpc/releases/download/4.5.0/rules_proto_grpc-4.5.0.tar.gz"],
)

http_archive(
    name = "rules_buf",
    sha256 = "bc2488ee497c3fbf2efee19ce21dceed89310a08b5a9366cc133dd0eb2118498",
    strip_prefix = "rules_buf-0.2.0",
    urls = [
        "https://github.com/bufbuild/rules_buf/archive/refs/tags/v0.2.0.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "b7387f72efb59f876e4daae42f1d3912d0d45563eac7cb23d1de0b094ab588cf",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.34.0/bazel-gazelle-v0.34.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.34.0/bazel-gazelle-v0.34.0.tar.gz",
    ],
)

http_archive(
    name = "rules_rust",
    integrity = "sha256-ZQGWDD5NoySV0eEAfe0HaaU0yxlcMN6jaqVPnYo/A2E=",
    urls = ["https://github.com/bazelbuild/rules_rust/releases/download/0.38.0/rules_rust-v0.38.0.tar.gz"],
)

# Container rules
http_archive(
    name = "rules_oci",
    sha256 = "d41d0ba7855f029ad0e5ee35025f882cbe45b0d5d570842c52704f7a47ba8668",
    strip_prefix = "rules_oci-1.4.3",
    url = "https://github.com/bazel-contrib/rules_oci/releases/download/v1.4.3/rules_oci-v1.4.3.tar.gz",
)

http_archive(
    name = "rules_pkg",
    sha256 = "8c20f74bca25d2d442b327ae26768c02cf3c99e93fad0381f32be9aab1967675",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/0.8.1/rules_pkg-0.8.1.tar.gz",
        "https://github.com/bazelbuild/rules_pkg/releases/download/0.8.1/rules_pkg-0.8.1.tar.gz",
    ],
)

http_archive(
    name = "container_structure_test",
    sha256 = "42edb647b51710cb917b5850380cc18a6c925ad195986f16e3b716887267a2d7",
    strip_prefix = "container-structure-test-104a53ede5f78fff72172639781ac52df9f5b18f",
    urls = ["https://github.com/GoogleContainerTools/container-structure-test/archive/104a53ede5f78fff72172639781ac52df9f5b18f.zip"],
)

http_archive(
    name = "buildifier_prebuilt",
    sha256 = "e46c16180bc49487bfd0f1ffa7345364718c57334fa0b5b67cb5f27eba10f309",
    strip_prefix = "buildifier-prebuilt-6.1.0",
    urls = ["https://github.com/keith/buildifier-prebuilt/archive/6.1.0.tar.gz"],
)

http_archive(
    name = "aspect_cli",
    repo_mapping = {
        "@com_github_smacker_go_tree_sitter": "@aspectcli-com_github_smacker_go_tree_sitter",
    },
    sha256 = "045f0186edb25706dfe77d9c4916eec630a2b2736f9abb59e37eaac122d4b771",
    strip_prefix = "aspect-cli-5.8.20",
    url = "https://github.com/aspect-build/aspect-cli/archive/5.8.20.tar.gz",
)

http_archive(
    name = "rules_multirun",
    sha256 = "9cd384e42b2da00104f0e18f25e66285aa21f64b573c667638a7a213206885ab",
    strip_prefix = "rules_multirun-0.6.1",
    url = "https://github.com/keith/rules_multirun/archive/refs/tags/0.6.1.tar.gz",
)

http_archive(
    name = "with_cfg.bzl",
    sha256 = "c6b80cad298afa8a46bc01cd96df4f4d8660651101f6bf5af58f2724e349017d",
    strip_prefix = "with_cfg.bzl-0.2.1",
    url = "https://github.com/fmeum/with_cfg.bzl/releases/download/v0.2.1/with_cfg.bzl-v0.2.1.tar.gz",
)

http_archive(
    name = "rules_apko",
    patch_args = ["-p1"],
    patches = [
        # required due to https://github.com/chainguard-dev/apko/issues/1052
        "//third_party/rules_apko:repository_label_strip.patch",
    ],
    sha256 = "f176171f95ee2b6eef1572c6da796d627940a1e898a32d476a2d7a9a99332960",
    strip_prefix = "rules_apko-1.2.2",
    url = "https://github.com/chainguard-dev/rules_apko/releases/download/v1.2.2/rules_apko-v1.2.2.tar.gz",
)

# hermetic_cc_toolchain setup ================================
HERMETIC_CC_TOOLCHAIN_VERSION = "v2.2.1"

# Please note that we only use hermetic-cc for local development purpose and Nix, at it eases the path to cross-compile
# so we can produce container images locally on Mac laptops.
#
# @jhchabran See https://github.com/sourcegraph/sourcegraph/pull/55969, there is an ongoing issue with UBSAN
# and treesitter, that breaks the compilation of syntax-highlighter. Since we only use
# hermetic_cc for local development purposes, while it's a bit heavy handed for a --copt, it's acceptable
# at this point. Passing --copt=-fno-sanitize=undefined sadly doesn't fix the problem, which is why
# we have to patch to inject the flag.
http_archive(
    name = "hermetic_cc_toolchain",
    patch_args = ["-p1"],
    patches = [
        "//third_party/hermetic_cc:disable_ubsan.patch",
    ],
    sha256 = "3b8107de0d017fe32e6434086a9568f97c60a111b49dc34fc7001e139c30fdea",
    urls = [
        "https://mirror.bazel.build/github.com/uber/hermetic_cc_toolchain/releases/download/{0}/hermetic_cc_toolchain-{0}.tar.gz".format(HERMETIC_CC_TOOLCHAIN_VERSION),
        "https://github.com/uber/hermetic_cc_toolchain/releases/download/{0}/hermetic_cc_toolchain-{0}.tar.gz".format(HERMETIC_CC_TOOLCHAIN_VERSION),
    ],
)

# rules_js setup ================================
load("@aspect_rules_js//js:repositories.bzl", "rules_js_dependencies")

rules_js_dependencies()

# node toolchain setup ==========================
load("@rules_nodejs//nodejs:repositories.bzl", "nodejs_register_toolchains")

nodejs_register_toolchains(
    name = "nodejs",
    node_version = "20.8.0",
)

# rules_js npm setup ============================
load("@aspect_rules_js//npm:npm_import.bzl", "npm_translate_lock")

npm_translate_lock(
    name = "npm",
    npm_package_target_name = "{dirname}_pkg",
    npmrc = "//:.npmrc",
    pnpm_lock = "//:pnpm-lock.yaml",
    # Required for ESLint test targets.
    # See https://github.com/aspect-build/rules_js/issues/239
    # See `public-hoist-pattern[]=*eslint*` in the `.npmrc` of this monorepo.
    public_hoist_packages = {
        "@typescript-eslint/eslint-plugin": [""],
        "@typescript-eslint/parser@5.56.0_qxbo2xm47qt6fxnlmgbosp4hva": [""],
        "eslint-config-prettier": [""],
        "eslint-plugin-ban": [""],
        "eslint-plugin-etc": [""],
        "eslint-plugin-import": [""],
        "eslint-plugin-jest-dom": [""],
        "eslint-plugin-jsdoc": [""],
        "eslint-plugin-jsx-a11y": [""],
        "eslint-plugin-react@7.32.1_eslint_8.34.0": [""],
        "eslint-plugin-react-hooks": [""],
        "eslint-plugin-rxjs": [""],
        "eslint-plugin-unicorn": [""],
        "eslint-plugin-unused-imports": [""],
        "eslint-import-resolver-node": [""],
    },
    verify_node_modules_ignored = "//:.bazelignore",
)

# rules_ts npm setup ============================
load("@npm//:repositories.bzl", "npm_repositories")

npm_repositories()

load("@aspect_rules_ts//ts:repositories.bzl", "rules_ts_dependencies")

rules_ts_dependencies(ts_version = "4.9.5")

# rules_swc setup ==============================
load("@aspect_rules_swc//swc:dependencies.bzl", "rules_swc_dependencies")

rules_swc_dependencies()

load("@aspect_rules_swc//swc:repositories.bzl", "LATEST_SWC_VERSION", "swc_register_toolchains")

swc_register_toolchains(
    name = "swc",
    swc_version = LATEST_SWC_VERSION,
)

# rules_esbuild setup ===========================
http_archive(
    name = "aspect_rules_esbuild",
    sha256 = "84419868e43c714c0d909dca73039e2f25427fc04f352d2f4f7343ca33f60deb",
    strip_prefix = "rules_esbuild-0.15.3",
    url = "https://github.com/aspect-build/rules_esbuild/releases/download/v0.15.3/rules_esbuild-v0.15.3.tar.gz",
)

load("@aspect_rules_esbuild//esbuild:dependencies.bzl", "rules_esbuild_dependencies")

rules_esbuild_dependencies()

# Register a toolchain containing esbuild npm package and native bindings
load("@aspect_rules_esbuild//esbuild:repositories.bzl", "LATEST_ESBUILD_VERSION", "esbuild_register_toolchains")

esbuild_register_toolchains(
    name = "esbuild",
    esbuild_version = LATEST_ESBUILD_VERSION,
)

# Go toolchain setup

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")
load("//:linter_deps.bzl", "linter_dependencies")
load("//:deps.bzl", "go_dependencies")

go_repository(
    name = "com_github_aws_aws_sdk_go_v2_service_ssooidc",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/aws/aws-sdk-go-v2/service/ssooidc",
    sum = "h1:HFiiRkf1SdaAmV3/BHOFZ9DjFynPHj8G/UIO1lQS+fk=",
    version = "v1.17.3",
)

# Overrides the default provided protobuf dep from rules_go by a more
# recent one.
go_repository(
    name = "org_golang_google_protobuf",
    build_file_proto_mode = "disable_global",
    importpath = "google.golang.org/protobuf",
    sum = "h1:pPC6BG5ex8PDFnkbrGU3EixyhKcQ2aDuBS36lqK/C7I=",
    version = "v1.32.0",
)

# Pin protoc-gen-go-grpc to 1.3.0
# See also //:gen-go-grpc
go_repository(
    name = "org_golang_google_grpc_cmd_protoc_gen_go_grpc",
    build_file_proto_mode = "disable_global",
    importpath = "google.golang.org/grpc/cmd/protoc-gen-go-grpc",
    sum = "h1:rNBFJjBCOgVr9pWD7rs/knKL4FRTKgpZmsRfV214zcA=",
    version = "v1.3.0",
)  # keep

# Pin specific version for aspect-cli's gazelle rules, with versions
# that it requires but that our codebase doesnt support.
go_repository(
    name = "aspectcli-com_github_smacker_go_tree_sitter",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/smacker/go-tree-sitter",
    sum = "h1:DxgjlvWYsb80WEN2Zv3WqJFAg2DKjUQJO6URGdf1x6Y=",
    version = "v0.0.0-20230720070738-0d0a9f78d8f8",
)  # keep

load("@aspect_cli//:go.bzl", aspect_cli_deps = "deps")

aspect_cli_deps()

# gazelle:repository_macro deps.bzl%go_dependencies
go_dependencies()

go_rules_dependencies()

go_register_toolchains(
    nogo = "@//:sg_nogo",
    version = "1.21.6",
)

linter_dependencies()

gazelle_dependencies()

# rust toolchain setup
load("@rules_rust//rust:repositories.bzl", "rules_rust_dependencies", "rust_register_toolchains", "rust_repository_set")

rules_rust_dependencies()

rust_version = "1.73.0"

rust_register_toolchains(
    edition = "2021",
    # Keep in sync with docker-images/syntax-highlighter/Dockerfile
    # and docker-images/syntax-highlighter/rust-toolchain.toml
    versions = [
        rust_version,
    ],
)

# Needed for locally cross-compiling rust binaries to linux/amd64 on a Mac laptop, when seeking to
# create container images in local for testing purposes.
rust_repository_set(
    name = "macos_arm_64",
    edition = "2021",
    exec_triple = "aarch64-apple-darwin",
    extra_target_triples = ["x86_64-unknown-linux-gnu"],
    versions = [rust_version],
)

load("@rules_rust//crate_universe:defs.bzl", "crates_repository")

crates_repository(
    name = "crate_index",
    cargo_config = "//docker-images/syntax-highlighter:.cargo/config.toml",
    cargo_lockfile = "//docker-images/syntax-highlighter:Cargo.lock",
    # this file has to be manually created and it will be filled when
    # the target is ran.
    # To regenerate this file run: CARGO_BAZEL_REPIN=1 bazel sync --only=crate_index
    lockfile = "//docker-images/syntax-highlighter:Cargo.Bazel.lock",
    # glob doesn't work in WORKSPACE files: https://github.com/bazelbuild/bazel/issues/11935
    manifests = [
        "//docker-images/syntax-highlighter:Cargo.toml",
        "//docker-images/syntax-highlighter:crates/syntax-analysis/Cargo.toml",
        "//docker-images/syntax-highlighter:crates/tree-sitter-all-languages/Cargo.toml",
        "//docker-images/syntax-highlighter:crates/scip-syntax/Cargo.toml",
    ],
)

load("@crate_index//:defs.bzl", "crate_repositories")

crate_repositories()

load("@hermetic_cc_toolchain//toolchain:defs.bzl", zig_toolchains = "toolchains")

zig_toolchains()

# containers steup       ===============================
load("@rules_oci//oci:dependencies.bzl", "rules_oci_dependencies")

rules_oci_dependencies()

load("@rules_oci//oci:repositories.bzl", "LATEST_CRANE_VERSION", "oci_register_toolchains")

oci_register_toolchains(
    name = "oci",
    crane_version = LATEST_CRANE_VERSION,
    # Uncommenting the zot toolchain will cause it to be used instead of crane for some tasks.
    # Note that it does not support docker-format images.
    # zot_version = LATEST_ZOT_VERSION,
)

# Optional, for oci_tarball rule
load("@rules_pkg//:deps.bzl", "rules_pkg_dependencies")

rules_pkg_dependencies()

load("//dev:oci_deps.bzl", "oci_deps")

oci_deps()

load("@container_structure_test//:repositories.bzl", "container_structure_test_register_toolchain")

container_structure_test_register_toolchain(name = "cst")

load("//dev:tool_deps.bzl", "tool_deps")

tool_deps()

load("//tools/release:schema_deps.bzl", "schema_deps")

schema_deps()

# Buildifier
load("@buildifier_prebuilt//:deps.bzl", "buildifier_prebuilt_deps")

buildifier_prebuilt_deps()

load("@buildifier_prebuilt//:defs.bzl", "buildifier_prebuilt_register_toolchains")

buildifier_prebuilt_register_toolchains()

load("@rules_proto//proto:repositories.bzl", "rules_proto_dependencies", "rules_proto_toolchains")

rules_proto_dependencies()

rules_proto_toolchains()

load("@rules_proto_grpc//:repositories.bzl", "rules_proto_grpc_repos", "rules_proto_grpc_toolchains")
load("@rules_proto_grpc//go:repositories.bzl", rules_proto_grpc_go_repos = "go_repos")
load("@rules_proto_grpc//doc:repositories.bzl", rules_proto_grpc_doc_repos = "doc_repos")

rules_proto_grpc_toolchains()

rules_proto_grpc_repos()

rules_proto_grpc_go_repos()

rules_proto_grpc_doc_repos()

load("@rules_buf//buf:repositories.bzl", "rules_buf_dependencies", "rules_buf_toolchains")

rules_buf_dependencies()

rules_buf_toolchains(
    sha256 = "05dfb45d2330559d258e1230f5a25e154f0a328afda2a434348b5ba4c124ece7",
    version = "v1.28.1",
)

load("@rules_buf//gazelle/buf:repositories.bzl", "gazelle_buf_dependencies")

gazelle_buf_dependencies()

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

# keep revision up-to-date with client/browser/scripts/build-inline-extensions.js
http_archive(
    name = "sourcegraph_extensions_bundle",
    add_prefix = "bundle",
    build_file_content = """
package(default_visibility = ["//visibility:public"])

exports_files(["bundle"])

filegroup(
    name = "srcs",
    srcs = glob(["**"]),
)
    """,
    integrity = "sha256-Spx8LyM7k+dsGOlZ4TdAq+CNk5EzvYB/oxnY4zGpqPg=",
    strip_prefix = "sourcegraph-extensions-bundles-5.0.1",
    url = "https://github.com/sourcegraph/sourcegraph-extensions-bundles/archive/v5.0.1.zip",
)

load("//dev:schema_migrations.bzl", "schema_migrations")

schema_migrations(
    name = "schemas_migrations",
)

load("@rules_apko//apko:repositories.bzl", "apko_register_toolchains", "rules_apko_dependencies")

rules_apko_dependencies()

apko_register_toolchains(
    name = "apko",
    register = False,
)

register_toolchains("//:apko_linux_toolchain")

register_toolchains("//:apko_darwin_arm64_toolchain")

register_toolchains("//:apko_darwin_amd64_toolchain")

load("@rules_apko//apko:translate_lock.bzl", "translate_apko_lock")

# rules_apko setup
translate_apko_lock(
    name = "batcheshelper_lock",
    lock = "@//wolfi-images:batcheshelper.lock.json",
)

load("@batcheshelper_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "blobstore_lock",
    lock = "@//wolfi-images:blobstore.lock.json",
)

load("@blobstore_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "bundled-executor_lock",
    lock = "@//wolfi-images:bundled-executor.lock.json",
)

load("@bundled-executor_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "executor_lock",
    lock = "@//wolfi-images:executor.lock.json",
)

load("@executor_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "cadvisor_lock",
    lock = "@//wolfi-images:cadvisor.lock.json",
)

load("@cadvisor_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "cloud-mi2_lock",
    lock = "@//wolfi-images:cloud-mi2.lock.json",
)

load("@cloud-mi2_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "executor-kubernetes_lock",
    lock = "@//wolfi-images:executor-kubernetes.lock.json",
)

load("@executor-kubernetes_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "gitserver_lock",
    lock = "@//wolfi-images:gitserver.lock.json",
)

load("@gitserver_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "grafana_lock",
    lock = "@//wolfi-images:grafana.lock.json",
)

load("@grafana_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "jaeger-agent_lock",
    lock = "@//wolfi-images:jaeger-agent.lock.json",
)

load("@jaeger-agent_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "jaeger-all-in-one_lock",
    lock = "@//wolfi-images:jaeger-all-in-one.lock.json",
)

load("@jaeger-all-in-one_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "node-exporter_lock",
    lock = "@//wolfi-images:node-exporter.lock.json",
)

load("@node-exporter_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "opentelemetry-collector_lock",
    lock = "@//wolfi-images:opentelemetry-collector.lock.json",
)

load("@opentelemetry-collector_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "postgres-exporter_lock",
    lock = "@//wolfi-images:postgres-exporter.lock.json",
)

load("@postgres-exporter_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "postgresql-12-codeinsights_lock",
    lock = "@//wolfi-images:postgresql-12-codeinsights.lock.json",
)

load("@postgresql-12-codeinsights_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "postgresql-12_lock",
    lock = "@//wolfi-images:postgresql-12.lock.json",
)

load("@postgresql-12_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "prometheus-gcp_lock",
    lock = "@//wolfi-images:prometheus-gcp.lock.json",
)

load("@prometheus-gcp_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "prometheus_lock",
    lock = "@//wolfi-images:prometheus.lock.json",
)

load("@prometheus_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "qdrant_lock",
    lock = "@//wolfi-images:qdrant.lock.json",
)

load("@qdrant_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "redis-exporter_lock",
    lock = "@//wolfi-images:redis-exporter.lock.json",
)

load("@redis-exporter_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "redis_lock",
    lock = "@//wolfi-images:redis.lock.json",
)

load("@redis_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "repo-updater_lock",
    lock = "@//wolfi-images:repo-updater.lock.json",
)

load("@repo-updater_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "search-indexer_lock",
    lock = "@//wolfi-images:search-indexer.lock.json",
)

load("@search-indexer_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "searcher_lock",
    lock = "@//wolfi-images:searcher.lock.json",
)

load("@searcher_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "server_lock",
    lock = "@//wolfi-images:server.lock.json",
)

load("@server_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "sourcegraph-base_lock",
    lock = "@//wolfi-images:sourcegraph-base.lock.json",
)

load("@sourcegraph-base_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "sourcegraph_lock",
    lock = "@//wolfi-images:sourcegraph.lock.json",
)

load("@sourcegraph_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "sourcegraph-dev_lock",
    lock = "@//wolfi-images:sourcegraph-dev.lock.json",
)

load("@sourcegraph-dev_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "symbols_lock",
    lock = "@//wolfi-images:symbols.lock.json",
)

load("@symbols_lock//:repositories.bzl", "apko_repositories")

apko_repositories()

translate_apko_lock(
    name = "syntax-highlighter_lock",
    lock = "@//wolfi-images:syntax-highlighter.lock.json",
)

load("@syntax-highlighter_lock//:repositories.bzl", "apko_repositories")

apko_repositories()
