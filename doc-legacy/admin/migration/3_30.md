# Migrating from 3.30.0, 3.30.1, and 3.30.2

The Sourcegraph 3.30 release introduced a change that caused corruption in certain indexes, breaking a number of Sourcegraph features. **This issue affects Sourcegraph 3.30.0, 3.30.1, and 3.30.2**, and was fixed in 3.30.3.

- Users on 3.29.x are advised to upgrade directly to 3.30.3.
- Users that have already upgraded to one of the affected releases must fix the already corrupt databases manually by following this guide: [**How to rebuild corrupt Postgres indexes**](../how-to/rebuild-corrupt-postgres-indexes.md).

> WARNING: If you have already upgraded to one of the affected releases, **do not upgrade to 3.30.3** after applying the above fix. Instead, [please upgrade directly to 3.31](./3_31.md).

If you need any additional assistance, please reach out to [support@sourcegraph.com](mailto:support@sourcegraph.com).

## Background

The 3.30 release introduced a `pgsql` and `codeinteldb` base image change from debian to alpine which changed the default OS locale.
This caused corruption in indexes that have collatable key columns (e.g. any index with a `text` column).
Read more about this [here](https://postgresql.verite.pro/blog/2018/08/27/glibc-upgrade.html).

After we found the root-cause of the [issues many customers were seeing](https://github.com/sourcegraph/sourcegraph/issues/23288), we cut [a patch release, 3.30.3](../../CHANGELOG.md#3-30-3), that reverted the images to be based on debian, buying us time to change the alpine based version of the images to [reindex affected indexes on startup, before accepting new connections](https://github.com/sourcegraph/sourcegraph/issues/23310).

However, this means that after fixing the corrupt indexes on the alpine images in the affected releases, upgrading to debian based images in 3.30.3 will cause index corruption again. For this reason, **do not upgrade to 3.30.3** after fixing corrupt Postgres indexes. Instead, [please upgrade directly to 3.31](./3_31.md).
