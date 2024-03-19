# Search subexpressions


<video class="theme-dark-only" width="1760" height="1060" autoplay loop muted playsinline style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/search-subexpressions/case-dark.webm" type="video/webm">
  <source src="https://storage.googleapis.com/sourcegraph-assets/search-subexpressions/case-dark.mp4" type="video/mp4">
  <p>Search subexpression example with case</p>
</video>
<video class="theme-light-only" width="1760" height="1060" autoplay loop muted playsinline style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/search-subexpressions/case-light.webm" type="video/webm">
  <source src="https://storage.googleapis.com/sourcegraph-assets/search-subexpressions/case-light.mp4" type="video/mp4">
  <p>Search subexpression example with case</p>
</video>

Search subexpressions combine groups of
[filters](../reference/queries.md#keywords-all-searches) like `repo:` and
[operators](../reference/queries.md#boolean-operators) like `or`. Compared to [basic examples](examples.md), search subexpressions allow more sophisticated queries.
Here are examples of how they can help you:

→ [Noncompliant spelling where case-sensitivity differs depending on the word](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+%28%28Github+case:yes%29+or+%28organisation+case:no%29%29&patternType=literal).

```sgquery
 repo:sourcegraph ((Github case:yes) or (organisation case:no))
```

> Finds places to change the spelling of `Github` to `GitHub` (case-sensitivity matters) or
  change the spelling of `organisation` to `organization` (case-sensitivity does not matter).

<br/>

→ [Search for either a file name or file contents scoped to the same repository](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+-file:html+%28file:router+or+newRouter%29&patternType=literal).

```sgquery
repo:sourcegraph -file:html (file:router or newRouter)
```
> Finds both files containing the word `router` or file contents matching `newRouter` in the same repository, while excluding `html` files. Useful when exploring files or code that interact with a general term like `router`.

<br/>

→ [Scope file content searches to particular files or repositories](https://sourcegraph.com/search?q=+repo:sourcegraph+%28%28file:schema%5C.graphql+hover%28...%29%29+or+%28file:codeintel%5C.go+%28Line+or+Character%29%29%29&patternType=structural)


```sgquery
 repo:sourcegraph
 (
  (file:schema.graphql hover(...))
  or
  (file:codeintel.go (Line or Character))
 )
 patterntype:structural
```

> Combine matches of `hover(...)` in the `schema.graphql` file and matches of
 `Line` or `Character` in the `codeintel.go` file in the same repository. Useful
 for crafting queries that precisely match related fragments of a codebase to
 capture context and e.g., share with coworkers.

> <sup>Note: The query is formatted for readability, it is valid as a single line query.</sup>


<br/>


→ [Search across multiple repositories at multiple revisions](https://sourcegraph.com/search?q=%28repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24%40v3.22.0:v3.22.1+or+repo:%5Egithub%5C.com/sourcegraph/src-cli%24%403.22.0:3.22.1%29+file:CHANGELOG+campaigns&patternType=literal).

```sgquery
 (
   repo:^github\.com/sourcegraph/sourcegraph$@v3.22.0:v3.22.1
   or
   repo:^github\.com/sourcegraph/src-cli$@3.22.0:3.22.1
 )
 file:CHANGELOG campaigns
```

> Finds the word `campaigns` in `CHANGELOG` files for two repositories, `sourcegraph/sourcegraph  ` or `sourcegraph/src-cli`, at specific revisions. Useful for searching across a larger scope of repositories at particular revisions.

<br/>

## General tips for crafting queries with subexpressions

### Fully parenthesize subexpressions

It's best practice to parenthesize
queries to avoid confusion. For example, there are multiple ways to group the
query, which is not fully parenthesized:

```sgquery
repo:sourcegraph (Github case:yes) or (organisation case:no)
```

It could mean either of these:

```sgquery
(repo:sourcegraph (Github case:yes)) or (organisation case:no)
```

```sgquery
repo:sourcegraph ((Github case:yes) or (organisation case:no))
```

When parentheses are absent, we use the convention that operators divide
sequences of terms that should be grouped together. That is:

`file:main.c char c or (int i and int j)` generally means `(file:main.c char c) or (int i and int
j)`

This convention means we would pick the following meaning of the original query,
but it may not be what you intended:

```sgquery
(repo:sourcegraph (Github case:yes)) or (organisation case:no)
```

Fully parenthesizing subexpressions makes it clear what the intent is, so that
you can avoid relying on conventions that may not interpret the query the way
you intended.


### Subexpression expansion

If you're unsure whether a subexpression is valid, it may be useful to think in
terms of how a subexpression expands to multiple independent queries. Expansion
relies on a distributive property over `or`-expressions. For example:

```sgquery
repo:sourcegraph ((Github case:yes) or (organisation case:no))
```

is equivalent to expanding the query by putting `repo:sourcegraph` inside each
subexpression:

```sgquery
(repo:sourcegraph Github case:yes) or (repo:sourcegraph organisation case:no)
```

This query is valid because each side of the `or` operator is a valid query. On the other hand, the following query is _invalid_:

```sgquery
repo:sourcegraph case:yes (Github or (case:no organisation))
```

because after expanding it is equivalent to:

```sgquery
(repo:sourcegraph case:yes Github) or (repo:sourcegraph case:yes case:no organisation)
```

After expanding, the right-hand side contains both `case:yes` and `case:no`.
Since this subpart of the query is invalid, the original query is also invalid.
