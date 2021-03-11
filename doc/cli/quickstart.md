# Quickstart for `src`

Get started and run your first [code search](../../code_search/index.md) from the command line in 10 minutes or less.

## Introduction

In this guide, you'll install the Sourcegraph CLI, `src`, connect it to your Sourcegraph instance, and use it to run a code search.

## Installation

`src` is shipped as a single, standalone binary. You can get the latest release by following the instructions for your operating system below:

### macOS

```sh
curl -L https://sourcegraph.com/.api/src-cli/src_darwin_amd64 -o /usr/local/bin/src
chmod +x /usr/local/bin/src
```

### Linux

```sh
curl -L https://sourcegraph.com/.api/src-cli/src_linux_amd64 -o /usr/local/bin/src
chmod +x /usr/local/bin/src
```

### Windows

You can install using PowerShell as follows:

```powershell
New-Item -ItemType Directory 'C:\Program Files\Sourcegraph'

Invoke-WebRequest https://sourcegraph.com/.api/src-cli/src_windows_amd64.exe -OutFile 'C:\Program Files\Sourcegraph\src.exe'

[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', [EnvironmentVariableTarget]::Machine) + ';C:\Program Files\Sourcegraph', [EnvironmentVariableTarget]::Machine)
$env:Path += ';C:\Program Files\Sourcegraph'
```

For other options, please refer to [the Windows specific `src` documentation](explanations/windows.md).

## Connect to Sourcegraph

`src` needs to be authenticated against your Sourcegraph instance. The quickest way to do this is to run `src login https://YOUR-SOURCEGRAPH-INSTANCE` and follow the instructions:

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/campaigns/src_login_success.png" class="screenshot" alt="Output from src login showing success">

Once complete, you should have two new environment variables set: `SRC_ENDPOINT` and `SRC_ACCESS_TOKEN`.

## Run a code search

Searching is performed using the [`src search`](references/search.md) command. For example, to search for `ResolveRepositories` in the `src` repository, you can run:

```sh
src search 'r:github.com/sourcegraph/src-cli ResolveRepositories'
```

This should result in this output:

<img src="https://sourcegraphstatic.com/docs/images/integration/cli/quickstart-search.png" class="screenshot" alt="Terminal output from the above search">

## Congratulations!

You've run your first search from the command line! 🎉🎉

You can now explore the [range of commands `src` provides](references/index.md), including the extensive support for [batch changes](../../batch_changes/index.md).

To learn what else you can do with `src`, see "[CLI](index.md)" in the Sourcegraph documentation.
