TRANSLATE_TPL = """
load("@rules_apko//apko:translate_lock.bzl", "translate_apko_lock")

def apko_translate_locks():
    for name in %{LOCKNAMES}%:
        translate_apko_lock(
            name = name.partition(".")[0].replace("-", "_") + "_apko_lock",
            lock = "@//wolfi-images:" + name,
            visibility = ["//visibility:public"],
        )

"""

# A custom repository rule to automate away the extensive boilerplate needed to generate rules_apko repositories
# for every lockfile in ./wolfi-images
#
# For each lockfile we need to do the following:
#   - Create a repository that translates the lockfile into a rules_apko internal representations
#       - This representation is a macro that creates 3 more repositories to fetch the keyrings, packages and package index
#       - Implementation:https://sourcegraph.com/github.com/chainguard-dev/rules_apko@v1.2.3/-/blob/apko/translate_lock.bzl
#   - Load the macro from the newly overarching repository
#   - Call the macro to instantiate the 3 repositories and make them available to apko_image rule targets
#
# The original idea entertained would have been to just extract all the hand-written instantiations to a macro,
# and then maybe automating generating that single macro with a repo rule. But because we can't call `load` in a
# macro, we have to be a bit more imaginative in how we can automate this (thanks to Caleb Zulawski in the Bazel
# slack for the idea underpinning the approach here).
#
# Instead, what we do is generate two files:
#   - One contains all the `translate_apko_lock` calls in a macro
#   - The other has two components:
#       - All the `load` statements to load the macros from the repositories generated in the previous file
#       - And a macro that calls all the loaded macros to instantiate the 3 other repositories (as described above)
#
# Then, in WORKSPACE we perform the following order of operations:
#   - Instantiate this repository which generates the two .bzl files from the lockfiles
#   - Load & call the macro from the first file, creating all the overarching repositories
#   - Load & call the macro from the second file, instantiating the 3 repositories for every lockfile
#
# See the following commit to see what is being removed from WORKSPACE by this repository rule:
# https://github.com/sourcegraph/sourcegraph/pull/60785/commits/041fb7a177c8f9004a973306b2e045a25e64fc68
def _wolfi_lockfiles(rctx):
    result = rctx.execute(["ls", str(rctx.workspace_root) + "/wolfi-images"])
    if result.return_code != 0:
        fail("failed to list wolfi-images:", result.stderr)

    lockfiles = []

    repo_loads = ""
    repo_contents = "def apko_repositories():\n"

    for file in result.stdout.split("\n"):
        if not file.endswith(".lock.json"):
            continue

        lockfiles.append(file)

        # In bazel 7.1.0, we can replace this with `rctx.watch(file)` (they behave identically though).
        # Used to invalidate this repository when any of the lockfiles change.
        rctx.path(Label(file))

        lockname = file.partition(".")[0].replace("-", "_")

        # because load aliases and function calls cant be in quotes, we cant use templating with list comprehensions/for loops like with
        # apko_translate_locks, so we have to do this manually
        repo_loads += """load("@{}_apko_lock//:repositories.bzl", {}_apko_repositories = "apko_repositories")\n""".format(lockname, lockname)
        repo_contents += """    {}_apko_repositories()\n""".format(lockname)

    translate_tpl_path = rctx.path("wolfi_translate_lockfiles.bzl.tpl")
    rctx.file(translate_tpl_path, TRANSLATE_TPL)
    rctx.file("BUILD.bazel")

    rctx.template("translates.bzl", translate_tpl_path, {
        "%{LOCKNAMES}%": "[\"" + "\",\"".join(lockfiles) + "\"]",
    })

    # rctx.file("translates.bzl", content = translates_contents)
    rctx.file("repositories.bzl", content = repo_loads + repo_contents)

wolfi_lockfiles = repository_rule(
    implementation = _wolfi_lockfiles,
)
