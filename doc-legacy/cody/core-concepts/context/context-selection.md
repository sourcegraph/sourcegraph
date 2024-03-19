<style>
  .demo{
    display: table;
    width: 25%;
    margin: 0.1rem;
    padding: 0.5rem 0.5rem;
    color: var(--text-color);
    border-radius: 4px;
    border: 1px solid var(--sidebar-nav-active-bg);
    padding: 0.5rem;
    padding-top: 0.5rem;
    background-color: var(--sidebar-nav-active-bg);
  }
</style>

# Cody Context Selection

<p class="subtitle">Learn about where does context come from and how Cody ensures it's good.</p>

The quality of the context is critical, even with increasing context window sizes. Cody obtains context that is relevant to user input through the following methods:

## Local Context

Cody initially looks at your local context by examining the currently open or recently accessed files within your code editor. By doing this, Cody aims to provide relevant information based on your recent work.

## Keyword Search

Keyword Search is a traditional text search approach. It finds keywords matching your input and searches for those in the local code. It involves splitting content into terms and mapping terms to documents. At query time, terms from the query are matched to your documents. While powerful, this method can be used as a fallback solution when a codebase lacks embeddings.

<div class="getting-started">
  <a class="demo text-center" target="_blank" href="https://docs.sourcegraph.com/cody/core-concepts/keyword-search">Learn more →</a>
</div>

## Code Search

For more extensive searches, Cody utilizes Sourcegraph's Code Search. This powerful tool allows Cody to search beyond the local files and access non-local code repositories. Using an indexed trigram search, Cody can locate code snippets or relevant information from a broader range of sources.

<div class="getting-started">
  <a class="demo text-center" target="_blank" href="https://docs.sourcegraph.com/code_search">Learn more →</a>
</div>

## Code Graph

Code Graph involves analyzing the structure of the code rather than treating it as plain text. Cody examines how different components of the codebase are interconnected and how they are used. This method is dependent on the code's structure and inheritance relationships. It can help Cody find context related to your input based on how code elements are linked and utilized.

<div class="getting-started">
  <a class="demo text-center" target="_blank" href="https://docs.sourcegraph.com/cody/core-concepts/code-graph">Learn more →</a>
</div>

## Embeddings

Cody employs embeddings to find pieces of your codebase that are semantically related to your query. An LLM is used to pre-calculate the embeddings for your codebase so Cody can quickly find relevant chunks of code.

<div class="getting-started">
  <a class="demo text-center" target="_blank" href="https://docs.sourcegraph.com/cody/core-concepts/embeddings">Learn more →</a>
</div>
