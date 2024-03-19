# Testing a Phabricator and Gitolite instance

## Browser Extension

1. Verify [`sourcegraph.enabled`](https://phabricator.sgdev.org/config/edit/sourcegraph.enabled/) is set to `true`
2. Point your browser extension to a Sourcegraph instance with the following code host:
    ```
    {
      "prefix": "gitolite.sgdev.org/",
      "host": "git@gitolite.sgdev.org",
    }
    ```
3. Verify the [`sourcegraph.callsignMappings`](https://phabricator.sgdev.org/config/edit/sourcegraph.callsignMappings/) are correctly set
4. Make sure your browser extension has permissions for `https://phabricator.sgdev.org` (you can check this through the popup)
5. Navigate to a [single file](https://phabricator.sgdev.org/source/test/browse/master/main.go)
    - Verify "View on Sourcegraph" button is present and working correctly
    - Verify hovers work as expected
6. Navigate to a [diff](https://phabricator.sgdev.org/D3)
    - Verify "View on Sourcegraph" buttons are present on all change types, and working correctly
    - Verify hovers are working correctly on added, removed, unchanged lines

## Native Integration

1. Run a local Sourcegraph dev instance tunnelled through ngrok
2. Set `corsOrigin` to `"https://phabricator.sgdev.org"` in your site config
3. Add the following Gitolite code host:
    ```
    {
      "prefix": "gitolite.sgdev.org/",
      "host": "git@gitolite.sgdev.org",
    }
    ```
4. Verify that the phabricator assets are served:
    - `%NGROK_URL%/.assets/extension/scripts/phabricator.bundle.js`
    - `%NGROK_URL%/.assets/extension/css/main.css`
5. Set [`sourcegraph.url`](https://phabricator.sgdev.org/config/edit/sourcegraph.url/) to your tunnelled ngrok URL
6. Verify the [`sourcegraph.callsignMappings`](https://phabricator.sgdev.org/config/edit/sourcegraph.callsignMappings/) are correctly set
7. Verify [`sourcegraph.enabled`](https://phabricator.sgdev.org/config/edit/sourcegraph.enabled/) is set to `true`
8. Navigate to a [single file](https://phabricator.sgdev.org/source/test/browse/master/main.go)
    - Verify "View on Sourcegraph" button is present and working correctly
    - Verify hovers work as expected
9. Navigate to a [diff](https://phabricator.sgdev.org/D3)
    - Verify "View on Sourcegraph" buttons are present on all change types, and working correctly
    - Verify hovers are working correctly on added, removed, unchanged lines
