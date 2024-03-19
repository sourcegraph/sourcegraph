# Paid subscriptions for Sourcegraph Enterprise

> NOTE: Pricing documentation below applies to [Sourcegraph Enterprise](https://sourcegraph.com/pricing).

Organizations using Sourcegraph can [upgrade to Sourcegraph Enterprise](https://sourcegraph.com/pricing) to get the features that large organizations need (single sign-on, backups and recovery, cluster deployment, access to code navigation and intelligence, etc.). These additional features in Sourcegraph Enterprise are paid and not open source.

You can [contact Sourcegraph](https://sourcegraph.com/contact/sales) to purchase a subscription. This entitles you to a license key (provided immediately after your purchase), which activates Enterprise features on your Sourcegraph instance.

## Volume discounts

[Contact us](https://sourcegraph.com/contact) to ask about volume discounts for Sourcegraph Enterprise. 

## Pricing model

[Sourcegraph's pricing](https://sourcegraph.com/pricing) is based primarily on [total active user accounts](#how-user-accounts-are-counted). In certain situations, it is also a combination of the [number of lines of code indexed](#how-lines-of-code-are-counted). Please [contact Sourcegraph](https://sourcegraph.com/contact/sales) to answer specific questions about our pricing model for your use. 

## How user accounts are counted

This count is maintained on your Sourcegraph instance, viewable and auditable on the **Site admin > Users** page, and is reported back in aggregate to Sourcegraph.com via [pings](https://docs.sourcegraph.com/admin/pings).

A Sourcegraph user account is created when a user signs up or signs in for the first time. Sourcegraph user accounts can be deleted by administrators via the **Site admin > Users** page, or using the [GraphQL API](../../api/graphql/index.md) or the [Sourcegraph CLI](https://github.com/sourcegraph/src-cli).

## How lines of code are counted

We count the number of newlines (`\n`) appearing in the text search index. Our text search index contains the working copy of your default branch in all repositories synchronized by Sourcegraph. The default branch is typically called `master` or `main`. Additional branches can be configured to be indexed by site administrators. These will also be part of the newlines count.

Our text search index deduplicates for paths that are the same across branches. We only count the newlines once per unique file. Forks are separately indexed. They are included in the count of new lines.

Sourcegraph administrators can view the lines of code count for each repository by visiting the repository and clicking on **Settings > Indexing**.

## Updating your license key

- Navigate to **Site admin > Configuration > Site configuration**, update the `licenseKey` field with the new value, and then click **Save changes**.
