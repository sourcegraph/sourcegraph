# Search examples

## Sourcegraph Search Examples on Github Pages

Check out the [Sourcegraph Search Examples](https://sourcegraph.github.io/sourcegraph-search-examples/) site for filterable search examples with links to results on sourcegraph.com.

Below are some additional examples that search repositories on [Sourcegraph.com](https://sourcegraph.com/search), our open source code search solution for GitHub and GitLab. You can copy and adapt the following search queries for use on your company’s private instance.

> See [**search query syntax**](../reference/queries.md) reference.

[Search through all the repositories in an organization](https://sourcegraph.com/search?q=context:global+r:hashicorp/+terraform&patternType=standard&sm=1&groupBy=repo)
```sgquery
context:global r:hashicorp/ terraform
```

[Search a subset of repositories in an organization by language](https://sourcegraph.com/search?q=context:global+r:hashicorp/vault*+lang:yaml+terraform&patternType=standard&sm=1&groupBy=repo)
```sgquery
context:global r:hashicorp/vault* lang:yaml terraform
```

[Search for one term or another in a specific repository](https://sourcegraph.com/search?q=context:global+r:hashicorp/vault%24+print%28+OR+log%28&patternType=standard&sm=1&groupBy=repo)
```sgquery
context:global r:hashicorp/vault$ print( OR log(
```

[Find private keys and GitHub access tokens checked in to code](https://sourcegraph.com/search?q=context:global+%28-----BEGIN+%5BA-Z+%5D*PRIVATE+KEY------%29%7C%28%28%22gh%7C%27gh%29%5Bpousr%5D_%5BA-Za-z0-9_%5D%7B16%2C%7D%29&patternType=regexp&case=yes)
```sgquery
(-----BEGIN [A-Z ]*PRIVATE KEY------)|(("gh|'gh)[pousr]_[A-Za-z0-9_]{16,}) patternType:regexp case:yes
```

[Recent security-related changes on all branches](https://sourcegraph.com/search?q=type:diff+repo:github%5C.com/kubernetes/kubernetes%24+repo:%40*refs/heads/+after:"5+days+ago"+%5Cb%28auth%5B%5Eo%5D%5B%5Er%5D%7Csecurity%5Cb%7Ccve%7Cpassword%7Csecure%7Cunsafe%7Cperms%7Cpermissions%29&patternType=regexp)<br/>

```sgquery
type:diff repo:@*refs/heads/ after:"5 days ago"
\b(auth[^o][^r]|security\b|cve|password|secure|unsafe|perms|permissions)
```

[Admitted hacks and TODOs in app code](https://sourcegraph.com/search?q=-file:%5C.%28json%7Cmd%7Ctxt%29%24+hack%7Ctodo%7Ckludge%7Cfixme&patternType=regexp)<br/>

```sgquery
-file:\.(json|md|txt)$ hack|todo|kludge|fixme
```

[Removal of TODOs in the repository commit log](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+type:diff+TODO+select:commit.diff.removed+&patternType=literal)

```sgquery
repo:^github\.com/sourcegraph/sourcegraph$ type:diff select:commit.diff.removed TODO
```

[New usages of a function](https://sourcegraph.com/search?q=repo:github%5C.com/sourcegraph/+type:diff+after:%221+week+ago%22+%5C.subscribe%5C%28+lang:typescript&patternType=regexp)<br/>

```sgquery
type:diff after:"1 week ago" \.subscribe\( lang:typescript
```

[Find multiple terms in the same file, like testing of HTTP components](https://sourcegraph.com/search?q=repo:github%5C.com/sourcegraph/sourcegraph%24+%28test+AND+http+AND+NewRequest%29+lang:go&patternType=regexp)

```sgquery
repo:github\.com/sourcegraph/sourcegraph$ (test AND http AND NewRequest) lang:go
```

[Recent quality related changes on all branches (customize for your linters)](https://sourcegraph.com/search?q=repo:github%5C.com/sourcegraph/+repo:%40*refs/heads/:%5Emaster+type:diff+after:"1+week+ago"+%28eslint-disable%29&patternType=regexp)<br/>

```sgquery
repo:@*refs/heads/:^master type:diff after:"1 week ago" (eslint-disable)
```

[Recent dependency changes](https://sourcegraph.com/search?q=repo:github%5C.com/sourcegraph/+file:package.json+type:diff+after:%221+week+ago%22)<br/>

```sgquery
file:package.json type:diff after:"1 week ago"
```

[Files that are Apache licensed](https://sourcegraph.com/search?q=licensed+to+the+apache+software+foundation+select:file&patternType=literal)<br/>

```sgquery
licensed to the apache software foundation select:file
```

[Only _repositories_ with recent dependency changes](https://sourcegraph.com/search?q=repo:github%5C.com/sourcegraph/+file:package.json+type:diff+after:%221+week+ago%22+select:repo&patternType=regexp)

```sgquery
file:package.json type:diff after:"1 week ago" select:repo
```

[Search changes in a files that contain a keyword](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+type:diff+file:contains.content%28%22golang%5C.org/x/sync/errgroup%22%29+.Go&patternType=literal)

```sgquery
repo:^github\.com/sourcegraph/sourcegraph$ type:diff file:contains.content("golang\.org/x/sync/errgroup") .Go
```

## When to use regex search mode

Sourcegraph's default literal search mode is line-based and will not match across lines, so regex can be useful when you wish to do so:

[Matching multiple text strings in a file](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/Parsely/pykafka%24+Not+leader+for+partition&patternType=regexp)<br/>

```sgquery
repo:^github\.com/Parsely/pykafka$ Not leader for partition
```

Regex searches are also useful when searching boundaries that are not delimited by code structures:

[Finding css classes with word boundary regex](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+%5Cbbtn-secondary%5Cb&patternType=regexp) <br />
```sgquery
repo:^github\.com/sourcegraph/sourcegraph$ \bbtn-secondary\b
```

## When to use structural search mode

Use structural search when you want to match code boundaries such as () or {}:

[Finding try catch statements with varying content](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+try+%7B+:%5Bmatched_statements%5D+%7D+catch+%7B+:%5Bmatched_catch%5D+%7D&patternType=structural)<br/>
```sgquery
repo:^github\.com/sourcegraph/sourcegraph$
try { :[matched_statements] } catch { :[matched_catch] }
```
