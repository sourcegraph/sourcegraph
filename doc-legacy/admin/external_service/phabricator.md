# Phabricator

Site admins can associate Git repositories on [Phabricator](https://phabricator.org) with Sourcegraph so that users can jump to the Phabricator repository from Sourcegraph and use the [Phabricator extension](#native-extension) and [browser extension](../../integration/browser_extension.md) with Phabricator.

> ⚠️ NOTE: Sourcegraph support of Phabricator is limited ([learn more](../../integration/phabricator.md)), and not expected to evolve due to the [announced](https://admin.phacility.com/phame/post/view/11/phacility_is_winding_down_operations/) cease of support for Phabricator.

To connect Phabricator to Sourcegraph:

1. Go to **Site admin > Manage code hosts > Add repositories**
1. Select **Phabricator**.
1. Configure the connection to Phabricator using the action buttons above the text field, and additional fields can be added using <kbd>Cmd/Ctrl+Space</kbd> for auto-completion. See the [configuration documentation below](#configuration).
1. Press **Add repositories**.

## Repository association

Sourcegraph can provide users with links to various Phabricator pages (for files, commits, branches, etc.) if you add Phabricator as a connection (in **Site admin > Manage code hosts**).

A Phabricator configuration consists of the following fields:

- `url` field that maps to the url of the Phabricator host
- `token` an optional Conduit API token, which you may generate from the Phabricator web interface. The token is used to fetch the list of repos available on the Phabricator installation
- `repos` if your Phabricator installation mirrors repositories from a different origin than Sourcegraph, you must specify a list of repository `path`s (as displayed on Sourcegraph) and their corresponding Phabricator `callsign`s. For example: `[{ path: 'gitolite.example.org/foobar', callsign: 'FOO'}]`. _Note that the `callsign` is case sensitive._

At least one of `token` and `repos` should be provided.

For example:

```json
{
  // ...
  "phabricator": [
    {
      "url": "https://phabricator.example.com",
      "token": "api-abcdefghijklmnop",
      "repos": [{ "path": "gitolite.example.com/mux", "callsign": "MUX" }]
    }
  ]
  // ...
}
```

See [configuration documentation](#configuration) below for more information.

### Troubleshooting

If your outbound links to Phabricator are not present or not working, verify your Sourcegraph repository path matches the "normalized" URI output by Phabricator's `diffusion.repository.search` conduit API.

For example, if you have a repository on Sourcegraph whose URL is `https://sourcegraph.example.com/path/to/repo` then you should see a URI returned from `diffusion.repository.search` whose `normalized` field is `path/to/repo`. Check this by navigating to `$PHABRICATOR_URL/conduit/method/diffusion.repository.search/` and use the "Call Method" form with `attachments` field set to `{ "uris": true }` and `constraints` field set to `{ "callsigns": ["$CALLSIGN_FOR_REPO_ON_SOURCEGRAPH"]}`. In the generated output, verify that the first URI has a normalized path equal to `path/to/repo`.

## Native extension

For production usage, we recommend installing the Sourcegraph Phabricator extension for all users (so that each user doesn't need to install the browser extension individually). This involves adding a new extension to the extension directory of your Phabricator instance.

See the [phabricator-extension](https://github.com/sourcegraph/phabricator-extension) repository for installation instructions and configuration settings.

The Sourcegraph instance's site admin must [update the `corsOrigin` site config property](../config/site_config.md) to allow the Phabricator extension to communicate with the Sourcegraph instance. For example:

```json
{
  // ...
  "corsOrigin":
    "https://my-phabricator.example.com"
  // ...
}
```

## Configuration

<div markdown-func=jsonschemadoc jsonschemadoc:path="admin/external_service/phabricator.schema.json">[View page on docs.sourcegraph.com](https://docs.sourcegraph.com/admin/external_service/phabricator) to see rendered content.</div>
