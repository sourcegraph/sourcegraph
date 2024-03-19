# Sourcegraph New User Tutorials

## Find Your Way Around Sourcegraph
| Topic | Content Type | Description |
| ----------- | ----------- | ----------- |
| [Sourcegraph 101](../getting-started/index.md) | Explanation | What is Sourcegraph? Who should use it? Why do I need it? What does it do? |
| [High Level Product Demo](https://www.youtube.com/watch?v=Kk1ea2-l8Hk) | Explanation (video) | A short 3-minute video describing what Sourcegraph is and how it can be useful. |
| [Navigating the Sourcegraph UI](https://www.youtube.com/watch?v=6K7e74a7aC4) | Tutorial (video) | Take a look at how you can read code, find references, troubleshoot errors, gain insight, and make changes on a massive scale in Sourcegraph. |

## Get Started Searching
| Topic | Content Type | Description |
| -------- | --------| -------- |
| [Three Types of Code Search](https://www.youtube.com/watch?v=-EGn_-2d9CQ) | Tutorial (video) | Code search is a vital tool for developers. It's great for digging up answers to questions from your own codebase, but it's even better for exploring and understanding code. This video will show you the types of code search we have on Sourcegraph. |
| [Understanding Code Search Results](https://www.youtube.com/watch?v=oMWdYfG6-DQ) | Tutorial (video) | In this video, you'll understand the search results page and how to scope, export, save, and link to search results. |
| [Basic Code Search Filters](https://www.youtube.com/watch?v=h1Kw0Wd9qZ4) | Tutorial (video) | In this video, you'll learn how to use Sourcegraph's code search filters and how they work. Filters are a great way to narrow down or search for specific code. This video covers language, repo, branch, file, and negative filters. |
| [Search Query Syntax](../code_search/reference/queries.md) | Reference | This page describes search pattern syntax and keywords available for code search |


## More Advanced Searching
| Topic | Content Type | Description |
| -------- | -------- | -------- |
| [Search Subexpressions](../code_search/tutorials/search_subexpressions.md) | Tutorial | Search subexpressions combine groups of filters like `repo:` and operators like `AND` & `OR`. Compared to basic examples, search subexpressions allow more sophisticated queries. |
| [Regular Expression Search Deep Dive](https://sourcegraph.com/blog/how-to-search-with-sourcegraph-using-regular-expression-patterns) | Tutorial | Regular expressions, often shortened as regex, help you find code that matches a pattern (including classes of characters like letters, numbers and whitespace), and can restrict the results to anchors like the start of a line, the end of a line, or word boundary. |
| [Structural Search Tutorial ](https://sourcegraph.com/blog/how-to-search-with-sourcegraph-using-structural-patterns) | Tutorial | Structural search helps you search code for syntactical code patterns like function calls, arguments, `if...else` statements, and `try...catch` statements. It's useful for finding nested and recursive patterns as well as multi-line blocks of code. |
| [Structural Search Tutorial ](https://youtu.be/GnubTdnilbc) | Tutorial (video) | Structural search helps you search code for syntactical code patterns like function calls, arguments, `if...else` statements, and `try...catch` statements. It's useful for finding nested and recursive patterns as well as multi-line blocks of code. |


## Code Navigation
| Topic | Content Type | Description |
| -------- | --------| -------- |
| [Introduction to Code Navigation](../code_navigation/explanations/introduction_to_code_navigation.md) | Explanation | There are 2 types of code navigation that Sourcegraph supports: search-based and precise. |
| [Code Navigation Features](../code_navigation/explanations/features.md) | Explanation | An overview of Code Navigation features, such as "find references", "go to definition", and "find implementations".|


## Search Notebooks
| Topic | Content Type | Description |
| -------- | --------| -------- |
| [Search Notebooks Quickstart Guide](../notebooks/quickstart.md) | Tutorial | Notebooks enable powerful live–and persistent–documentation, shareable with your organization or the world. |


## Code Insights
| Topic | Content Type | Description |
| -------- | --------| -------- |
| [Code Insights Overview](https://www.youtube.com/watch?v=fMCUJQHfbUA) | Explanation (video) | Learn about common Code Insights use cases and see how to create an insight.
| [Quickstart Guide](../code_insights/quickstart.md) | Tutorial | Get started and create your first code insight in 5 minutes or less. |
| [Common Use Cases](../code_insights/references/common_use_cases.md) | Reference | A list of common use cases for Code Insights and example data series queries you could use. |


## Batch Changes
| Topic | Content Type | Description |
| -------- | --------| -------- |
| [Introduction to Batch Changes](../batch_changes/explanations/introduction_to_batch_changes.md) | Explanation | A basic introduction to the concepts, processes, and supported environments behind Batch Changes |
| [Get Started With Batch Changes](https://www.youtube.com/watch?v=GKyHYqH6ggY) | Tutorial (video) | Learn how you can quickly use Sourcegraph Batch Changes to automate small and large-scale code changes server-side. |
| [Batch Changes Quickstart Guide](../batch_changes/quickstart.md) | Tutorial | Get started and create your first batch change in 10 minutes or less. This guide follows the local (CLI) method of running batch changes. |
| [Getting Started Running Batch Changes Server-Side](../batch_changes/explanations/server_side_getting_started.md) | How-To-Guide | Follow this guide to learn how to run batch changes server-side. |

## The Sourcegraph API
| Topic | Content Type | Description |
| -------- | --------| -------- |
| [GraphQL API](../api/graphql/index.md) | Reference | The Sourcegraph GraphQL API is a rich API that exposes data related to the code available on a Sourcegraph instance. |
| [GraphQL Examples](../api/graphql/examples.md) | Reference | This page demonstrates a few example GraphQL queries for the Sourcegraph GraphQL API.
| [Streaming API](../api/stream_api/index.md) | Reference | With the Stream API you can consume search results and related metadata as a stream of events. The Sourcegraph UI calls the Stream API for all interactive searches. Compared to our GraphQL API, it offers shorter times to first results and supports running exhaustive searches returning a large volume of results without putting pressure on the backend. |

## Customizing Your Sourcegraph User Environment
| Topic | Content Type | Description |
| -------- | -------- | -------- |
| [Using Sourcegraph with your IDE](../integration/editor.md) | How-To-Guide | Sourcegraph’s editor integrations allow you search and navigate across all of your repositories without ever leaving your IDE or checking them out locally. We have built-in integrations with VS Code and JetBrains. |
| [Using the Sourcegraph Browser Extension](../integration/browser_extension/index.md) | How-To-Guide | The open-source Sourcegraph browser extension adds code navigation to files and diffs on GitHub, GitHub Enterprise, GitLab, Phabricator, Bitbucket Server and Bitbucket Data Center. |
| [Using the Sourcegraph CLI](../cli/quickstart.md) | How-To-Guide | `src` is a command line interface to Sourcegraph that allows you to search code from your terminal, create and apply batch changes, and manage and administrate repositories, users, and more. |
| [Saving Searches](../code_search/how-to/saved_searches.md) | How-To-Guide | Saved searches let you save and describe search queries so you can easily find and use them again later. You can create a saved search for anything, including diffs and commits across all branches of your repositories. |
| [Search Contexts](../code_search/how-to/search_contexts.md) | How-To-Guide | Search contexts help you search the code you care about on Sourcegraph. A search context represents a set of repositories at specific revisions on a Sourcegraph instance that will be targeted by search queries by default. |

## Sourcegraph Use Cases
| Topic | Content Type | Description |
| -------- | -------- | -------- |
| [Sourcegraph Tour](../getting-started/tour.md) | How-to-Guide | A tour of a sample Sourcegraph use cases, including using Sourcegraph for code reviews, debugging, and understanding how a function works. |
| [Cody Use Cases](./../cody/use-cases.md) | How-to-Guide | A tour of Cody and where the AI code assistant fits into everyday workflows. |
