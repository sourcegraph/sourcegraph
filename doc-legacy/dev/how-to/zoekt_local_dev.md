# Set up local development with Zoekt and Sourcegraph

```
$ git clone https://github.com/sourcegraph/sourcegraph
$ git clone https://github.com/sourcegraph/zoekt
```

To see your Zoekt changes reflected live on your local Sourcegraph instance you can use [go workspaces](https://go.dev/ref/mod#workspaces):

```
$ cd sourcegraph
$ go work init . ../zoekt
$ cat go.work
go 1.19

use (
        .
        ../zoekt
)
```

This isn't hot reloaded so you might have to restart Sourcegraph on every Zoekt change. It may make sense to ensure your
changes in Zoekt are working first before trying them out in Sourcegraph.

When you are done do not forget to remove your `go.work` file.

## Install Ctags

Ctags is not required for Zoekt. However, we use ctags in production and tests on CI run with ctags enabled.

To install universal-ctags locally, follow the instructions
under ["how to build and install"](https://github.com/universal-ctags/ctags#how-to-build-and-install) with the following
changes:

```diff
$ git clone https://github.com/universal-ctags/ctags.git
$ cd ctags
$ ./autogen.sh
- $ ./configure --prefix=/where/you/want # defaults to /usr/local
+ $ ./configure --program-prefix=universal- --prefix=/where/you/want --enable-json
$ make
$ make install # may require extra privileges depending on where to install
```

The installation will place the binaries in `/where/you/want/bin/`. Make sure the `bin` directory is on your `$Path`.

`--program-prefix=universal-` ensures that ctags will be picked up by Zoekt just like it would in production. Note that
the binary name "universal-ctags" is treated differently in Zoekt. Setting the environment
variable `CTAGS_COMMAND=ctags` might lead to different results because Zoekt won't
use [go-ctags](https://github.com/sourcegraph/go-ctags).

On macOS, as BSD version of ctags is shipped as part of the XCode commandline developer tools. Using the BSD version for
Zoekt is not supported.

## Notes

Here are some commands you can run against Zoekt.

**Setup**

```
$ go install ./cmd/...
$ go install ./cmd/<specific command>
```

The components that Sourcegraph uses from Zoekt are `zoekt-git-index`, `zoekt-sourcegraph-indexserver`,
and `zoekt-webserver`.

**Usage**

```
# start indexserver, pointing it to a local dir instead of to an instance of Sourcegraph
$ zoekt-sourcegraph-indexserver --sourcegraph_url <dir>

# start the web interface
$ zoekt-webserver

# index 1 repo
$ zoekt-git-index /path/to/repo

# search shards directly, without webserver
$ zoekt <query>
```

Check `<cmd> --help` for more information.

**Misc**

Index files are stored in:

- `~/.zoekt` (Zoekt's default index dir)
- `~/.sourcegraph/zoekt/index-X` (sourcegraph)

Local Sourcegraph Zoekt UI can be accesed at localhost:3070 and localhost:3071 (we have multiple instances because of
horizontal scaling).
