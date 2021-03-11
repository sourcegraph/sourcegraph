# batch spec templating

<style>
.markdown-body h2 { margin-top: 50px; }
.markdown-body pre.chroma { font-size: 0.75em; }
</style>

## Overview

[Certain fields](#fields-with-template-support) in a [batch spec YAML](batch_spec_yaml_reference.md) support templating to create even more powerful and performant batch changes.

Templating in a batch spec uses the delimiters `${{` and `}}`. Inside the delimiters, [template variables](#template-variables) and [template helper functions](#template-helpers-functions) may be used to produce a text value.

### Example batch spec

Here is an excerpt of a batch spec that uses templating:

```yaml
on:
  - repositoriesMatchingQuery: lang:go fmt.Sprintf("%d", :[v]) patterntype:structural -file:vendor

steps:
  - run: comby -in-place 'fmt.Sprintf("%d", :[v])' 'strconv.Itoa(:[v])' ${{ join repository.search_result_paths " " }}
    #                                                                   ^ templating starts here
    container: comby/comby
  - run: goimports -w ${{ join previous_step.modified_files " " }}
    #                 ^ templating starts here
    container: unibeautify/goimports
```

Before executing the first `run` command, `repository.search_result_paths` will be replaced with the relative-to-root-dir file paths of each search result yielded by `repositoriesMatchingQuery`. By using the [template helper function](#template-helper-functions) `join`, an argument list of whitespace-separated values is constructed.

The final `run` value, that will be executed, will look similar to this:

```yaml
run: comby -in-place 'fmt.Sprintf("%d", :[v])' 'strconv.Itoa(:[v])' cmd/src/main.go internal/fmt/fmt.go
```

The result is that `comby` only search and replaces in those files, instead of having to search through the complete repository.

Before the second step is executed `previous_step.modified_files` will be replaced with the list of files that the previous `comby` step modified. It will look similar to this:

```yaml
run: goimports -w cmd/src/main.go internal/fmt/fmt.go
```

See "[Examples](#examples)" for more examples of how to use and leverage templating in batch specs.

## Fields with template support

Templating is supported in the following fields:

- [`steps.run`](batch_spec_yaml_reference.md#steps-run)
- [`steps.env`](batch_spec_yaml_reference.md#steps-run) values
- [`steps.files`](batch_spec_yaml_reference.md#steps-run) values
- [`steps.outputs.<name>.value`](batch_spec_yaml_reference.md#steps-outputs)

Additionally, with Sourcegraph 3.24 and [Sourcegraph CLI](../../cli/index.md) 3.24 or later:

- [`changesetTemplate.title`](batch_spec_yaml_reference.md#changesettemplate-title)
- [`changesetTemplate.body`](batch_spec_yaml_reference.md#changesettemplate-body)
- [`changesetTemplate.branch`](batch_spec_yaml_reference.md#changesettemplate-branch)
- [`changesetTemplate.commit.message`](batch_spec_yaml_reference.md#changesettemplate-commit-message)
- [`changesetTemplate.commit.author.name`](batch_spec_yaml_reference.md#changesettemplate-commit-author)
- [`changesetTemplate.commit.author.email`](batch_spec_yaml_reference.md#changesettemplate-commit-author)

## Template variables

Template variables are the names that are defined and accessible when using templating syntax in a given context.

Depending on the context in which templating is used, different variables are available.

For example: in the context of `steps` the template variable `previous_step` is available, but not in the context of `changesetTemplate`.

### `steps` context

The following template variables are available in the fields under `steps`.

They are evaluated before the execution of each entry in `steps`, except for the `step.*` variables, which only contain values _after_ the step has executed.

| Template variable | Type | Description |
| --- | --- | --- |
| `batch_change.name` | `string` | The `name` of the batch change, as set in the batch spec. </br><i><small>Requires [Sourcegraph CLI](../../cli/index.md) 3.26 or later</small></i>. |
| `batch_change.description` | `string` | The `description` of the batch change, as set in the batch spec. </br><i><small>Requires [Sourcegraph CLI](../../cli/index.md) 3.26 or later</small></i>. 
| `repository.search_result_paths` | `list of strings` | Unique list of file paths relative to the repository root directory in which the search results of the `repositoriesMatchingQuery`s have been found. |
| `repository.name` | `string` | Full name of the repository in which the step is being executed. |
| `previous_step.modified_files` | `list of strings` | List of files that have been modified by the previous step in `steps`. Empty list if no files have been modified. |
| `previous_step.added_files` | `list of strings` | List of files that have been added by the previous step in `steps`. Empty list if no files have been added. |
| `previous_step.deleted_files` | `list of strings` | List of files that have been deleted by the previous step in `steps`. Empty list if no files have been deleted. |
| `previous_step.stdout` | `string` | The complete output of the previous step on standard output. |
| `previous_step.stderr` | `string` | The complete output of the previous step on standard error. |
| `step.modified_files` | `list of strings` | Only in `steps.outputs`: List of files that have been modified by the just-executed step. Empty list if no files have been modified. </br><i><small>Requires Sourcegraph 3.24 and [Sourcegraph CLI](../../cli/index.md) 3.24 or later</small></i>. |
| `step.added_files` | `list of strings` | Only in `steps.outputs`: List of files that have been added by the just-executed step. Empty list if no files have been added. </br><i><small>Requires Sourcegraph 3.24 and [Sourcegraph CLI](../../cli/index.md) 3.24 or later</small></i>. |
| `step.deleted_files` | `list of strings` | Only in `steps.outputs`: List of files that have been deleted by the just-executed step. Empty list if no files have been deleted. </br><i><small>Requires Sourcegraph 3.24 and [Sourcegraph CLI](../../cli/index.md) 3.24 or later</small></i>. |
| `step.stdout` | `string` | Only in `steps.outputs`: The complete output of the just-executed step on standard output.</br><i><small>Requires Sourcegraph 3.24 and [Sourcegraph CLI](../../cli/index.md) 3.24 or later</small></i>. |
| `step.stderr` | `string` | Only in `steps.outputs`: The complete output of the just-executed step on standard error. </br><i><small>Requires Sourcegraph 3.24 and [Sourcegraph CLI](../../cli/index.md) 3.24 or later</small></i>. |

### `changesetTemplate` context

> NOTE: Templating in `changsetTemplate` is only supported in Sourcegraph 3.24 and [Sourcegraph CLI](../../cli/index.md) 3.24 or later.

The following template variables are available in the fields under `changesetTemplate`.

They are evaluated after the execution of all entries in `steps`.

| Template variable | Type | Description |
| --- | --- | --- |
| `batch_change.name` | `string` | The `name` of the batch change, as set in the batch spec. </br><i><small>Requires [Sourcegraph CLI](../../cli/index.md) 3.26 or later</small></i>. |
| `batch_change.description` | `string` | The `description` of the batch change, as set in the batch spec. </br><i><small>Requires [Sourcegraph CLI](../../cli/index.md) 3.26 or later</small></i>. 
| `repository.search_result_paths` | `list of strings` | Unique list of file paths relative to the repository root directory in which the search results of the `repositoriesMatchingQuery`s have been found. |
| `repository.name` | `string` | Full name of the repository in which the step is being executed. |
| `steps.modified_files` | `list of strings` | List of files that have been modified by the `steps`. Empty list if no files have been modified. |
| `steps.added_files` | `list of strings` | List of files that have been added by the `steps`. Empty list if no files have been added. |
| `steps.deleted_files` | `list of strings` | List of files that have been deleted by the `steps`. Empty list if no files have been deleted. |
| `outputs.<name>` | depends on `outputs.<name>.format`, default: `string`| Value of an [`output`](batch_spec_yaml_reference.md#steps-outputs) set by `steps`. If the [`outputs.<name>.format`](batch_spec_yaml_reference.md#steps-outputs-format) is `yaml` or `json` and the `value` a data structure (i.e. array, object, ...), then subfields can be accessed too. See "[Examples](#examples)" below. |

## Template helper functions

- `${{ join repository.search_result_paths "\n" }}` - joins the list of strings given as first argument with the separator as last argument.
- `${{ join_if "---" "a" "b" "" "d" }}` - uses the first argument as separator to join the remaining arguments, ignoring blank strings.
- `${{ replace "a/b/c/d" "/" "-" }}` - replaces occurrences of second argument in the first one with the last one.
- `${{ split repository.name "/" }}` - splits the first argument into a list of strings at each occurrence of the last argument.

The features of Go's [`text/template`](https://golang.org/pkg/text/template/) package are also available, including conditionals and loops, since it is the underlying templating engine.

## Examples

Pass the exact list of search result file paths to a command:

```yaml
steps:
  - run: comby -in-place -config /tmp/go-sprintf.toml -f ${{ join repository.search_result_paths "," }}
    container: comby/comby
    files:
      /tmp/go-sprintf.toml: |
        [sprintf_to_strconv]
        match='fmt.Sprintf("%d", :[v])'
        rewrite='strconv.Itoa(:[v])'
```

Run a command for each search result file path:

```yaml
steps:
  - run: |
      for file in "${{ join repository.search_result_paths " " }}";
      do
        sed -i 's/mydockerhub-user/ci-dockerhub-user/g;' ${file}
      done
    container: alpine:3
```

Format and fix files after a previous step modified them:

```yaml
steps:
  - run: |
      find . -type f -name '*.go' -not -path "*/vendor/*" |\
      xargs sed -i 's/fmt.Println/log.Println/g'
    container: alpine:3
  - run: goimports -w ${{ join previous_step.modified_files " " }}
    container: unibeautify/goimports
```

Use the `steps.files` combined with template variables to construct files inside the container:

```yaml
steps:
  - run: |
      cat /tmp/search-results | while read file;
      do
        ruplacer --subvert whitelist allowlist --go ${file} || echo "nothing to replace";
        ruplacer --subvert blacklist denylist --go ${file} || echo "nothing to replace";
      done
    container: ruplacer
    files:
      /tmp/search-results: ${{ join repository.search_result_paths "\n" }}
```

Put information in environment variables, based on the output of previous step `steps.env` also

```yaml
steps:
  - run: echo $LINTER_ERRROS >> linter_errors.txt
    container: alpine:3
    env:
      LINTER_ERRORS: ${{ previous_step.stdout }}
```

If you need to escape the `${{` and `}}` delimiters you can simply render them as string literals:

```yaml
steps:
  - run: cp /tmp/escaped.txt .
    container: alpine:3
    files:
      /tmp/escaped.txt: ${{ "${{" }} ${{ "}}" }}
```

Accessing the `outputs` set by `steps` in subsequent `steps` and the `changesetTemplate`:

```yaml
steps:
  - run: echo "Hello there!"
    container: alpine:3
    outputs:
      myFriendlyMessage:
        value: "${{ step.stdout }}"
  - run: echo "We have access to the output here: ${{ outputs.myFriendlyMessage }}"
    container: alpine:3
    outputs:
      stepTwoOutput:
        otherMessage: "here too: ${{ outputs.myFriendlyMessage }}"

changesetTemplate:
  # [...]
  body: |
    The first step left us the following message: ${{ outputs.myFriendlyMessage }}
    The second step left this one: ${{ outputs.otherMessage }}
```

Using the [`steps.outputs.<name>.format`](batch_spec_yaml_reference.md#steps-outputs-name-format) field, it's possible to parse the value of an output as JSON or YAML and access it as a data structure instead of just text:

```yaml
steps:
  - run: cat .goreleaser.yml
    container: alpine:3
    outputs:
      goreleaserConfig:
        value: "${{ step.stdout }}"
        # The step's output is parsed as YAML, making it accessible as a YAML
        # object in the other templating fields.
        format: yaml
      goreleaserConfigExists:
        # We can use the power of Go's text/template engine to dynamically produce complex values
        value: "exists: ${{ gt (len step.stderr) 0 }}"
        format: yaml

changesetTemplate:
  # [...]

  # Since templating fields use Go's `text/template` and `goreleaserConfig` was
  # parsed as YAML we can iterate over every field:
  body: |
    This repository has a `gorelaserConfig`: ${{ outputs.goreleaserConfigExists.exists }}.

    The `goreleaser.yml` defines the following `before.hooks`:

    ${{ range $index, $hook := outputs.goreleaserConfig.before.hooks }}
    - `${{ $hook }}`
    ${{ end }}
```
