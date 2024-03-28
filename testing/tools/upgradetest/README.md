## Overview
The upgradetest folder contains the code for release tooling intended used to asses the stability of a new migrator and frontend build before tagging a new release. 
The tooling takes the form of a cli interface and is intended to orchestrate a psuedo-release and upgrade using bazel built images. The tests are  inocations of our services relevant to schema migrations and versioning. 
This test does not test Sourcegraph features, only the basic operations of database and frontend versioning.

Commands are intended to be invoked using `bazil targets`, with each command executing tests intrended to be run in CI. The CI tests treat the current repo branch of `sourcegraph/sourcegraph` as a prospective release. 

This "release branch" may be stamped with a version, or will be versioned `0.0.0+dev`. A stamped version of the release branch must be invoked with a certain bazel flags `--stamp`, `--workspace_status_command=./dev/bazel_stamp_vars.sh`,
and additionally requires a `VERSION` env var to be set with a semantic version string `X.X.X`. 

The general idea of the tests is to verify that a given upgrade process works as expected in a containerized end to end test.

We initialize the three primary Sourcegraph databases (frontend, codeintel-db, and codeinsights-db) from a historic version. Then use candidate builds of `frontend` and `migrator` to conduct a series of upgrades and validations. Validating expected state between steps.

We conduct multiversion upgrades, and standard upgrades, based on their respective upgrade policies, only using MVU for versions in which it is necessary.
- For Standard upgrades (migrator up) we test each patch version defined in the previous minor version of sourcegraph.
- For MVU upgrades (migrator upgrade) we test all versions defined at least two minor versions prior to the latest patch release. i.e. all versions for which a standard upgrade will not work.
- For autoupgrades we attempt an upgrade accross all versions.

### Run Bazel Test
```bash
bazel test //testing/tools/upgradetest:sh_upgradetest --config darwin-docker -- <test args>
```

### Run Bazel Action
- Version 0.0.0+dev (no autoupgrade):
  ```bash
  bazel run //testing/tools/upgradetest:sh_upgradetest_run --config darwin-docker -- <command>
  ```
- Stamp version:
  ```bash
  VERSION=x.x.x bazel run //testing/tools/upgradetest:sh_upgradetest_run --config darwin-docker --stamp --workspace_status_command=./dev/bazel_stamp_vars.sh -- <command>
  ```
### Run in CI
```bash
sg ci bazel run //testing/tools/upgradetest:sh_upgradetest
```
## TODO
- Test things in CI
- Log levels
  - Optional container logs indentation formating
  - On Error log depths
  - Streaming log behavior
    - Print stuff (fail/pass/errs) as it goes through.
- Make it so it can fail early if needed perhaps?
- test OOB migrations by seeding data.
- read known bug versions from file, improve visability of known bugs versions, and select by test type
- The stitched migration file requires that the local branch have `consts.go` `maxVersionString` updated before a new stitched-migration graph version is stamped via `VERSION` then `bazel run //dev:write_all_generated` is run. (this will be handled in bazel)
