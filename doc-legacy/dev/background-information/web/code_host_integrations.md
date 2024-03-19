# Developing the code host integrations

**Code host integrations** are how Sourcegraph delivers features directly into a code host's interface. These integrations allow developers to benefit from Sourcegraph without leaving their code host as they navigate repositories, source code files, commits, and diffs.

For a guide on usage of integrations, see the [Integrations section](../../../integration/index.md).

## How code host integrations are delivered

Code host integrations are implemented as a JavaScript bundle that is injected directly into the code host's web UI.

Sourcegraph has two channels available to deliver this JavaScript bundle:

- **Native integrations** which configure the code host to natively load the JavaScript as an additional resource on every page. These integrations can be configured once by an administrator (typically on a self-hosted code host instance) and are then automatically available to all users.
- [**Browser extensions**](../../../integration/browser_extension.md) for Chrome and Firefox, which inject the JavaScript bundle on all code host pages. These integrations don't require any central configuration of the code host, but they require each user to individually install the browser extension in order to benefit from it.

## Code views

Code host integrations mainly operate on code views.

A code view is any instance of source code being displayed on any code host page. It can be an entire source file, a snippet of a file, or a diff view. There may be many code views on a single page, for example on a pull request page.

Code host integrations work by first [identifying all code views on a given page](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@a3b40f3ae9376b42ce9a67b5a33f177ba98ac050/-/blob/browser/src/shared/code-hosts/shared/codeHost.tsx?subtree=true#L715), and then by adding interface elements (such as [action buttons](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@a3b40f3ae9376b42ce9a67b5a33f177ba98ac050/-/blob/browser/src/shared/code-hosts/shared/codeHost.tsx?subtree=true#L747-765)), and [listening for hover events](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@a3b40f3ae9376b42ce9a67b5a33f177ba98ac050/-/blob/browser/src/shared/code-hosts/shared/codeHost.tsx?subtree=true#L971-992) over specific code tokens in order to show hover pop-ups.

## Contributing to code host integrations

The source code for code host integrations is located in the [`browser`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/tree/main/client/browser) directory of the Sourcegraph repository.

For build steps and details of the directory layout, see [`browser/README.md`](https://github.com/sourcegraph/sourcegraph/tree/main/client/browser/README.md)
