# Testing a pull request 

When submitting a pull request to Sourcegraph, there are a few tests you'll need to pass before the request can be reviewed. These tests run logical tests that we author to make sure our code runs as expected.

## What are the tests?
- **Prettier** is responsible for mostly visual choices like if we have a line of code that's longer than 70 characters, it will fail. This has nothing to do with the correctness of the code, more of a stylistic choice we make as a team to make sure we all use the same standard.
- **eslint** is similar but more so focused on the logic aspect of the code. We use eslint to avoid bad patterns which otherwise stylistically make sense but lead to potentially buggy code
- **Husky** ([link](https://typicode.github.io/husky/#/)) and **Semantic Pull Requests** ([link](https://github.com/zeke/semantic-pull-requests)) ensure your commit messages have enough semantic information to be able to trigger a release. For example, `feat:` tag is for new features.

## Manual formatting and linting tools

If you don't want to wait for CI to find out whether you've made a mistake, you can either use an editor or IDE that hooks itself up with prettier and eslint, _or_ use the following commands manually before submitting your pull request:

1. `pnpm format` : applies prettier to your code (takes about 30s to run)
1. `pnpm format:check`: checks if your code is passing the prettier checks (takes about 30s to run)
1. `pnpm eslint`: checks if your code is passing the eslint checks (takes about 2s to run)
1. `sg lint`: can run any prettier and lint checks. ([complete docs](https://docs.sourcegraph.com/dev/background-information/sg/reference#sg-lint)) The benefit is that it runs them the same way that CI runs them (On Buildkite, it’s called “Linters and static analysis” → “Run sg lint”). It’s also fast. The downside might be that it doesn’t necessarily run on all files, e.g. Prettier skipped `client/jetbrains` during some testing.
