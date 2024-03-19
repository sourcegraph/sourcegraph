# `sg` Vision

## Principles

- `sg` should be fun to use.
- If you think "it would be cool if `sg` could do X": add it! Let's go :)
- `sg` should make Sourcegraph developers productive and happy.
- `sg` is not and should not be a build system.
- `sg` is not and should not be a container orchestrator.
- Try to fix [a lot of the problems in this RFC](https://docs.google.com/document/d/18hrRIN0pUBRwUFF7vkcVmstJccqWeHiecNF2t1GAZfU/edit) by encoding conventions in executable code.
- No bash. `sg` was built to get rid of all the bash scripts in `./dev/`. If you have a chance to build something into `sg` to avoid another bash script: do it. Try to keep shell scripts to easy-to-understand one liners if you must. Replicating something in Go code that could be done in 4 lines of bash is probably a good idea.
- Duplicated data is fine as long as it's dumb data. Copying some lines in `sg.config.yaml` to get something working is often (but not always) better than trying to be clever.

You can also watch [this video](https://drive.google.com/file/d/1DXjjf1YXr8Od8vG4R74Ko-soLOx_tXa6/view?usp=sharing) to get an overview of the original thinking that lead to `sg`.

## Inspiration

- [GitLab Developer Kit (GDK)](https://gitlab.com/gitlab-org/gitlab-development-kit)
- Stripe's `pay` command, [described here](https://buttondown.email/nelhage/archive/papers-i-love-gg/)
- [Stack Exchange Local Environment Setup](https://twitter.com/nick_craver/status/1375871107773956103?s=21) command

## Roadmap

The [`sg` label](https://github.com/sourcegraph/sourcegraph/issues?q=is%3Aopen+label%3Asg+sort%3Aupdated-desc) tracks features and improvements for `sg` that we want to make.

Have feedback or ideas? Feel free to create an issue or [open a discussion](https://github.com/sourcegraph/sourcegraph/discussions/categories/developer-experience)! Sourcegraph teammates can also leave a message in [#dev-experience](https://sourcegraph.slack.com/archives/C01N83PS4TU).

Inspired and want to contribute? Head on over to [contributing to `sg`](index.md#contributing-to-sg)!
