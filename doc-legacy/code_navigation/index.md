<style>

.markdown-body h2 {
  margin-top: 2em;
}

.markdown-body ul {
  padding-left: 1em;
}

.markdown-body ul li {
  margin: 0.5em 0;
}

.markdown-body .lead-screenshot {
    float: right;
    display: block;
    margin: 1em auto;
    max-width: 500px;
    margin-left: 0.5em;
    border: 1px solid lightgrey;
    border-radius: 10px;
}

</style>

# Code navigation

<p class="subtitle">Quickly navigate your code with high precision.</p>

<div>

<p class="lead">
Code navigation helps you quickly explore code, dependencies, and symbols within the Sourcegraph file view. Code navigation consists of a number of features that make it easier to move through your codebase without getting lost:
</p>

- Jump to definition
- Find references
- Find implementations
- Browse symbols defined in current document or folder
- Navigate dependencies
- See docstrings in hover tooltips

</div>

<div style="display: block; float: clear;"> </div>

<div class="cta-group">
<a class="btn btn-primary" href="https://sourcegraph.com/github.com/dgrijalva/jwt-go/-/blob/token.go?L37:6#tab=references">★ Try it on public code!</a>
<a class="btn" href="#code-navigation-for-your-code">Enable it for your code</a>
<a class="btn" href="references/precise_examples">Examples</a>
<a class="btn" href="references/indexers">🗂 Indexers</a>
</div>

Code navigation has two different implementations which complement one another:

- [Search-based code navigation](explanations/search_based_code_navigation.md) works out of the box with all of the most popular programming languages, powered by Sourcegraph's code search. Our default search-based code navigation uses syntactic-level heuristics (no language-level semantic information) for fast, performant searches across large code bases.
- [Precise code navigation](explanations/precise_code_navigation.md) uses compile-time information to provide users with an extremely precise and accurate cross-repository[^1] experience. This means you'll get an accurate view of all symbols and where they are used across your code base.

Sourcegraph automatically uses precise code navigation whenever available, and search-based code navigation is used as a fallback when precise navigation is not available.

Precise code navigation requires language-specific indexes to be generated and uploaded to your Sourcegraph instance. We currently have precise code navigation support for the languages below. See the [indexers page](references/indexers.md) for a detailed breakdown of each indexer's status.
| Language | Indexer | Status |
|-|-|-|  
| Go | [lsif-go](https://sourcegraph.com/github.com/sourcegraph/lsif-go) | 🟢 Generally available |
| TypeScript, JavaScript | [scip-typescript](https://sourcegraph.com/github.com/sourcegraph/scip-typescript) | 🟢 Generally available |
| C, C++ | [scip-clang](https://sourcegraph.com/github.com/sourcegraph/scip-clang) | 🟡 Partially available |
| Java, Kotlin, Scala | [scip-java](https://sourcegraph.com/github.com/sourcegraph/scip-java) | 🟢 Generally available |
| Rust | [rust-analyzer](https://sourcegraph.com/github.com/rust-lang/rust-analyzer) | 🟢 Generally available |
| Python | [scip-python](https://sourcegraph.com/github.com/sourcegraph/scip-python) | 🟢 Generally available |
| Ruby | [scip-ruby](https://sourcegraph.com/github.com/sourcegraph/scip-ruby) | 🟢 Generally available |
| C#, Visual Basic | [scip-dotnet](https://github.com/sourcegraph/scip-dotnet) | 🟡 Partially available |

The easiest way to configure precise code navigation is with [auto-indexing](explanations/auto_indexing.md). This feature uses [Sourcegraph executors](../admin/executors/index.md) to automatically create indexes for the code, keeping precise code navigation available and up-to-date.

## Setting up code navigation for your codebase

Sourcegraph provides search-based code navigation out-of-the-box. There are several options for setting up precise code navigation:

1. **Manual indexing**. Index a repository and upload it to your Sourcegraph instance:

    - [Index a Go repository](how-to/index_a_go_repository.md#manual-indexing)
    - [Index a TypeScript or JavaScript repository](how-to/index_a_typescript_and_javascript_repository.md#manual-indexing)
    - [Index a Java, Scala, or Kotlin repository](https://sourcegraph.github.io/scip-java/docs/getting-started.html)
    - [Index a Python repository](https://sourcegraph.com/github.com/sourcegraph/scip-python)
    - [Index a Ruby repository](https://sourcegraph.com/github.com/sourcegraph/scip-ruby)

2. [**Automate indexing via CI**](how-to/adding_lsif_to_workflows.md): Add indexing and uploading to your CI setup.
3. [**Auto-indexing**](how-to/enable_auto_indexing.md): Sourcegraph will automatically index your repositories and enable precise code navigation for them.
4. Set up **auto-dependency indexing** to navigate and search through the dependencies your code uses:
    - **Go**: Enable [auto-indexing](explanations/auto_indexing.md) and Sourcegraph will start indexing your dependencies.
    - **JavaScript, TypeScript**: Enable [auto-indexing](explanations/auto_indexing.md) and set up an [npm dependencies code host](../../integration/npm.md).
    - **Java, Scala, Kotlin**: Enable [auto-indexing](explanations/auto_indexing.md) and set up a [JVM dependencies code host](../../integration/jvm.md).

## [Explanations](explanations/index.md)

- [Introduction to code navigation](explanations/introduction_to_code_navigation.md)
- [Precise code navigation](explanations/precise_code_navigation.md)
  - [Code graph data uploads](explanations/uploads.md)
- [Search-based code navigation](explanations/search_based_code_navigation.md)
- [Code navigation features](explanations/features.md)
- <span class="badge badge-beta">Beta</span> [Rockskip: Faster search-based code navigation](explanations/rockskip.md)
- [Writing an indexer](explanations/writing_an_indexer.md)
- [Auto-indexing](explanations/auto_indexing.md)
- [Auto-indexing inference](explanations/auto_indexing_inference.md)


## [How-to guides](how-to/index.md)

- General
  - [Configure data retention policies](how-to/configure_data_retention.md)

- Language-specific guides
  - [Index a Go repository](how-to/index_a_go_repository.md)
  - [Index a TypeScript or JavaScript repository](how-to/index_a_typescript_and_javascript_repository.md)
  - [Index a Java, Scala, or Kotlin repository](https://sourcegraph.github.io/scip-java/docs/getting-started.html)
- Automate uploading code graph data
  - [Add code graph data to many repositories](how-to/adding_lsif_to_many_repos.md)
  - [Adding code graph data to CI workflows](how-to/adding_lsif_to_workflows.md)
  - [Enable auto-indexing](how-to/enable_auto_indexing.md)
  - [Configure auto-indexing](how-to/configure_auto_indexing.md)
- Best practices
  - [Guide to defining policies](how-to/policies_resource_usage_best_practices.md) as it relates to resource usage
## [Reference](references/index.md)

- [Requirements](references/requirements.md)
- [Troubleshooting](references/troubleshooting.md)
- [FAQ](references/faq.md)
- [Sourcegraph-recommended indexers](references/indexers.md)
- [Environment variables](references/envvars.md)
- [Auto-indexing configuration](references/auto_indexing_configuration.md)


[^1]: Supported for any language with a SCIP indexer.
