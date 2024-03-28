# Perforce

## `base/`

A directory that contains a depot that uses a file/folder layout that's similar enough (with obfuscated names) to the depot described in [this Slack message](https://sourcegraph.slack.com/archives/C02EDAQAJQZ/p1659583885164999)

## `templates/`

A directory that contains templates that `./p4-upload.sh` and `./p4-setup-protections.sh` use to submit various parametrized Perforce objects / configurations to the Perforce server

### `templates/p4_protects.tmpl`

[This file](./templates/p4_protects.tmpl) is the **full** [protections table](https://www.perforce.com/manuals/cmdref/Content/CmdRef/p4_protect.html#p4_protect) that [`p4-setup-protections.sh`](#p4-setup-protections.sh) that will apply to the specified Perforce server.

When modifying the protections rule for integration tests, place the rules between the `# AWK-START` and `# AWK-END` comments, like so:

```text
...
# AWK-START ## (place protection rules for integration tests below me)
    =read group foobar * -//${DEPOT_NAME}/...
    ...
# AWK-END ## (place protection rules for integration tests above me
...
```

[`p4-setup-protections.sh`](#p4-setup-protections.sh) uses the `# AWK-START` and `# AWK-END` comments to discover the set of groups that it needs to (re-create).

In addition, [`p4-setup-protections.sh`](#p4-setup-protections.sh) will handing replace the `-//${DEPOT_NAME}/...` line with the name of the test depot that the user specified when running the script.

_Note: Our Perforce instance has multiple users inside of Sourcegraph. We don't want to break their workflows by accidentally overwriting any rules that they were relying on. `p4` also doesn't let you delete individual protection rules - you can only apply an entire table at once. So, the compromise we found is to keep all the pre-existing protection rules in [templates/p4_protects.tmpl](./templates/p4_protects.tmpl), but only create/modify the groups that we add between the `# AWK-START` and `# AWK-END` comments._

## `create_path_files.sh`

A helper script that walks the example depot specified in `base/` and generates `path.txt` files in each folder that contain the folder's path relative to the depot root.

**Run this script whenever you modify the example depot's structure.**

_The depot description in [the Slack message](https://sourcegraph.slack.com/archives/C02EDAQAJQZ/p1659583885164999) doesn't fully enumerate all the file / folder contents. As a result, there are some folders that we know exist, but we don't know their exact file contents. Since we can't upload empty folders to Perforce, the `path.txt` files act as stubs that let us work around this._

## `p4-upload.sh`

A helper script that idempotently:

- creates a test depot
- uploads the entire contents of `base/` as separate [changelists](https://www.perforce.com/manuals/cmdref/Content/CmdRef/p4_changes.html).

**Run this script whenever you begin a new round of testing.**

_The address of the Perforce server, the superuser credentials to use, the depot name, and the client name are all configurable via environment variables. See the beginning of [./p4-upload.sh](./p4-upload.sh) for more information._

## `p4-setup-protections.sh`

A helper script that idempotently:

- creates a test user (`P4_TEST_USERNAME`)
- creates all the groups mentioned in [templates/p4_protects.tmpl](./templates/p4_protects.tmpl)
- prompts the user to select which group(s) they'd like `P4_TEST_USERNAME` to be a member of, then applies those selections
- uploads the [bundled protections table](./templates/p4_protects.tmpl)

**Run this script whenever you run a new test case and need to change the group membership.**

_The address of the Perforce server, the superuser credentials to use, the test user credentials, and the depot name are all configurable via environment variables. See the beginning of [./p4-setup-protections.sh](./p4-setup-protections.sh) for more information._
Hello World
