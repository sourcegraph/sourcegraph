# Code search

<p class="subtitle">Search code across all your repositories and code hosts</p>

[A recently published research paper from Google](https://static.googleusercontent.com/media/research.google.com/en//pubs/archive/43835.pdf) and a [Google developer survey](https://docs.google.com/document/d/1LQxLk4E3lrb3fIsVKlANu_pUjnILteoWMMNiJQmqNVU/edit#heading=h.xxziwxixfqq3) showed that 98% of developers consider their Sourcegraph-like internal code search tool to be critical, and developers use it on average for 5.3 sessions each day, primarily to (in order of frequency):

- find example code
- explore/read code
- debug issues
- determine the impact of changes

Sourcegraph code search helps developers perform these tasks more quickly and effectively by providing fast, advanced code search across multiple repositories.

<div class="cta-group">
<a class="btn btn-primary" href="reference/queries">★ Search query language</a>
<a class="btn" href="explanations/features">Search features</a>
</div>

## Recommended

<div>
  <a class="cloud-cta" href="https://sourcegraph.com/get-started?t=enterprise" target="_blank" rel="noopener noreferrer">
    <div class="cloud-cta-copy">
      <h2>Get Sourcegraph on your code.</h2>
      <h3>A single-tenant instance managed by Sourcegraph.</h3>
      <p>Sign up for a 30 day trial for your team.</p>
    </div>
    <div class="cloud-cta-btn-container">
      <div class="visual-btn">Get free trial now</div>
    </div>
  </a>
</div>

## Getting started

<div class="getting-started">
  <a href="tutorials/examples" class="btn" alt="See search examples">
   <span>New to search?</span>
   <br>
   See search examples for inspiration.
  </a>

  <a href="https://www.youtube.com/watch?v=GQj5jXdON3A" class="btn" alt="Watch the intro to code search video">
   <span>Intro to code search video</span>
   <br>
   Watch the intro to code search video to see what you can do with Sourcegraph search.
  </a>

  <a href="reference/queries" class="btn" alt="Learn the search syntax">
   <span>Learn the search syntax</span>
   <br>
   Learn the search syntax for writing powerful search queries.
  </a>
</div>

## [Explanations](explanations/index.md)

- [Search features](explanations/features.md)
  - Use regular expressions and exact queries to perform full-text searches.
  - Perform [language-aware structural search](explanations/features.md#language-aware-structural-code-search) on code structure.
  - Search any branch and commit, with no indexing required.
  - Search [commit diffs](explanations/features.md#commit-diff-search) and [commit messages](explanations/features.md#commit-message-search) to see how code has changed.
  - Narrow your search by repository and file pattern.
  - How our [Smart Search](explanations/features.md#smart-search) query assistant works.
  - Use [search contexts](explanations/features.md#search-contexts) to search across a set of repositories at specific revisions.
  - Curate [saved searches](explanations/features.md#saved-searches) for yourself or your org.
  - Use [code monitoring](../code_monitoring/index.md) to set up notifications for code changes that match a query.
  - View [language statistics](explanations/features.md#statistics) for search results.
- [Search details](explanations/search_details.md)
- [Search tips](explanations/tips.md)

## [How-tos](how-to/index.md)

- [Switch from Oracle OpenGrok to Sourcegraph](how-to/opengrok.md)
- [Create a saved search](how-to/saved_searches.md)
- [Create a custom search snippet](how-to/snippets.md)
- [Using and creating search contexts](how-to/search_contexts.md)
- [Exhaustive search](how-to/exhaustive.md)
- [How to create a search context with the GraphQL API](how-to/create_search_context_graphql.md)


## [Tutorials](tutorials/index.md)

- [Useful search examples](tutorials/examples.md)

## [References](reference/index.md)

- [Search query syntax](reference/queries.md)
- [Sourcegraph search query language](reference/language.md)
- [Structural search reference](reference/structural.md)
