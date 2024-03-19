# Batch spec cheat sheet

<style>
.markdown-body h2 { margin-top: 50px; }
.markdown-body pre.chroma { font-size: 0.75em; }
</style>

## Overview

There are some common patterns that we reuse all the time when writing [batch specs](batch_spec_yaml_reference.md). In this document we collect these patterns to make it easy for others to copy and reuse them. See our own curated collection of [batch change examples](https://github.com/sourcegraph/batch-change-examples) for even more complete examples of batch specs.

Since most of the examples here make use of [batch spec templating](batch_spec_templating.md), be sure to also take a look at that page.

### Loop over search result paths in shell script

```yaml
on:
  - repositoriesMatchingQuery: OLD-VALUE

steps:
  - run: |
      for file in "${{ join repository.search_result_paths " " }}";
      do
        sed -i 's/OLD-VALUE/NEW-VALUE/g;' ${file}
      done
    container: alpine:3
```

### Put search result paths in file and loop over them

```yaml
on:
  - repositoriesMatchingQuery: OLD-VALUE

steps:
  - run: |
      cat /tmp/search-results | while read file;
      do
        sed -i 's/OLD-VALUE/NEW-VALUE/g;' "${file}"
      done
    container: alpine:3
    files:
      /tmp/search-results: ${{ join repository.search_result_paths "\n" }}
```

### Use search result paths as arguments for single command

```yaml
on:
  - repositoriesMatchingQuery: lang:go fmt.Sprintf("%d", :[v]) patterntype:structural -file:vendor count:10

steps:
  - run: comby -in-place 'fmt.Sprintf("%d", :[v])' 'strconv.Itoa(:[v])' ${{ join repository.search_result_paths " " }}
    container: comby/comby
```

### Format files modified by previous step

```yaml
steps:
  - run: comby -in-place 'fmt.Sprintf("%d", :[v])' 'strconv.Itoa(:[v])' ${{ join repository.search_result_paths " " }}
    container: comby/comby
  - run: goimports -w ${{ join previous_step.modified_files " " }}
    container: unibeautify/goimports
```

### Dynamically set branch name based on workspace

```yaml
workspaces:
  - rootAtLocationOf: package.json
    in: github.com/sourcegraph/*

steps:
  # [... other steps ... ]
  - run:  if [[ -f "package.json" ]]; then cat package.json | jq -j .name; fi
    container: jiapantw/jq-alpine:latest
    outputs:
      projectName:
        value: ${{ step.stdout }}

changesetTemplate:
  # [...]

  # If we have an `outputs.projectName` we use it, otherwise we append the path
  # of the workspace. If the path is emtpy (as is the case in the root folder),
  # we ignore it.
  branch: |
    ${{ if eq outputs.projectName "" }}
    ${{ join_if "-" "thorsten/workspace-discovery" (replace steps.path "/" "-") }}
    ${{ else }}
    thorsten/workspace-discovery-${{ outputs.projectName }}
    ${{ end }}
```

### Process search result paths with script

```yaml
steps:
  - run: |
      for result in "${{ join repository.search_result_paths " " }}"; do
        ruby /tmp/script "${result}" > "${result}.new"
        mv ${result}.new "${result}"
      done;
    container: ruby
    files:
      /tmp/script: |
        #! /usr/bin/env ruby
        require 'yaml';
        content = YAML.load(ARGF.read)
        content['batchchanges'] = 'say hello'
        puts YAML.dump(content)
```

### Use separate file as config file for command

```yaml
steps:
  - run: comby -in-place -matcher .go -config /tmp/comby-conf.toml -f ${{ join repository.search_result_paths "," }}
    container: comby/comby
    files:
      /tmp/comby-conf.toml: |
        [log_to_log15]
        match='log.Printf(":[format]", :[args])'
        rewrite='log15.Warn(":[format]", :[args])'
        rule='where
        rewrite :[format] { "%:[[_]] " -> "" },
        rewrite :[format] { " %:[[_]]" -> "" },
        rewrite :[args] { ":[arg~[a-zA-Z0-9.()]+]" -> "\":[arg]\", :[arg]" }'
```

### Publish only changesets on specific branches

```yaml
changesetTemplate:
  # [...]
  published:
    - github.com/my-org/my-repo@my-branch-name: draft
```

### Create new files in repository

```yaml
steps:
  - run: cat /tmp/global-gitignore >> .gitignore
    container: alpine:3
    files:
      /tmp/global-gitignore: |
        # Vim
        *.swp
        # JetBrains/IntelliJ
        .idea
        # Emacs
        *~
        \#*\#
        /.emacs.desktop
        /.emacs.desktop.lock
        .\#*
        .dir-locals.el
```

### Execute steps only in repositories matching name

```yaml
steps:
  # [... other steps ...]
  - run: echo "name contains sourcegraph-testing" >> message.txt
    if: ${{ matches repository.name "*sourcegraph-testing*" }}
    container: alpine:3
```

### Execute steps based on output of previous command

```yaml
steps:
  - run:  if [[ -f "go.mod" ]]; then echo "true"; else echo "false"; fi
    container: alpine:3
    outputs:
      goModExists:
        value: ${{ step.stdout }}

  - run: go fmt ./...
    container: golang
    if: ${{ outputs.goModExists }}
```

### Write a GitHub Actions workflow that includes [GitHub expression syntax](https://docs.github.com/en/actions/reference/context-and-expression-syntax-for-github-actions)

```yaml
steps:
  - container: alpine:3
    run: |
      #!/usr/bin/env bash

      mkdir -p .github/workflows

      cat <<EOF >.github/workflows/lsif.yml
      name: Index
      on:
        - push
      jobs:
        lsif-go:
          runs-on: ubuntu-latest
          container: sourcegraph/lsif-go
          steps:
            - uses: actions/checkout@v1
            - name: Generate LSIF data
              run: lsif-go
            - name: Upload LSIF data
              run: src code-intel upload -github-token=${{ "\\${{secrets.GITHUB_TOKEN}}" }}
      EOF
```

Since GitHub expression syntax conflicts with Sourcegraph's own template expression syntax, including the expression again as a quoted string within a template expression means that the inner expression will be output as a string (effectively, "ignoring" the contents of the inner expression). For `run:` fields specifically, to avoid the shell also interpreting the GitHub expression as a variable when executing the script, we need to escape the quoted `$` with two backslashes: firstly for the shell script itself, and secondly to escape the backslash within the template expression string.

To use the literal sequence `${{ }}` in non-`run:` fields of the batch spec that [supports templating](batch_spec_templating.md#fields-with-template-support), quoted strings are normally sufficient: `${{ "${{ leave me alone! }}" }}`

### List what files were modified by the batch change in the changeset

```
changesetTemplate:
  title: A batch change
  body: | 
    This batch change modifies:
      ${{ range $index, $file := steps.modified_files }}
       - ${{ $file }}
      ${{ end }}
```
