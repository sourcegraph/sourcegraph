def _release_patch_impl(ctx):
    if not ctx.attr.version.startswith("v"):
        fail("version attribute must start with a 'v'")

    executable = ctx.actions.declare_file("release_patch_%s.sh" % ctx.label.name)
    ctx.actions.expand_template(
        template = ctx.file._release_patch_sh_tpl,
        output = executable,
        is_executable = True,
        substitutions = {
            "{{new_version}}": ctx.attr.version,
        }
    )
    return DefaultInfo(executable = executable)

release_patch = rule(
    implementation = _release_patch_impl,
    executable = True,
    attrs = {
        "version": attr.string(mandatory = True),
        "_release_patch_sh_tpl": attr.label(
            default = "release_patch.sh.tpl",
            allow_single_file = True,
        )
    },
)

