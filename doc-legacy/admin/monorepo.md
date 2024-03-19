# Using Sourcegraph with a monorepo

Sourcegraph can be used on Git [monorepos](https://trunkbaseddevelopment.com/monorepos/) and provides functionality that developers may be missing:

- Indexed code search across projects in the monorepo.
- Code navigation across projects in the monorepo.

The tools on a developer's local machine often can't cope with the scale of a monorepo. For example, a developer often cannot load the whole monorepo into their IDE, and therefore has to do without code exploration functionality useful when doing cross-project work. Sourcegraph brings this functionality back to developers, enabling them to work efficiently at the day-to-day task of understanding code.

## Scale and performance

Sourcegraph is designed to scale to large monorepos, even when local Git operations and other code search tools become slow.

Sourcegraph uses the standard `git` binary to interact with repositories, but structures its use of `git` to ensure that user requests remain fast. Commands like `git rev-parse` and `git show` are called when rendering a page for a user. Git operations that scale with the total number of files are left to background indexing jobs. Operations that scale with the total number of commits or refs/tags may be in the user path, but these operations use pagination to remain fast.

Sourcegraph's code search index scales horizontally with the number of files being indexed for search. Multiple shards may be allocated for one repository, and the index is agnostic to whether the code exists in one massive repository or many smaller ones. Sourcegraph has been used to index both large monorepos and tens of thousands of smaller repositories.

### Known Limitations

- Sourcegraph will inspect the full tree for language detection. It incrementally caches and builds the language statistics to reuse information across commits. However, this has been shown to create too much load in monorepos. You can disable this feature by setting the environment variable `USE_ENHANCED_LANGUAGE_DETECTION=false` on `sourcegraph-frontend`.

## Custom git binaries

Sourcegraph clones code from your code host via the usual `git clone` or `git fetch` commands. Some organisations use custom `git` binaries or commands to speed up these operations. Sourcegraph supports using alternative git binaries to allow cloning. This can be done by inheriting from the `gitserver` docker image and installing the custom `git` onto the `$PATH`.

Some monorepos use a custom command for `git fetch` to speed up fetch. Sourcegraph provides the `experimentalFeatures.customGitFetch` site setting to specify the custom command.

## Statistics

You can help the Sourcegraph developers understand the scale of your monorepo by sharing some statistics with the team. The bash script [`git-stats`](https://github.com/sourcegraph/sourcegraph/blob/main/dev/git-stats) when run in your git repository will calculate these statistics.

Example output on the Sourcegraph repository:

``` shellsession
$ wget https://github.com/sourcegraph/sourcegraph/raw/main/dev/git-stats
$ chmod +x git-stats
$ ./git-stats
725M	. gitdir
19671 commits

HEAD statistics
0.096GiB
8638 files
histogram:
10^0	6
10^1	69
10^2	667
10^3	2236
10^4	4589
10^5	971
10^6	86
10^7	14
```
