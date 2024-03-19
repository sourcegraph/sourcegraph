# `src serve-git`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-addr` | Address on which to serve (end with : for unused port) | `:3434` |
| `-list` | list found repository names | `false` |


## Usage

```
'src serve-git' serves your local git repositories over HTTP for Sourcegraph to pull.

USAGE
  src [-v] serve-git [-list] [-addr :3434] [path/to/dir]

By default 'src serve-git' will recursively serve your current directory on the address ':3434'.

'src serve-git -list' will not start up the server. Instead it will write to stdout a list of
repository names it would serve.

Documentation at https://docs.sourcegraph.com/admin/external_service/src_serve_git

```
	