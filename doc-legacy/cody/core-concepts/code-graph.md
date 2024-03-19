# Code Graph

<p class="subtitle">Understand what is Code Graph and how Cody use it to gather context.</p>

Code Graph is a key component of Cody's capacity to generate contextual responses based on your codebase. It involves analyzing the structure of the code rather than treating it as plain text. Cody examines how different components of the codebase are interconnected and how they are used.

This method is dependent on the code's structure and inheritance relationships. It can help Cody find context related to your input based on how code elements are linked and utilized.

## Code Graph data

Code graph data refers to the information that describes various semantic elements within your source code, like definitions, references, symbols, and doc comments. This dataset is produced by an indexer and subsequently transferred to a Sourcegraph instance.

The process of generating this data can vary based on factors such as the programming language and build system in use. In some cases, Sourcegraph can automatically create this data through auto-indexing within the platform itself.

Alternately, you may need to set up a periodic CI job, specifically designed to produce and upload this index to your Sourcegraph instance.

>NOTE: As of January 23, 2023, open-source projects have the capability to generate Code Graph data in their CI pipelines and then upload it to Sourcegraph.com, enhancing precision in code navigation.
