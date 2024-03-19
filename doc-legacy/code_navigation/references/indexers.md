<style>
.indexer-status:hover {
  text-decoration: none;
}
</style>

# Indexers

Language support is an ever-evolving feature of Sourcegraph. Some languages may be better supported than others due to demand or developer bandwidth/expertise. This page clarifies the status of the indexers which the Sourcegraph team can both recommend to customers and provide support for.

## Quick reference

This table is maintained as an authoritative resource for users, Sales, and Customer Engineers. Any major changes to the development of our indexers will be reflected here.



<table>
   <thead>
      <tr>
        <th>Language</th>
        <th>Indexer</th>
        <th>Status</th>
        <th>Hover docs</th>
        <th>Go to definition</th>
        <th>Find references</th>
        <th>Cross-file</th>
        <th>Cross-repository</th>
        <th>Find implementations</th>
        <th>Build tooling</th>
      </tr>
   </thead>
   <tbody>
      <tr>
        <td>Go</td>
        <td><a href="https://github.com/sourcegraph/lsif-go">lsif-go</a></td>
        <td><a href="#status-definitions" class="indexer-status">🟢</a></td>
        <td class="indexer-implemented-y">✓</td> <!-- Hover documentation -->
        <td class="indexer-implemented-y">✓</td> <!-- Go to definition -->
        <td class="indexer-implemented-y">✓</td> <!-- Find references -->
        <td class="indexer-implemented-y">✓</td> <!-- Cross-file -->
        <td class="indexer-implemented-y">✓</td> <!-- Cross-repository -->
        <td class="indexer-implemented-y">✓</td> <!-- Find implementations -->
        <td>-</td> <!-- Build tooling -->
      </tr>
      <tr>
        <td>TypeScript/JavaScript</td>
        <td><a href="https://github.com/sourcegraph/scip-typescript">scip-typescript</a></td>
        <td><a href="#status-definitions" class="indexer-status">🟢</a></td>
        <td class="indexer-implemented-y">✓</td> <!-- Hover documentation -->
        <td class="indexer-implemented-y">✓</td> <!-- Go to definition -->
        <td class="indexer-implemented-y">✓</td> <!-- Find references -->
        <td class="indexer-implemented-y">✓</td> <!-- Cross-file -->
        <td class="indexer-implemented-y">✓</td> <!-- Cross-repository -->
        <td class="indexer-implemented-y">✓</td> <!-- Find implementations -->
        <td>-</td> <!-- Build tooling -->
      </tr>
      <tr>
        <td>C/C++</td>
        <td><a href="https://github.com/sourcegraph/scip-clang">scip-clang</a></td>
        <td><a href="#status-definitions" class="indexer-status">🟡</a></td>
        <td class="indexer-implemented-y">✓</td> <!-- Hover documentation -->
        <td class="indexer-implemented-y">✓</td> <!-- Go to definition -->
        <td class="indexer-implemented-y">✓</td> <!-- Find references -->
        <td class="indexer-implemented-y">✓</td> <!-- Cross-file -->
        <td class="indexer-implemented-y">✓</td> <!-- Cross-repository -->
        <td class="indexer-implemented-n">✗</td> <!-- Find implementations -->
        <td><a href="https://github.com/sourcegraph/scip-clang#supported-platforms">See notes</a></td> <!-- Build tooling -->
      </tr>
      <tr>
         <td>Java</td>
        <td><a href="https://github.com/sourcegraph/scip-java">scip-java</a></td>
        <td><a href="#status-definitions" class="indexer-status">🟢</a></td>
        <td class="indexer-implemented-y">✓</td> <!-- Hover documentation -->
        <td class="indexer-implemented-y">✓</td> <!-- Go to definition -->
        <td class="indexer-implemented-y">✓</td> <!-- Find references -->
        <td class="indexer-implemented-y">✓</td> <!-- Cross-file -->
        <td class="indexer-implemented-y">✓*</td> <!-- Cross-repository -->
        <td class="indexer-implemented-y">✓</td> <!-- Find implementations -->
        <td><a href="https://sourcegraph.github.io/scip-java/docs/getting-started.html#supported-build-tools">See notes</a></td> <!-- Build tooling -->
      </tr>
      <tr>
        <td>Scala</td>
        <td><a href="https://github.com/sourcegraph/scip-java">scip-java</a></td>
        <td><a href="#status-definitions" class="indexer-status">🟢</a></td>
        <td class="indexer-implemented-y">✓</td> <!-- Hover documentation -->
        <td class="indexer-implemented-y">✓</td> <!-- Go to definition -->
        <td class="indexer-implemented-y">✓</td> <!-- Find references -->
        <td class="indexer-implemented-y">✓</td> <!-- Cross-file -->
        <td class="indexer-implemented-y">✓*</td> <!-- Cross-repository -->
        <td class="indexer-implemented-y">✓</td> <!-- Find implementations -->
        <td><a href="https://sourcegraph.github.io/scip-java/docs/getting-started.html#supported-build-tools">See notes</a></td> <!-- Build tooling -->
      </tr>
      <tr>
        <td>Kotlin</td>
        <td><a href="https://github.com/sourcegraph/scip-java">scip-java</a></td>
        <td><a href="#status-definitions" class="indexer-status">🟢</a></td>
        <td class="indexer-implemented-y">✓</td> <!-- Hover documentation -->
        <td class="indexer-implemented-y">✓</td> <!-- Go to definition -->
        <td class="indexer-implemented-y">✓</td> <!-- Find references -->
        <td class="indexer-implemented-y">✓</td> <!-- Cross-file -->
        <td class="indexer-implemented-y">✓*</td> <!-- Cross-repository -->
        <td class="indexer-implemented-n">✗</td> <!-- Find implementations -->
        <td><a href="https://sourcegraph.github.io/scip-java/docs/getting-started.html#supported-build-tools">See notes</a></td> <!-- Build tooling -->
      </tr>
      <tr>
        <td>Rust</td>
        <td><a href="https://github.com/rust-lang/rust-analyzer">rust-analyzer</a></td>
        <td><a href="#status-definitions" class="indexer-status">🟢</a></td>
        <td class="indexer-implemented-y">✓</td> <!-- Hover documentation -->
        <td class="indexer-implemented-y">✓</td> <!-- Go to definition -->
        <td class="indexer-implemented-y">✓</td> <!-- Find references -->
        <td class="indexer-implemented-y">✓</td> <!-- Cross-file -->
        <td class="indexer-implemented-y">✓*</td> <!-- Cross-repository -->
        <td class="indexer-implemented-n">✗</td> <!-- Find implementations -->
        <td><a href="https://rust-analyzer.github.io/">See notes</a></td> <!-- Build tooling -->
      </tr>
     <tr>
        <td>Python</td>
        <td><a href="https://github.com/sourcegraph/scip-python">scip-python</a></td>
        <td><a href="#status-definitions" class="indexer-status">🟢</a></td>
        <td class="indexer-implemented-y">✓</td> <!-- Hover documentation -->
        <td class="indexer-implemented-y">✓</td> <!-- Go to definition -->
        <td class="indexer-implemented-y">✓</td> <!-- Find references -->
        <td class="indexer-implemented-y">✓</td> <!-- Cross-file -->
        <td class="indexer-implemented-y">✓</td> <!-- Cross-repository -->
        <td class="indexer-implemented-y">✓</td> <!-- Find implementations -->
        <td><a href="https://github.com/sourcegraph/scip-python">See notes</a></td> <!-- Build tooling -->
      </tr>
     <tr>
        <td>Ruby</td>
        <td><a href="https://github.com/sourcegraph/scip-ruby">scip-ruby</a></td>
        <td><a href="#status-definitions" class="indexer-status">🟢</a></td>
        <td class="indexer-implemented-y">✓</td> <!-- Hover documentation -->
        <td class="indexer-implemented-y">✓</td> <!-- Go to definition -->
        <td class="indexer-implemented-y">✓</td> <!-- Find references -->
        <td class="indexer-implemented-y">✓</td> <!-- Cross-file -->
        <td class="indexer-implemented-y">✓</td> <!-- Cross-repository -->
        <td class="indexer-implemented-n">✗</td> <!-- Find implementations -->
        <td><a href="https://github.com/sourcegraph/scip-ruby#scip-ruby">See notes</a></td> <!-- Build tooling -->
      </tr>
     <tr>
        <td>C#</td>
        <td><a href="https://github.com/sourcegraph/scip-dotnet">scip-dotnet</a></td>
        <td><a href="#status-definitions" class="indexer-status">🟠</a></td>
        <td class="indexer-implemented-y">✓</td> <!-- Hover documentation -->
        <td class="indexer-implemented-y">✓</td> <!-- Go to definition -->
        <td class="indexer-implemented-y">✓</td> <!-- Find references -->
        <td class="indexer-implemented-y">✓</td> <!-- Cross-file -->
        <td class="indexer-implemented-n">✗</td> <!-- Cross-repository -->
        <td class="indexer-implemented-y">✓</td> <!-- Find implementations -->
        <td><code>.sln</code> and <code>.csproj</code></td> <!-- Build tooling -->
      </tr>
   </tbody>
</table>

*Requires enabling and setting up Sourcegraph's auto-indexing feature. Further information in [Auto-indexing](../explanations/auto_indexing.md).

#### Status definitions
An indexer status is:

- 🟢 _Generally Available_: Available as a normal product feature, no major features are absent.
- 🟡 _Partially available_: Available, but may be limited in some significant ways. No major features are absent but edge cases remain.
- 🟠 _Beta_: Available in pre-release form on a limited basis. Could be useful to early adopters despite lack of features.
- 🟣 _Experimental_: Available in pre-release form, with significant caveats. Early adopters can try it with expectations of failure.

## Milestone definitions

A common set of steps required to build feature-complete indexers is broadly outlined below. The implementation order and _doneness criteria_ of these steps may differ between language and development ecosystems. Major divergences will be detailed in the notes below.

### Cross repository: Emits monikers for cross-repository support

The next milestone provides support for cross-repository definitions and references.

The indexer can emit a valid index including import monikers for each symbol defined non-locally, and export monikers for each symbol importable by another repository. This index should be consumed without error by the latest Sourcegraph instance and Go to Definition and Find References should work on cross-repository symbols _given that both repositories are indexed at the exact commit imported_.

At this point, the indexer may be generally considered **ready**. Some languages and ecosystems may require some of the additional following milestones to be considered ready due to a bad out-of-the-box developer experience or absence of a critical language features. For example, scip-java is nearly useless without built-in support for build systems such as gradle.

### Common build tool integration

The next milestone integrates the indexer with a common build tool or framework for the language and surrounding ecosystem. The priority and applicability of this milestone will vary wildly by languages. For example, lsif-go uses the standard library and the language has a built-in dependency manager; all of our customers use Gradle, making scip-java effectively unusable without native Gradle support.

The indexer integrates natively with common mainstream build tools. We should aim to cover _at least_ the majority of build tools used by existing enterprise customers.

The implementation of this milestone will also vary wildly by language. It could be that the indexer runs as a step in a larger tool, or the indexer contains build-specific logic in order to determine the set of source code that needs to be analyzed.

### The long tail: Find implementations

The next milestone represents the never-finished long tail of feature additions. The remaining _20%_ of the language should be supported via incremental updates and releases over time. Support of Find implementations should be prioritized by popularity and demand.

We may lack the language expertise or bandwidth to implement certain features on indexers. We will consider investing resources to add additional features when the demand for support is sufficiently high and the implementation is sufficiently difficult.

<style type="text/css">
.indexer-implemented-y { text-align: center; background-color: #28a745; }
.indexer-implemented-n { text-align: center; background-color: #dc3545; }
</style>
