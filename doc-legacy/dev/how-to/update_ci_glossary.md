# How to update the CI Glossary

This guide describes how to edit the annotation that is posted on every CI build, providing a brief description about the various tools we use to build Sourcegraph.

## Principles 

We want to keep the glossary succint and maintainable. Therefore, when adding or removing an item, please consider the following rules: 

- Only add tools used to build `sourcegraph/sourcegraph`. 
- Assume the reader is a fellow software engineer. 
- Describe the tool from a general standpoint. 
- Stay succint, ideally not more than one sentence. 

**Good:** _ASDF is a CLI tool that can manage multiple language runtime versions on a per-project basis._ 

Why? Because this description will stay correct for a very long time. This glossary is not for acting as single source of truth for a given tool, but instead to enough context to the reader to understand what it is about.

**Bad:** _ASDF is a CLI tool that can manage multiple language runtime versions on a per-project basis that we have used for years and is now going to be deprecated as we're migrating to Bazel. Only stateless agents run `asdf`, Bazel agents are skipping it._ 

Why? This description covers both general context as well as Sourcegraph specific context. The latter changes really quickly, and chances are that this description will become out of date, thus adding more confusion for the reader. 

## Steps to update 

1. Check out a new branch.
1. Edit `./dev/ci/glossary.md`.
1. Sort the entries by alphabetical order.
1. Commit and push. The CI build will show the newly edited annotation. 
1. Submit the PR as usual. Set the DevX team as reviewers. 
