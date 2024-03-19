# Product documentation implementation

The [documentation guidelines](https://handbook.sourcegraph.com/engineering/product_documentation) apply to product documentation. This page has information specific to this repository's documentation.

## Documentation directory structure

The documentation is broken down into 3 different areas:

1. User
1. Admin
1. Development

Each of these areas has the docs organized by the 4 different types:

1. Tutorials
1. How-to guides
1. Explanation or background information
1. Reference

This structure is inspired by the [Divio documentation system](https://documentation.divio.com/).

## Previewing changes locally

You can preview the documentation site at http://localhost:5080 when running Sourcegraph in [local development](../setup/index.md) (using `sg start`). It uses content, templates, and assets from the local disk. There is no caching or background build process, so you'll see all changes reflected immediately after you reload the page in your browser.

You can also run the docsite on its own with the following command:

```sh
sg run docsite
```

## Linking to documentation in-product

In-product documentation links should point to `/help/PATH` instead of using an absolute URL of the form https://docs.sourcegraph.com/PATH. This ensures they link to the documentation for the current product version. There is a redirect (when using either `<a>` or react-router `<Link>`) from `/help/PATH` to the versioned docs.sourcegraph.com URL (https://docs.sourcegraph.com/@VERSION/PATH).

## Adding images to the documentation

We generally try to avoid adding large binary files to our repository. Images to be used in documentation fall under that category, but there can be exceptions if the images are small.

- If the image is less than 100kb in size, it can be added to the `./doc` folder.
- If it is bigger than 100kb, upload it to the [sourcegraph-assets/docs/images](https://console.cloud.google.com/storage/browser/sourcegraph-assets/docs/images/?project=sourcegraph-de&folder=true&organizationId=true) on Google Cloud storage and link to it.

## Updating documentation

To update documentation content, templates, or assets on https://docs.sourcegraph.com, push changes in the `doc/` directory to this repository's `main` branch, then wait up to 5 minutes. Every 5 minutes, docs.sourcegraph.com reloads all content, templates, and assets from `main`.

- Documentation content lives in `doc/**/*.md`.
- The sidebar lives in `doc/sidebar.md`. Only important pages belong in the sidebar; use section index page links for other documents.
- Assets and templates live in `doc/_resources/{templates,assets}`.

Updates to the redirects in `doc/_resources/assets/redirects` require a full reload of the service, which involves deleting the relevant Kubernetes pods. See ["Forcing immediate reload of data"](#forcing-immediate-reload-of-data) for more details.

## Advanced documentation site

Our documentation site (https://docs.sourcegraph.com) runs [docsite](https://github.com/sourcegraph/docsite).

See "[Updating documentation](#updating-documentation)" and "[Previewing changes locally](#previewing-changes-locally)" for the most common workflows involving the documentation site.

## SEO

Every markdown document can be prefixed with front matter, which comes with a specific set of fields that are used
to generate metatags and opengraph tags that improve our ranking. The excerpt below highlights all available fields, which are all optional. Invalid fields will raise an error at runtime but can be caught ahead by running `docsite check`

```
---
title: "That current page title"
description: "A brief description of what that page is about"
category: "Which category of the content does this belong"
type: "article (will default to website otherwise)"
imageURL: "https://sourcegraph.com/.assets/img/sourcegraph-logo-dark.svg"
tags: 
  - A list of tags such as
  - Code Search
  - How to
---

# My markdown title 

My content
```

See [the Open Graph protocl](https://ogp.me) to learn more about these tags.

## Forcing immediate reload of data

The docs.sourcegraph.com site reloads content, templates, and assets every 5 minutes. After you push a [documentation update](#updating-documentation), just wait up to 5 minutes to see your changes reflected on docs.sourcegraph.com.

If you need to force a reload — either because your change is _extremely_ urgent and can't wait 5 minutes, or because you've updated redirects — you can delete the `docs-sourcegraph-com-*` Kubernetes pod on the Sourcegraph.com Kubernetes cluster. Once done, it will restart and come back online with the latest data.

>WARNING: There may be a few seconds of downtime when restarting the docs cluster this way. This shouldn't be a routine part of your workflow!

To do this, follow the ["Restarting sourcegraph.com and docs.sourcegraph.com" playbook](https://handbook.sourcegraph.com/departments/engineering/dev/process/deployments/playbooks/#restarting-docssourcegraphcom).

## Other ways of previewing changes locally (very rare)

The [local documentation server](#previewing-changes-locally) on http://localhost:5080 only serves a single version of the documentation (from the `doc/` directory of your working tree). This usually suffices.

In very rare cases, you may want to run a local documentation server with a different configuration (described in the following sections).

### Running a local server that mimics prod configuration

If you want to run the doc site *exactly* as it's deployed (reading templates and assets from the remote Git repository, too), consult the current Kubernetes deployment spec and invoke `docsite serve` with the deployment's `DOCSITE_CONFIG` env var, the end result looking something like:

```bash
DOCSITE_CONFIG=$(cat <<-'DOCSITE'
{
  "templates": "https://codeload.github.com/sourcegraph/sourcegraph/zip/docsite-fallback#*/doc/_resources/templates/",
  "assets": "https://codeload.github.com/sourcegraph/sourcegraph/zip/docsite-fallback#*/doc/_resources/assets/",
  "content": "https://codeload.github.com/sourcegraph/sourcegraph/zip/refs/heads/$VERSION#*/doc/",
  "defaultContentBranch": "docsite-fallback",
  "baseURLPath": "/",
  "assetsBaseURLPath": "/assets/"
}
DOCSITE
) docsite serve -http=localhost:5081
```

You can test your changes alongside prod configuration (including things like multi-version support, i.e. `/@version/content`) by pushing your changes to a branch (e.g. `my-branch`) and run the above command after replacing all instances of `main` with `my-branch`.
