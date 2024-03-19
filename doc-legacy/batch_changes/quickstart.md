# Quickstart for Batch Changes

Get started and create your first [batch change](index.md) in 10 minutes or less.

## Introduction

In this guide, you'll create a Sourcegraph batch change that appends text to `README.md` files in all of your repositories.

For more information about Batch Changes, watch the [Batch Changes demo video](https://www.youtube.com/watch?v=EfKwKFzOs3E).

## Requirements

- A Sourcegraph instance with some repositories in it. See "[Quick install](../index.md#quick-install)" on how to set up a Sourcegraph instance.
- A local environment matching "[Requirements](./references/requirements.md)" to create batch changes with the Sourcegraph CLI.

## Install the Sourcegraph CLI

In order to create batch changes we need to [install the Sourcegraph CLI](../cli/index.md) (`src`).

1. Install the version of `src` that's compatible with your Sourcegraph instance:

    **macOS**:
    ```
    curl -L https://YOUR-SOURCEGRAPH-INSTANCE/.api/src-cli/src_darwin_amd64 -o /usr/local/bin/src
    chmod +x /usr/local/bin/src
    ```
    **Linux**:
    ```
    curl -L https://YOUR-SOURCEGRAPH-INSTANCE/.api/src-cli/src_linux_amd64 -o /usr/local/bin/src
    chmod +x /usr/local/bin/src
    ```
1. Authenticate `src` with your Sourcegraph instance by running **`src login`** and following the instructions:

    ```
    src login https://YOUR-SOURCEGRAPH-INSTANCE
    ```
    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/src_login_success.png" class="screenshot">


Once `src login` reports that you're authenticated, we're ready for the next step.

## Write a batch spec

A **batch spec** is a YAML file that defines a batch change. It specifies which changes should be made in which repositories.

See the ["batch spec YAML reference"](references/batch_spec_yaml_reference.md) for details.

Save the following batch spec as `hello-world.batch.yaml`:

```yaml
name: hello-world
description: Add Hello World to READMEs

# Find all repositories that contain a README.md file.
on:
  - repositoriesMatchingQuery: file:README.md

# In each repository, run this command. Each repository's resulting diff is captured.
steps:
  - run: IFS=$'\n'; echo Hello World | tee -a $(find -name README.md)
    container: alpine:3

# Describe the changeset (e.g., GitHub pull request) you want for each repository.
changesetTemplate:
  title: Hello World
  body: My first batch change!
  branch: hello-world # Push the commit to this branch.
  commit:
    message: Append Hello World to all README.md files
```

The commits you create here will use the git config values for `user.name` and `user.email` from your local environment, or "batch-changes@sourcegraph.com" if no user is set. Alternatively, you can also [specify an `author`](./references/batch_spec_yaml_reference.md#changesettemplate-commit-author) in this spec.

## Create the batch change

Let's see the changes that will be made. Don't worry---no commits, branches, or changesets will be published yet (the repositories on your code host will be untouched).

1. In your terminal, run this command:

    <pre>src batch preview -f hello-world.batch.yaml</pre>
1. Wait for it to run and compute the changes for each repository.
    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/quickstart/src_batch_preview_waiting.png" class="screenshot">
1. When it's done, follow the link to the *preview page* to see all the changes that will be made.
    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/quickstart/src_batch_preview_link.png" class="screenshot">
1. Make sure the changes look right.
    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/quickstart/browser_preview.png" class="screenshot">
1. Click **Apply** to create the batch change. You should see something like this:
    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/quickstart/browser_created.png" class="screenshot">

**You've now created your first batch change!**

The batch change's *changesets* are still unpublished, which means they exist only on Sourcegraph and haven't been pushed to your code host yet. This is good news, as you probably don't want to publish these toy "Hello World" changesets to actively-developed repositories, because that might confuse people ("Why did you add this line to our READMEs?"). In the next steps, we'll prepare to publish a single test changeset.

## Publish a changeset

So far, nothing has been created on your code hosts. For that to happen, we need to tell Sourcegraph to *publish a changeset*.

Publishing causes commits, branches, and pull requests/merge requests to be written to your code host.

### Configure code host credentials

Batch Changes needs permission to publish changesets on your behalf. To grant permission, you will need to [add a personal access token](how-tos/configuring_credentials.md#adding-a-token) for each code host you'll be publishing changesets on.

This is a one-time operation, so don't worry---we won't need to do this for every batch change. You can also ask the administrators of your Sourcegraph instance to [configure global credentials](how-tos/configuring_credentials.md#global-service-account-tokens) instead.

### (Optional) Modify the batch spec to only target a specific repository

Before publishing, you might want to change the `repositoriesMatchingQuery` in `hello-world.batch.yaml` to target only a single, test repository that you could open a toy pull request/merge request on, such as one that you are the owner of. For example:

```yaml
# Find all repositories that contain a README.md file and whose name matches our test repo.
on:
  - repositoriesMatchingQuery: file:README.md repo:sourcegraph-testing/batch-changes-test-repo
```

With your updated batch spec, re-run the preview command, `src batch preview -f hello-world.batch.yaml` (you should notice it's a lot quicker this time thanks to the caching!). Once again, follow the link to the *preview page*. You should now see something like this:

<img src="https://sourcegraphstatic.com/docs/images/batch_changes/quickstart/browser_preview_update.png" class="screenshot">

As before, you get a preview before any changes are applied, but this time, you are *updating your existing changesets*. Now, all of the changesets listed will be *archived*, except for the one you're about to publish. Archiving will close the changesets on the codehost but leave them attached to your batch change for historical referencing.

Once you are ready, click **Apply** again to apply the update to your batch change.

### Publish to code host

There are [multiple ways to publish a changeset](how-tos/publishing_changesets.md#publishing-changesets). Let's look at how to do so from the screen you are currently on.

1. Select the changeset you would like to publish (in our case it's the only one).
    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/quickstart/browser_publish_select_changesets.png" class="screenshot">
1. Choose the "Publish changesets" action from the dropdown.
    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/quickstart/browser_publish_select_action.png" class="screenshot">
1. Click **Publish changesets**. You'll be prompted to confirm. You may also choose to publish your changeset(s) as draft(s), if the code host supports it.
    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/quickstart/browser_publish_confirm.png" class="screenshot">
1. Click **Publish**, and wait for an alert to appear (it may take a couple seconds).
1. Sit tight---once it's done, the page should update, and you should see something like this:
    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/quickstart/browser_publish_complete.png" class="screenshot">

## Congratulations!

**You've published your first Batch Changes changeset!** 🎉

Feel free to customize your batch spec and experiment with making other types of changes. You can also [explore the documentation](index.md) to learn what else you can do with Batch Changes!
