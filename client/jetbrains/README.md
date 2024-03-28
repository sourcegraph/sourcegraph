# Sourcegraph Code Search for IntelliJ

This directory contains **web assets** for the [Sourcegraph JetBrains
plugin](https://plugins.jetbrains.com/plugin/9682-sourcegraph-cody--code-search/versions/stable).
The JetBrains plugin lives in the
[sourcegraph/jetbrains](https://github.com/sourcegraph/jetbrains) repository.

## Building the assets

Run the following commands to build the assets.

```sh
pnpm i
pnpm build
# (optional) pnpm watch
```

The assets get generated to `src/main/resources/dist`, which are ignored by
git. The JetBrains plugin embeds the contents of that directory into the
plugin.

## Previewing the assets locally

Run the following commands to preview the components in a standalone HTML file.

```sh
pnpm standalone && open http://localhost:3000/
```
Hello World
