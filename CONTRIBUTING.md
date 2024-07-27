# Contributing

## What contributions are accepted?

In short, we are open to nearly all contributions! We love feedback in all forms, issues, comments, PRs, etc!

## Contributing large changes, new features, etc.

Unless you feel confident your change will be accepted (trivial bug fixes, code cleanup, etc) you should first create an issue or a [Sourcegraph RFC](https://handbook.sourcegraph.com/communication/rfcs#external-contributors) (preferred for bigger changes) to discuss your change with us. This lets us all discuss the design and proposed implementation of your change, which helps ensure your time is well spent and that your contribution will be accepted.

> Exception: If you contribute functionality that already exists as a [paid Sourcegraph feature](https://sourcegraph.com/pricing/), we are unlikely to accept it. Consult us beforehand for a definitive answer. (We'll add more details about the process here, and they'll be similar to [GitLab's stewardship principles](https://handbook.gitlab.com/handbook/company/stewardship/#contributing-an-existing-feature-to-open-source-it).)

## Code of Conduct

All interactions with the Sourcegraph open source project are governed by the
[Sourcegraph Community Code of Conduct](https://handbook.sourcegraph.com/company-info-and-process/community/code_of_conduct/).

## How to contribute

1. Select one of the issues labeled as [good first issue](https://github.com/orgs/sourcegraph/projects/210).
2. Clone the repo: `git clone https://github.com/sourcegraph/sourcegraph`.
3. [Set up your development environment](https://docs-legacy.sourcegraph.com/dev/setup/quickstart) to run the project locally.
4. Before creating a pull request, ensure that [recommended checks](https://docs-legacy.sourcegraph.com/dev/contributing) pass locally. We're actively working on making our CI pipeline public to automate this step.
5. **IMPORTANT:** Once you have a pull request ready to review, the 'verification/cla-signed' check will be flagged, and you will be prompted to sign the CLA with a link provided by our bot. Once you sign, add a comment tagging `@sourcegraph/contribution-reviewers`. After that your pull request will be ready for review.
   - For Sourcegraph team members, see [these notes](https://docs-legacy.sourcegraph.com/dev/contributing/accepting_contribution#cla-bot) for how to verify that the CLA has been signed.
6. Once you've chosen an issue, **comment on it to announce that you will be working on it**, making it visible for others that this issue is being tackled. If you end up not creating a pull request for this issue, please delete your comment.
7. If you have any questions, please [refer to the docs first](https://sourcegraph.com/docs/). If you don’t find any relevant information, mention the issue author.
8. The issue author will try to provide guidance. Sourcegraph always works in async mode. We will try to answer as soon as possible, but please keep time zones differences in mind.
9. Join the [Sourcegraph Community Space](https://srcgr.ph/join-community-space) on Discord where the Sourcegraph team can help you!

## Can I pick up this issue?

All open issues are not yet solved. If the task is interesting to you, take it and feel free to do it. There is no need to ask for permission or get in line. Even if someone else can do the task faster than you, don't stop - your solution may be better. It is the beauty of Open Source!

## Relevant development docs

### Getting applications up and running

- [Getting Started Guide](https://sourcegraph.com/docs/getting-started)
- [Troubleshooting section](https://docs-legacy.sourcegraph.com/dev/setup/troubleshooting)

### How to write tests

- [How to write tests](https://docs-legacy.sourcegraph.com/dev/how-to/testing)
- [Testing principles](https://docs-legacy.sourcegraph.com/dev/background-information/testing_principles)
- [Testing web code](https://docs-legacy.sourcegraph.com/dev/background-information/testing_web_code)

### Pull Requests

- [How to structure](https://docs-legacy.sourcegraph.com/dev/background-information/pull_request_reviews#what-makes-an-effective-pull-request-pr)
- Git branch name convention: `[developer-initials]/short-feature-description`
- [Examples on Github](https://github.com/sourcegraph/sourcegraph/pulls?q=is%3Apr+label%3Ateam%2Ffrontend-platform)
- (For Sourcegraph team) [How to accept contributions](https://docs-legacy.sourcegraph.com/dev/contributing/accepting_contribution)
