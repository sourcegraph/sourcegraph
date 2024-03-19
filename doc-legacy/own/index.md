# Code ownership

<aside class="beta">
<p>
<span class="badge badge-beta">Beta</span> This feature is currently in beta.
</p>

<p><b>We're very much looking for input and feedback on this feature.</b> You can either <a href="https://sourcegraph.com/contact">contact us directly</a>, <a href="https://github.com/sourcegraph/sourcegraph">file an issue</a>, or <a href="https://twitter.com/sourcegraph">tweet at us</a>.</p>
</aside>

Code ownership is aimed at helping find the right person and team to contact, for any question, at any time. We are starting out with code ownership, ownership inference and assignments and are exploring ways to help you find someone to answer _every_ question.

## Enabling code ownership

Code ownership is enabled by default. If you like to disable it from being shown in the UI, you can create the feature flag `enable-ownership-panels` and set it to `false`:

- Go to **Site-admin > Feature flags**
- If the feature flag `enable-ownership-panels` doesn't yet exist, click **Create feature flag**
- Under **Name**, put `enable-ownership-panels`
- For **Type** select **Boolean**
- And set **Value** to **False**
- Click **Create flag**

## Concepts

**Owner**: An owner is defined as a person or a team in Sourcegraph.

A _person_ can be:
- a Sourcegraph user which we were able to resolve from the `CODEOWNERS` handle or email, in which case we link to their profile.
- an unknown user for which we were unable to resolve a profile, in which case we will return the `CODEOWNERS` data we have.

A _team_ is a group of Sourcegraph users represented by a common handle, which is a new feature that we added. 
[Read more about how to manage teams in Sourcegraph](../admin/teams/index.md).

## Code ownership

<!-- TODO: There are 3 ways: add docs about inference as a second part of https://github.com/sourcegraph/sourcegraph/issues/53654 -->
Code ownership is set in 2 different ways:

- [The `CODEOWNERS` format](codeowners_format.md)
- [Assigned ownership](assigned_ownership.md)

## Limitations

- Code ownership support has been released as an MVP for 5.0. In the future of the product we intend to infer ownership beyond `CODEOWNERS` data.
- The feature has not been fully validated to work well on large repositories or large `CODEOWNERS` rulesets. This is a future area of improvement, but please contact us if you run into issues.

## Browsing ownership

The ownership information is available for browsing once ownership data is available through [a `CODEOWNERS` file](#code-ownership).

When displaying a source file, there is a bar above the file contents.

- On the left-hand side, it displays the most recent change to the file.
- On the right-hand side it displays the code ownership bar with at most 2 file owners. Any additional number of owners is also displayed.

![File view showing code ownership bar on the right hand side above the file contents](https://storage.googleapis.com/sourcegraph-assets/docs/own/blob-view.png)

After clicking on the code ownsership bar, a bottom panel appears listing all the owners.

![File view with the ownership tab selected in the bottom panel](https://storage.googleapis.com/sourcegraph-assets/docs/own/blob-view-panel.png)

There is always a single rule in a `CODEOWNERS` file that determines ownership (if any). Each owner listed in the bottom panel has a description found by clicking the collapsible arrow: _Owner is associated with a rule in a `CODEOWNERS` file_. Clicking this description links to the line containing the responsible rule in the `CODEOWNERS` file.

If any email information has been found for the owner, clicking the mail icon will  start an email to them. 

## Ownership search

### Searching for files with owners

Code ownership is a first-class citizen in search. Ownership can be either a query input or a search result:

- `file:has.owner(user@example.com)` keeps only the search results associated with given user (here referred to by e-mail).
- `-file:has.owner(@username)` removes all results owned by specific user (here referred to by name).

Ownership predicate can also be used without parameters:

-`file:has.owner()` will only include files with an owner assigned to them.
-`-file:has.owner()` will only include files without an owner.

When performing a search the `select:file.owners` predicate will return the owners for the result of that search.

For instance one can find all the owners of TypeScript files in a given repository by using `repo:^github\.com/sourcegraph/sourcegraph$ lang:TypeScript select:file.owners`.

### Find commits in given release for given owner

To find all commits between versions `5.0` and `5.1` made by `sourcegraph/own` team, the following query could be used:

`repo:^github\.com/sourcegraph/sourcegraph$@5.1:^5.0 type:commit file:has.owner(sourcegraph/own)`

Same query can be run for any owner (a person or a team).

## Further reading

In order to learn more please check out our references:

- [CODEOWNERS format](codeowners_format.md) - Guide to using the CODEOWNERS file format to define ownership
- [Configuration](configuration_reference.md) - Full list of ownership configuration options
