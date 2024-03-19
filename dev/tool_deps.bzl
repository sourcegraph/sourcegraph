"Third party dev tooling dependencies"

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")

DOCSITE_VERSION = "1.9.4"
SRC_CLI_VERSION = "5.3.0"
CTAGS_VERSION = "6.0.0.2783f009"
PACKER_VERSION = "1.8.3"
P4_FUSION_VERSION = "v1.13.2-sg.04a293a"
GH_VERSION = "2.45.0"

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
