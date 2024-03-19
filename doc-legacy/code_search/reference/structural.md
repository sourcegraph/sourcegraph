# Structural search

<style>
table td:first-child {
  width: 8em;
  min-width: 8em;
  max-width: 8em;
}
table td:nth-child(2) {
  width: 10em;
  min-width: 10em;
  max-width: 10em;
}
table td {
    border: none;
}
table tr:nth-child(2n) {
  background-color: transparent;
}

</style>

With structural search, you can match richer syntax patterns specifically in code and structured data formats like JSON. It can be awkward or difficult to match code blocks or nested expressions with regular expressions. To meet this challenge we've introduced a new and easier way to search code that operates more closely on a program's parse tree. We use [Comby syntax](https://comby.dev/docs/syntax-reference) for structural matching. Below you'll find examples and notes for this language-aware search functionality.

## Example

The `fmt.Sprintf` function is a popular print function in Go. Here is a pattern that matches all the arguments in `fmt.Sprintf` calls in our code:

```go
fmt.Sprintf(...)
```

[See it live on Sourcegraph's code ↗](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+fmt.Sprintf%28...%29&patternType=structural)

The `...` part is special syntax that matches all characters inside the _balanced_ parentheses `(...)`. Let's look at two interesting variants of
matches in our codebase. Here's one:

```go
fmt.Sprintf("must be authenticated as an admin (%s)", isSiteAdminErr.Error())
```

Note that to match this code, we didn't have to do any special thinking about handling the parentheses `(%s)` that happen _inside_ the first string argument, or the nested parentheses that form part of `Error()`. Unlike regular expressions, no "overmatching" can happen and the match will always respect balanced parentheses. With regular expressions, taking care to match the closing parentheses for this call could, in general, really complicate matters.

Here is a second match:

```go
fmt.Sprintf(
		"rest/api/1.0/projects/%s/repos/%s/pull-requests/%d",
		pr.ToRef.Repository.Project.Key,
		pr.ToRef.Repository.Slug,
		pr.ID,
	)
```

Here we didn't have to do any special thinking about matching contents that spread over multiple lines. The `...` syntax by default matches across newlines. Structural search supports balanced syntax like `()`, `[]`, and `{}` in a language-aware way. This allows it to match large, logical blocks or expressions without the limitations of typical line-based regular expression patterns.

## Syntax reference

The syntax `...` above is an alias for a canonical syntax `:[hole]`, where `hole` is a descriptive identifier for the matched content. Identifiers are useful when expressing that matched content should be equal (see the [`return :[v.], :[v.]`](#match-equivalent-expressions) example below). Here is a table of additional syntax.

| Syntax                  | Alias                            | Description                                                                                                                                                                                                                                                         |
|-------------------------|----------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `...`                   | `:[hole]`<br>`:[_]`              | Match zero or more characters in a lazy fashion. When `:[hole]` is inside delimiters, as in `{:[h1], :[h2]}` or `(:[h])`, holes match within that group or code block, including newlines.                                                                          |
| `:[~regexp]`            | `:[hole~regexp]`                 | Match an arbitrary [regular expression](https://golang.org/s/re2syntax) `regexp`. A descriptive identifier like `hole` is optional. Avoid regular expressions that match special syntax like `)` or `.*`, otherwise your pattern may fail to match balanced blocks. |
| `:[[_]]`<br>`:[[hole]]` | `:[~\w+]`<br>`:[hole~\w+]`       | Match one or more alphanumeric characters and underscore.                                                                                                                                                                                                           |
| `:[hole\n]`             | `:[~.*\n]`<br>`:[hole~.*\n]`     | Match zero or more characters up to a newline, including the newline.                                                                                                                                                                                               |
| `:[ ]`<br>`:[ hole]`    | `:[~[ \t]+]`<br>`:[hole~[ \t]+]` | Match only whitespace characters, excluding newlines.                                                                                                                                                                                                               |
| `:[hole.]`              | `[_.]`                           | Match one or more alphanumeric characters and punctuation like `.`, `;`, and `-` that do not affect balanced syntax. Language dependent.                                                                                                                            |

Note: To match the string `...` literally, use regular expression patterns like `:[~[.]{3}]` or `:[~\.\.\.]`.

**Rules.** [Comby supports rules](https://comby.dev/docs/advanced-usage) to express equality constraints or pattern-based matching. Comby rules are not officially supported in Sourcegraph yet. We are in the process of making that happen and are taking care to address stable performance and usability. That said, you can explore rule functionality with an experimental `rule:` parameter. For example:

[`buildSearchURLQuery(:[first], ...) rule:'where match :[first] { | " query: string" -> true }'` ↗](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:.ts+buildSearchURLQuery%28:%5Bfirst%5D%2C+...%29+rule:%27where+match+:%5Bfirst%5D+%7B+%7C+%22+query:+string%22+-%3E+true+%7D%27&patternType=structural)

### More examples

Here are some additional examples. Visit our [structural search blog post](https://sourcegraph.com/blog/going-beyond-regular-expressions-with-structural-code-search) for more.

#### Match stringy data

Taking the original `fmt.Sprintf(...)` example, let's modify the original pattern slightly to match only if the first argument is a string. We do this by adding string quotes around `...`. Adding quotes communicates _structural context_ and changes how the hole behaves: it will match the contents of a single string delimited by `"`. It _won't_ match multiple strings like `"foo", "bar"`.

```go
fmt.Sprintf("...", ...)
```

[See it live on Sourcegraph's code ↗](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+fmt.Sprintf%28%22...%22%2C+...%29&patternType=structural)

Some matched examples are:

```go
fmt.Sprintf("external service not found: %v", e.id)
```

```go
fmt.Sprintf("%s/campaigns/%s", externalURL, string(campaignID))
```

Holes stop matching based on the first fragment of syntax that comes after it, similar to lazy regular expression matching. So, we could write:

```go
fmt.Sprintf(:[first], :[second], ...)
```

to match all functions with three or more arguments, matching the `first` and `second` arguments based on the contextual position around the commas.

#### Match equivalent expressions

Using the same identifier in multiple holes adds a constraint that both of the matched values must be syntactically equal. So, the pattern:

```go
return :[v.], :[v.]
```

will match code where two identifier-like tokens in the `return` statement are the same. For example, `return true, true`, `return nil, nil`, or `return 0, 0`.

[See it live on Sourcegraph's code ↗](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+lang:go+return+:%5Bv.%5D%2C+:%5Bv.%5D&patternType=structural)

#### Match JSON

Structural search also works on structured data, like JSON. Use patterns to declaratively describe pieces of data to match. For example, the pattern:

```json
"exclude": [...]
```

matches all parts of a JSON document that have a member `"exclude"` where the value is an array of items.

[See it live on Sourcegraph's code ↗](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24++%22exclude%22:+%5B...%5D+lang:json+file:tsconfig.json&patternType=structural)

### Current functionality and configuration

Structural search behaves differently to plain text search in key ways. We are continually improving the functionality of this new feature, so please note the following:

- **Only indexed repos.** Structural search can currently only be performed on _indexed_ repositories. See [configuration](../../../admin/search.md) for more details if you host your own Sourcegraph installation. Our service hosted at [sourcegraph.com](https://sourcegraph.com/search) indexes approximately 200,000 of the most popular repositories on GitHub. Other repositories are currently unsupported. To see whether a repository on your instance is indexed, visit `https://<sourcegraph-host>.com/repo-org/repo-name/-/settings/index`.

- **The `lang` keyword is semantically significant.** Adding the `lang` [keyword](queries.md) informs the parser about language-specific syntax for comments, strings, and code. This makes structural search more accurate for that language. For example, `fmt.Sprintf(...) lang:go`. If `lang` is omitted, Sourcegraph will attempt a best-effort inference of the language based on matching file extensions or fall back to a generic structural matcher.

- **Saved searches are not supported.** It is not currently possible to save structural searches.

- **Matching blocks in indentation-sensitive languages.** It's not currently possible to match blocks of code that are indentation-sensitive. This is a feature planned for future work.
