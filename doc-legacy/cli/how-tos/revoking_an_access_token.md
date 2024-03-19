# Revoking an access token

Access tokens are tokens starting with `sgp_` which permit authenticated access to the Sourcegraph API. If you accidentally disclose an access token - such as by committing it to a source code repository - you should revoke it to prevent it from being used without your permission.

1. From any Sourcegraph page, click on your avatar at the top right of the page.
1. Select **Settings** from the dropdown menu.
1. Select **Access tokens** from the sidebar menu.
1. Identify the access token that was leaked and click the **Delete** button next to it.

You can then confirm that the access token was revoked by making a request to the GraphQL API:

```
curl \
  -H 'Authorization: token <YOUR_REVOKED_TOKEN>' \
  -d '{"query":"query { currentUser { username } }"}' \
  https://sourcegraph.com/.api/graphql
```

If the token has been revoked, you will receive a response containing "Invalid access token".
