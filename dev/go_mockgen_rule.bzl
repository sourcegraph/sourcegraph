load("@io_bazel_rules_go//go:def.bzl", "GoArchive", "GoSource")

def _go_mockgen_run(ctx):
    dst = ctx.actions.declare_file(ctx.attr.out)
    config_dst = ctx.actions.declare_file("mockgen.yaml")

    stdlib_root = "{path}/{os}_{arch}".format(
        path = ctx.attr._go_stdlib[GoSource].stdlib.libs[0].path,
        os = ctx.attr._go_stdlib[GoSource].mode.goos,
        arch = ctx.attr._go_stdlib[GoSource].mode.goarch,
    )

    transformer_args = [
        "--outfile",
        config_dst.path,
        "--final-generated-file",
        ctx.label.package + "/" + ctx.attr.out[1:],
        "--intermediary-generated-file",
        dst.path,
        "--stdlibroot",
        stdlib_root,
        "--goimports",
        ctx.executable._goimports.path,
        "--output-importpath",
        "github.com/sourcegraph/sourcegraph/" + ctx.label.package,
    ]

    deps = []
    for dep in ctx.attr.deps:
        for a in dep[GoArchive].direct:
            transformer_args.append("--archives=%s=%s" % (
                a.data.importmap,
                # (anecdotaly) export_file is a .x file and file is a .a file. See here for more info on this
                # https://github.com/bazelbuild/rules_go/issues/1803
                # and here for further reading on the compiler side of things
                # https://groups.google.com/g/golang-codereviews/c/UXJeeuTS7oQ
                a.data.export_file.path if a.data.export_file else a.data.file.path,
            ))
            deps.append(depset(direct = [a.data.export_file if a.data.export_file else a.data.file]))

    for dep in ctx.attr.deps:
        for src in dep[GoArchive].data.srcs:
            transformer_args.append("--source-files=%s=%s" % (dep[GoArchive].data.importpath, src.path))
            deps.append(depset(direct = [src]))

    manifests = []
    for f in ctx.attr.manifests:
        manifests.extend(f.files.to_list())

    ctx.actions.run(
        mnemonic = "GoMockgenConfigTransform",
        executable = ctx.executable._gomockgen_transformer,
        arguments = transformer_args,
        outputs = [config_dst],
        tools = [ctx.executable._goimports],
        inputs = depset(direct = manifests),
        progress_message = "Transforming go-mockgen config for %s" % str(ctx.label),  # TODO: figure out what we wanna put here
    )

    action_direct_deps = [config_dst, ctx.attr._go_stdlib[GoSource].stdlib.libs[0]]
    for dep in ctx.attr.deps:
        action_direct_deps.append(dep[GoArchive].data.file)

    ctx.actions.run(
        mnemonic = "GoMockgen",
        arguments = ["--manifest-dir", config_dst.dirname],
        executable = ctx.executable._gomockgen,
        outputs = [dst],
        tools = [ctx.executable._goimports],
        inputs = depset(
            direct = action_direct_deps,
            transitive = deps,
        ),
        progress_message = "Running go-mockgen to generate %s" % dst.short_path,
    )

    return [
        DefaultInfo(
            files = depset([dst]),
        ),
    ]

go_mockgen_generate = rule(
    implementation = _go_mockgen_run,
    attrs = {
        "deps": attr.label_list(
            providers = [GoArchive],
            allow_empty = False,
            mandatory = True,
        ),
        "out": attr.string(
            mandatory = True,
        ),
        "manifests": attr.label_list(
            allow_files = True,
            mandatory = False,
        ),
        "_gomockgen_transformer": attr.label(
            default = Label("//dev/go-mockgen-transformer:go-mockgen-transformer"),
            executable = True,
            cfg = "exec",
        ),
        "_gomockgen": attr.label(
            default = Label("@com_github_derision_test_go_mockgen_v2//cmd/go-mockgen:go-mockgen"),
            executable = True,
            cfg = "exec",
        ),
        "_goimports": attr.label(
            default = Label("@org_golang_x_tools//cmd/goimports:goimports"),
            executable = True,
            cfg = "exec",
        ),
        "_go_stdlib": attr.label(
            providers = [GoSource],
            default = Label("@io_bazel_rules_go//:stdlib"),
        ),
    },
)
