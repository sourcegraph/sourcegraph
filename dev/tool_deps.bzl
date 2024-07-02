"Third party dev tooling dependencies"

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")

DOCSITE_VERSION = "1.9.4"
SRC_CLI_VERSION = "5.3.0"
KUBEBUILDER_ASSETS_VERSION = "1.28.0"
CTAGS_VERSION = "6.0.0.2783f009"
PACKER_VERSION = "1.8.3"
P4_FUSION_VERSION = "v1.13.2-sg.04a293a"
GH_VERSION = "2.45.0"
PGUTILS_VERSION = "ad082497"
LINEAR_SDK_VERSION = "21.1.0"
SCIP_GO_VERSION = "v0.1.14"

GH_BUILDFILE = """
filegroup(
    name = "gh",
    srcs = ["bin/gh"],
    visibility = ["//visibility:public"],
)
"""

SRC_CLI_BUILDFILE = """
filegroup(
    name = "src-cli-{}",
    srcs = ["src"],
    visibility = ["//visibility:public"],
)
"""

KUBEBUILDER_ASSETS_BUILDFILE = """
filegroup(
    name = "kubebuilder-assets",
    srcs = glob(["*"]),
    visibility = ["//visibility:public"],
)
"""

GCLOUD_VERSION = "456.0.0"
GCLOUD_BUILDFILE = """
package(default_visibility = ["//visibility:public"])

exports_files(["gcloud", "gsutil", "bq", "git-credential-gcloud"])
"""
GCLOUD_PATCH_CMDS = [
    "ln -s google-cloud-sdk/bin/gcloud gcloud",
    "ln -s google-cloud-sdk/bin/gsutil gsutil",
    "ln -s google-cloud-sdk/bin/bq bq",
    "ln -s google-cloud-sdk/bin/git-credential-gcloud.sh git-credential-gcloud",
]

PACKER_BUILDFILE = """
filegroup(
    name = "packer-{}",
    srcs = ["packer"],
    visibility = ["//visibility:public"],
)
"""

PGUTILS_BUILDFILE = """\
package(default_visibility = ["//visibility:public"])
filegroup(
    name = "files",
    srcs = glob(["**/*"]),
)
"""

CHROMIUM_BUILDFILE = """
load("@aspect_rules_js//js:defs.bzl", "js_library")
js_library(
    name = "chromium",
    srcs = ["{}"],
    data = glob(["**/*"]),
    visibility = ["//visibility:public"],
)
"""

def tool_deps():
    "Repository rules to fetch third party tooling used for dev purposes"

    # Docsite #
    http_file(
        name = "docsite_darwin_amd64",
        urls = ["https://github.com/sourcegraph/docsite/releases/download/v{0}/docsite_v{0}_darwin_amd64".format(DOCSITE_VERSION)],
        sha256 = "f3ad94e1398cc30e45518c82bd6fa9f7c386a8c395811ba49def24113215a2d9",
        executable = True,
    )

    http_file(
        name = "docsite_darwin_arm64",
        urls = ["https://github.com/sourcegraph/docsite/releases/download/v{0}/docsite_v{0}_darwin_arm64".format(DOCSITE_VERSION)],
        sha256 = "b817d794537f38720d5c07eb323e729391b2d4ff85d1dac691e3cfe7a3cb6d13",
        executable = True,
    )

    http_file(
        name = "docsite_linux_amd64",
        urls = ["https://github.com/sourcegraph/docsite/releases/download/v{0}/docsite_v{0}_linux_amd64".format(DOCSITE_VERSION)],
        sha256 = "7d60a55eb5017ebeb3a523143bd3007f74297db685491fad84499b2c60b3a872",
        executable = True,
    )

    # src-cli #
    http_archive(
        name = "src-cli-linux-amd64",
        build_file_content = SRC_CLI_BUILDFILE.format("linux-amd64"),
        sha256 = "417a4329b0abf557a5fc36eedb626c53aaf886e48a49a594d98c095beaba0d02",
        url = "https://github.com/sourcegraph/src-cli/releases/download/{0}/src-cli_{0}_linux_amd64.tar.gz".format(SRC_CLI_VERSION),
    )

    http_archive(
        name = "src-cli-darwin-amd64",
        build_file_content = SRC_CLI_BUILDFILE.format("darwin-amd64"),
        sha256 = "10c24717d97d54c7380011ce297bbd398242408ba11ad55b0952eb96a08bf84c",
        url = "https://github.com/sourcegraph/src-cli/releases/download/{0}/src-cli_{0}_darwin_amd64.tar.gz".format(SRC_CLI_VERSION),
    )

    http_archive(
        name = "src-cli-darwin-arm64",
        build_file_content = SRC_CLI_BUILDFILE.format("darwin-arm64"),
        sha256 = "d2100e9dce86036c405490b89ab0dec40ee427884dead883c4ba69cc474caf45",
        url = "https://github.com/sourcegraph/src-cli/releases/download/{0}/src-cli_{0}_darwin_arm64.tar.gz".format(SRC_CLI_VERSION),
    )

    # Needed for internal/appliance tests
    http_archive(
        name = "kubebuilder-assets-darwin-arm64",
        build_file_content = KUBEBUILDER_ASSETS_BUILDFILE,
        sha256 = "c87c6b3c0aec4233e68a12dc9690bcbe2f8d6cd72c23e670602b17b2d7118325",
        urls = ["https://storage.googleapis.com/kubebuilder-tools/kubebuilder-tools-{}-darwin-arm64.tar.gz".format(KUBEBUILDER_ASSETS_VERSION)],
        strip_prefix = "kubebuilder/bin",
    )

    http_archive(
        name = "kubebuilder-assets-darwin-amd64",
        build_file_content = KUBEBUILDER_ASSETS_BUILDFILE,
        sha256 = "a02e33a3981712c8d2702520f95357bd6c7d03d24b83a4f8ac1c89a9ba4d78c1",
        urls = ["https://storage.googleapis.com/kubebuilder-tools/kubebuilder-tools-{}-darwin-amd64.tar.gz".format(KUBEBUILDER_ASSETS_VERSION)],
        strip_prefix = "kubebuilder/bin",
    )

    http_archive(
        name = "kubebuilder-assets-linux-amd64",
        build_file_content = KUBEBUILDER_ASSETS_BUILDFILE,
        sha256 = "8c816871604cbe119ca9dd8072b576552ae369b96eebc3cdaaf50edd7e3c0c7b",
        urls = ["https://storage.googleapis.com/kubebuilder-tools/kubebuilder-tools-{}-linux-amd64.tar.gz".format(KUBEBUILDER_ASSETS_VERSION)],
        strip_prefix = "kubebuilder/bin",
    )

    # universal-ctags
    #
    # Two step process to update these. First land a commit in main updating
    # the version in dev/nix/ctags.nix. Then copy the hashes from
    # https://github.com/sourcegraph/sourcegraph/actions/workflows/universal-ctags.yml
    http_file(
        name = "universal-ctags-darwin-amd64",
        sha256 = "7aead221c07d8092a0cbfad6e69ae526292e405bbe2f06d38d346969e23a6f68",
        url = "https://storage.googleapis.com/universal_ctags/x86_64-darwin/dist/universal-ctags-{0}".format(CTAGS_VERSION),
        executable = True,
    )

    http_file(
        name = "universal-ctags-darwin-arm64",
        sha256 = "ac4a73b69042c60e68f80f8819d5c6b05e233386042ba867205a252046d6471e",
        url = "https://storage.googleapis.com/universal_ctags/aarch64-darwin/dist/universal-ctags-{0}".format(CTAGS_VERSION),
        executable = True,
    )

    http_file(
        name = "universal-ctags-linux-amd64",
        sha256 = "e99b942754ce9d55c9445513236c010753d26decbb38ed932780bec098ac0809",
        url = "https://storage.googleapis.com/universal_ctags/x86_64-linux/dist/universal-ctags-{0}".format(CTAGS_VERSION),
        executable = True,
    )

    http_archive(
        name = "gcloud-darwin-arm64",
        build_file_content = GCLOUD_BUILDFILE,
        patch_cmds = GCLOUD_PATCH_CMDS,
        sha256 = "80c31937d3a3dce98d730844ff028715a46cd9fd5d5d44096b16e85fa54e6df1",
        url = "https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-{}-darwin-arm.tar.gz".format(GCLOUD_VERSION),
    )

    http_archive(
        name = "gcloud-darwin-amd64",
        build_file_content = GCLOUD_BUILDFILE,
        patch_cmds = GCLOUD_PATCH_CMDS,
        sha256 = "2961471b9d81092443456de15509f46fea685dfaf401f1b6c444eab63b45ccb7",
        url = "https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-{}-darwin-x86_64.tar.gz".format(GCLOUD_VERSION),
    )

    http_archive(
        name = "gcloud-linux-amd64",
        build_file_content = GCLOUD_BUILDFILE,
        patch_cmds = GCLOUD_PATCH_CMDS,
        sha256 = "03d87f71e15f2143e5f2b64b3594464ac51e791658848fc33d748f545ef97889",
        url = "https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-{}-linux-x86_64.tar.gz".format(GCLOUD_VERSION),
    )

    http_archive(
        name = "packer-linux-amd64",
        build_file_content = PACKER_BUILDFILE.format("linux-amd64"),
        sha256 = "0587f7815ed79589cd9c2b754c82115731c8d0b8fd3b746fe40055d969facba5",
        url = "https://releases.hashicorp.com/packer/{0}/packer_{0}_linux_amd64.zip".format(PACKER_VERSION),
    )

    http_archive(
        name = "packer-darwin-arm64",
        build_file_content = PACKER_BUILDFILE.format("darwin-arm64"),
        sha256 = "5cc53abbc345fc5f714c8ebe46fd79d5f503f29375981bee6c77f89e5ced92d3",
        url = "https://releases.hashicorp.com/packer/{0}/packer_{0}_darwin_arm64.zip".format(PACKER_VERSION),
    )

    http_archive(
        name = "packer-darwin-amd64",
        build_file_content = PACKER_BUILDFILE.format("darwin-amd64"),
        sha256 = "ef1ceaaafcdada65bdbb45793ad6eedbc7c368d415864776b9d3fa26fb30b896",
        url = "https://releases.hashicorp.com/packer/{0}/packer_{0}_darwin_amd64.zip".format(PACKER_VERSION),
    )

    http_file(
        name = "p4-fusion-linux-amd64",
        sha256 = "4c32aa00fa220733faea27a1c6ec4acd0998c1a7f870e08de9947685621f0d06",
        url = "https://storage.googleapis.com/p4-fusion/x86_64-linux/dist/p4-fusion-{0}".format(P4_FUSION_VERSION),
        executable = True,
    )

    http_file(
        name = "p4-fusion-darwin-amd64",
        sha256 = "bfa525a8a38d2c2ea205865b1a6d5be0b680e3160a64ba9505953be3294d1b9c",
        url = "https://storage.googleapis.com/p4-fusion/x86_64-darwin/dist/p4-fusion-{0}".format(P4_FUSION_VERSION),
        executable = True,
    )

    http_file(
        name = "p4-fusion-darwin-arm64",
        sha256 = "f97942e145902e682a5c1bc2608071a24d17bf943f35faaf18f359cbbaacddcd",
        url = "https://storage.googleapis.com/p4-fusion/aarch64-darwin/dist/p4-fusion-{0}".format(P4_FUSION_VERSION),
        executable = True,
    )

    http_file(
        name = "pg_dump-linux-amd64",
        url = "https://storage.googleapis.com/pg-utils/x86_64-linux/dist/pg_dump.{0}".format(PGUTILS_VERSION),
        sha256 = "fdf416c349a1cada342d0a8738fd8413066203ad66324ca5da504a0f4d628bb5",
        executable = True,
    )

    http_file(
        name = "pg_dump-darwin-amd64",
        url = "https://storage.googleapis.com/pg-utils/x86_64-darwin/dist/pg_dump.{0}".format(PGUTILS_VERSION),
        sha256 = "f16b24d798f3c25f0bf268172be21a03aa11ccd0bf436fc00d680da7f35d4303",
        executable = True,
    )

    http_file(
        name = "pg_dump-darwin-arm64",
        url = "https://storage.googleapis.com/pg-utils/aarch64-darwin/dist/pg_dump.{0}".format(PGUTILS_VERSION),
        sha256 = "d142961bdf84e87c14b55b6dabc4311b8f44483c8d31d39f37dc81cf8b456cc7",
        executable = True,
    )

    http_file(
        name = "dropdb-linux-amd64",
        url = "https://storage.googleapis.com/pg-utils/x86_64-linux/dist/dropdb.{0}".format(PGUTILS_VERSION),
        sha256 = "445ee469a26e4486c0765876eb8a7c04fd71ee1c2221430b42436323b30f3ca4",
        executable = True,
    )

    http_file(
        name = "dropdb-darwin-amd64",
        url = "https://storage.googleapis.com/pg-utils/x86_64-darwin/dist/dropdb.{0}".format(PGUTILS_VERSION),
        sha256 = "c614403a788298a28b9d7042ebba0628024ec93d9443fd2467bfc496a5fd51da",
        executable = True,
    )

    http_file(
        name = "dropdb-darwin-arm64",
        url = "https://storage.googleapis.com/pg-utils/aarch64-darwin/dist/dropdb.{0}".format(PGUTILS_VERSION),
        sha256 = "4e4a92cbcd2e5d804b8fa5402488c4521fbc927059baf43e6f1e639834400c0e",
        executable = True,
    )

    http_file(
        name = "createdb-linux-amd64",
        url = "https://storage.googleapis.com/pg-utils/x86_64-linux/dist/createdb.{0}".format(PGUTILS_VERSION),
        sha256 = "55b22dec6f24dc38bd16e7f727479ce493fdfb91a226901531b57387089c2843",
        executable = True,
    )

    http_file(
        name = "createdb-darwin-amd64",
        url = "https://storage.googleapis.com/pg-utils/x86_64-darwin/dist/createdb.{0}".format(PGUTILS_VERSION),
        sha256 = "d2acf9a9e20c3967cb4a8a1a50fccbff8ae6d01f30a34913acef7035822bea89",
        executable = True,
    )

    http_file(
        name = "createdb-darwin-arm64",
        url = "https://storage.googleapis.com/pg-utils/aarch64-darwin/dist/createdb.{0}".format(PGUTILS_VERSION),
        sha256 = "e6e5ee13fb2f1e4a55dffb282e5021d5486008ab74cd4f39c944439ed0e7765f",
        executable = True,
    )

    http_archive(
        name = "gh_darwin-arm64",
        build_file_content = GH_BUILDFILE,
        sha256 = "a0423acd5954932a817d531a8160b67cf0456ea6c9e68c11c054c19ea7a6714b",
        strip_prefix = "gh_{0}_macOS_arm64".format(GH_VERSION),
        url = "https://github.com/cli/cli/releases/download/v{0}/gh_{0}_macOS_arm64.zip".format(GH_VERSION),
    )

    http_archive(
        name = "gh_darwin-amd64",
        build_file_content = GH_BUILDFILE,
        sha256 = "82bea89eea5ddfcd5f88c53857fc2220ee361e0b65629f153d10695971a44195",
        strip_prefix = "gh_{0}_macOS_amd64".format(GH_VERSION),
        url = "https://github.com/cli/cli/releases/download/v{0}/gh_{0}_macOS_amd64.zip".format(GH_VERSION),
    )

    http_archive(
        name = "gh_linux-amd64",
        build_file_content = GH_BUILDFILE,
        sha256 = "79e89a14af6fc69163aee00e764e86d5809d0c6c77e6f229aebe7a4ed115ee67",
        strip_prefix = "gh_{0}_linux_amd64".format(GH_VERSION),
        url = "https://github.com/cli/cli/releases/download/v{0}/gh_{0}_linux_amd64.tar.gz".format(GH_VERSION),
    )

    http_archive(
        name = "postgresql-13-linux-amd64",
        url = "https://github.com/cedarai/embedded-postgres-binaries/releases/download/13.6-with-tools-20220304/postgresql-13.6-linux-amd64.txz",
        build_file_content = PGUTILS_BUILDFILE,
        sha256 = "ff673163a110b82e212139cd8a0ab4df89c030f324b2412b107d48f6764ad8b7",
    )
    http_archive(
        name = "postgresql-13-darwin-amd64",
        url = "https://github.com/cedarai/embedded-postgres-binaries/releases/download/13.6-with-tools-20220304/postgresql-13.6-darwin-amd64.txz",
        build_file_content = PGUTILS_BUILDFILE,
        sha256 = "e9ac855ca1d428cd2fe2a50c996ec5766f6db1c26b8f3f6ab3c929961c39d2e2",
    )
    http_archive(
        name = "postgresql-13-darwin-arm64",
        url = "https://github.com/cedarai/embedded-postgres-binaries/releases/download/13.6-with-tools-20220304/postgresql-13.6-darwin-arm64.txz",
        build_file_content = PGUTILS_BUILDFILE,
        sha256 = "32fd723dc8a64efaebc18e78f293bc7c5523fbb659a82be0f9da900f3a28c510",
    )

    http_file(
        name = "linear-sdk-graphql-schema",
        url = "https://raw.githubusercontent.com/linear/linear/%40linear/sdk%40{0}/packages/sdk/src/schema.graphql".format(LINEAR_SDK_VERSION),
        integrity = "sha256-9WUYPWt4iWcE/fhm6guqrfbk41y+Hb3jIR9I0/yCzwk=",
    )

    # Chromium deps for playwright
    # to find the update URLs try running:
    # npx playwright install --dry-run
    http_archive(
        name = "chromium-darwin-arm64",
        integrity = "sha256-5wj+iZyUU7WSAyA8Unriu9swRag3JyAxUUgGgVM+fTw=",
        url = "https://playwright.azureedge.net/builds/chromium/1117/chromium-mac-arm64.zip",
        build_file_content = CHROMIUM_BUILDFILE.format("chrome-mac/Chromium.app/Contents/MacOS/Chromium"),
    )

    http_archive(
        name = "chromium-darwin-x86_64",
        integrity = "sha256-kzTbTaznfQFD9HK1LMrDGdcs1ZZiq2Rfv+l5qjM5Cus=",
        url = "https://playwright.azureedge.net/builds/chromium/1117/chromium-mac.zip",
        build_file_content = CHROMIUM_BUILDFILE.format("chrome-mac/Chromium.app/Contents/MacOS/Chromium"),
    )

    http_archive(
        name = "chromium-linux-x86_64",
        integrity = "sha256-T7teJtSwhf7LIpQMEp4zp3Ey3T/p4Y7dQI/7VGVHdkE=",
        url = "https://playwright.azureedge.net/builds/chromium/1117/chromium-linux.zip",
        build_file_content = CHROMIUM_BUILDFILE.format("chrome-linux/chrome"),
    )

    http_file(
        name = "honeyvent-linux-x86_64",
        url = "https://github.com/honeycombio/honeyvent/releases/download/v1.1.3/honeyvent-linux-amd64",
        sha256 = "3810ad6d70836d5b4f2ef5de27c3c8a3ed4f35bb331635137d44223e285d6fc5",
        executable = True,
    )

    http_file(
        name = "honeyvent-darwin-x86_64",
        url = "https://github.com/honeycombio/honeyvent/releases/download/v1.1.3/honeyvent-darwin-amd64",
        sha256 = "c9acaab8a48aa3345fd323c4315c8aaca52b2f6ce4c6f83b6fa162cd4c516725",
        executable = True,
    )

    http_archive(
        name = "scip-go-linux-amd64",
        url = "https://github.com/sourcegraph/scip-go/releases/download/v0.1.14/scip-go_0.1.14_linux_amd64.tar.gz",
        integrity = "sha256-8ja7Q6fXAmftnFbmww9S1EpTDYVqgdm8pU2VjEW3hJs=",
        build_file_content = """
package(default_visibility = ["//visibility:public"])

exports_files(["scip-go"])
        """,
    )

    http_archive(
        name = "scip-go-darwin-arm64",
        url = "https://github.com/sourcegraph/scip-go/releases/download/v0.1.14/scip-go_0.1.14_darwin_arm64.tar.gz",
        integrity = "sha256-wzFsXZiMnvp1yU4sJDFVYFD6Okf70w3a2jfjCDSxlqM=",
        build_file_content = """
package(default_visibility = ["//visibility:public"])

exports_files(["scip-go"])
        """,
    )

    http_archive(
        name = "scip-go-darwin-amd64",
        url = "https://github.com/sourcegraph/scip-go/releases/download/v0.1.14/scip-go_0.1.14_darwin_amd64.tar.gz",
        integrity = "sha256-N+KSKkV0KvJGJV6+25W2A9WmvkRVjeAavwhSq0AsfOs=",
        build_file_content = """
package(default_visibility = ["//visibility:public"])

exports_files(["scip-go"])
        """,
    )

    # Test repo for generating scip-go index
    http_archive(
        name = "go-diff",
        url = "https://github.com/sgtest/go-diff/archive/f935979a5d70d2f3034edc6de94c77be621ce500.zip",
        integrity = "sha256-dEGXFKOQXUq32RBKaw9AMCpdSZRJDul+wl66Ktdu1PI=",
        build_file_content = """
filegroup(
    name = "src",
    srcs = glob(["*/**"]),
    visibility = ["//visibility:public"],
)
        """,
    )
