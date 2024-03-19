# Custom repository metadata

Repositories tracked by Sourcegraph can be associated with user-provided key-value pairs. Once this metadata is added, it can be used to filter searches to the subset of matching repositories.

Metadata can be added either as key-value pairs or as tags. Key-value pairs can be searched with the filter `repo:has.meta(mykey:myvalue)`. `repo:has.meta(mykey)` can be used to search over repositories with a given key irrespective of its value. Tags are just key-value pairs with a `null` value and can be searched with the filter `repo:has.meta(mytag:)`.

## Examples
### Repository owners

One way this feature might be used is to add the owning team of each repository as a key-value pair. For example, the repository `github.com/sourcegraph/security-onboarding` repository is owned by the security team, so we could add `owning-team:security` as a key-value pair on that repository. 

Once those key-value pairs are added, they can be used to filter searches to only the code that is owned by a specific team with a search like `repo:has.meta(owning-team:security) account creation`.

### Maintenance status

Another way this could be used is to associate repos with a maintenance status. Do you have a library that is commonly used but is unmaintained, deprecated, or replaced by a better solution? You can associate these custom statuses with repository metadata. After adding this info to your repositories, you can do things like `-repo:has.meta(status:deprecated)` to exclude all results from deprecated repos.

## Adding metadata

There are three ways to add metadata to a repository: via web UI, Sourcegraph's GraphQL API, and the [`src-cli` command line tool](https://github.com/sourcegraph/src-cli). 

**NOTE:** That user needs to have `Repository metadata / Write` [RBAC permission](../access_control/index.md) to be able to edit repo metadata.

### Limitations

- There are no scale limits in terms of number of pairs per repo, or number of pairs globally.
- The size of a field is unbounded, but practically it's better to keep it small for performance reasons.
- There are no limits on special characters in the key-value pairs, but in practice we recommend not using special characters because the search query language doesn’t have full support for escaping arbitrary sequences, in particular `:`, `(` and`)`.

### Web UI
Go to repository root page, hover over "Metadata" section and click on a gear icon, it will open a repository metadata editing page.

### GraphQL

Metadata can be added with the `addRepoMetadata` mutation, updated with the `updateRepoMetadata` mutation, and deleted with the `deleteRepoMetadata` mutation. You will need the GraphQL ID for the repository being targeted.

```graphql
mutation AddSecurityOwner($repoID: ID!) {
  addRepoMetadata(repo: $repoID, key: "owning-team", value: "security") {
    alwaysNil
  }
}

mutation UpdateSecurityOwner($repoID: ID!) {
  updateRepoMetadata(repo: $repoID, key: "owning-team", value: "security++") {
    alwaysNil
  }
}

mutation DeleteSecurityOwner($repoID: ID!) {
  deleteRepoMetadata(repo: $repoID, key: "owning-team") {
    alwaysNil
  }
}
```

### src-cli

Metadata can be added using `src repos add-metadata`, updated using `src repos update-metadata`, and deleted using `src repos delete-metadata`. You will need the GraphQL ID for the repository being targeted.

```text
$ src repos add-metadata -repo-name=repoName -key=owning-team -value=security
Key-value pair 'owning-team:security' created.

$ src repos update-metadata -repo-name=repoName -key=owning-team -value=security++
Value of key 'owning-team' updated to 'security++'

$ src repos delete-metadata -repo-name=repoName -key=owning-team
Key-value pair with key 'owning-team' deleted.
```
