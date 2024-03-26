load("@aspect_bazel_lib//lib:yq.bzl", "yq")
load("@rules_apko//apko:defs.bzl", "apko_image")
load("//dev:oci_defs.bzl", "oci_image", "oci_tarball")

def wolfi_base(name = "wolfi", target = None):
    if target == None:
        target = native.package_name().split("/")[-1]

    yq(
        name = "wolfi_config",
        expression = ". as $item ireduce ({}; . *+ $item) | del(.include)",
        srcs = [
            "//wolfi-images:{}.yaml".format(target),
            "//wolfi-images:sourcegraph-base.yaml",
        ],
        visibility = ["//visibility:private"],
    )

    apko_image(
        name = "wolfi_base_apko",
        architecture = "amd64",
        config = ":wolfi_config",
        contents = "@{}_apko_lock//:contents".format(target.replace("-", "_")),
        tag = "{}-base:latest".format(target),
        visibility = ["//visibility:private"],
    )

    oci_image(
        name = "wolfi_base_image",
        base = ":wolfi_base_apko",
        visibility = ["//visibility:public"],
    )

    oci_tarball(
        name = "wolfi_base_tarball",
        image = ":wolfi_base_image",
        repo_tags = ["{}-base:latest".format(target)],
    )
