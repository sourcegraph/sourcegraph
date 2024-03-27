load("@npm//:mocha/package_json.bzl", mocha_bin = "bin")
load("@aspect_rules_esbuild//esbuild:defs.bzl", "esbuild")
load("@aspect_rules_js//js:defs.bzl", "js_test")

NON_BUNDLED = [
    # Dependencies loaded by mocha itself before the tests.
    # Should be outside the test bundle to ensure a single copy is used
    # mocha (launched by bazel) and the test bundle.
    "mocha",
    "puppeteer",

    # Dependencies used by mocha setup scripts.
    "abort-controller",
    "node-fetch",
    "console",

    # UMD modules
    "jsonc-parser",

    # Used by require.resolve
    "axe-core",
]

# ... some of which are needed at runtime
NON_BUNDLED_DEPS = [
    "//:node_modules/jsonc-parser",
    "//:node_modules/puppeteer",
    "//:node_modules/@axe-core/puppeteer",
    "//:node_modules/axe-core",
]

def mocha_test(name, tests, deps = [], args = [], data = [], env = {}, is_percy_enabled = False, **kwargs):
    bundle_name = "%s_bundle" % name

    # Bundle the tests to remove the use of esm modules in tests
    esbuild(
        name = bundle_name,
        testonly = True,
        entry_points = tests,
        platform = "node",
        target = "esnext",
        output_dir = True,
        external = NON_BUNDLED,
        sourcemap = "linked",
        deps = deps,
        config = {
            "loader": {
                # Packages such as fsevents require native .node binaries.
                ".node": "copy",
            },
        },
    )

    args = [
        "--config",
        "$(rootpath //:mocha_config)",
        "'$(rootpath :%s)/**/*.test.js'" % bundle_name,
        "--retries 4",
    ] + args

    data = data + NON_BUNDLED_DEPS + [
        "//:mocha_config",
        ":%s" % bundle_name,
    ]

    # Some values are passed down from --action_env. Bazel unfortunately
    # doesn't let us rename them without attempting to do analysis-time
    # variable substitution, which causes analysis-time errors if the variables
    # are not declared.
    # - SOURCEGRAPH_BASE_URL
    # - GH_TOKEN
    # - DISPLAY
    # - HEADLESS
    # - PERCY_TOKEN
    env = dict(env, **{
        # Add environment variable so that mocha writes its test xml
        # to the location Bazel expects.
        "MOCHA_FILE": "$$XML_OUTPUT_FILE",

        # TODO(bazel): e2e test environment
        "TEST_USER_EMAIL": "test@sourcegraph.com",
        "TEST_USER_PASSWORD": "supersecurepassword",
        "SOURCEGRAPH_SUDO_TOKEN": "fake-sg-token",
        "NO_CLEANUP": "false",
        "KEEP_BROWSER": "false",
        "DEVTOOLS": "false",
        "BROWSER": "chrome",
        "WINDOW_WIDTH": "1280",
        "WINDOW_HEIGHT": "1024",
        "LOG_BROWSER_CONSOLE": "false",

        # Enable findDom on CodeMirror
        "INTEGRATION_TESTS": "true",
    })

    if is_percy_enabled:
        js_test(
            name = name,
            args = args,
            env = dict(env, **{
                "PERCY_ON": "true",
            }),
            data = data + [
                "//:node_modules/@percy/cli",
                "//:node_modules/@percy/puppeteer",
                "//:node_modules/mocha",
                "//:node_modules/resolve-bin",
                "//client/shared/dev:run_mocha_tests_with_percy",
            ],
            # Executed mocha tests with Percy enabled via `percy exec -- mocha ...`
            # Prepends volatile env variables to the command to make Percy aware of the
            # current git branch and commit.
            entry_point = "//client/shared/dev:run_mocha_tests_with_percy",
            flaky = kwargs.pop("flaky"),
            timeout = kwargs.pop("timeout"),
            **kwargs
        )
    else:
        mocha_bin.mocha_test(
            name = name,
            args = args,
            data = data,
            env = env,
            **kwargs
        )
