---
ignoreDisconnectedPageCheck: true
---

# AWS CodeCommit integration with Sourcegraph

You can use Sourcegraph with Git repositories hosted on [AWS CodeCommit](https://aws.amazon.com/codecommit/).

Feature | Supported?
------- | ----------
[Repository syncing](../admin/external_service/aws_codecommit.md) | ✅
[Browser extension](browser_extension.md) | ❌

## Repository syncing

Site admins can [add AWS CodeCommit repositories to Sourcegraph](../admin/external_service/aws_codecommit.md).

## Browser extension

The [Sourcegraph browser extension](browser_extension.md) does not yet support AWS CodeCommit. This means that you won't get hovers, go-to-definition, and find-references from Sourcegraph when viewing your code on AWS CodeCommit's web interface.
