# The `CODEOWNERS` format

Code ownership is defined as a strict ruleset. Files can be assigned to owners.
To define rulesets for code ownership, we make use of the `CODEOWNERS` format.

`CODEOWNERS` files contain a sequence of matching rules - a glob pattern and zero or more owners.
**A repository has at most one `CODEOWNERS` file.**

## Specifying Owner information

Owners can be defined by a username/team name or an email address.

Using email addresses is generally recommended, as email addresses are most likely the same across different platforms, and are independent of a user having registered yet.
In Sourcegraph, a user can add multiple email addresses to their profile. All of those would match to the same user.

For committed `CODEOWNERS` files, the usernames are usually the username **on the code host**, so they don't necessarily match with the Sourcegraph username.
This is a known limitation, and in the future, we will provide ways to map external code host names to Sourcegraph users.
For now, you can search for a user by their code host username, or switch to using emails in the `CODEOWNERS` files, which will work across both Sourcegraph and the code host.

## File format

The following snippet shows an example of a valid `CODEOWNERS` file.

```
*.txt @text-team
# this is a comment explaining why Alice owns this
/build/logs/ alice@sourcegraph.com 
/cmd/**/test @qa-team @user
```

- Asterisk `*` is a wildcard that matches N tokens in a path segment.  
  **Example**: `doc/*/own` will match `doc/ref/own` and `doc/tutorial/own`, but not `doc/a/b/own`
- Double `**` asterisk matches any sub-path.  
  **Example**: `doc/**/own` will match `doc/ref/own` and `doc/a/b/own`
- Starting a pattern with `/` anchors matches at the repository root.  
  **Example**: `/docs/*` matches `/docs/a.md` and `/docs/b.md` but not `/src/docs/a.md`.
- Trailing slash `/` matches any file within the directory tree (so it is equivalent to trailing `/**`).  
  **Example**: `docs/` matches `/testing/docs/foo` and `/docs/foo/bar`, but does not match `/docs` or `/testing/docs`.


The rules are considered independently and in order. Rules farther down the file take precedence. Only **one** rule matches. So for instance for `/build/logs/log-1.txt` the owner will only be `alice@sourcegraph.com` and not `@text-team` since the `/build/logs/` rule will take precedence over `*.txt` rule.

## Limitations

- GitLab allows sections in `CODEOWNERS` files, these are not yet supported and section markers are ignored
- [Code Owners for Bitbucket](https://marketplace.atlassian.com/apps/1218598/code-owners-for-bitbucket?tab=overview&hosting=cloud) inline defined groups are not yet supported

To configure ownership in Sourcegraph, you have two options:

## Committing a `CODEOWNERS` file to your repositories

> Use this approach if you prefer versioned ownership data.

You can simply commit a `CODEOWNERS` file at any of the following locations for it to be picked up automatically by code ownership:

```
CODEOWNERS
.github/CODEOWNERS
.gitlab/CODEOWNERS
docs/CODEOWNERS
```

Searches at specific commits will return any `CODEOWNERS` data that exists at that specific commit.

## Uploading a `CODEOWNERS` file to Sourcegraph

> Use this approach if you don't want to commit `CODEOWNERS` files to your repos, or if you have an existing system that tracks ownership data and want to sync that data with Sourcegraph.

Read more on how to [manually ingest `CODEOWNERS` data](codeowners_ingestion.md) into your Sourcegraph instance.

The [docs](codeowners_ingestion.md) detail how to use the UI or `src-cli` to upload `CODEOWNERS` files to Sourcegraph.
