# Creating an access token

Access tokens permit authenticated access to the Sourcegraph API. This is required for [the `src` command line interface to Sourcegraph](../index.md) to operate, and also allows other tools that integrate with Sourcegraph to issue requests on your behalf.

Creating an access token is done through your user settings. This video shows the steps, which are then described below:

<video width="1920" height="1080" autoplay controls loop muted playsinline style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://sourcegraphstatic.com/docs/images/integration/cli/token.webm" type="video/webm">
  <source src="https://sourcegraphstatic.com/docs/images/integration/cli/token.mp4" type="video/mp4">
</video>

1. From any Sourcegraph page, click on your avatar at the top right of the page.
1. Select **Settings** from the dropdown menu.
1. Select **Access tokens** from the sidebar menu.
1. Click **Generate new token**.
1. Enter a description, such as `src`.

    > NOTE: The `user:all` scope that is selected by default is sufficient for all normal `src` usage, and most uses of the GraphQL API. If you're an admin, you should only enable `site-admin:sudo` if you intend to impersonate other users.
1. Click **Generate token**.
1. Sourcegraph will now display your access token. You **must copy it from this screen**: once this page is closed, you cannot access the token again and can only revoke it and issue a new one.

You can then set [the `SRC_ACCESS_TOKEN` environment variable](../explanations/env.md) to the token to use it with `src`.
