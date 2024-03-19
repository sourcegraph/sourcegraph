# Bazel cookbook

The `bazel` command is very powerful, and it can be used both locally and in CI. The goal of this page is list useful recipes that you might need during your development journey.

## Testing 

### Making sure a test isn't flaky anymore

The only way of ensuring that a test isn't flaky anymore is to run it many times in a row, and to observe no failures. You can easily
do this with Bazel, with the `--runs_per_test=N` flag: 

```
# Target will be tested three times in parallel.
bazel test //lib/gitservice/... --runs_per_test=3
```

Some tests, particularly integration tests, are not designed to be run concurrently (which they should), so we need to run them one after the other with the 
`--local_test_jobs` flag:

```
# Target will be tested three times in serial.
bazel test //lib/gitservice/... --runs_per_test=3 --local_test_jobs=1
```

If you really want to make sure your test is not flaky anymore, you'll want to really raise the number of runs, which might not be practical to do locally. Why 
not leverage our beefy CI agents for that purpose? 

You can use the `bazel-do` CI runtype to ask the CI to run a single bazel command and report back: 

- Commit all your changes 
- Create a new branch prefixed with `bazel-do` 
  - ex `bazel-do/jh/flaky-foobar`
- Amend your last commit or create an empty one that contains in the commit description a new line starting with `!bazel [my-command]`:
  - ex: `!bazel test //lib/gitservice/... --runs_per_test=50` 
- Git push that branch 
- Run `sg ci status --web` to open the build page in your browser or `sg ci status --wait` to wait for the results in your terminal.
