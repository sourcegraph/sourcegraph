# Commit message guidelines

This document contains recommendations on how to write good commit messages. Having consistent commit messages makes it easier to see at a glance what code has been affected by recent changes. It also makes it easier to search through our commit history.

To get some background, please read [How to Write a Git Commit Message](https://chris.beams.io/posts/git-commit/)

These guidelines are not hard requirements and not enforced in any way. If any of these guidelines turn in to hard requirements in the future, they must be enforced by automated tooling.

## Format

A commit message has two parts: a subject and a body. These are separated by a blank line.

### Subject

The subject line should be concise and easy to visually scan in a list of commits, giving context around what code has changed.

1. Prefix the subject with the primary area of code that was affected (e.g. `web:`, `cmd/searcher:`).
2. Limit the subject line to 50 characters.
3. Do not end the subject line with punctuation.
4. Use the [imperative mood](https://chris.beams.io/posts/git-commit/#imperative) in the subject line.

    | Prefer | Instead of |
    |--------|------------|
    | Fix bug in XYZ | Fixed a bug in XYZ |
    | Change behavior of X | Changing behavior of X |

Example:

> cmd/searcher: Add scaffolding for structural search

## Body

The purpose of a commit body is to explain _what_ changed and _why_. The _how_ is the diff.

[Pull request reviews](pull_request_reviews.md) happens in PRs, which should contain a good subject and description. When a PR is approved, we prefer to squash merge all commits on the PR branch into a single commit on master. After clicking "Squash and merge", edit the body of the final commit message so that it is clean and informative. The commit body should either be empty (assuming that you have a good PR description), or a brief summary of your change (which could be exactly your PR description if it is concise). It should not include unimportant details from incremental commits on the PR branch (e.g. `* add test`, `* fix test`, `* try X`, `Update filename.md`, `Co-Authored-By...`).
