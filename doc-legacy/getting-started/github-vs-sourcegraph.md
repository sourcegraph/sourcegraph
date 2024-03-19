
# GitHub code search vs. Sourcegraph

GitHub code search is the next iteration of GitHub’s native code search and navigation functionality that can be used to search code stored in GitHub. It’s currently in beta preview. 

Sourcegraph is a code intelligence platform that makes codebases intelligible by semantically indexing and analyzing all of an organization’s code, providing developers and engineering leaders with a complete understanding of their codebase. In addition to universal code search across every code host, Sourcegraph has features to help developers find code, understand and answer questions about code, and fix code faster.

## Which is best for you?

**Who should use GitHub code search?**

If you’re brand new to code search and you simply want to try it out, start with GitHub code search across your own code. 

If you have a small codebase hosted exclusively in GitHub, GitHub's native code search will likely be sufficient as you get started with code search.

**Who should use Sourcegraph?**

As your codebase grows in complexity, the value of code search quickly increases. Sourcegraph may be a good fit for your team if:


* You have a large number of repositories or a large and complex monorepo
* You host code across multiple code hosts (or you don’t have any code in GitHub)

If you frequently rely on your editor’s “go to definition” and “find references” features, you’ll also be able to take advantage of Sourcegraph's precise code navigation. Only Sourcegraph's offering features IDE-level accuracy and works across repositories.

If you're brand new to code search and you want to try it, visit [Sourcegraph.com](https://sourcegraph.com/search) to search across open source repositories. 

For a high-level overview of how Sourcegraph compares to GitHub code search, see this [document](https://storage.googleapis.com/sourcegraph-assets/docs/PDFs/Sourcegraph%20vs.%20GitHub%20code%20search%20chart.pdf).  

## Searching code


### Code host integrations

GitHub code search can only be used to search across code that is stored in GitHub. Organizations with code stored in multiple code hosts cannot use GitHub code search to search across their entire codebase. 

Sourcegraph integrates with multiple code hosts to provide universal code search across all of your organization’s code, no matter where it’s stored. Sourcegraph has out-of-the-box [integrations](https://docs.sourcegraph.com/integration) with: 



* GitHub
* GitLab
* Bitbucket Cloud and Bitbucket Server / Bitbucket Data Center
* Perforce

You can also integrate Sourcegraph with other Git-based code hosts using [these](https://docs.sourcegraph.com/admin/external_service/other) instructions or use the [Sourcegraph CLI (src)](https://docs.sourcegraph.com/admin/external_service/src_serve_git) to load local repositories without a code host into Sourcegraph. Sourcegraph is continuously adding [Tier 1](https://docs.sourcegraph.com/admin/external_service) support for additional code hosts. 


<table>
  <tr>
   <td>
   </td>
   <td><strong>GitHub</strong>
   </td>
   <td><strong>Sourcegraph</strong>
   </td>
  </tr>
  <tr>
   <td><strong>GitHub</strong>
   </td>
   <td>✓
   </td>
   <td>✓
   </td>
  </tr>
  <tr>
   <td><strong>GitLab</strong>
   </td>
   <td>✗
   </td>
   <td>✓
   </td>
  </tr>
  <tr>
   <td><strong>Bitbucket Cloud</strong>
   </td>
   <td>✗
   </td>
   <td>✓
   </td>
  </tr>
  <tr>
   <td><strong>Bitbucket Server / Bitbucket Data Center</strong>
   </td>
   <td>✗
   </td>
   <td>✓
   </td>
  </tr>
  <tr>
   <td><strong>Perforce</strong>
   </td>
   <td>✗
   </td>
   <td>✓
   </td>
  </tr>
  <tr>
   <td><strong>Any Git-based code host</strong>
   </td>
   <td>✗
   </td>
   <td>✓
   </td>
  </tr>
</table>



### Searching repositories, branches, and forks

GitHub allows you to search indexed code, but not all code is indexed. GitHub’s [current limitations](https://cs.github.com/about/faq#indexed-content) on indexed code are:



* Files over 350 KiB and empty files are excluded
* Only UTF-8 encoded files are included
* All code inside very large repositories may not be indexed
* Vendored and generated code is excluded (as determined by [Enry](https://github.com/go-enry/go-enry))

With GitHub, only the default **branch** is searchable (though GitHub is planning to support branch search in the future). 

**Forks** are included in the index but are subject to the same limitations as other repositories, so not all forks are indexed. You may need to include the fork filter to retrieve results for the fork repos, but an admin can adjust global settings to include forks in search query results automatically. 

GitHub code search supports searching across issues, pull requests, and discussions. In addition to searching your private code, GitHub has indexed over 7 million public GitHub repositories which are also searchable.

Sourcegraph allows you to search indexed and [unindexed](https://docs.sourcegraph.com/code_search/how-to/exhaustive#non-indexed-backends) code. Sourcegraph’s [current limitations](https://docs.sourcegraph.com/admin/search) on indexed code are: 



* Files larger than 1 MB are excluded unless you explicitly specify them in [search.largeFiles](https://docs.sourcegraph.com/admin/config/site_config#search-largeFile) to be indexed and searched regardless of size 
* Binary files are excluded 
* Files other than UTF-8 are excluded 

With Sourcegraph, typically, the latest code on the default **branch** of each repository is indexed (usually the master or main), but Sourcegraph can also index other non-default branches, such as long-running branches like release branches. If you’re searching outside of indexed branches, you can use unindexed search. You should expect slightly slower results when searching unindexed code.  

In addition to searching your organization’s private code, you can use Sourcegraph.com to search across 2.8 million public repositories from multiple code hosts. 


<table>
  <tr>
   <td>
   </td>
   <td><strong>GitHub</strong>
   </td>
   <td><strong>Sourcegraph</strong>
   </td>
  </tr>
  <tr>
   <td><strong>Search across all repositories and forks</strong>
   </td>
   <td>✓ with limitations 
   </td>
   <td>✓ with limitations 
   </td>
  </tr>
  <tr>
   <td><strong>Search across files larger than 350 KiB</strong>
   </td>
   <td>✗
   </td>
   <td>✓ 
<p>
Using the <a href="https://docs.sourcegraph.com/admin/config/site_config#search-largeFile">search.largeFiles</a> keyword
   </td>
  </tr>
  <tr>
   <td><strong>Search across all branches</strong>
   </td>
   <td>Only the default branch is searchable
   </td>
   <td>✓
   </td>
  </tr>
  <tr>
   <td><strong>Number of open source repositories indexed</strong>
   </td>
   <td>7 million 
   </td>
   <td>2.8 million 
   </td>
  </tr>
  <tr>
   <td><strong>Search across issues, pull requests, and discussions. </strong>
   </td>
   <td>✓
   </td>
   <td>✗
   </td>
</table>



### Search syntax

Sourcegraph offers [structural search](https://docs.sourcegraph.com/code_search/reference/structural), and GitHub code search does not offer this search method. Structural search lets you match richer syntax patterns, specifically in code and structured data formats like JSON. Sourcegraph offers structural search on indexed code and uses [Comby syntax](https://comby.dev/docs/syntax-reference) for structural matching of code blocks or nested expressions. For example, the `fmt.Sprintf` function is a popular print function in Go. [Here](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+fmt.Sprintf%28...%29&patternType=structural&_ga=2.204781593.827352295.1667227568-1057140468.1661198534&_gac=1.118615675.1665776224.CjwKCAjwkaSaBhA4EiwALBgQaJCOc6GlhIDQyg6HQScgfSBQpoFTUf7T_NNqEX5JaobtCS08GUEJuRoCIlIQAvD_BwE&_gl=1*1r2u5zs*_ga*MTA1NzE0MDQ2OC4xNjYxMTk4NTM0*_ga_E82CCDYYS1*MTY2NzUwODExNC4xMTQuMS4xNjY3NTA5NjUyLjAuMC4w) is a pattern that matches all of the arguments in `fmt.Sprintf` in our code using structural search compared to the [search](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+fmt.Sprintf%28...%29&patternType=regexp) using regex. 

Both GitHub code search and Sourcegraph support regular expression and literal search. [Regular expression](https://docs.sourcegraph.com/code_search/reference/queries#standard-search-default) helps you find code that matches a pattern (including classes of characters like letters, numbers, and whitespace) and can restrict the results to anchors like the start of a line, the end of a line, or word boundary. Literal (standard) search matches literal patterns exactly, including punctuation, like quotes.


GitHub’s search syntax can be found [here](https://cs.github.com/about/syntax), and Sourcegraph’s search syntax can be found [here](https://docs.sourcegraph.com/code_search/reference/queries). 

Regardless of the type of search method you use, GitHub’s search is line-oriented, and Sourcegraph supports multi-line search. This means that Sourcegraph’s search queries can find results that cross multiple lines. For example, here is an [example](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/Parsely/pykafka%24+Not+leader+for+partition&patternType=regexp&_ga=2.114069518.827352295.1667227568-1057140468.1661198534&_gac=1.47310293.1665776224.CjwKCAjwkaSaBhA4EiwALBgQaJCOc6GlhIDQyg6HQScgfSBQpoFTUf7T_NNqEX5JaobtCS08GUEJuRoCIlIQAvD_BwE&_gl=1*zwylx9*_ga*MTA1NzE0MDQ2OC4xNjYxMTk4NTM0*_ga_E82CCDYYS1*MTY2NzU3NDA3OC4xMTcuMS4xNjY3NTc2NjIyLjAuMC4w) of matching multiple text strings in a file using regex, and here is a second explicit multi-line search [example](https://sourcegraph.com/search?q=context:global+app.terraform.io/example_corp+%5Cn+version+%3D%28.*%290.9.%5Cd+lang:Terraform&patternType=regexp) for terraform module verisions.   


### Commit diff search and commit message search

Commit diff and commit message searches help you see how your codebase has changed over time. With Sourcegraph, you can search within [commit diffs](https://docs.sourcegraph.com/code_search/explanations/features#commit-diff-search) to find changes to particular functions, classes, or specific areas of the codebase, and you can search over [commit messages](https://docs.sourcegraph.com/code_search/explanations/features#commit-message-search) and narrow down searches with additional filters such as author or time. 

GitHub code search does not offer commit diff search or commit message search. 


### Symbol search

Symbol search makes it easier to find specific functions, variables, and more by filtering searches for symbol results. Symbol results also allow you to jump directly to symbols by name.

Both GitHub code search and Sourcegraph support symbol searching. GitHub supports symbol search in 10 languages, including C#, Python, Go, Java, JavaScript, TypeScript, PHP, Protocol Buffers, Ruby, and Rust. 

Sourcegraph’s [symbol search](https://docs.sourcegraph.com/code_navigation/explanations/features#symbol-search) is available for more than [75 languages](https://sourcegraph.com/blog/introducing-sourcegraph-server-2-6). 


<table>
  <tr>
   <td>
   </td>
   <td><strong>GitHub</strong>
   </td>
   <td><strong>Sourcegraph</strong>
   </td>
  </tr>
  <tr>
   <td><strong>Structural search </strong>
   </td>
   <td>✗
   </td>
   <td>✓
   </td>
  </tr>
  <tr>
   <td><strong>Literal search </strong>
   </td>
   <td> ✓
   </td>
   <td>✓
   </td>
  </tr>
  <tr>
   <td><strong>Regular expression search</strong>
   </td>
   <td> ✓
   </td>
   <td>✓
   </td>
  <tr>
   <td><strong>Multi-line search support</strong>
   </td>
   <td>✗
   </td>
   <td>✓
   </td>
  </tr>
  <tr>
   <td><strong>Commit diff and commit message searches</strong>
   </td>
   <td>✗
   </td>
   <td>✓
   </td>
  </tr>
  <tr>
   <td><strong>Symbol search</strong>
   </td>
   <td>10 languages
<p>
C#, Python, Go, Java, JavaScript, TypeScript, PHP, Protocol Buffers, Ruby, and Rust.
   </td>
   <td>75+ languages
   </td>
  </tr>
</table>



### Search results and result types

**Search results**

GitHub only returns the first 10 pages of search results. You cannot currently go past the 10th page or retrieve all search results.

Sourcegraph can retrieve all search results. By default, Sourcegraph returns 500 search results, but this number can be increased by increasing the ‘count’ value. Sourcegraph can display a maximum of 1,500 results, but all matches can be fetched using the [src CLI](https://docs.sourcegraph.com/cli/quickstart), the [Stream API](https://docs.sourcegraph.com/api/stream_api), or [GraphQL API](https://docs.sourcegraph.com/api/graphql). You can also export the results via CSV. 

Souregraph's Smart Search is a query assistant that activates when a search returns no results. It helps you find search results by trying slight variations of your original query when a search shows "no results," and the alternative results are shown automatically once Smart Search is enabled. 

GitHub code search includes suggestions, completions, and the ability to save your searches. Sourcegraph offers suggestions through search query examples and [saved searches](https://docs.sourcegraph.com/code_search/how-to/saved_searches#creating-saved-searches). 

GitHub code search returns a list of repositories and files. Sourcegraph results can include repositories, files, diffs, commits, and symbols; however, you must use the ‘type’ filter to return anything outside of repositories and files. 

**Ranking**

Both GitHub and Sourcegraph display the most relevant search results first using a variety of heuristics. 

GitHub analyzes how many matches are in the file, the quality of the matches, the kind of file, whether the searches match symbols, and the number of repository stars as inputs for ranking results. 

Sourcegraph uses a repository’s number of stars to [rank](https://docs.sourcegraph.com/dev/background-information/architecture/indexed-ranking) the most important repositories first. The priority of a repository can be altered by admins with this [configuration](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+repoRankFromConfig&patternType=regexp&_ga=2.18586544.827352295.1667227568-1057140468.1661198534&_gac=1.82582244.1665776224.CjwKCAjwkaSaBhA4EiwALBgQaJCOc6GlhIDQyg6HQScgfSBQpoFTUf7T_NNqEX5JaobtCS08GUEJuRoCIlIQAvD_BwE&_gl=1*5o6st*_ga*MTA1NzE0MDQ2OC4xNjYxMTk4NTM0*_ga_E82CCDYYS1*MTY2NzM0MDAzNy4xMDQuMS4xNjY3MzQxNjUyLjAuMC4w). There are several other heuristic signals that help to make sure that the most important documents are searched first, including:

* Up rank files with a lot of symbols
* Up rank small files
* Up rank short names
* Up rank branch count
* Down rank generated code
* Down rank vendored code
* Down rank test code

When submitting a search query, the quality of a match is scored based on language-specific heuristics (Java classes rank higher than variables), word boundaries, and symbol ranges. The score that is received at the time of the query is combined with the index time rank and the repository’s priority to determine the final ranking.

[Sourcegraph.com](https://sourcegraph.com/search) (Sourcegraph's public instance for searching open source code) utilizes an algorithm inspired by Google PageRank to measure code reuse and return the most relevant search results first. This new ranking algorithm will be implemented for Sourcegraph customer instances (self-hosted and Cloud) in the future.

<table>
  <tr>
   <td>
   </td>
   <td><strong>GitHub</strong>
   </td>
   <td><strong>Sourcegraph</strong>
   </td>
  </tr>
  <tr>
   <td><strong>Comprehensive search results</strong>
   </td>
   <td>✗
<p>
Limited to 10 pages of results
   </td>
   <td>✓ 
<p>
500 search results are returned by default, but this number can be increased by increasing the ‘count’ value. A maximum of 1,500 results can be displayed, and more matches can be fetched using the src CLI, the Stream API, or GraphQL API.
   </td>
  </tr>
  <tr>
   <td><strong>Ranking based on a variety of heuristics including usage of code, files, and repositories </strong>
   </td>
   <td>✓ 
   </td>
   <td>✓ 
   </td>
  </tr>
</table>



### Search aggregations

To help you understand and refine search results, Sourcegraph’s search results also include visual search aggregation charts. These charts help you answer unique questions about the overall results set that individual search results cannot, like how many different versions of a library or package are present in your code, which repositories in a given library are most used, and more. They can also help you quickly refine your search by seeing which files, repositories, authors, or capture group returned the most results, and then clicking a result in the chart to add a filter to your query. 

You can group your search results by location (repository or file), author, or arbitrary capture group pattern. Example search aggregations can be found [here](https://docs.sourcegraph.com/code_insights/references/search_aggregations_use_cases). 

GitHub code search does not currently offer this functionality. 


### Search filters and contexts

**Filters**

Code search filters help you refine and narrow search query results to be more relevant. Both GitHub code search and Sourcegraph include filters.

GitHub code search includes filters such as language, repository, path, and file size. GitHub automatically suggests filters to apply to your search based on your search history and information about you, such as your organization. GitHub also offers auto-complete using filters to complete a code search query. 

Sourcegraph filters reduce the scope of search query results by language, repository, path, author, message, content, timeframe, visibility, and more. Sourcegraph offers auto-completion on filters in the search query. 

**Search contexts**

Search contexts are an alternative way to narrow down the scope of your search to the code you care about. Search contexts are available with both GitHub and Sourcegraph. 

You can create custom search contexts to be simple or advanced with GitHub. Simple search contexts are things such as repository or organization name, and advanced search contexts can mix multiple attributes, including languages. GitHub’s search contexts allow for more personalization than Sourcegraph. 

With Sourcegraph, a [search context](https://docs.sourcegraph.com/code_search/how-to/search_contexts) represents the body of code that will be searched. Search contexts can be private to the user who creates it or shared with other users on the same Sourcegraph instance. [Query-based ](https://docs.sourcegraph.com/code_search/how-to/search_contexts#beta-query-based-search-contexts)search contexts (beta) are an additional way to create search contexts based on variables like repository, rev, file, lang, case, fork, and visibility. Both [OR] and [AND] expressions are allowed to help further narrow the scope of query-based search contexts.


<table>
  <tr>
   <td>
   </td>
   <td><strong>GitHub</strong>
   </td>
   <td><strong>Sourcegraph</strong>
   </td>
  </tr>
  <tr>
   <td><strong>Filters</strong>
   </td>
   <td>✓ 
   </td>
   <td>✓ 
   </td>
  </tr>
  <tr>
   <td><strong>Search contexts</strong>
   </td>
   <td>✓ 
<p>
Simple and advanced
   </td>
   <td>✓ 
<p>
Repository-based and query-based
   </td>
  </tr>
</table>



## Understanding and navigating code

**Code navigation** helps you explore code in depth. It includes features such as “Go to definition” and “Find references,” which let you quickly move between files to understand code.


### Search-based code navigation

Both GitHub and Sourcegraph offer out-of-the-box, search-based code navigation. This version of code navigation uses search-based heuristics to find references and definitions across a codebase without any setup required. It is powerful and convenient, but it can sometimes present inaccurate results. For example, it can return false positive references for symbols with common names. It is limited to returning definitions and references within a single repository (it won’t track references across multiple repositories).

GitHub’s [search-based code navigation](https://docs.github.com/en/repositories/working-with-files/using-files/navigating-code-on-github) officially supports 10 languages according to the documentation, but more languages are supported in the latest beta preview. Sourcegraph’s [search-based code navigation](https://docs.sourcegraph.com/code_navigation/explanations/search_based_code_navigation) supports 40 languages.


<table>
  <tr>
   <td>
   </td>
   <td><strong>GitHub</strong>
   </td>
   <td><strong>Sourcegraph</strong>
   </td>
  </tr>
  <tr>
   <td><strong>Technical implementation</strong>
   </td>
   <td>Heuristic-based
   </td>
   <td>Heuristic-based
   </td>
  </tr>
  <tr>
   <td><strong>Language support</strong>
   </td>
   <td>10 languages
<p>
C#, CodeQL, Elixir, Go, Java, JavaScript, TypeScript, PHP, Python, Ruby
   </td>
   <td>40 languages
<p>
<a href="https://docs.sourcegraph.com/code_navigation/explanations/search_based_code_navigation">The full list of supported languages can be found here. </a>
   </td>
  </tr>
  <tr>
   <td><strong>Setup</strong>
   </td>
   <td>No setup required
   </td>
   <td>No setup required
   </td>
  </tr>
  <tr>
   <td><strong>Accuracy</strong>
   </td>
   <td>Moderate (some false positives)
   </td>
   <td>Moderate (some false positives)
   </td>
  </tr>
  <tr>
   <td><strong>Cross-repository</strong>
   </td>
   <td>✗
   </td>
   <td>✗
   </td>
  </tr>
</table>



### Precise code navigation

GitHub and Sourcegraph both offer precise code navigation as well. Despite having the same name, the two versions of precise code navigation are very different in terms of the underlying technology and accuracy they provide. Only Sourcegraph’s precise code navigation is 100% accurate. 

GitHub’s [precise code navigation](https://docs.github.com/en/repositories/working-with-files/using-files/navigating-code-on-github#precise-and-search-based-navigation) is an improved form of heuristic-based code navigation which uses syntax trees to offer higher accuracy for references and definitions and cross-repository navigation. It is more accurate than GitHub’s search-based code navigation, but it can still present inaccuracies. It is available out-of-the-box on GitHub and is automatically used over search-based code navigation when available. It is supported for 1 language, Python.

Sourcegraph’s [precise code navigation](https://docs.sourcegraph.com/code_navigation/explanations/precise_code_navigation) is not heuristic-based. Instead, it uses [SCIP and LSIF](https://docs.sourcegraph.com/code_navigation/references/indexers) data to deliver precomputed code navigation, meaning that it is fast and compiler-accurate. It is the only 100% accurate solution for code navigation between Sourcegraph’s and GitHub’s offerings. 

Because precise code navigation uses code graph (SCIP) data, it is not susceptible to false positives or other potential errors (such as those caused by symbols with the same name). It also supports cross-repository navigation, which shows symbol usage across repositories and [transitive dependencies](https://docs.sourcegraph.com/code_navigation/explanations/features#beta-dependency-navigation). It also has a unique feature, “Find implementations,” which allows you to navigate to a symbol’s interface definition or find all the places an interface is being implemented.

Sourcegraph’s precise code navigation is opt-in and requires you to upload code graph data ([LSIF or SCIP](https://docs.sourcegraph.com/code_navigation/references/indexers)) to Sourcegraph. This data can be automatically generated and uploaded to Sourcegraph via [auto-indexing](https://docs.sourcegraph.com/code_navigation/explanations/auto_indexing). For repositories without SCIP or LSIF data, Sourcegraph automatically falls back to search-based code navigation.

Sourcegraph’s precise code navigation [supports 11 languages](https://docs.sourcegraph.com/code_navigation/references/indexers). 


<table>
  <tr>
   <td>
   </td>
   <td><strong>GitHub</strong>
   </td>
   <td><strong>Sourcegraph</strong>
   </td>
  </tr>
  <tr>
   <td><strong>Technical implementation</strong>
   </td>
   <td>Heuristic-based
   </td>
   <td>SCIP-based (code graph data)
   </td>
  </tr>
  <tr>
   <td><strong>Accuracy</strong>
   </td>
   <td>High (some false positives)
   </td>
   <td>Perfect (compiler-accurate)
   </td>
  </tr>
  <tr>
   <td><strong>Language support</strong>
   </td>
   <td>1 language
<p>
Python
   </td>
   <td> 11 languages
<p>
Go, TypeScript, JavaScript, C, C++, Java, Scala, Kotlin, Rust, Python, Ruby
   </td>
  </tr>
  <tr>
   <td><strong>Setup</strong>
   </td>
   <td>No setup required
   </td>
   <td>Opt-in (Must set up LSIF/SCIP indexing. <a href="https://docs.sourcegraph.com/code_navigation/explanations/auto_indexing">Auto-indexing</a> available.)
   </td>
  </tr>
  <tr>
   <td><strong>Cross-repository</strong>
   </td>
   <td>✓
   </td>
   <td>✓ 
   </td>
  </tr>
</table>



## Codebase insights and analytics


GitHub and Sourcegraph have offerings that are called “insights,” but they differ. Generally, insights can be split into two categories:


### Codebase insights

GitHub does not offer comprehensive insights that account for the content of the code itself. Rather, GitHub’s insights are based primarily on GitHub’s product-level data. GitHub offers [dependency insights](https://docs.github.com/en/enterprise-cloud@latest/admin/policies/enforcing-policies-for-your-enterprise/enforcing-policies-for-dependency-insights-in-your-enterprise) that show all the packages your organization’s repositories depend on, e.g. aggregated information about security advisories and licenses. 


[Sourcegraph’s Code Insights](https://docs.sourcegraph.com/code_insights) is based on codebase-level data: aggregation of lines, patterns, and other search targets in the codebase. It reveals high-level information about the entire codebase, accounting for all of your repositories and code hosts. With Code Insights, you can track anything that can be expressed with a Sourcegraph search query and turn it into customizable dashboards. Code Insights can be used to track migrations, package use, version adoption, code smells, vulnerability remediation, codebase size, and more. 


###  App activity insights

With [insights for Projects](https://docs.github.com/en/issues/planning-and-tracking-with-projects/viewing-insights-from-your-project/about-insights-for-projects) in GitHub, you can view, create, and customize charts that are built from the project’s data. [Organization activity insights](https://docs.github.com/en/enterprise-cloud@latest/organizations/collaborating-with-groups-in-organizations/viewing-insights-for-your-organization) help you understand how members of the organization are using GitHub, e.g. issue and pull request activity, top languages used, and cumulative information about where members spend their time. 

Sourcegraph offers [in-product analytics](https://docs.sourcegraph.com/admin/analytics) to help Sourcegraph administrators understand user engagement across the various Sourcegraph features, identify power users, and convey value to engineering leaders.


<table>
  <tr>
   <td>
   </td>
   <td><strong>GitHub</strong>
   </td>
   <td><strong>Sourcegraph</strong>
   </td>
  </tr>
  <tr>
   <td><strong>Codebase insights</strong>
   </td>
   <td>✗
<p>
Offers dependency insights. Does not offer customizable insights about the content of the code
   </td>
   <td>✓ 
   </td>
  </tr>
  <tr>
   <td><strong>App activity and project activity insights</strong>
   </td>
   <td>✓ 
   </td>
   <td>✓ 
   </td>
  </tr>
</table>



## Large-scale code changes

GitHub does not offer a way to automate arbitrary large-scale code changes. For repositories where [Dependabot security updates](https://docs.github.com/en/enterprise-cloud@latest/code-security/dependabot/dependabot-security-updates/about-dependabot-security-updates) are enabled, when GitHub Enterprise Cloud detects a vulnerable dependency in the default branch, Dependabot creates a pull request to fix it.

With Sourcegraph’s [Batch Changes](https://docs.sourcegraph.com/batch_changes/explanations/introduction_to_batch_changes), you can make any large-scale code change across many repositories and code hosts. You can create pull requests on all affected repositories and [track the progress](https://docs.sourcegraph.com/batch_changes/how-tos/tracking_existing_changesets) until they are all merged. You can also preview the changes and update them at any time. Batch Changes is often used to make the following types of changes: updating API callsites after a library upgrade, updating configuration files, changing boilerplate code, renaming symbols, patching critical security issues, and more.


<table>
  <tr>
   <td>
   </td>
   <td><strong>GitHub</strong>
   </td>
   <td><strong>Sourcegraph</strong>
   </td>
  </tr>
  <tr>
   <td><strong>Large-scale code changes</strong>
   </td>
   <td><a href="https://docs.github.com/en/enterprise-cloud@latest/code-security/dependabot/dependabot-security-updates/about-dependabot-security-updates">Automatic Dependabot updates</a> are possible, but there is no support for applying arbitrary large-scale changes
   </td>
   <td>You can apply arbitrary code changes across many repositories and code hosts by running any codemod tool, using a template, or writing your own; You can manage and track the resulting PRs until they are merged
   </td>
  </tr>
  <tr>
   <td><strong>Completeness</strong>
   </td>
   <td>Dependency upgrades only; in GitHub only
   </td>
   <td>Any code change across multiple code hosts
   </td>
  </tr>
</table>



## Integrations and API 


### Integrations

Both GitHub and Sourcegraph offer integrations to help optimize your workflow. GitHub’s owned integrations are built and managed by GitHub, and they have a marketplace with nearly a thousand third-party applications spanning across categories such as code quality, code review, IDEs, monitoring, security, and more. These integrations are available for GitHub overall, but there aren’t any integrations related to GitHub code search. For example, the [VS Code integration](https://marketplace.visualstudio.com/items?itemName=GitHub.vscode-pull-request-github) allows you to review and manage pull requests, but it does not let you use GitHub code search in VS Code. 

Sourcegraph’s [editor integrations](../integration/editor.md) let you search and navigate across all of your repositories from all your code hosts and across all GitHub instances and organizations without leaving your IDE. Sourcegraph currently integrates with VS Code, JetBrains IDEs, and Gitpod. You can also add Sourcegraph to your preferred [browser](../integration/browser_extension/how-tos/browser_search_engine.md) to quickly search across your entire codebase from within your browser. 


### API

GitHub has a REST API for web clients, but it is not yet documented. On the other hand, Sourcegraph offers different APIs that help you access code-related data available on a Sourcegraph instance. The [GraphQL API](https://docs.sourcegraph.com/api/graphql) accesses data stored and computed by Sourcegraph. This API can [fetch](https://docs.sourcegraph.com/api/graphql/examples) file contents without cloning a repository or search for a new API and determine all of the repositories that haven’t migrated to it yet. The [Stream API](https://docs.sourcegraph.com/api/stream_api) supports consuming search results as a stream of events and it can be used to [search](https://docs.sourcegraph.com/api/stream_api#example-curl) over all indexed repositories. Lastly, use the [interactive API explorer](https://sourcegraph.com/api/console#%7B%22query%22%3A%22%23%20Type%20queries%20here%2C%20with%20completion%2C%20validation%2C%20and%20hovers.%5Cn%23%5Cn%23%20Here's%20an%20example%20query%20to%20get%20you%20started%3A%5Cn%5Cnquery%20%7B%5Cn%20%20currentUser%20%7B%5Cn%20%20%20%20username%5Cn%20%20%7D%5Cn%20%20repositories%28first%3A%201%29%20%7B%5Cn%20%20%20%20nodes%20%7B%5Cn%20%20%20%20%20%20name%5Cn%20%20%20%20%7D%5Cn%20%20%7D%5Cn%7D%5Cn%22%7D) to build and test your API queries. 


<table>
  <tr>
   <td>
   </td>
   <td><strong>GitHub</strong>
   </td>
   <td><strong>Sourcegraph</strong>
   </td>
  </tr>
  <tr>
   <td><strong>Code search integrations and extensions</strong>
   </td>
   <td>✗
<p>
GitHub built and marketplace with integrations, but no code search integration or extensions  
   </td>
   <td>✓ 
<p>
Editor integrations and browser extensions
   </td>
  </tr>
  <tr>
   <td><strong>API</strong>
   </td>
   <td>✗
   </td>
   <td>✓ 
<p>
GraphQL API and 
<p>
Stream API
   </td>
  </tr>
</table>



## Alerting

GitHub and Sourcegraph offer alerting on code in different forms.

GitHub offers [code scanning](https://docs.github.com/en/enterprise-cloud@latest/code-security/code-scanning/automatically-scanning-your-code-for-vulnerabilities-and-errors/setting-up-code-scanning-for-a-repository), which allows you to set up CodeQL or third-party analysis to run over code and generate alerts. CodeQL is a flexible code analysis tool used to query code and return arbitrary data. 

Code scanning is applied at the repository level and can be used to alert on both merged code and pull requests. Individuals can then personally configure notifications for when code scanning workflows are completed on a given repository. Notifications can go through email, the GitHub web interface, or GitHub Mobile.

GitHub also offers [Dependabot alerts](https://docs.github.com/en/enterprise-cloud@latest/code-security/dependabot/dependabot-alerts/about-dependabot-alerts). Dependabot detects insecure dependencies of repositories and triggers alerts when a new advisory is added to the [GitHub Advisory Database](https://docs.github.com/en/enterprise-cloud@latest/code-security/dependabot/dependabot-alerts/browsing-security-advisories-in-the-github-advisory-database) or the [dependency graph](https://docs.github.com/en/enterprise-cloud@latest/code-security/supply-chain-security/understanding-your-software-supply-chain/about-the-dependency-graph) for a repository changes. GitHub can also review and alert on dependency changes in pull requests made against the default branch of a repository. These alerts are viewable by admins in each repository, and admins can make them viewable to other users as well. These alerts can be sent to you via several channels: email, web interface, command line, and/or GitHub Mobile.

Sourcegraph offers [code monitors](https://docs.sourcegraph.com/code_monitoring), which continuously monitor the results of a specific search query and generate alerts when new results are returned. Code monitors only look at merged code, and they can be used to trigger alerts when undesirable code is added to a codebase. This can include known vulnerabilities, bad patterns, file changes, or consumption of deprecated endpoints (anything that can be queried via Sourcegraph).

Each code monitor can span any scope of your choosing, such as a single repository, multiple repositories, or multiple code hosts. They can also be scoped to specific branches of a repository. Code monitor alerts can be configured to send notifications via email, Slack message, or webhook. Most queries used for a code monitor can also be reused for a Code Insights chart or a fix with Batch Changes. 


## Embedded code search and documentation

Sourcegaph’s [Notebooks](https://docs.sourcegraph.com/notebooks) integrate code search with Markdown for knowledge sharing. Notebooks are created with blocks, and each block can be Markdown, a code search query, a live code snippet, a file, or a symbol.

Notebooks pull information directly from your codebase, so the information served in notebooks (via code search blocks, for example) always reflects what is live in your code at that moment in time. By referencing live code, notebooks are useful for onboarding teammates, documenting vulnerabilities, walking through complex parts of a codebase, or keeping track of useful queries. 

GitHub does not currently offer functionality to embed code search within notebooks or documentation.


## Licensing

GitHub is closed source, while Sourcegraph’s code is publicly available. 

## Availability

GitHub code search is currently available through a beta preview (you can sign up for their [waitlist](https://github.com/features/code-search-code-view/signup) to request access). The beta preview is not yet available for GitHub Enterprise. 

Sourcegraph’s code intelligence platform is generally available, and you can sign up for a free trial for your team [here](https://sourcegraph.com/get-started?t=enterprise/).
