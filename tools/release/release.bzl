# TODO RFC 795
# This is probably unecessary, we can use a simple sh_binary.
# Revisit once we progress more.
def _release_patch_impl(ctx):
    executable = ctx.actions.declare_file("release_patch_%s.sh" % ctx.label.name)
    ctx.actions.expand_template(
        template = ctx.file._release_patch_sh_tpl,
        output = executable,
        is_executable = True,
    )
    return DefaultInfo(executable = executable)

release_patch = rule(
    implementation = _release_patch_impl,
    executable = True,
    attrs = {
        "_release_patch_sh_tpl": attr.label(
            default = "release_patch.sh.tpl",
            allow_single_file = True,
        )
    },
)

