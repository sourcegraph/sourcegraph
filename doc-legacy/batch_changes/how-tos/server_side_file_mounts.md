# Using file mounts with server-side execution

<aside class="note">
<p>
<span class="badge badge-note">Note</span> This is new!
</p>

<p><b>We're very much looking for input and feedback on this feature.</b> You can either <a href="https://sourcegraph.com/contact">contact us directly</a>, leave feedback <a href="https://github.com/sourcegraph/sourcegraph/issues/14851">in this issue</a>, or <a href="https://twitter.com/sourcegraph">tweet at us</a>.</p>

<p>This feature is available in Sourcegraph 4.1 with <a href="https://github.com/sourcegraph/src-cli">Sourcegraph CLI</a> 4.0.1 and later.</p>
</aside>

> NOTE: Running a batch spec [server-side](../explanations/server_side.md) with file mounts is
> currently only supported with <a href="https://github.com/sourcegraph/src-cli">Sourcegraph CLI</a>.

File [`mounts`](../references/batch_spec_yaml_reference.md#steps-mount) are a powerful way to run custom files without
directly embedding the files in your batch spec.

## Writing a batch spec

In the following example, you have a Python script that appends "Hello World" to all `README.md` files.

```python
#!/usr/bin/env python3
import os.path


def main():
  if os.path.exists('README.md'):
    with open('README.md', 'a') as f:
      f.write('\nHello World')


if __name__ == "__main__":
  main()
```

To use the Python script in your batch change, mount the script in a `step` using
the [`mounts`](../references/batch_spec_yaml_reference.md#steps-mount) field. The following is an example of mounting
the above Python script in a `step`.

```yaml
name: hello-world
description: Add Hello World to READMEs

# Find all repositories that contain a README.md file.
on:
  - repositoriesMatchingQuery: file:README.md

# In each repository, run this command. Each repository's resulting diff is captured.
steps:
  - run: python /tmp/hello_appender.py
    container: python:latest
    mount:
      - path: ./hello_appender.py
        mountpoint: /tmp/hello_appender.py

# Describe the changeset (e.g., GitHub pull request) you want for each repository.
changesetTemplate:
  title: Hello World
  body: My first batch change!
  branch: hello-world # Push the commit to this branch.
  commit:
    message: Append Hello World to all README.md files
  published: false # Do not publish any changes to the code hosts yet
```

In this example, the Python script should live besides the batch spec file, as indicated by the `path`:

```text
.
├── batch-spec.yml
└── hello_appender.py
```

Note that a `container` appropriate for the mounted file has also been chosen for this step.

## Running server-side

After writing the batch spec, use the <a href="https://github.com/sourcegraph/src-cli">Sourcegraph CLI (`src`)</a>
command [`remote`](../../cli/references/batch/remote.md) to execute the batch spec server-side.

```shell
src batch remote -f batch-spec.yml
```

Once successful, `src` provides a URL to the execution of the batch change.
