# Quickstart for Browser Extensions

Get started with our Browser Extension in 5 minutes or less

## Installation

Use one of the following links to install the Sourcegraph Browser Extension for your favourite browser.

<p>
  <a target="_blank" href="https://chrome.google.com/webstore/detail/sourcegraph/dgjhfomjieaadpoljlnidmbgkdffpack" style="display:flex;align-items:center">
  <img src="../img/chrome.svg" width="24" height="24" style="margin-right:5px" /> <strong>Install Sourcegraph for Chrome</strong>
  </a>
</p>

<p>
  <a target="_blank" href="https://apps.apple.com/us/app/sourcegraph-for-safari/id1543262193" style="display:flex;align-items:center">
  <img src="../img/safari.svg" width="24" height="24" style="margin-right:5px" /> <strong>Install Sourcegraph for Safari</strong>
  </a>
</p>

<p>
  <a target="_blank" href="https://addons.mozilla.org/en-US/firefox/addon/sourcegraph-for-firefox/" style="display:flex;align-items:center">
  <img src="../img/firefox.svg" width="24" height="24" style="margin-right:5px" /> <strong>Install Sourcegraph for Firefox</strong>
  </a>
</p>

>NOTE: If you were using our self-hosted version of Firefox Extension and are looking to upgrade, please check our [migration guide](../migrating_firefox_extension.md).

## Configure to use your Sourcegraph instance

<video class="theme-dark-only" width="1764" height="1390" autoplay loop muted playsinline style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/code-host-integration/PrivateInstanceDark.webm" type="video/webm">
  <source src="https://storage.googleapis.com/sourcegraph-assets/code-host-integration/PrivateInstanceDark.mp4" type="video/mp4">
  <p>Configure browser extension for your private Sourcegraph instance</p>
</video>
<video class="theme-light-only" width="1764" height="1390" autoplay loop muted playsinline style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/code-host-integration/PrivateInstanceLight.webm" type="video/webm">
  <source src="https://storage.googleapis.com/sourcegraph-assets/code-host-integration/PrivateInstanceLight.mp4" type="video/mp4">
  <p>Configure browser extension for your private Sourcegraph instance</p>
</video>

By default, the browser extension communicates with [sourcegraph.com](https://sourcegraph.com), which has only public code.

To use the browser extension with your private repositories, you need to set up a private Sourcegraph instance and connect the extension to it.

Follow these instructions:

1. [Install Sourcegraph](https://docs.sourcegraph.com/admin/install). Skip this step if you already have a private Sourcegraph instance.
2. Click the Sourcegraph extension icon in the browser toolbar on the browser tab for your private Sourcegraph instance then open the settings page.
3. Enter the URL (including the protocol) of your Sourcegraph instance (such as `https://sourcegraph.example.com`)
4. Make sure the connection status shows "Looks good!"


## Grant permissions for your code host

<video class="theme-dark-only" width="1762" height="1384" autoplay loop muted playsinline style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/code-host-integration/GrantPermissionsDark.webm" type="video/webm">
  <source src="https://storage.googleapis.com/sourcegraph-assets/code-host-integration/GrantPermissionsDark.mp4" type="video/mp4">
  <p>Grant permissions</p>
</video>
<video class="theme-light-only" width="1762" height="1384" autoplay loop muted playsinline style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/code-host-integration/GrantPermissionsLight.webm" type="video/webm">
  <source src="https://storage.googleapis.com/sourcegraph-assets/code-host-integration/GrantPermissionsLight.mp4" type="video/mp4">
  <p>Grant permissions</p>
</video>

- [GitHub.com](https://github.com/) - no action required.
- GitHub Enterprise, GitLab, Bitbucket Server, Bitbucket Data and Phabricator - you need to grant the extension permissions first.

To grant these permissions:

1. Navigate to any page on your code host.
2. Right-click the Sourcegraph icon in the browser extension toolbar.
3. Click "Enable Sourcegraph on this domain".
4. Click "Allow" in the permissions request popup.


