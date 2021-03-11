# Quickstart for Batch Changes

Get started and create your first [Sourcegraph batch change](index.md) in 10 minutes or less.

## Introduction

In this guide, you'll create a Sourcegraph batch change that appends text to all `README.md` files in all of your repositories.

For more information about Batch Changes see the ["Batch Changes"](index.md) documentation and watch the [Batch Changes demo video](https://www.youtube.com/watch?v=EfKwKFzOs3E).

## Requirements
- A Sourcegraph instance with some repositories in it. See "[Quickstart](../../index.md#quickstart)" on how to setup a Sourcegraph instance.
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
    **Windows**: see ["Sourcegraph CLI for Windows"](../cli/explanations/windows.md)
2. Authenticate `src` with your Sourcegraph instance by running **`src login`** and following the instructions:

    ```
    src login https://YOUR-SOURCEGRAPH-INSTANCE
    ```
    <img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/campaigns/src_login_success.png" class="screenshot">


Once `src login` reports that you're authenticated, we're ready for the next step.

## Write a batch spec

A **batch spec** is a YAML file that defines a batch change. It specifies which changes should be made in which repositories and how those should be published on the code host.

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

## Create the batch change

Let's see the changes that will be made. Don't worry---no commits, branches, or changesets will be published yet (the repositories on your code host will be untouched).

1. In your terminal, run this command:

    <pre>src batch preview -f hello-world.batch.yaml</pre>
1. Wait for it to run and compute the changes for each repository.
    <img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/campaigns/src_campaign_preview_waiting.png" class="screenshot">
1. When it's done, click the displayed link to see all of the changes that will be made.
    <img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/campaigns/src_campaign_preview_link.png" class="screenshot">
1. Make sure the changes look right.
    <img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/campaigns/browser_campaign_preview.png" class="screenshot">
1. If you want to modify which changes are made, edit the `hello-world.batch.yaml` file, rerun the `src batch preview` command and open the newly generated preview URL.

    >NOTE: If you want to run the batch change on fewer repositories, change the `repositoriesMatchingQuery` in `hello-world.batch.yaml` to something like `file:README.md repo:myproject` (to only match repositories whose name contains `myproject`).
1. Click the **Apply spec** button to create the batch change. You should see something like this:
    <img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/campaigns/browser_campaign_created.png" class="screenshot">

You created your first batch change! The batch change's changesets are still unpublished, which means they exist only on Sourcegraph and haven't been pushed to your code host yet.

## Publish the changes

So far, nothing has been created on the code hosts yet. For that to happen, we need to publish the changesets in our batch change.

Publishing causes commits, branches, and pull requests/merge requests to be created on your code host.

_You probably don't want to publish these toy "Hello World" changesets to actively developed repositories, because that might confuse people ("Why did you add this line to our READMEs?")._

### Configure code host credentials

Since Batch Changes need write permissions to open changesets, you'll need to add a personal access token for each code host you'll be publishing changesets on. This is a one time operation that you don't need to do for each batch change.

See "[Configuring user credentials](how-tos/configuring_user_credentials.md)" for more detail on adding and removing user tokens beyond the quickstart below, or ["Code host interactions in Batch Changes"](explanations/permissions_in_batch_changes.md#code-host-interactions-in-batch-changes) for details on what the permissions are used for.

To add a personal access token:

1. From any Sourcegraph page, click on your avatar at the top right of the page.
1. Select **Settings** from the dropdown menu.
1. Click **Batch Changes** on the sidebar menu.
1. Click **Add token** next to the code host you want to configure.
1. Go to the code host and create a personal access token with the exact scopes or permissions required, which are noted below the token text field. For more provider-specific detail, please refer to "[GitHub](how-tos/configuring_user_credentials.md#github)", "[GitLab](how-tos/configuring_user_credentials.md#gitlab)", or "[Bitbucket Server](how-tos/configuring_user_credentials.md#bitbucket-server)".
1. Click **Add token** to save the token.

The red circle next to the code host will now change to a green tick. Sourcegraph has everything it needs to publish changesets to that code host!

### Publishing changesets

Now that you have credentials set up, you can publish the changesets in the batch change. On a real batch change, you would do the following:

1. Change the `published: false` in `hello-world.batch.yaml` to `published: true`.
    <img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/campaigns/campaign_publish_true.png" class="screenshot">

    > NOTE: Change [`published` to an array](references/batch_spec_yaml_reference.md#publishing-only-specific-changesets) to publish only some of the changesets, or set [`'draft'` to create changesets as drafts on code hosts that support drafts](references/batch_spec_yaml_reference.md#changesettemplate-published).
1. Run the `src batch preview` command again and open the URL.
    <img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/campaigns/src_rerun_preview.png" class="screenshot">
1. On the preview page you can confirm that changesets will be published when the spec is applied.
    <img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/campaigns/browser_campaign_preview_publish.png" class="screenshot">
1. Click the **Apply spec** button and those changesets will be published on the code host.
    <img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/campaigns/browser_campaign_async.png" class="screenshot">

    > NOTE: You can also create or update a batch change by running `src batch apply`. This skips the preview stage, and is especially useful when updating an existing batch change.

## Congratulations!

You've created your first batch change! 🎉🎉

You can customize your batch spec and experiment with making other types of changes.

To update your batch change, edit `hello-world.batch.yaml` and run `src batch preview` again. (As before, you'll see a preview before any changes are applied.)

To learn what else you can do with Batch Changes, see "[Batch Changes](index.md)" in Sourcegraph documentation.
