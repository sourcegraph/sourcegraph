Blocks are the compositional units of a notebook. You can interleave the various block types in a notebook to create rich, powerful documentation. There are four supported block types.

# Block types

## Markdown blocks
Markdown blocks support standard Markdown formatting, enabling you to create headings, lists, code blocks, and more. They are the foundational block type for providing additional context to the specialized block types described below.

## Query blocks
Query blocks support the full Sourcegraph search query language, allowing you to use our full [search syntax](../code_search/reference/index.md), including any of our types, filters, patterns, and predicates, to display the most relevant results.

> Note: Notebook block searches don't use your Sourcegraph instance's default search configuration. For example you'll need to explicitly specify things like `patterntype:regexp` or `context:sourcegraph` even if your user or global settings have a configured default for these values.

## Symbol blocks
With symbol blocks, you can identify the symbol you want to highlight. Symbol blocks are special. As long as the symbol definition stays within the file you selected when you created the block, you don't need to update it when the file changes. Symbol blocks "follow" the symbol around the file and so always display its current location.

Symbol blocks provide some UI affordances to make it easier to find symbols, such as a default `type:symbol` in the query, as well as a purpose-built typeahead specialized for symbol selection.

## File blocks
File blocks are similar to symbol blocks in that they are some special affordances to make them easier to create. You can add an entire file the file block, or you can select a line range of a file. File ranges are great for embedding code snippets into a notebook or highlighting important files. File blocks are editable so you can modify a full file to only show a line range from it, or remove the line range to show an entire file.

If you're viewing a file in Sourcegraph search, you can also copy the URL and paste it directly into a file block or the command palette. If you have a line range selected it will be preserved on paste.
