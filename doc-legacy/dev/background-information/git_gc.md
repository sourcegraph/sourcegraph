# Git GC and its modes of operations in Sourcegraph

`git-gc` is git's built-in mechanism to optimise git objects in a repository. Sourcegraph runs `git gc` in `gitserver` to clean up cruft in git repos, sometimes when executing git commands and other times as part of regular clean up jobs.

We have three possible modes of operation. Two of them circumvent git's default behaviour (more on it later):

1. If `SRC_ENABLE_GC_AUTO` is set to `true` and `SRC_ENABLE_SG_MAINTENANCE` is `false`, then we run `git gc --auto` with the value of `gc.auto` set to `1`. This tells `git gc --auto` to pack all loose objects if the number of these objects is greater than `1`.
2. But if the opposite is true, that is `SRC_ENABLE_GC_AUTO` is set to `false` while `SRC_ENABLE_SG_MAINTENANCE` is `true` then we run `sg maintenance` and `git prune`. In this mode `gc.auto` is set to `0` which effectively disables automatic packing of loose objects along with any other heuristics that `git gc --auto` keeps an eye out to decide if it should run or not.

The frequency of both these modes of operation is controlled by an environment variable `SRC_REPOS_JANITOR_INTERVAL` - which is set to 1 minute by default. But if the job itself takes longer than the interval, then we ensure to wait for it to finish and then wait for the interval as determined by the environment variable to expire before launching a new iteration of that job.

However the third and final mode of operation is git's default behaviour and is not controlled by Sourcegraph. The value of `SRC_REPOS_JANITOR_INTERVAL` has no effect on its frequency.

If both `SRC_ENABLE_GC_AUTO` and `SRC_ENABLE_SG_MAINTENANCE` are enabled or disabled at the same time, we fall back to the default value of the `gc.auto` flag - `6700`, indicating the number of loose objects above which `git gc --auto` will automatically start repacking them. The frequency of this depends on the frequency and volume of updates to the repository. However, it should be noted that this is not the only heuristic monitored by `git` to decide if it should run `git gc --auto` or not. For more information, see: _[git-gc(1)](https://www.man7.org/linux/man-pages/man1/git-gc.1.html)_.
