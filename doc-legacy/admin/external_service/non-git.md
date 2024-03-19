# Non-Git code hosts (Perforce, Mercurial, Subversion, raw text, etc.)

Sourcegraph natively supports all Git-based Version Control Systems (VCSs) and code hosts. For non-Git code hosts, Sourcegraph provides a CLI tool called `src-expose` to periodically sync and continuously serve local directories as Git repositories over HTTP. 

>NOTE: If using Perforce, see the [Perforce repositories with Sourcegraph guide](../repo/perforce.md).

## Use `src serve-git`

Since Sourcegraph 3.19 we recommend users to use [`src serve-git`](src_serve_git.md). `src serve-git` only provides the serving of git repositories (no snapshotting). We found users generally wanted to control the git repos and snapshotting complicated the setup. Additionally `src serve-git` uses a fast and modern git transfer protocol.

## Installing `src-expose`

Navigate to the directory that contains the Git repositories that you want to serve, then run the following commands.

For Linux:

```bash
wget https://storage.googleapis.com/sourcegraph-artifacts/src-expose/latest/linux-amd64/src-expose

chmod +x src-expose
```

For macOS:

```bash
wget https://storage.googleapis.com/sourcegraph-artifacts/src-expose/latest/darwin-amd64/src-expose

chmod +x src-expose
```

You can run `src-expose -h` any time for help. 

## Using `src-expose`

`src-expose` can be used in two ways:

- **Serving local Git repositories**<br/>
`src-expose` can serve local Git repositories over the network, and make them available to Sourcegraph (as if they were available on a traditional code host). See [serving repositories](#serving-repositories), or run `src-expose serve -h`.

- **Syncing changes, turning them into Git commits, and serving the resulting Git repositories**<br/>
`src-expose` can periodically run a command to sync changes to the code, and then combine those changes into a new Git commit in the local repository. See [syncing and serving repositories](#syncing-and-serving-repositories), or run `src-expose -h`.

### Serving repositories

`src-expose` serves a list of local Git repositories over HTTP, making them available to Sourcegraph. In addition to simply providing a Git endpoint, it also provides a repository listing API that Sourcegraph expects a code host to have. 

If you wish to serve a local directory without running any syncing commands automatically, you can run `src-expose serve` (instead of the default `src-expose`) to only perform this function.

In order to keep the code in the local repository up to date, you will need to run another command periodically to fetch changes. For example, if you are using Perforce, you can set up a cron job to run `git p4 sync` every few minutes or hours to fetch changes and convert them to Git commits that can then be served. Similar options exist for other non-Git VCSs.

### Syncing and serving repositories

In addition to serving a local directory, `src-expose` can periodically run a command of your choice to fetch changes from a remote and combine them into a single new Git commit.

For example, if your `src-expose` is using a [configuration YAML file](#next-steps--advanced-configuration) that contains the following:

```yaml
# before is a command run before sync. before is run from root.
before: p4 sync
# duration defines how often sync should happen. Defaults to 10s.
duration: 10s
```

Then Sourcegraph will run `p4 sync` every 10 seconds, and combine all of the fetched changes into a new Git commit. The new Git commit's author will be `src-expose`, and will contain all changes since the last time the syncing command was run.

While this syncing functionality means that the original change history will be lost, it eliminates any slow and costly Perforce-to-Git or Hg-to-Git or similar conversions that would otherwise be required. If you prefer to retain history, see [serving repositories](#serving-repositories).

## Quickstart

1. Start up a Sourcegraph instance (using our [Quickstart](../../index.md) or our [full installation documentation](../deploy/index.md)).

1. [Install `src-expose`](#installing-src-expose)

1. Pick the directory you want to export from, then run:

```bash
# Run a command periodically to sync changes, commit those changes as Git commits, and serve over HTTP.
./src-expose dir1 dir2 dir3
```

or

```bash
# Serve local Git repositories over HTTP. This command serves all Git repositories at the provided directory.
./src-expose serve dir
```

depending on whether you want to automatically sync and commit changes, or just serve the local directories.

1. `src-expose` will output a configuration to use. It may scroll by quickly due to logging, so if so, just scroll up. However, this configuration should work:

```json
 {
    // url is the HTTP url to src-expose (listening on 127.0.0.1:3434). url should be reachable by Sourcegraph.
    //
    // "http://host.docker.internal:3434" works from Sourcegraph when using Docker for Desktop.
    "url": "http://host.docker.internal:3434",
    // By using the special value ("src-expose") below, Sourcegraph will pull all of the repositories that src-expose is serving.
    "repos": ["src-expose"]
}
```

**IMPORTANT:** If you are using a Linux host machine, replace `host.docker.internal` in the above with the IP address of your actual host machine because `host.docker.internal` [does not work on Linux](https://github.com/docker/for-linux/issues/264). You should use the network-accessible IP shown by `ifconfig` (rather than 127.0.0.1 or localhost).

Go to **Admin > Manage code hosts > Add repositories > Single Git repositories**. Input the above configuration. Your directories should now be syncing in Sourcegraph.

### Next steps: advanced configuration

Please consult `src-expose -help` to learn more about the options available. 

For more complex setups, configure your `src-expose` by providing a local configuration file:

```bash
src-expose -snapshot-config config.yaml
```

See [an example YAML file containing available configuration options](https://github.com/sourcegraph/sourcegraph/blob/main/dev/src-expose/examples/example.yaml). 
