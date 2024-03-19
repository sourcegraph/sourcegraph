<style>

.markdown-body h2 {
  margin-top: 2em;
}

.markdown-body ul {
  list-style:none;
  padding-left: 1em;
}

.markdown-body ul li {
  margin: 0.5em 0;
}

.markdown-body ul li:before {
  content: '';
  display: inline-block;
  height: 1.2em;
  width: 1em;
  background-size: contain;
  background-repeat: no-repeat;
  background-image: url(code_monitoring/file-icon.svg);
  margin-right: 0.5em;
  margin-bottom: -0.29em;
}

body.theme-dark .markdown-body ul li:before {
  filter: invert(50%);
}

</style>

# Notebooks
Notebooks enable powerful live–and persistent–documentation, shareable with your organization or the world. As of release 3.39, Notebooks are Generally Available (GA). If you're a Sourcegraph enterprise user and are on version 3.39 or later, you'll find Notebooks in the header navigation, or by going to `https://<your-internal-sourcegraph-url>/notebooks`. You can also explore [public notebooks on Sourcegraph.com](https://sourcegraph.com/notebooks).

Inspired by Jupyter Notebooks and powered by Markdown and Sourcegraph's code search, Notebooks let you and your team create living documentation that interacts directly with your code. You can leverage Notebooks to onboard a new teammate, document [known vulnerabilities](https://sourcegraph.com/notebooks/Tm90ZWJvb2s6MQ==), a [common pattern](https://sourcegraph.com/notebooks/Tm90ZWJvb2s6OTI=) in your codebase, or [useful Sourcegraph queries](https://sourcegraph.com/notebooks/Tm90ZWJvb2s6MTU=).

![](https://storage.googleapis.com/sourcegraph-assets/docs/images/notebooks/notebooks_home.gif)

Notebooks have powerful content creation features, like multiple block types, each with their own unique capabilities.
If you're familiar with Jupyter Notebooks, then you already understand the blocks concept. You can add as many of each
block as you want to a Sourcegraph notebook.

## Notebook types
Notebooks can be created in two ways. Through the web interface or via special Markdown files with the special `.snb.md` extension. To view file-based notebooks you must view the files in the file view on sourcegraph.com or on your Sourcegraph instance.

### Web-based notebooks

The simplest way to get started with Notebooks is to create one using the web interface. Notebooks created this way have
the advantage of being interactive, letting you see the content of your blocks in realtime as you create your notebook.

You can also create web-based notebooks by importing plain Markdown files and then augmenting them with Sourcegraph notebook block types in the web interface. A new notebook will automatically be created when you import a standard markdown file. From there, you can modify it however you like in the web interface.

Web-based notebooks are automatically saved as they're edited. There is currently no version control, version history, or versioning system.

### File-based notebooks
Alternatively, you can create notebooks using text files with the `.snb.md` file extension. These files are rendered specially by Sourcegraph (either on sourcegraph.com or within your Sourcegraph instance) to display notebook blocks alongside standard Markdown blocks.

Whenever you view a `.snb.md` file on Sourcegraph, you'll see a "Run all blocks" button near the top of the notebook, which will execute all the notebook blocks at once. Markdown and file blocks are rendered by default.

File-based notebooks have the advantage of living anywhere you store text files. The disadvantage comes during composition, as you won't be able to see the contents of your blocks while you create your notebook.

### Combined approaches
#### Compose online and export to disk
If you prefer to keep your notebooks in your repos but want to compose them on the web, you can get the best of both worlds by composing your notebooks on your sourcegraph instance and then exporting them to your repositories on disk.

#### Embed notebooks anywhere
Sourcegraph notebooks can be [embedded](../notebooks/notebook-embedding.md) anywhere that allows iframes. Notebooks hosted on sourcegraph.com can be embedded anywhere. Notebooks hosted on your private instance are subject to your organization's security policies, but can generally be viewed by any user with access to your instance as long as they're logged in.

## Notebook blocks

The currently supported block types are:

- Query
- File
- Symbol
- Markdown

[Read more about block types](../notebooks/blocks.md).

## Searching notebooks
Notebooks created through the web interface are full text searchable from the `/notebooks` page. Each tab has its own search box and each search box is scoped to that tab. The exception is the Explore tab, which searches all notebooks you have access to.

Searches will match on notebook titles and any text in blocks. For example any text in Markdown blocks and any of the query text in file, symbol, and search query blocks. Searching through results in symbol, file, and query block types is not supported because they are dynamic in nature.


## Enabling Notebooks in older versions of Sourcegraph
In versions older than 3.39 (beginning in 3.36) Notebooks are behind an experimental feature flag. If you're running versions 3.36-3.38 and want to try out Notebooks, enable them in global settings:

```
"experimentalFeatures": {
    "showSearchNotebook": true
}
```

<div class="cta-group">
  <a class="btn btn-primary" href="quickstart">★ Quickstart</a>
</div>

## Explanations
- [Sharing notebooks](../notebooks/notebook-sharing.md)
- [Embedding notebooks](../notebooks/notebook-embedding.md)
- [Block types](../notebooks/blocks.md)
