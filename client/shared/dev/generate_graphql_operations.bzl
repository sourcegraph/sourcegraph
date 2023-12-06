"GraphQL generation rule"

load("@aspect_rules_js//js:defs.bzl", "js_run_binary")

def generate_graphql_operations(name, config, srcs, out, **kwargs):
    """Generate a graphql operations Typescript interface.

    Args:
        name: Name of the target
        config: Name of the predefined codgen configuration in generateGraphQlOperations.ts.
        srcs: Files required by the glob to generate the interface. Should include all files
            in the glob pattern specified in generateGraphQlOperations.ts. The files should be
            produced in or copied to the Bazel output tree, so it's recommended to use js_library
            with a glob pattern equivalent to the one in generateGraphQlOperations.ts.
        out: Name of the TypeScript file to output.
        **kwargs: general args
    """
    js_run_binary(
        name = name,
        outs = [out],
        srcs = srcs,
        args = [
            config,
            out,
        ],
        chdir = native.package_name(),
        tool = "//client/shared/dev:generate_graphql_operations",
        **kwargs
    )
