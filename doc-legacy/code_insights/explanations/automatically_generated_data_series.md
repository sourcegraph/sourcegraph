# Detect and track patterns with automatically generated data series 

> Note: "Detect and track patterns" insights are available in Sourcegraph 3.35 and above. In Sourcegraph 3.35 this feature is in its earliest version. Stability and additional features coming in following releases.

Code Insights with automatically generated data series allow you to "detect and track patterns" and versions within your codebase. You can use these insights to track versions of languages, packages, terraform, docker images, or anything else that can be captured with a [regular expression capture group](#regular-expression-capture-group-resources).

## Automatic generation of individual series

Match patterns can be returned with a regular expression containing a capture group, or a pattern within parentheses. 

For example, `file:pom\.xml|\.pom$ <java\.version>(.*)</java\.version>` will match all of: 

```
<java.version>1.5</java.version>
<java.version>1.7</java.version>
<java.version>1.8</java.version>
```

Code Insights will find all matches, and then automatically generate a data series and color for each unique value of the capture group. In this case, the chart would show data series for 1.5, 1.7, and 1.8, with the values being the number of matches for each unique value. 

## New matching data gets automatically added 

Capture groups will automatically create new data series for new matches as they appear in your codebase. You do not need to update or manually re-create the insights to track newly added versions or patterns.

For the above example, this means that if `<java.version>1.9</java.version>` was committed to the codebase in the future, it would appear on the insight without any additional action, and you would see a series for `1.9`. 

## Current limitations 

This feature has some yet-released limitations. In rough order, with limitations listed first likely to be removed soonest, they are: 

### Limited to 20 matches

Capture groups will only display 20 returned match values to prevent extremely large result sets from being rendered. As of version 3.41.0 controls are available to make this deterministic and configurable.

### No capture groups in filter strings 

This type of Code Insight only supports capture groups in the main query string. You cannot use a capture group in a filter keyword, like a `repo:` or `file:` filter. 

### No multiline matches (<3.43)

On Sourcegraph instances on earlier versions than 3.43, you can only use a single-line regular expression. This means that `^` and `$` characters are still valid but needing to match on a `\n` is not. As a potential workaround to get more granularity, you can still use `file:has.content()` and other [search predicates](https://docs.sourcegraph.com/code_search/reference/language#built-in-file-predicate). 

### No combinations of capture groups 

You cannot combinatorially combine capture groups. Queries containing multiple capture groups will return data series with a label for a single capture group only. For example, `([0-9])\.([0-9])\.([0-9])` will match results like 1.2.3, 4.5.6, and 4.5.7, but will return data series for 1, 2, 3, 4, 5, 6, and 7 individually. (If you made the string one capture group `([0-9]\.[0-9]\.[0-9])` instead, it would return series and counts for 1.2.3, 4.5.6, and 4.5.7). 

### Searches are limited to file content

You can't use capture groups in queries for `type:commit`, `type:repo`, `type:path`, `type:diff`, or `type:symbol`. 

### Match values are limited to a 100 characters

Any matches that return values over a 100 characters will be truncated. This is to avoid issues when storing the data.

### Regular expression capture group resources

- [Example regular expressions for common use cases](../references/common_use_cases.md#automatic-version-and-pattern-tracking)
- [General additional capture groups documentation](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Guide/Regular_Expressions/Groups_and_Ranges)
