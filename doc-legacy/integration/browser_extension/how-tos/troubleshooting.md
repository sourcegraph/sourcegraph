# Troubleshooting

Below are the most common issues and how to resolve them.

## No code navigation or buttons ("View repository", "View file", etc.) are displayed on the code host

![Browser extension not working on code host](https://sourcegraphstatic.com/BrowserExtensionNotWorkingCodeHost.gif)

Try the following:

1. Click the Sourcegraph extension icon in the browser toolbar to open the settings page.
    - Ensure that the Sourcegraph URL is correct. It must point to your own Sourcegraph instance to work on private code.
    - Check whether any permissions must be granted. If so, the settings page will offer you to "grant the Sourcegraph browser extension additional permissions".
1. On some code hosts, you need to be signed in (to the code host) to use the browser extension. Try signing in.

## The *Enable Sourcegraph on this domain* option is not available

In rare cases, Chrome can get into the state where the option to **Enable Sourcegraph on this domain** is not available when right-clicking on the extension icon. One fix we've observed is to toggle the site access from on, to off, then on again (see below).

![Toggle site access for browser extension](https://sourcegraphstatic.com/ToggleSiteAccess.gif)

If that still doesn't work, viewing the console and network activity of the extension is the next step.

## Viewing browser extension console and network activity in Chrome and Safari

If you are still experiencing issues, the next step is to inspect the browser extension console output and network activity, often revealing subtle configuration errors.

![Chrome extension console and network activity](https://sourcegraphstatic.com/ChromeExtensionConsoleNetworkActivity.gif)

In Chrome:

1. Right click the Sourcegraph browser extension icon
2. Select Manage Extensions
3. Under Inspect Views select background page, this will open a dev console to the extension background page
4. In the developer console select the network tab

In Safari:

1. Ensure you have access to the develop tab by selecting Safari > Preferences > Advanced, at the bottom of the preference UI check the box labelled `Show Develop menu in menu bar`
2. In Develop select Web Extension Background Pages > Sourcegraph
3. Select the Network tab

If that still doesn't help, take a screenshot of the console and network activity and attach it [to a new issue](https://github.com/sourcegraph/sourcegraph/issues/new?assignees=&labels=&template=bug_report.md&title=Browser%20extension%20-%20) so we can investigate further.

## Unable to connect to `http://...` Ensure the URL is correct and you are logged in

Since `v3.14.0+`, the Sourcegraph browser extension can only authenticate with Sourcegraph instances that have [HTTPS](../../../admin/tls_ssl.md) configured.

Previously, the Sourcegraph browser extension was able to authenticate with instances that hadn't enabled tls / ssl. However, modern web browsers have started to adopt and implement [an IETF proposal](https://web.dev/samesite-cookies-explained/) that removes the deprecated logic that allowed this behavior. Please configure [HTTPS](../../../admin/tls_ssl.md) in order to continue using the browser extension with your private instance.

## `The string did not match the expected pattern` error in Safari

The above validation error occurs in Safari when the browser tries to parse a non-JSON text as JSON. Communication between the graphql server & browser extension is always assumed to be JSON. However, when a proxy/saml is used with Sourcegraph, sometimes the proxy can send text/html instead.

If you see this error displayed beneath the Url field of the sourcegraph extension, make sure that your current browser session is not in private mode. 
