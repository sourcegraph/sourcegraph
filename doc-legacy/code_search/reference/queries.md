
# Search query syntax

<!-- Search syntax styling overrides -->
<style>
.markdown-body tr td:nth-child(1) {
  min-width:175px;
}

.markdown-body tr td:nth-child(3) {
  min-width: 250px;
}
.markdown-body tr td:nth-child(3) code {
  word-break: break-all;
}

body.theme-dark img.toggle {
    filter: invert(100%);
}

img.toggle {
    width: 20px;
    height: 20px;
}

.toggle-container {
  border: 1px solid;
  border-radius: 3px;
  display: inline-flex;
  vertical-align: bottom;
}

</style>

This page describes search pattern syntax and keywords available for code search. See the complementary [language reference](language.md) for a visual breakdown. A typical search pattern describes content or filenames to find across all repositories. At the most basic level, a search pattern can simply be a word like `hello`. See our [search patterns](#search-pattern-syntax) documentation for detailed usage. Queries can also include keywords. For example, a typical search query will include a `repo:` keyword that filters search results for a specific repository. See our [keywords](#keywords-all-searches) documentation for more examples.

<div>
  <a class="cloud-cta" href="https://sourcegraph.com/get-started?t=enterprise" target="_blank" rel="noopener noreferrer">
    <div class="cloud-cta-copy">
      <h2>Get Sourcegraph on your code.</h2>
      <h3>A single-tenant instance managed by Sourcegraph.</h3>
      <p>Sign up for a 30 day trial for your team.</p>
    </div>
    <div class="cloud-cta-btn-container">
      <div class="visual-btn">Get free trial now</div>
    </div>
  </a>
</div>

## Search pattern syntax

This section documents the available search pattern syntax and interpretation in Sourcegraph. A search pattern is _required_ to match file content. A search pattern is _optional_ and may be omitted when searching for [commits](#keywords-diff-and-commit-searches-only), [filenames](#filename-search), or [repository names](#repository-name-search).

### Standard search (default)

Standard search matches literal patterns exactly, including puncutation like quotes. Specify regular expressions inside `/.../`.

| Search pattern syntax                                                             | Description                                                                                                                                                                                                                           |
| ---                                                                               | ---                                                                                                                                                                                                                                   |
| [`foo bar`](https://sourcegraph.com/search?q=foo+bar&patternType=standard)         | Match the string `foo bar` exactly. No need to add quotes, we match `foo` followed by `bar`, with exactly one space between the terms. |
| [`"foo bar"`](https://sourcegraph.com/search?q=%22foo+bar%22&patternType=standard) | Match the string `"foo bar"` exactly. The quotes are matched literally.                                                                                                                                                                       |
| [`/foo.*bar/`](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+/foo.*bar/&patternType=standard) | Match the **regular expression** `foo.*bar`.                      We support [RE2 syntax](https://golang.org/s/re2syntax).                                                        |
| [`foo` `AND` `bar`](https://sourcegraph.com/search?q=context:global+foo+AND+bar&patternType=standard) | Match documents containing both `foo` _and_ `bar` anywhere in the document. |

Matching is case-_insensitive_ (toggle the <span class="toggle-container"><img class="toggle" src=../img/case.png alt="case"></span> button to change).

<details>
  <summary><strong>Dedicated regular expression search with <span class="toggle-container"><img class="toggle" src=../img/regex.png alt="regular expression"></span></strong></summary>

Clicking the <span class="toggle-container"><img class="toggle" src=../img/regex.png alt="regular expression"></span> toggle interprets _all_
search patterns as regular expressions.

**Note.** You can achieve the same regular expression searches in the [default Standard mode](#standard-search-default) by enclosing patterns in `/.../`, so
only use this mode if you find it more convenient to write out regular
expressions without enclosing them in `/.../`. In this mode spaces between patterns mean "match anything". Patterns inside quotes mean "match
exactly".

| Search pattern syntax | Description |
| --- | --- |
| [`foo bar`](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+foo+bar&patternType=regexp) | Search for the regexp `foo(.*?)bar`. Spaces between non-whitespace strings is converted to `.*?` to create a fuzzy search. |
| [`foo\ bar`](https://sourcegraph.com/search?q=foo%5C+bar&patternType=regexp) or<br/>[`/foo bar/`](https://sourcegraph.com/search?q=/foo+bar/&patternType=regexp) | Search for the regexp `foo bar`. The `\` escapes the space and treats the space as part of the pattern. Using the delimiter syntax `/.../` avoids the need for escaping spaces. |
| [`foo\nbar`](https://sourcegraph.com/search?q=foo%5Cnbar&patternType=regexp) | Perform a multiline regexp search. `\n` is interpreted as a newline. |
| [`"foo bar"`](https://sourcegraph.com/search?q=%27foo+bar%27&patternType=regexp) | Match the _string literal_ `foo bar`. Quoting strings in this mode are interpreted exactly, except that special characters like `"` and `\` may be escaped, and whitespace escape sequences like `\n` are interpreted normally. |

As in Standard search, we support [RE2 syntax](https://golang.org/s/re2syntax). Matching is case-_insensitive_ (toggle the <span class="toggle-container"><img class="toggle" src=../img/case.png alt="case"></span> button to change).
</details>

### Structural search

Click the <span class="toggle-container"><img class="toggle" src=../img/brackets.png alt="square brackets"></span> toggle to activate structural search. Structural search is a way to match richer syntactic structures like multiline code blocks. See the dedicated [usage documentation](structural.md) for more details. Here is a  brief overview of valid syntax:

| Search pattern syntax | Description |
| --- | --- |
| [`New(ctx, ...)`](https://sourcegraph.com/search?q=repo:github.com/sourcegraph/sourcegraph++New%28ctx%2C+...%29+lang:go&patternType=structural) | Match call-like syntax with an identifier `New` having two or more arguments, and the first argument matches `ctx`. Make the search language-aware by adding a `lang:` [keyword](#keywords-all-searches). |

## Keywords (all searches)

The following keywords can be used on all searches (using [RE2 syntax](https://golang.org/s/re2syntax) any place a regex is accepted):

| Keyword | Description | Examples |
| --- | --- | --- |
| **repo:regexp-pattern** <br> **repo:regexp-pattern@rev** <br> **repo:regexp-pattern rev:rev**<br>_alias: r_  | Only include results from repositories whose path matches the regexp-pattern. A repository's path is a string such as _github.com/myteam/abc_ or _code.example.com/xyz_ that depends on your organization's repository host. If the regexp ends in [`@rev`](#repository-revisions), that revision is searched instead of the default branch (usually `master`).  `repo:regexp-pattern@rev` is equivalent to `repo:regexp-pattern rev:rev`.| [`repo:gorilla/mux testroute`](https://sourcegraph.com/search?q=repo:gorilla/mux+testroute) <br/> [`repo:^github\.com/sourcegraph/sourcegraph$@v3.14.0 mux`](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24%40v3.14.0+mux&patternType=literal) |
| **-repo:regexp-pattern** <br> _alias: -r_ | Exclude results from repositories whose path matches the regexp. | `repo:alice/ -repo:old-repo` |
|**rev:revision-pattern** <br> _alias: revision_| Search a revision instead of the default branch. `rev:` can only be used in conjunction with `repo:` and may not be used more than once. See our [revision syntax](#repository-revisions) documentation to learn more.| [`repo:sourcegraph/sourcegraph rev:v3.14.0 mux`](https://sourcegraph.com/search?q=repo:sourcegraph/sourcegraph+rev:v3.14.0+mux&patternType=literal) |
| **file:regexp-pattern** <br> _alias: f_ | Only include results in files whose full path matches the regexp. | [`file:\.js$ httptest`](https://sourcegraph.com/search?q=file:%5C.js%24+httptest) <br> [`file:internal/ httptest`](https://sourcegraph.com/search?q=file:internal/+httptest) |
| **-file:regexp-pattern** <br> _alias: -f_ | Exclude results from files whose full path matches the regexp. | [`file:\.js$ -file:test http`](https://sourcegraph.com/search?q=file:%5C.js%24+-file:test+http) |
| **content:"pattern"** | Set the search pattern with a dedicated parameter. Useful when searching literally for a string that may conflict with the [search pattern syntax](#search-pattern-syntax). In between the quotes, the `\` character will need to be escaped (`\\` to evaluate for `\`). | [`repo:sourcegraph content:"repo:sourcegraph"`](https://sourcegraph.com/search?q=repo:sourcegraph+content:"repo:sourcegraph"&patternType=literal) |
| **-content:"pattern"** | Exclude results from files whose content matches the pattern. Not supported for structural search. | [`file:Dockerfile alpine -content:alpine:latest`](https://sourcegraph.com/search?q=file:Dockerfile+alpine+-content:alpine:latest&patternType=literal) |
| **select:_result-type_** <br> **select:repo** <br> **select:commit.diff.added** <br> **select:commit.diff.removed** <br> **select:file** <br> **select:content** <br> **select:symbol._symbol-type_** <br> **select:file.owners** _(Experimental)_ | Shows only query results for a given type. For example, `select:repo` displays only distinct repository paths from search results, and `select:commit.diff.added` shows only added code matching the search. See [language definition](language.md#select) for full list of possible values. | [`fmt.Errorf select:repo`](https://sourcegraph.com/search?q=fmt.Errorf+select:repo&patternType=literal) |
| **language:language-name** <br> _alias: lang, l_ | Only include results from files in the specified programming language. | [`language:typescript encoding`](https://sourcegraph.com/search?q=language:typescript+encoding) |
| **-language:language-name** <br> _alias: -lang, -l_ | Exclude results from files in the specified programming language. | [`-language:typescript encoding`](https://sourcegraph.com/search?q=-language:typescript+encoding) |
| **type:symbol** | Perform a symbol search. | [`type:symbol path`](https://sourcegraph.com/search?q=type:symbol+path)  ||
| **case:yes**  | Perform a case sensitive query. Without this, everything is matched case insensitively. | [`OPEN_FILE case:yes`](https://sourcegraph.com/search?q=OPEN_FILE+case:yes) |
| **fork:yes, fork:only** | Include results from repository forks or filter results to only repository forks. Results in repository forks are excluded by default. | [`fork:yes repo:sourcegraph`](https://sourcegraph.com/search?q=fork:yes+repo:sourcegraph) |
| **archived:yes, archived:only** | The yes option, includes archived repositories. The only option, filters results to only archived repositories. Results in archived repositories are excluded by default. | [`repo:sourcegraph/ archived:only`](https://sourcegraph.com/search?q=repo:%5Egithub.com/sourcegraph/+archived:only) |
| **repo:has.meta(...)** | **Experimental** Conditionally search inside repositories only if they are associated with a specified metadata: <br> 1. key-value pair, or<br> 2. key with any value, or <br>3. key with no value <br>See [built-in predicates](language.md#built-in-repo-predicate) for more. | 1. `repo:has.meta(owning-team:security)` <br> 2. `repo:has.meta(owning-team)` <br> 3. `repo:has.meta(archived:)` |
| **repo:has.path(...)** | Conditionally search inside repositories only if they contain a file path matching the regular expression. See [built-in predicates](language.md#built-in-repo-predicate) for more. | [`repo:has.path(\.py) file:Dockerfile pip`](https://sourcegraph.com/search?q=context:global+repo:has.path%28%5C.py%29+file:Dockerfile+pip&patternType=lucky) |
| **repo:has.topic(...)** | Search only in repos repositories if they have the given GitHub or GitLab topic. See [built-in predicates](language.md#built-in-repo-predicate) for more. | [`repo:has.topic(code-search) rank`](https://sourcegraph.com/search?q=context:global+repo:sourcegraph/sourcegraph%24+rank&patternType=standard&sm=1&groupBy=repo) |
| **repo:has.commit.after(...)** | Filter out stale repositories that don't contain commits past the specified time frame. See [built-in predicates](language.md#built-in-repo-predicate) for more. | [`repo:has.commit.after(yesterday)`](https://sourcegraph.com/search?q=context:global+repo:.*sourcegraph.*+repo:has.commit.after%28yesterday%29&patternType=lucky) <br> [`repo:has.commit.after(june 25 2017)`](https://sourcegraph.com/search?q=context:global+repo:.*sourcegraph.*+repo:has.commit.after%28june+25+2017%29&patternType=lucky) |
| **file:has.content(...)** | Conditionally search files only if they contain contents that match the provided regex pattern. See [built-in predicates](language.md#built-in-repo-predicate) for more. | [`file:has.content(Copyright) Sourcegraph`](https://sourcegraph.com/search?q=context:global+file:has.content%28Copyright%29+Sourcegraph&patternType=lucky) |
| **file:has.owners(...)** | **Beta** Conditionally search files only if they are owned by the given owner. Empty means _any owner_. See [code ownership documentation](../../own/index.md) for more. | [`file:has.owner(alice@sourcegraph.com) Sourcegraph`](https://sourcegraph.com/search?q=context:global+file:has.owner%28alice@sourcegraph.com%29+Sourcegraph&patternType=lucky) |
| **file:has.contributor(...)** | Conditionally search files only if a file contributor's name or email matches the provided regex pattern. See [built-in predicates](language.md#built-in-file-predicate) for more. | [`file:has.contributor(alice@sourcegraph.com) Sourcegraph`](https://sourcegraph.com/search?q=context:global+file:has.owner%28alice@sourcegraph.com%29+Sourcegraph&patternType=lucky) |
| **count:_N_,<br> count:all**<br/> | Retrieve <em>N</em> results. By default, Sourcegraph stops searching early and returns if it finds a full page of results. This is desirable for most interactive searches. To wait for all results, use **count:all**. | [`count:1000 function`](https://sourcegraph.com/search?q=count:1000+repo:sourcegraph/sourcegraph$+function) <br> [`count:all err`](https://sourcegraph.com/search?q=repo:github.com/sourcegraph/sourcegraph+err+count:all&patternType=literal) |
| **timeout:_go-duration-value_**<br/> | Customizes the timeout for searches. The value of the parameter is a string that can be parsed by the [Go time package's `ParseDuration`](https://golang.org/pkg/time/#ParseDuration) (e.g. 10s, 100ms). By default, the timeout is set to 10 seconds, and the search will optimize for returning results as soon as possible. The timeout value cannot be set longer than 1 minute. When provided, the search is given the full timeout to complete. | [`repo:^github.com/sourcegraph timeout:15s func count:10000`](https://sourcegraph.com/search?q=repo:%5Egithub.com/sourcegraph/+timeout:15s+func+count:10000) |
| **patterntype:literal, patterntype:regexp, patterntype:structural**  | Configure your query to be interpreted literally, as a regular expression, or a [structural search pattern](structural.md). Note: this keyword is available as an accessibility option in addition to the visual toggles. | [`test. patternType:literal`](https://sourcegraph.com/search?q=test.+patternType:literal)<br/>[`(open\|close)file patternType:regexp`](https://sourcegraph.com/search?q=%28open%7Cclose%29file&patternType=regexp) |
| **visibility:any, visibility:public, visibility:private** | Filter results to only public or private repositories. The default is to include both private and public repositories. | [`type:repo visibility:public`](https://sourcegraph.com/search?q=type:repo+visibility:public) |

Multiple or combined **repo:** and **file:** keywords are intersected. For example, `repo:foo repo:bar` limits your search to repositories whose path contains **both** _foo_ and _bar_ (such as _github.com/alice/foobar_). To include results from repositories whose path contains **either** _foo_ or _bar_, use `repo:foo|bar`.

## Boolean operators

Use boolean operators to create more expressive searches.

| Operator | Example |
| --- | --- |
| `and`, `AND` | [`conf.Get( and log15.Error(`](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+conf.Get%28+and+log15.Error%28&patternType=regexp), [`conf.Get( and log15.Error( and after`](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+conf.Get%28+and+log15.Error%28+and+after&patternType=regexp) |

Returns results for files containing matches on the left _and_ right side of the `and` (set intersection).

| Operator | Example |
| --- | --- |
| `or`, `OR` | [`conf.Get( or log15.Error(`](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+conf.Get%28+or+log15.Error%28&patternType=regexp), [<code>conf.Get( or log15.Error( or after   </code>](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+conf.Get%28+or+log15.Error%28+or+after&patternType=regexp)|

Returns file content matching either on the left or right side, or both (set union). The number of results reports the number of matches of both strings. Note the regex or operator `|` may not work as expected with certain operators for example `file:(internal/repos)|(internal/gitserver)`, to receive the expected results use [subexpressions](../tutorials/search_subexpressions.md), `(file:internal/repos or file:internal/gitserver)`

| Operator | Example |
| --- | --- |
| `not`, `NOT` | [`lang:go not file:main.go panic`](https://sourcegraph.com/search?q=lang:go+not+file:main.go+panic&patternType=literal), [`panic NOT ever`](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+panic+not+ever&patternType=literal)

`NOT` can be used in place of `-` to negate keywords, such as `file`, `content`, `lang`, `repohasfile`, and `repo`. For
search patterns, `NOT` excludes documents that contain the term after `NOT`. For readability, you can also include the
`AND` operator before a `NOT` (i.e. `panic NOT ever` is equivalent to `panic AND NOT ever`).

> If you want to actually search for reserved keywords like `OR` in your code use `content` like this: <br>
> `content:"query with OR"`.

### Operator precedence and groups

Operators may be combined. `and` expressions have higher precedence (bind tighter) than `or` expressions so that `a and b or c and d` means `(a and b) or (c and d)`.

Expressions may be grouped with parentheses to change the default precedence and meaning. For example: `a and (b or c) and d`.

### Filter scope

When parentheses are absent, we use the convention that operators divide
sequences of terms that should be grouped together. That is:

`file:main.c char c or (int i and int j)` generally means `(file:main.c char c) or (int i and int
j)`

To apply the scope of the `file` filter to the entire subexpression, fully group the subexpression:

`file:main.c (char c or (int i and int j))`.

Browse the [search subexpressions examples](../tutorials/search_subexpressions.md) to
learn more about use cases.

## Keywords (diff and commit searches only)

The following keywords are only used for **commit diff** and **commit message** searches, which show changes over time:

| Keyword  | Description | Examples |
| --- | --- | --- |
| **repo:regexp-pattern@rev** | Specifies which Git revisions to search for commits. See our [repository revisions](#repository-revisions) documentation to learn more about the revision syntax. | [`repo:vscode@*refs/heads/:^refs/heads/master type:diff task`](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/Microsoft/vscode%24%40*refs/heads/:%5Erefs/heads/master+type:diff+after:%221+month+ago%22+task#1) (unmerged commit diffs containing `task`) |
| **type:diff** <br> **type:commit**  | Specifies the type of search. By default, searches are executed on all code at a given point in time (a branch or a commit). Specify the `type:` if you want to search over changes to code or commit messages instead (diffs or commits).  | [`type:diff func`](https://sourcegraph.com/search?q=type:diff+func+repo:sourcegraph/sourcegraph$) <br> [`type:commit test`](https://sourcegraph.com/search?q=type:commit+test+repo:sourcegraph/sourcegraph$) |
| **author:name** | Only include results from diffs or commits authored by the user. Regexps are supported. Note that they match the whole author string of the form `Full Name <user@example.com>`, so to include only authors from a specific domain, use `author:example.com>$`. You can also use `author:@SourcegraphUserName` to search on a Sourcegraph user's list of verified emails.<br><br> You can also search by `committer:git-email`. _Note: there is a committer only when they are a different user than the author._ | [`type:diff author:nick`](https://sourcegraph.com/search?q=repo:sourcegraph/sourcegraph$+type:diff+author:nick) |
| **-author:name** | Exclude results from diffs or commits authored by the user. Regexps are supported. Note that they match the whole author string of the form `Full Name <user@example.com>`, so to exclude authors from a specific domain, use `author:example.com>$`. You can also use `author:@SourcegraphUserName` to search on a Sourcegraph user's list of verified emails.<br><br> You can also search by `committer:git-email`. _Note: there is a committer only when they are a different user than the author._ | [`type:diff author:nick`](https://sourcegraph.com/search?q=repo:sourcegraph/sourcegraph$+type:diff+author:nick) |
| **before:"string specifying time frame"** | Only include results from diffs or commits which have a commit date before the specified time frame | [`before:"last thursday"`](https://sourcegraph.com/search?q=repo:sourcegraph/sourcegraph$+type:diff+author:nick+before:%22last+thursday%22) <br> [`before:"november 1 2019"`](https://sourcegraph.com/search?q=repo:sourcegraph/sourcegraph$+type:diff+author:nick+before:%22november+1+2019%22) |
| **after:"string specifying time frame"**  | Only include results from diffs or commits which have a commit date after the specified time frame| [`after:"6 weeks ago"`](https://sourcegraph.com/search?q=repo:sourcegraph/sourcegraph$+type:diff+author:nick+after:%226+weeks+ago%22) <br> [`after:"november 1 2019"`](https://sourcegraph.com/search?q=repo:sourcegraph/sourcegraph$+type:diff+author:nick+after:%22november+1+2019%22) |
| **message:"any string"** | Only include results from diffs or commits which have commit messages containing the string | [`type:commit message:"testing"`](https://sourcegraph.com/search?q=type:commit+repo:sourcegraph/sourcegraph$+message:%22testing%22) <br> [`type:diff message:"testing"`](https://sourcegraph.com/search?q=type:diff+repo:sourcegraph/sourcegraph$+message:%22testing%22) |
| **-message:"any string"** | Exclude results from diffs or commits which have commit messages containing the string | [`type:commit message:"testing"`](https://sourcegraph.com/search?q=type:commit+repo:sourcegraph/sourcegraph$+message:%22testing%22) <br> [`type:diff message:"testing"`](https://sourcegraph.com/search?q=type:diff+repo:sourcegraph/sourcegraph$+message:%22testing%22) |

## Repository search

### Repository revisions

To search revisions other than the default branch, specify the revisions by either appending them to the
`repo:` filter  or by listing them separately with the `rev:` filter. This means:

`repo:github.com/myteam/abc@<revisions>`

is equivalent to

`repo:github.com/myteam/abc rev:<revisions>`.

 The `<revisions>` part refers to repository
 revisions (branches, commit hashes, and tags) and may take on the following forms:

(All examples apply to `@` as well as `rev:`)

- `@branch` - a branch name
- `@1735d48` - a commit hash
- `@3.15` - a tag

You can separate revisions by a colon to search multiple revisions at the same time, `@branch:1735d48:3.15`.

Per default, we match revisions to tags, branches, and commits. You can limit the search to branches or tags by adding
the prefix `refs/tags` or `refs/heads`. For example `@refs/tags/3.18` will search the commit tagged
with `3.18`, but not a branch called `3.18` and vice versa for `@refs/heads/3.18`.

**Glob patterns** allow you to search over a range of branches or tags. Prepend `*` to mark a revision
as glob pattern and add the glob-pattern after it like this `repo:<repo>@*<glob-pattern>`. For example:

 - [`@*refs/heads/*`](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/docker/machine%24%40*refs/heads/*+middleware&patternType=literal) - search across all branches
 - [`@*refs/tags/*`](https://sourcegraph.com/search?q=repo:github.com/docker/machine%24%40*refs/tags/*+server&patternType=literal) - search across all tags

We automatically add a trailing `/*` if it is missing from the glob pattern.

You can negate a glob pattern by prepending `*!`, for example:

- [`@*refs/heads/*:*!refs/heads/release* type:commit `](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/kubernetes/kubernetes%24%40*refs/heads/*:*%21refs/heads/release*+type:commit+&patternType=literal) - search commits on all branches except on those that start with "release"
- [`@*refs/tags/v3.*:*!refs/tags/v3.*-* context`](https://sourcegraph.com/search?q=repo:%5Egithub.com/sourcegraph/sourcegraph%24%40*refs/tags/v3.*:*%21refs/tags/v3.*-*+context&patternType=literal) - search all versions starting with `3.` except release candidates, alpha and beta versions.

### Repository names

A query with only `repo:` filters returns a list of repositories with matching names.

Example: [`repo:docker repo:registry`](https://sourcegraph.com/search?q=repo:docker+repo:registry) matches repositories with names that contain _both_ `docker` _and_ `registry` substrings.

Example: [`repo:docker OR repo:registry`](https://sourcegraph.com/search?q=repo:docker+OR+repo:registry&patternType=literal) matches repositories with names that contain _either_ `docker` _or_ `registry` substrings.

### Commit and Diff searches
Commit and diff searches act on sets of commits. A set is defined by a revision (branch, commit hash, or tag), and it
contains all commits reachable from that revision. A commit is reachable from another commit if it can be
reached by following the pointers to parent commits.

For commit and diff searches it is possible to exclude a set of commits by prepending a caret `^`. The caret acts as a set
difference. For example, `repo:github.com/myteam/abc@main:^3.15 type:commit` will show all commits in `main`
minus the commits reachable from the commit tagged with `3.15`.

## Filename search

A query with `type:path` restricts terms to matching filenames only (not file contents).

Example: [`type:path repo:/docker/ registry`](https://sourcegraph.com/search?q=type:path+repo:/docker/+registry)

## Content search

A query with `type:file` restricts terms to matching file contents only (not filenames).

Example: [`type:file repo:^github\.com/sourcegraph/about$ website`](https://sourcegraph.com/search?q=type:file+repo:%5Egithub%5C.com/sourcegraph/about%24+website&patternType=literal)
