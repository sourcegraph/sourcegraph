"""OCI bazel defs"""

load("@rules_oci//oci:defs.bzl", _oci_image = "oci_image", _oci_push = "oci_push", _oci_tarball = "oci_tarball")

REGISTRY_REPOSITORY_PREFIX = "europe-west1-docker.pkg.dev/sourcegraph-security-logging/rules-oci-test/{}"
# REGISTRY_REPOSITORY_PREFIX = "us.gcr.io/sourcegraph-dev/{}"

# Passthrough the @rules_oci oci_push, so users only have to import this file and not @rules_oci//oci:defs.bzl
oci_push = _oci_push

def image_repository(image):
    return REGISTRY_REPOSITORY_PREFIX.format(image)

def oci_tarball(name, **kwargs):
    _oci_tarball(
        name = name,
        # Don't build this by default with bazel build //... since most oci_tarball
        # targets do not need to be built on CI. This prevents the remote cache from
        # being overwhelmed in the event that oci_tarballs are cache busted en masse.
        tags = kwargs.pop("tags", []) + ["manual"],
        **kwargs
    )

# Apply a transition on oci_image targets and their deps to apply a transition on platforms
# to build binaries for Linux when building on MacOS.
def oci_image(name, **kwargs):
    _oci_image(
        name = name + "_underlying",
        **kwargs
    )

    oci_image_cross(
        name = name,
        image = ":" + name + "_underlying",
        platforms = select({
            "@platforms//os:macos": [Label("@zig_sdk//platform:linux_amd64")],
            "//conditions:default": [],
        }),
        visibility = kwargs.pop("visibility", ["//visibility:public"]),
    )

# rule that allows transitioning in order to transition an oci_image target and its deps
oci_image_cross = rule(
    implementation = lambda ctx: DefaultInfo(files = depset(ctx.files.image)),
    attrs = {
        "image": attr.label(cfg = transition(
            implementation = lambda settings, attr: [
                {"//command_line_option:platforms": str(platform)}
                for platform in attr.platforms
            ],
            inputs = [],
            outputs = ["//command_line_option:platforms"],
        )),
        "platforms": attr.label_list(),
        "_allowlist_function_transition": attr.label(
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    },
)
