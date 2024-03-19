# Gitolite

> NOTE: While it is possible to connect Gitolite repositories to Sourcegraph, we currently do not recommend it. If you have specific questions, please discuss with your account team. 

Site admins can link and sync Git repositories on [Gitolite](https://gitolite.com) with Sourcegraph so that users can search and navigate the repositories.

To connect Gitolite to Sourcegraph:

1. Set up [git SSH authentication](../repo/auth.md) for your gitolite server.
1. Go to **Site admin > Manage code hosts > Add repositories**
1. Select **Gitolite**.
1. Configure the connection to Gitolite using the action buttons above the text field, and additional fields can be added using <kbd>Cmd/Ctrl+Space</kbd> for auto-completion. See the [configuration documentation below](#configuration).
1. Press **Add repositories**.

## Configuration

<div markdown-func=jsonschemadoc jsonschemadoc:path="admin/external_service/gitolite.schema.json">[View page on docs.sourcegraph.com](https://docs.sourcegraph.com/admin/external_service/gitolite) to see rendered content.</div>
