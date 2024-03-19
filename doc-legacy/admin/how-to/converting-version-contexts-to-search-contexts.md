# How to convert version contexts to search contexts

This guide will provide steps for migrating from [version contexts](../../code_search/explanations/features.md#version-contexts-sunsetting) to [search contexts](../../code_search/explanations/features.md#search-contexts) on your private Sourcegraph instance.

## Prerequisites

* This document assumes that you have already configured version contexts in your site configuration.
* Search contexts and search context management are enabled in global settings.

Site admins can enable search contexts on private Sourcegraph instances in **global settings** using the following:

```json
"experimentalFeatures": {  
  "showSearchContext": true,
}
```

**Note**: While version contexts are located in the site configuration, search contexts are enabled through the global settings.

Reload after saving changes to see search contexts enabled.

## Steps to convert version contexts to search contexts

1. Log in to your private Sourcegraph instance as a site admin.
2. Navigate to `https://your_sourcegraph_instance.com/contexts`.
3. Press `Convert version contexts`. A list of [existing version contexts](../../code_search/explanations/features.md#version-contexts-sunsetting) found in the site configuration will be shown.
4. Convert either all version contexts at once, or specific individual version contexts as desired.
5. Navigate back to `https://your_sourcegraph_instance.com/contexts`. Converted version contexts will be listed.

Converted search contexts can be used immediately by users on the Sourcegraph instance. The contexts selector will be shown in the search input.

## Discontinuing use of version contexts on your private Sourcegraph instance

Once desired existing version contexts have been converted into search contexts, we recommend discontinuing use of version contexts.

To discontinue use of version contexts:

1. Navigate to your site configuration.
2. Locate `experimentalFeatures.versionContexts` in the site configuration, and remove the `versionContexts` object and all of its contents.
3. Save changes.

After removing version contexts from the site configuration, reload the page. The version contexts UI dropdown will no longer be shown in the search input.

**Note:** Once version contexts are removed from site configuration, they will no longer be available for use or conversion into search contexts.
