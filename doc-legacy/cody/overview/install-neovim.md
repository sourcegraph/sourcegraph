<style>

  .markdown-body .cards {
  display: flex;
  align-items: stretch;
}

.markdown-body .cards .card {
  flex: 1;
  margin: 0.5em;
  color: var(--text-color);
  border-radius: 4px;
  border: 1px solid var(--sidebar-nav-active-bg);
  padding: 1.5rem;
  padding-top: 1.25rem;
}

.markdown-body .cards .card:hover {
  color: var(--link-color);
}

.markdown-body .cards .card span {
  color: var(--link-color);
  font-weight: bold;
}

.markdown-body .cards {
  display: flex;
  align-items: stretch;
}

.markdown-body .cards .card {
  flex: 1;
  margin: 0.5em;
  color: var(--text-color);
  border-radius: 4px;
  border: 1px solid var(--sidebar-nav-active-bg);
  padding: 1.5rem;
  padding-top: 1.25rem;
}

.markdown-body .cards .card:hover {
  color: var(--link-color);
}

.markdown-body .cards .card span {
  color: var(--link-color);
  font-weight: bold;
}

.limg {
  list-style: none;
  margin: 3rem 0 !important;
  padding: 0 !important;
}
.limg li {
  margin-bottom: 1rem;
  padding: 0 !important;
}

.limg li:last {
  margin-bottom: 0;
}

.limg a {
    display: flex;
    flex-direction: column;
    transition-property: all;
   transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
     transition-duration: 350ms;
     border-radius: 0.75rem;
  padding-top: 1rem;
  padding-bottom: 1rem;

}

.limg a {
  padding-left: 1rem;
  padding-right: 1rem;
  background: rgb(113 220 232 / 19%);
}

.limg p {
  margin: 0rem;
}
.limg a img {
  width: 1rem;
}

.limg h3 {
  display:flex;
  gap: 0.6rem;
  margin-top: 0;
  margin-bottom: .25rem

}

th:first-child,
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

# Install Cody for Neovim

<p class="subtitle">Learn how to use Cody and its features with the Neovim editor.</p>

<aside class="experimental">
<p>
<span style="margin-right:0.25rem;" class="badge badge-experimental">Experimental</span> Cody support for Neovim is in the experimental stage.
<br />
For any feedback, you can <a href="https://sourcegraph.com/contact">contact us</a> directly, file an <a href="https://github.com/sourcegraph/cody/issues">issue</a>, join our <a href="https://discord.com/servers/sourcegraph-969688426372825169">Discord</a>, or <a href="https://twitter.com/sourcegraphcody">tweet</a>.
</p>
</aside>

The Cody extension for Neovim by Sourcegraph enhances your coding experience in your IDE by providing intelligent code suggestions, context-aware completions, and advanced code analysis.

This guide will walk you through installing and setting up the Cody within your Neovim environment.

<ul class="limg">
  <li>
    <a class="card text-left" target="_blank" href="https://github.com/sourcegraph/sg.nvim">
      <h3><img alt="Neovim" src="https://storage.googleapis.com/sourcegraph-assets/Docs/neovim-logo.png" />Neovim Extension (Experimental)</h3>
      <p>Install Cody's free extension for Neovim.</p>
    </a>
  </li>
  </ul>

## Prerequisites

- `nvim 0.9 or nvim nightly` version of <a href="https://github.com/neovim/neovim/wiki/Installing-Neovim" target="_blank">Neovim</a> installed
- `Node.js >= 18.17.0 (LTS)` at runtime for [cody-agent.js](https://github.com/sourcegraph/cody)
- You have enabled an instance for [Cody from your Sourcegraph.com](cody-with-sourcegraph.md) account

## Installation

`sg.nvim` is a plugin that uses Sourcegraph's code intelligence features directly within the Neovim text editor. You can install the plugin using different Neovim plugin managers like:

### `lazy.nvim`

```lua
return {
  {
    "sourcegraph/sg.nvim",
    dependencies = { "nvim-lua/plenary.nvim" },

    -- If you have a recent version of lazy.nvim, you don't need to add this!
    build = "nvim -l build/init.lua",
  },
}
```

### `packer.nvim`

```lua
-- Packer.nvim, also make sure to install nvim-lua/plenary.nvim
use { 'sourcegraph/sg.nvim', run = 'nvim -l build/init.lua' }
```

### `vim-plug`

```lua
-- Using vim-plug
Plug 'sourcegraph/sg.nvim', { 'do': 'nvim -l build/init.lua' }
```

Once you have installed the plugin, run `:checkhealth sg` to verify a successful installation. Next, you are prompted to log in as a free user by connecting to your Sourcegraph.com account or using the enterprise instance.

## Setting up with Sourcegraph instance

To connect `sg.nvim` with Sourcegraph, you need to follow these steps:

- Log in on your Sourcegraph instance
- Go to **Settings > Access tokens** from the top right corner
- Create your access token, and then run `:SourcegraphLogin` in your neovim editor after installation
- Type in the link to your Sourcegraph instance (for example, https://sourcegraph.com)
- Next, paste your generated access token

An alternative way to this is to use the environment variables specified for [`src-cli`](https://github.com/sourcegraph/src-cli#log-into-your-sourcegraph-instance).

At any point, you can run `:checkhealth sg` to ensure you're logged in and connected to your Sourcegraph instance.

## Features

The `sg.nvim` plugin supports a wide range of features that helps you integrate and use Cody and Sourcegraph Search directly within your Neovim environment.

### Cody

The `sg.nvim` extension supports the following features for Cody:

|     **Feature**     |                                                                                         **Description**                                                                                         |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Chat**    | Chat interface and associated commands|
| **Autocompletions**    | Support both prompted and suggested autocompletions|

### Search

The `sg.nvim` extension supports the following features for Sourcegraph Search:

|     **Feature**     |                                                                                         **Description**                                                                                         |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Read files**    | Directly from sourcegraph links: `:edit <sourcegraph url>`|
|     | Automatically adds protocols for `https://sourcegraph.com/*` links|
|     | Directly from buffer names `:edit sg://github.com/tjdevries/sam.py/-/src/sam.py`|
|   **Read non-files**  | Repository roots, folders (both expanded and non-expanded), open file from folder|
|   **Built-in LSP client**  | Connects to Sourcegraph via `Goto Definition` and `Goto References` (<20 references)|
|   **Basic search**  | Literal, regexp, and structural search support, `type:symbol` support, and repo support|
|   **Advanced search features**  | Autocompletions and memory of last searches |

## Commands

The `sg.nvim` extension also supports pre-built reusable prompts for Cody called "Commands" that help you quickly get started with common programming tasks like:

- `:CodyAsk`: Ask a question about the current selection
- `:CodyChat {title}`: Starts a new Cody chat, with an optional `{title}`
- `:CodyRestart`: Restarts Cody and Sourcegraph
- `:CodyTask {task_description}`: Instructs Cody to perform a task on a selected text
- `:CodyTaskAccept`: Accepts the current `CodyTask`
- `:CodyTaskNext`: Cycles to the next `CodyTask`
- `:CodyTaskPrev`: Cycles to the previous `CodyTask`
- `:CodyTaskView`: Opens the last active `CodyTask`
- `:CodyToggle`: Toggle to the current Cody Chat window

## More benefits

Read more about [Cody capabilities](./../capabilities.md) to learn about all the features it provides to boost your development productivity.

## More resources

For more information on what to do next, we recommend the following resources:

<div class="cards">
  <a class="card text-left" href="./../quickstart"><b>Cody Quickstart</b><p>This guide recommends how to use Cody once you have installed the extension in your VS Code editor.</p></a>
  <a class="card text-left" href="./../use-cases/generate-unit-tests"><b>Cody Use Cases</b><p>Explore some of the most common use cases of Cody that helps you with your development workflow.</p></a>
</div>
