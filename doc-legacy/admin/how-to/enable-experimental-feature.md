# How to add, enable or disable an experimental feature

This document will take you through how to add, enable, or disable an experimental feature in Sourcegraph. Experimental features are not fully released, but we welcome your feedback at feedback@sourcegraph.com or on twitter @sourcegraph!

> NOTE: Changing these settings will affect the entire instance. We advise testing settings in a development environment before changing them in production.

## Prerequisites

* This document assumes that Sourcegraph is installed
* Assumes you have site-admin privileges on the instance

## Steps to enable/disable

1. Navigate to Site Admin > Global settings in the UI, or follow this link format for your `externalUrl/site-admin/global-settings`
2. Scroll down to find where `experimentalFeatures` is located. Example:

```json
"experimentalFeatures": {
    "searchStreaming": true,
    "showSearchContext": false,
},
```
3. Locate the feature you would like to disable or enable, setting `true` for enable or `false` for disable.
4. If adding a feature, follow the format `"featureName": true,`
5. After changing the values in Site admin > Global settings, the frontend will either restart automatically or you might be asked to restart the frontend for the changes to take effect.
6. For more information, see [Editing global settings for site-admins](https://docs.sourcegraph.com/admin/config/settings#editing-global-settings-for-site-admins)

## Further resources

* [Sourcegraph - Configuration Settings](https://docs.sourcegraph.com/admin/config/settings)
* [Sourcegraph - Site configuration](https://docs.sourcegraph.com/admin/config/site_config)
* Learn more about new experimental features on our [Blog](https://sourcegraph.com/blog) or Twitter [@sourcegraph](https://twitter.com/sourcegraph/)
