def server_integration_test(name, port, runner_src, **kwargs):
    if not port:
        fail("port must be specified")

    args = kwargs.pop("args", [])
    data = kwargs.pop("data", [])
    env = kwargs.pop("env", {})
    env_inherit = kwargs.pop("env_inherit", [])
    tags = kwargs.pop("tags", [])
    deps = kwargs.pop("deps", [])

    # First two arguments are always the server image and the image name
    args = ["$(location //cmd/server:image_tarball)", "server:candidate"] + args

    # Explicitly define the port, needs to be different for each test so we can run them concurrently.
    env["PORT"] = port

    # These tests are making network calls to the running server image, so we need the network.
    tags.append("requires-network")

    # Finally, we depend on the integration runner script helper.
    deps.append("//testing/tools:integration_runner")

    native.sh_test(
        name = name,
        srcs = [runner_src],
        args = args,
        data = data,
        env = env,
        env_inherit = env_inherit,
        deps = deps,
        tags = tags,
        **kwargs
    )

def migration_test_binary(target_version):
    native.sh_binary(
        name = "migration_test_{}".format(target_version),
        srcs = [":migration_test.sh"],
        args = [
            "$(location //cmd/migrator:image_tarball)",
            target_version,
        ],
        data = ["//cmd/migrator:image_tarball"]
    )
