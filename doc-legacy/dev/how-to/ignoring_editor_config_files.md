# Ignoring editor config files in Git

Many editors create config files when working in a project directory. For example, the `.idea/` directory that IntelliJ creates.

You can ignore these in Git using one of two options:

- Globally, in all Git repositories via `~/.gitignore`
- Per-repository, via `.git/info/exclude` (which is just like `.gitignore` but not committed to the repository)

## Why not commit .gitignore editor config files?

There are many editors that produce repository config files, so if we commit them to `.gitignore` questions arise:

- Do we do this for every editor people at Sourcegraph use?
- How do we keep this `.gitignore` in sync with all the other repositories?
- When creating a new repository, what should go in `.gitignore`?

Instead of maintaining all that, we keep things simple and do not commit editor configs, nor their exclusions, to `.gitignore` in the repository and configure it as part of our development environments instead.
