# Details

## Data freshness

Searches scoped to specific repositories are always up-to-date. Sourcegraph automatically fetches repository contents with any user action specific to the repository and makes new commits and branches available for searching and browsing immediately.

Unscoped search results over large repository sets may trail latest default branch revisions by some interval of time. This interval is a function of the number of repositories and the computational resources devoted to search indexing.

## Max file size

By default, files larger than 1 MB are excluded from search results. Use the [search.largeFiles](../../../admin/config/site_config.md#search-largeFiles) keyword to specify files to be indexed and searched regardless of size. Regardless of where you set the `search.largeFiles` environment variable, Sourcegraph will continue to ignore binary files, even if the size of the file is less than the limit you set.

## Exclude files and directories

You can exclude files and directories from search by adding the file _.sourcegraph/ignore_ to
the root directory of your repository. Sourcegraph interprets each line in the _ignore_ file as a glob
pattern. Files or directories matching those patterns will not show up in the search results.

The _ignore_ file is tied to a commit. This means that if you committed an _ignore_ file to a 
feature branch but not to your default branch, then only search results for the feature branch
will be filtered, while the default branch will show all results.

Example:
```
# .sourcegraph/ignore
# lines starting with # are comments and are ignored
# empty lines are ignored, too

# ignore the directory node_modules/
node_modules/

# ignore the directory src/data/
src/data/

# ** matches all characters, while * matches all characters except /
# ignore all JSON files
**.json

# ignore all JSON files at the root of the repository
*.json

# ignore all JSON files within the directory data/
data/**.json

# ignore all data folders
data/
**/data/

# ignore all files that start with numbers
[0-9]*.*
**/[0-9]*.*
```

Our syntax follows closely what is documented in 
[the linux documentation project](https://tldp.org/LDP/GNU-Linux-Tools-Summary/html/x11655.htm).
However, we distinguish between `*` and `**`: While `**` matches all characters, `*` matches all characters 
except the path separator `/`.

Note that invalid globbing patterns will cause an error and searches over commits containing a broken _ignore_ file 
will not return any result.

## Shard merging

Shard merging is a feature of Zoekt that enables the combination of smaller
index files, or shards, into one larger file, a compound shard. This can reduce
memory costs for Zoekt webserver. This feature is particularly useful for
customers with many small and rarely updated repositories, and can result in a
significant reduction in memory. Shard merging can be enabled by setting
`SRC_ENABLE_SHARD_MERGING="1"` for Zoekt indexserver.

Shard merging can be fine-tuned by setting ENV variables for Zoekt indexserver:

| Env Variable           | Description                                                                                     | Default                                                |
|------------------------|-------------------------------------------------------------------------------------------------|--------------------------------------------------------|
| SRC_VACUUM_INTERVAL    | Run vacuum this often, specified as a duration                                                 | 24 hours                                               |
| SRC_MERGE_INTERVAL     | Run merge this often, specified as a duration                                                  | 8 hours                                                |
| SRC_MERGE_TARGET_SIZE  | The target size of compound shards in MiB                                                      | 2000                                                   |
| SRC_MERGE_MIN_SIZE     | The minimum size of a compound shard in MiB                                                    | 1800                                                   |
| SRC_MERGE_MIN_AGE      | The time since the last commit in days. Shards with newer commits are excluded from merging.   | 7                                                      |
| SRC_MERGE_MAX_PRIORITY | The maximum priority a shard can have to be considered for merging, specified as a float value | 100.0                                                  |

When repostiories receive udpates, Zoekt reindexes them and tombstones their
old index data. As a result, compound shards can shrink and be dismantled into
individual shards once they reach a critical minimum size. These individual
shards are then considered for future merge operations.

Shard merging can be monitored via the "Compound shards" panel in Zoekt's
Grafana dashboard.

## RE2 Regular Expressions

The Sourcegraph search language supports [RE2](https://golang.org/s/re2syntax) syntax. If you're used to tools like Perl which uses [PCRE syntax](https://www.pcre.org/original/doc/html/pcresyntax.html), you may notice that there are some features that are missing from RE2 like backreferences and lookarounds. We choose to use RE2 for a few reasons:
* It makes it possible to build [worst-case linear](https://swtch.com/~rsc/regexp/regexp1.html) evaluation engines, which is very desirable for building a production-ready regex search engine.
* It's well-supported in Go, allowing us to take advantage of a rich ecosystem (notably including [Zoekt](https://github.com/sourcegraph/zoekt))
* Our API and tooling makes it straightforward to use Sourcegraph with other tools that provide facilities not built in to the search language.

As an example of how you can use Sourcegraph tooling with other tools, we can use `jq` (which supports Perl regexes) along with `src` to post-filter search results. In this case, we want to use backreferences to find go functions take a single pointer argument and return a non-pointer of the same type as the input.

```bash
re2_regex='func \w+\(\w+ \*\w+\) \w+'
pcre2_regex='func \w+\(\w+ \*(\w+)\) \1'

src search --json --stream -- "/$re2_regex/" \
  | jq '
    # Filter to only content events
    select(.type == "content")
    
    # Flatten to a single object per match
    | {content: .chunkMatches[].content} + del(.chunkMatches)

    # Select only matches that match the PCRE regex
    | select(.content | test($ARGS.positional[0]))
  ' --args "$pcre2_regex"
```
