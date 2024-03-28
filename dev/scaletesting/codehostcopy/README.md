# CodeHost Copy

A small tool to copy all repos from a given user on GitHub to GitLab or Bitbucket.

State is persisted in a SQLite database to enable to resume the copy if it was interrupted.

## Usage

```
codehostcopy --config my-config.cue --state my-config.db
```

To get an example config, you can do:

```
codehostcopy example > my-config.cue
```
Hello World
