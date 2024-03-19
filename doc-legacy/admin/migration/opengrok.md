# Migrating from Oracle OpenGrok to Sourcegraph for code search

> NOTE: This guide helps Sourcegraph admins migrate from deploying Oracle OpenGrok to Sourcegraph. See our [Oracle OpenGrok end user migration guide](../../../code_search/how-to/opengrok.md) to learn how to switch from OpenGrok's search syntax to Sourcegraph's.

You can migrate from Oracle's [OpenGrok](https://oracle.github.io/opengrok/) to [Sourcegraph](https://sourcegraph.com) for code search by following the steps in this document.

- [Background](opengrok.md#background)
- [Migration guide](opengrok.md#migration-guide)
  1.  [Deploying Sourcegraph](opengrok.md#deploying-sourcegraph)
  1.  [Configuring repositories](opengrok.md#configuring-repositories)
  1.  [Configuring user authentication](opengrok.md#configuring-user-authentication)
  1.  [Rolling out Sourcegraph organization-wide](opengrok.md#rolling-out-sourcegraph-organization-wide)

## Background

Sourcegraph is a self-hosted code search and intelligence tool that helps developers find, review, understand, and debug code. Organizations that switch from OpenGrok to Sourcegraph typically cite the following reasons:

- Sourcegraph supports [searching any revision](../../code_search/explanations/features.md) (not just specific branches) and does not require waiting for periodic reindexing.
- Sourcegraph's [query syntax](../../code_search/reference/queries.md), user interface, and [integrations](../../integration/index.md) are superior and easier to use.
- Sourcegraph's [code navigation](../../code_navigation/index.md), has better language support (hover tooltips, definitions, references, implementations, etc.) and is based on the Language Server Protocol standard.
- The [Sourcegraph API](../../api/graphql/index.md) is more powerful, better documented, and easier to use than OpenGrok's API.
- Sourcegraph scales to more repositories/users and supports Kubernetes-based clustered/high-availability deployments better (with the [cluster deployment option](../deploy/kubernetes/index.md)).

Both Sourcegraph and OpenGrok are self-hosted, and your code never touches Sourcegraph's (or Oracle's) servers.

Oracle releases OpenGrok under the open-source CDDL license and does not (currently) have any monetization plans for it. Sourcegraph is a commercial product, with a free tier and [paid premium features](https://sourcegraph.com/pricing) available.

Every organization's needs are different. [Try Sourcegraph for free](../deploy/index.md) to see if it's right for your organization.

For more information about Sourcegraph, see:

- "[What is Sourcegraph?](../../getting-started/index.md#what-is-sourcegraph)"
- "[Code search overview](../../code_search/index.md)"
- Live examples on public code: [Sourcegraph tour](../../getting-started/tour.md)

## Migration guide

Migrating from Oracle OpenGrok to Sourcegraph consists of 4 steps:

1.  [Deploying Sourcegraph](opengrok.md#deploying-sourcegraph)
1.  [Configuring repositories](opengrok.md#configuring-repositories)
1.  [Configuring user authentication](opengrok.md#configuring-user-authentication)
1.  [Rolling out Sourcegraph organization-wide](opengrok.md#rolling-out-sourcegraph-organization-wide)

The following sections guide you through the migration process.

### Deploying Sourcegraph

We offer several methods for deploying Sourcegraph for various requirements—see [Getting started](../../index.md#getting-started) to learn more about how to get started with Sourcegraph.

Choose a deployment option and follow the instructions. When you've signed into your Sourcegraph instance as a site admin, continue to the next section.

### Configuring repositories

Sourcegraph and Oracle OpenGrok differ in how they access repositories:

- **"Passive":** OpenGrok reads all repositories underneath the `SRC_ROOT` path on disk. You place repositories there and configure the [sync.py tool](https://github.com/oracle/opengrok/wiki/Repository-synchronization) to fetch updates.
- **"Active":** Sourcegraph automatically handles cloning and updating repositories from [code hosts (GitHub, GitLab, Bitbucket Server / Bitbucket Data Center, AWS CodeCommit, etc.](../repo/add.md).

Sourcegraph's "active" model lets it:

- serve fresher repository data (to search and browse just-`git push`ed data);
- synchronize the list of repositories on the code host (so that newly added repositories are searchable/browseable)
- offer code host integrations and "View file on code host" links

Sourcegraph also partially supports the "passive" model like OpenGrok, but it's not recommended because you lose these benefits. To use it anyway, see "[Add repositories already cloned to disk](../repo/pre_load_from_local_disk.md)".

To configure which repositories Sourcegraph will make available for searching and browsing:

- For repositories on popular code hosts, use the **Quick configure** actions on your Sourcegraph instance's site admin "Site configuration" page.
- For other repositories and for more information, see "[Add repositories](../repo/add.md)" in the Sourcegraph documentation.

When you've added repositories and confirmed that you can search and browse them, continue to the next section.

### Configuring user authentication

Like Oracle OpenGrok, Sourcegraph is self-hosted. You control who can access it. Sourcegraph supports many user authentication and security options:

- [OpenID Connect user authentication](../auth/index.md#openid-connect) and [SAML user authentication](../auth/index.md#saml) (for Google/Google Workspace accounts, Okta, OneLogin, etc.)
- [HTTP user authentication proxies](../auth/index.md#http-authentication-proxies)
- [Builtin username-password authentication](../auth/index.md#builtin-authentication)
- [TLS/SSL and other HTTP/HTTPS configuration](../http_https_configuration.md)

### Rolling out Sourcegraph organization-wide

After you've set Sourcegraph up, it's time to share it with your organization. Successful roll-outs of Sourcegraph typically involve the following steps.

- Send a message (in team chat or email) announcing Sourcegraph:

  > I set up Sourcegraph as a possible alternative to OpenGrok for code search. [Describe the perceived benefits vs. OpenGrok that are most relevant to your organization.]
  >
  > Try it:
  >
  > - Search: [URL to an example search results page on your Sourcegraph instance]
  > - Code browsing: [URL to a code file page on your Sourcegraph instance]
  >
  > [Include screenshots of your Sourcegraph instance here]
  >
  > Post feedback at https://github.com/sourcegraph/sourcegraph [change if needed]

- Create an internal document based on the [Sourcegraph tour](../../getting-started/tour.md), substituting links to and names of your organization's code. This explains how Sourcegraph helps developers perform common tasks better.
- Encourage installation of the [browser extension](../../integration/browser_extension.md) to get Sourcegraph code navigation and search in your organization's existing code host.
- Roll out the Chrome extension using [Google Workspace automatic installation](../../integration/browser_extension/how-tos/google_workspace.md) to everyone in your organization.
- Check the access logs for OpenGrok to see what users search for. Try searching for the same things on Sourcegraph, and ensure that you get the expected results. (Note: Sourcegraph's [search query syntax](../../code_search/reference/queries.md) differs from OpenGrok's.)
- Monitor your Sourcegraph instance's site admin "Analytics" page to see who's using it. Ask them for specific feedback. Also seek feedback from the most frequent users of OpenGrok.

If there are any blockers preventing your organization from switching to Sourcegraph, we'd love to hear from you so we can address them.

Let us know how we can help! [File an issue](https://github.com/sourcegraph/sourcegraph) with feedback/problems/questions, or [contact us directly](https://sourcegraph.com/contact).
