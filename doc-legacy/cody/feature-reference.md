<style>

.th:first-child,
td:first-child {
   min-width: 200px;
}

.markdown-body table thead tr{
  border-top:0;
}

.markdown-body table th, .markdown-body table td {
    text-align: left;
    vertical-align: baseline;
    padding: 0.5714286em;
}

.markdown-body table tr:nth-child(2n) {
  background: unset;
}

.markdown-body table th, .markdown-body table td {
    border: none;
}
</style>

# Feature Parity Reference for Cody Clients

<p class="subtitle">This document compares Cody's features and capabilities across different clients.
</p>

Here's a feature parity matrix that compares the capabilities of Cody Clients across different platforms like VS Code, JetBrains, Neovim, and Sourcegraph.com (Web UI).

## Chat

| **Feature**               | **VS Code** | **JetBrains** | **Neovim** |**Web** |
|-------------------------|---------|-----------|--------|--------|
| Talk to Cody     |    ✓    |     ✓     |   ✓    |  ✓      |
| Chat history     |    ✓    |     x     |   x    |     ✓   |
| Stop chat generating     |    ✓    |     ✓     |   x    |   ✓     |
| Edit sent messages     |    ✓    |     x     |   x    |     ✓   |
| Slash (`/`) commands     |    ✓    |     x     |   x   |    x    |
| Show context files     |    ✓    |     ✓     |   ✓    |    ✓    |
| Custom commands     |    ✓    |     x     |   x   |    x    |
| Clear chat history     |    ✓    |     ✓     |   x    |    ✓    |
| LLM Selection   | ✓           | x             | x          |     x   |
| Enhanced  Context Selection   | ✓           | x             | x          |   x     |

## Code Autocomplete

| **Feature**               | **VS Code** | **JetBrains** | **Neovim** |**Web** |
|-------------------------|---------|-----------|--------|--------|
| Single-line autocompletion     |    ✓    |     ✓     |   ✓    |    x    |
| Single-line, multi-part autocompletion     |    ✓    |     ✓     |   ✓    |    x    |
| Multi-line, inline autocompletion     |    ✓    |     ✓     |   ✓    |    x    |
| Enable/Disable by language     |    ✓    |     ✓     |   ✓    |    x    |
| Customize autocomplete colors     |    x    |     ✓     |   ✓    |    x    |
| Cycle through multiple completion suggestions     |    ✓    |     ✓     |   ✓    |    x    |

## Code Context

| **Feature**               | **VS Code** | **JetBrains** | **Neovim** |**Web** |
|-------------------------|---------|-----------|--------|--------|
| Multi-repo context     |    x    |     x     |   x    |  ✓      |
| Repo selection for context     |    ✓    |     ✓     |   x    |  ✓      |
| Local repo context     |    ✓    |     x     |   x    |  x      |
| Embeddings     |    ✓    |     ✓     |   ✓    |  ✓      |

## Commands

| **Feature**               | **VS Code** | **JetBrains** | **Neovim** |**Web** |
|-------------------------|---------|-----------|--------|--------|
| Generate `docstring`     |    ✓    |     ✓     |   ✓    |   ✓    |
| Generate unit test     |    ✓    |     ✓     |   ✓    |   ✓    |
| Explain code     |    ✓    |     ✓     |   ✓    |   ✓    |
| Smell code     |    ✓    |     ✓     |   ✓    |   ✓    |
| Ask a question     |    ✓    |     x     |   ✓    |   ✓    |
| Reset chat     |    ✓    |     x     |   x    |   x    |
| Task instruction     |    x    |     x     |   ✓    |   x    |
| Restart Cody/Sourcegraph     |    x    |     x     |   ✓    |   x    |
| Toggle chat window     |    x    |     x     |   ✓    |   x    |
| Improve variable names     |    x    |     ✓     |   x    |   ✓    |
