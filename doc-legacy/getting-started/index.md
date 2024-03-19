# Using Sourcegraph

Welcome to Sourcegraph docs!

## What is Sourcegraph?

Sourcegraph is a code intelligence platform that helps devs answer questions in their code by searching and understanding across their organization's codebase.

## Who should use Sourcegraph?

In addition to the [companies listed on sourcegraph.com](https://sourcegraph.com), companies with a few hundred developers all the way up to those with more than 40,000 use Sourcegraph daily.

More specifically, Sourcegraph is great for all developers, except:

- those on smaller teams with a small amount of code
- those that rarely search, read, or review code

## Why do I need code search?

Both Facebook and Google provide an in-house Sourcegraph-like code search and intelligence tool to their employees. A [published research paper from Google](https://static.googleusercontent.com/media/research.google.com/en//pubs/archive/43835.pdf) and a [Google developer survey](https://docs.google.com/document/d/1LQxLk4E3lrb3fIsVKlANu_pUjnILteoWMMNiJQmqNVU/edit#heading=h.xxziwxixfqq3) showed that 98% of developers surveyed consider their Sourcegraph-like internal tool to be critical. Developers use it on average for 5.3 sessions each day. (Facebook's and Google's in-house tools are not available to other companies; use Sourcegraph instead.)

## What do I use Sourcegraph for?

Sourcegraph helps you:

- Find example code
- Explore/read code (including during a code review)
- Debug issues
- Locate a specific piece of code
- Determine the impact of changes
- And more!

Sourcegraph makes it faster and easier to perform these tasks, for you and everyone else at your organization.

## What does Sourcegraph do?

Sourcegraph's main features are:

- [Code navigation](#code-navigation): jump-to-definition, find references, and other smart, IDE-like code browsing features on any branch, commit, or PR/code review
- [Code search](#code-search): fast, up-to-date, and scalable, with regexp support on any branch or commit without an indexing delay (and diff search)
- [Notebooks](#notebooks): pair code and markdown to create powerful live and persistent documentation
- [Cody](#cody): read and write code with the help of a context-aware AI code assistant
- [Code Insights](#code-insights): reveal high-level information about your codebase at its current state and over time, to track migrations, version usage, vulnerability remediation, ownership, and anything else you can search in Sourcegraph
- [Batch Changes](#batch-changes): make large-scale code changes across many repositories and code hosts
- [Integrations](#integrations) with code hosts, code review tools, editors, web browsers, etc.

## How do I start using Sourcegraph?

1. [Deploy and Configure Sourcegraph](../admin/deploy/index.md) inside your organization on your internal code, if nobody else has yet
1. Install and configure the [web browser code host integrations](../integration/browser_extension.md) (recommended)
1. Start searching and browsing code on Sourcegraph by visiting the URL of your organization's internal Sourcegraph instance
1. [Personalize Sourcegraph](personalization/index.md) with themes, quick links, and badges!

You can also try [Sourcegraph.com](https://sourcegraph.com/search), which is a public instance of Sourcegraph for use on open-source code only.

## How is Sourcegraph licensed?

Sourcegraph Enterprise is Sourcegraph’s primary offering and includes all code intelligence platform features. Sourcegraph Enterprise is the best solution for enterprises who want to use Sourcegraph with their organization’s code.

Sourcegraph extensions are also OSS licensed (Apache 2), such as:

- [Sourcegraph browser extension](https://github.com/sourcegraph/sourcegraph/tree/master/client/browser)
- [Sourcegraph JetBrains extension](https://github.com/sourcegraph/sourcegraph/tree/main/client/jetbrains)

## How is Sourcegraph different than GitHub code search?

- [See how GitHub code search compares to Sourcegraph](./github-vs-sourcegraph.md)

## Code search

Sourcegraph code search is fast, works across all your repositories at any commit, and has minimal indexing delay. Code search also includes advanced features, including:

- [Powerful, flexible query syntax](../code_search/reference/queries.md)
- [Commit diff search](../code_search/explanations/features.md#commit-diff-search)
- [Commit message search](../code_search/explanations/features.md#commit-message-search)
- [Saved search scopes](../code_search/explanations/features.md#search-scopes)
- [Search contexts to search across a set of repositories at specific revisions](../code_search/explanations/features.md#search-contexts)
- [Saved search monitoring](../code_monitoring/index.md)

Read the [code search documentation](../code_search/index.md) to learn more and discover the full feature set. Here's a video to help you get started:
- [How to Search commits and diffs with Sourcegraph](https://youtu.be/w-RrDz9hyGI)
- [Search Examples](https://sourcegraph.github.io/sourcegraph-search-examples/)

## Code navigation

Sourcegraph gives your development team cross-repository IDE-like features on your code:

- Hover tooltips
- Go-to-definition
- Find references
- Symbol search

Sourcegraph gives you code navigation in:

- **code files in Sourcegraph's web UI**

![Hover tooltip](https://storage.googleapis.com/sourcegraph-assets/code-graph/docs/hover-tooltip.png)

- **diffs in your code review tool**, via [integrations](../integration/index.md)

![GitHub pull request integration](https://storage.googleapis.com/sourcegraph-assets/code-graph/docs/github-pr.png)

- **code files on your code host**, via [integrations](../integration/index.md)

![GitHub file integration](https://storage.googleapis.com/sourcegraph-assets/code-graph/docs/github-file.png)

Read the [code navigation documentation](../code_navigation/index.md) to learn more and to set it up.

## Cody

Cody is an AI code assistant that uses Sourcegraph code search, the code graph, and LLMs to provide context-aware answers about your codebase. Cody can explain code, refactor code, and write code, all within the context of your existing codebase.

[Learn more about about Cody](../cody/overview/index.md).

## Notebooks

_Note: GA in version 3.39 or later_

Increase dev collaboration and self-service through live documentation covering best practice, shared code and processes.

Read the [Notebooks documentation](../notebooks/index.md) to learn more, and check out these [publicly available notebooks](https://sourcegraph.com/notebooks?_ga=2.9922293.1906238367.1667257632-528424761.1615652680&_gl=1*1xalma3*_ga*NTI4NDI0NzYxLjE2MTU2NTI2ODA.*_ga_E82CCDYYS1*MTY2NzI1NzYzMi41NS4xLjE2NjcyNjA3NjIuMC4wLjA.).

## Batch Changes

_Note: Enterprise feature_

Automate changes to your codebase. Reduce effort, reduce errors and enable developers to focus on high value work.

Read the [batch changes documentation](../batch_changes/index.md) to learn more, including useful how-to guides. Want a video tutorial? Check out this [batch change tutorial](https://www.youtube.com/watch?v=eOmiyXIWTCw)

## Code Insights

_Note: Enterprise feature_

Sourcegraph lets you understand and analyze code trends by visualizing how the codebase is changing over time. Measure and act on engineering goals such as migration and component deprecation.

Read the [code insights documentation](../code_insights/index.md) to learn more, including useful how-to guides.

## Integrations

Sourcegraph allows you to get code navigation and code search on code files and code review diffs in your code host and review tool or from your IDE.

### IDE integration

Sourcegraph’s editor integrations allow you search and navigate across all of your repositories without ever leaving your IDE or checking them out locally. Learn more about how to set them up [here](../integration/editor.md).

### Browser extension

Our browser extension add code navigation within your code hosts (GitHub, GitLab, Bitbucket, and Phabricator) directly via Chrome, Safari, and Firefox browsers. Learn more about how to set up the browser extension [here](../integration/browser_extension/index.md).
