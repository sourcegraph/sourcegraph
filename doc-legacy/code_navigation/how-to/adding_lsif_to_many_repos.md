# Tutorial: Adding precise indexing to many repositories

## Prerequisites

* You have a private Sourcegraph instance (i.e., you're not using
  Sourcegraph.com), and the repositories you wish to explore have been added to
  that instance.
* You have admin access on your Sourcegraph instance.
* You have installed [`src-cli`](https://github.com/sourcegraph/src-cli) to your local machine.
* You have the ability to create org-level secrets in your GitHub organization.
* Your Sourcegraph instance is accessible from your GitHub instance. If you are
  using GitHub.com, this means your Sourcegraph instance is accessible on the
  public Internet.

## Directions

Watch the video or follow the written directions below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/tfk3nwvltAw" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

1. Generate an access token for your Sourcegraph instance. The access token does
   NOT need `sudo` privileges, but should have permission to read every
   repository for which precise indexing should be enabled.
  1. Create the following secrets in each GitHub organization represented in the
     set of repositories. Follow the [GitHub
     documentation](https://docs.github.com/en/free-pro-team@latest/actions/reference/encrypted-secrets#creating-encrypted-secrets-for-an-organization)
     for doing this.
     1. `srcEndpoint`: the URL of the Sourcegraph instance, from the perspective
        of the GitHub instance. If using GitHub.com, this is just the URL you use
        to access Sourcegraph.
     1. `srcAccessToken`: the Sourcegraph access token you just created.

1. Download
   [`lsif-go.campaign.yaml`](https://raw.githubusercontent.com/sourcegraph/snippets/main/lsif/lsif-go.campaign.yaml)
   to your local machine.

1. Verify the list of repositories for which you wish to enable precise indexing:
   ```
   # Use the values you set in your GitHub secrets for SRC_ENDPOINT and SRC_ACCESS_TOKEN
   SRC_ENDPOINT= SRC_ACCESS_TOKEN= src batch repositories -f lsif-go.campaign.yaml
   ```
   If the set of repositories displayed is not the set of repositories for which you want to enable precise indexing, modify the `repositoriesMatchingQuery` line in `lsif-go.campaign.yaml` to specify the Sourcegraph search query that selects the desired repository set.
1. Execute the batch spec to generate a list of all pull requests that will be created:
   ```
   # Use the values you set in your GitHub secrets for SRC_ENDPOINT and SRC_ACCESS_TOKEN.
   SRC_ENDPOINT= SRC_ACCESS_TOKEN= src batch preview -f lsif-go.campaign.yaml
   ```
   This will create a batch change preview in Sourcegraph. Navigate to the URL printed in the
     terminal to preview all the pull requests that will be created.

1. Once you've verified the preview looks correct, change `published: false` to `published: true` in `lsif-go.campaign.yaml` and run the following:
  ```
   # Use the values you set in your GitHub secrets for SRC_ENDPOINT and SRC_ACCESS_TOKEN.
  SRC_ENDPOINT= SRC_ACCESS_TOKEN= src batch apply -f lsif-go.campaign.yaml
  ```
  Now, go back to the batch change page and verify the pull requests have been
  created.
  1. If there are errors creating any pull request, check to make sure the GitHub token in the
     Sourcegraph code host configuration (Site admin > Manage code hosts) has the
     necessary scopes (`repo` and (`read:discussion` or `read:org`)).
  1. Verify that the GitHub action has run successfully in the repositories on GitHub.
  1. Verify that the index has been successfully uploaded to Sourcegraph by
     navigating to any repository page on Sourcegraph > Settings > Code
     intelligence: Uploads. You should also be able to explore the code at that
     revision with precise code navigation.

1. Merge the pull requests created by the batch change and close the batch
   change. The GitHub action should now run on each push, generating an index for the
   pushed revision and uploading it to your Sourcegraph instance.
