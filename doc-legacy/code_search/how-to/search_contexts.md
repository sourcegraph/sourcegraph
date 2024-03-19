# Search contexts

Search contexts help you search the code you care about on Sourcegraph. A search context represents a set of repositories at specific revisions on a Sourcegraph instance that will be targeted by search queries by default.

Every search on Sourcegraph uses a search context. Search contexts can be defined with the contexts selector shown in the search input, or entered directly in a search query.

## Available contexts

**Sourcegraph.com** supports a [set of predefined search contexts](https://sourcegraph.com/contexts?order=spec-asc&visible=17&owner=all) that include:

- The global context, `context:global`, which includes all repositories on Sourcegraph.com.
- Search contexts for various software communities like [CNCF](https://sourcegraph.com/search?q=context:CNCF), [crates.io](https://sourcegraph.com/search?q=context:crates.io), [JVM](https://sourcegraph.com/search?q=context:JVM), and more.  

If no search context is specified, `context:global` is used.

**Private Sourcegraph instances** support custom search contexts:

- Contexts owned by a user, such as `context:@username/context-name`, which can be private to the user or public to all users on the Sourcegraph instance.
- Contexts at the global level, such as `context:example-context`, which can be private to site admins or public to all users on the Sourcegraph instance.
- The global context, `context:global`, which includes all repositories on the Sourcegraph instance.

## Using search contexts

The search contexts selector is shown in the search input. All search queries will target the currently selected search context. 

To change the current search context, press the contexts selector. All of your search contexts will be shown in the search contexts dropdown. Select or use the filter to narrow down to a specific search context. Selecting a different context will immediately re-run your current search query using the currently selected search context.

Search contexts can also be used in the search query itself. Type `context:` to begin defining the context as part of the search query. When a context is defined in the search query itself, it overrides the context shown in the context selector.

You can also search across multiple contexts at once using the `OR` [boolean operator](../reference/queries.md#boolean-operators). For example:

`(context:release1 OR context:release2 OR context:release3) someTerribleBug` 

## Organizing search contexts

To organize your search contexts better, you can use a specific context as your default and star any number of contexts. This affects what context is selected when loading Sourcegraph and how the list of contexts is sorted.

### Default context

Any authenticated user can use a search context as their default. To set a default, go to the search context management page, open the "..." menu for a context, and click on "Use as default". If the user doesn't have a default, `global` will be used.

If a user ever loses access to their default search context (eg. the search context is made private), they will see a warning at the top of the search contexts dropdown menu list and `global` will be used. If a user's default search context is deleted, `global` will immediately be set as their default.

The default search context is always selected when loading the Sourcegraph webapp. The one exception is when opening a link to a search query that does not contain a `context:` filter, in which case the `global` context will be used.

### Starred contexts

Any authenticated user can star a search context. To star a context, click on the star icon in the search context management page. This will cause the context to appear near the top of their search contexts list. The `global` context cannot be starred.

### Sort order

The order of search contexts in the search results dropdown menu list and in the search context management page is always the following:

- The `global` context first
- The user's default context, if set
- All of the user's starred contexts
- Any remaining contexts available

## Creating search contexts

When search contexts are [enabled on your private Sourcegraph instance](../explanations/features.md#search-contexts), you can create your own search contexts.

A search context consists of a name, description, and a set of repositories at one or many revisions.

Contexts can be owned by a user, and can be private to the user or public to all users on the Sourcegraph instance.

Contexts can also be at the global instance level, and can be private to site admins or public to all users on the Sourcegraph instance.

### Creating search contexts from header navigation

- Go to **User menu > Search contexts** in the top navigation bar.
- Press the **+ Create search context** button.
- In the **Owner** field, choose whether you will own the context or if it will be global to the Sourcegraph instance. **Note**: At present, the owner of a search context cannot be changed after being created.
- In the **Context name** field, type in a short, semantic name for the context. The name can be 32 characters max, and contain alphanumeric and `.` `_` `/` `-` characters.
- Optionally, enter a **Description** for the context. Markdown is supported.
- Choose the **Visibility** of this context.
  - Public contexts are available to everyone on the Sourcegraph instance. Note that private repositories will only be visible to users that have permission to view the repository via the code host.
  - Private contexts can only be viewed by their owner, or in the case being globally owned, by site admins.
- In the **Repositories and revisions** configuration, define which repositories and revisions should be included in the search context. Press **Add repository** to quickly add a template to the configuration.
  - Define repositories with valid URLs.
  - Define revisions as strings in an array. To specify a default branch, use `"HEAD"`.

For example:
  
```json
    [
      {
        "repository": "github.com/sourcegraph/sourcegraph",
        "revisions": [
          "3.15"
        ]
      }, {
        "repository": "github.com/sourcegraph/src-cli",
        "revisions": [
          "3.11.2"
        ]
      }
    ]
```

- Press **Test configuration** to validate the repositories and revisions.
- Press **Create search context** to finish creating your search context.

You will be returned to the list of search contexts. Your new search context will appear in the search contexts selector in the search input, and can be [used immediately](#using-search-contexts).

## Query-based search contexts
As of release 3.36, search contexts can be defined with a restricted search query as an alternative to a specific list of repositories and revisions. Allowed filters are: `repo`, `rev`, `file`, `lang`, `case`, `fork`, and `visibility`. `OR` and `AND` expressions are also allowed.

> NOTE: Currently, repo built in predicates for example `repo:has.file`, `repo:has.content` etc, aren't currently supported in search contexts.

If you're an admin, to enable this feature for all users set `experimentalFeatures.searchContextsQuery` to `true` in your global settings (for regular users, just use the normal settings menu). You'll then see a "Create context" button from the search results page and a "Query" input field in the search contexts form. If you want revisions specified in these query based search contexts to be indexed, set `experimentalFeatures.search.index.query.contexts` to `true` in site configuration.

### Creating search contexts from search results
You can now create new search contexts right from the search results page. Once you've enabled query-based search contexts you'll see a Create context button above the search results.

## Managing search contexts with the API

Learn how to [manage search contexts with the GraphQL API](../../api/graphql/managing-search-contexts-with-api.md).
