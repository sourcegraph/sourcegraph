# Troubleshooting

This page is meant to help narrow down and eliminate problems when trying to execute a batch spec with `src batch [apply|preview]` or managing an already created batch change and its changesets.

## Executing batch change steps

Since `src batch [apply|preview]` execute a batch spec on the host machine on which it is executed (and not on the Sourcegraph instance), there are a lot of different possibilities that can cause it to fail: from missing dependencies to missing credentials when trying to connect ot the Sourcegraph instance.

The following questions can be used to find out what's causing the problem and should ideally be answered with "yes".

### Is `src` connected to the right Sourcegraph instance?

Run the following command, replacing `sourcegraph.example.com` with the URL of your Sourcegraph instance, to make sure `src` is configured correctly:

```bash
src login https://sourcegraph.example.com
```

If `src` is correctly configured, then the output should look similar to the following:

```

✔️  Authenticated as my-username on https://sourcegraph.example.com

```

### Are dependencies installed?

When executing batch specs `src` uses Docker and git. Make sure that those are installed and accessible to you on your machine.

To test whether `git` is installed and accessible, run the following:

```bash
git version
```

To test whether Docker is installed and configured correctly, run the following:

```
docker run hello-world
```

That command will pull Docker's `hello-world` image and try to execute a container using that image. If it works, you should see a "Hello from Docker!" message telling you that your installation seems to work.

### Does "Hello World" work?

If executing your batch spec fails and you haven't tested Batch Changes with another batch spec before, it can help to run the "Hello World" batch change to nail down what the problem is.

Go through the "[Quickstart](../quickstart.md)" to run a batch change that adds "Hello World" to `README.md` files with the following batch spec:

```yaml
name: hello-world
description: Add Hello World to READMEs

# Find one repository with a README.md
on:
  - repositoriesMatchingQuery: repohasfile:README.md count:1

# In each repository, run this command. Each repository's resulting diff is captured.
steps:
  - run: echo Hello World | tee -a $(find -name README.md)
    container: alpine:3

# Describe the changeset (e.g., GitHub pull request) you want for each repository.
changesetTemplate:
  title: Hello World
  body: My first batch change!
  branch: hello-world # Push the commit to this branch.
  commit:
    message: Append Hello World to all README.md files
  published: false
```

If even that doesn't work, then we can at least exclude the possibility that _only_ something with _your batch spec_ is wrong.

### Does it work with a single repository? Five? Ten?

Debugging large batch changes that make changes in hundreds of repositories is hard.

In order to find out whether a problem is related to the _size_ or _scope_ of a batch change or with _what_ it's trying to achieve, try reducing the scope of your batch change.

You can do so by changing the [`on.repositoriesMatchingQuery`](batch_spec_yaml_reference.md#on-repositoriesmatchingquery) to yield less results or by using a concrete list of repositories with [`on.repository`](batch_spec_yaml_reference.md#on-repository).

For the former you can use Sourcegraph's [search filters](../../code_search/reference/queries.md#keywords-all-searches).

For example, this query will only yield repositories that have `github.com/my-org` in their name:

```yaml
# [...]
on:
  - repositoriesMatchingQuery: repo:^github.com/my-org
```

This one will only return a single repository matching that exact name:

```yaml
# [...]
on:
  - repositoriesMatchingQuery: repo:^github.com/my-org/my-repo$
```

That can also be achieved with the mentioned `on.repository` attribute:

```yaml
# [...]
on:
  - repository: github.com/my-org/my-repo1
  - repository: github.com/my-org/my-repo2
```

Once you know that executing the batch spec works with one or more repositories, you can expand the scope back to its original form and hopefully nail down in which repository execution breaks.

### Can you download a repository archive?

If `src` is stuck in the "Initializing workspace" phase for a repository or fails to initialize the workspace, try to see whether you can download an archive of the repository manually on your command line with `curl`:

```
curl -L -v -X GET -H 'Accept: application/zip' \
  -H "Authorization: token $SRC_ACCESS_TOKEN" \
  "$SRC_ENDPOINT/github.com/my-org/my-repo@refs/heads/master/-/raw" \
  --output ~/tmp/my-repo.zip
```

That command is equivalent to what `src` does under the hood when downloading an archive of `github.com/my-org/my-repo@master` to execute batch spec `steps`.

If that fails, then that points to the Sourcegraph setup or infrastructure as a likely source of the problem, not `src`.

### Can you manually execute the `steps.run` command in a container?

If executing the `steps.run` command fails, you can try to recreate whether executing the step manually in a container works.

An approximiation of what `src` does under the hood is the following command:

```
docker run --rm --init --workdir /work \
  --mount type=bind,source=/unzipped-archive-locally,target=/work \
  --mount type=bind,source=/tmp-script,target=/tmp-file-in-container \
  --entrypoint /bin/sh -- <IMAGE> /tmp-file-in-container
```

Make sure that you put your `steps.run` command in `/tmp-script` (or any other location), replace `<IMAGE>` with the name of the Docker image, and `/unzipped-archive-locally` (or any other location) with a local copy of the repository in which you want to execute the steps.

## Publishing changesets

### Do you have the right credentials?

When publishing changesets fails, make sure that the credentials you use have the correct credentials to create changesets on the code host: "[Configuring user credentials](../how-tos/configuring_user_credentials.md)"
