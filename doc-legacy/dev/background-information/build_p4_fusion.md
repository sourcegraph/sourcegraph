# Building p4-fusion

In order to import Perforce depots into Sourcegraph we first convert them into git repositories. We use an open source tool called [p4-fusion](https://github.com/salesforce/p4-fusion):

> A fast Perforce depot to Git repository converter using the Helix Core C/C++ API as an attempt to mitigate the performance bottlenecks in git-p4.py.

# Nix

[Building](https://github.com/salesforce/p4-fusion#build) p4-fusion can be a little tricky as it depends on some older libraries. To get around this we use [nix](https://nixos.org).

## How to build

Below are the instructions for building p4-fusion locally, assuming you have the [Sourcegraph repository](https://github.com/sourcegraph/sourcegraph) checked out.

1. Follow [these instruction](https://nixos.org/download.html) to install Nix. (Tested with version 2.15.0)
1. Navigate to the root of your Sourcegraph directory
1. Run `nix build ".#p4-fusion" --verbose --extra-experimental-features nix-command --extra-experimental-features flakes`

If the build completes successfully you should have a `p4-fusion` binary in `./result/bin/p4-fusion` which you can copy somewhere in your `$PATH`

## Troubleshooting

### `nix build` fails with "hash mismatch" error referencing `helix-core-api.drv`

The `p4-fusion` dependencies specified in [the `srcs` array of `dev/nix/p4-fusion.nix`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@8d513301f8a12f7c7e0b5a66ed20ba14b6679cca/-/blob/dev/nix/p4-fusion.nix?L45-77) are sometimes updated without being versioned, so their hashes change, which causes "hash mismatch" errors.

**Example**

```bash
building '/nix/store/cb2ssx8vykl1ghb4k87yp3q6wnvfvjj2-helix-core-api.drv'...
error: hash mismatch in fixed-output derivation '/nix/store/cb2ssx8vykl1ghb4k87yp3q6wnvfvjj2-helix-core-api.drv':
         specified: sha256-8yX9sjE1f798ns0UmHXz5I8kdpUXHS01FG46SU8nsZw=
            got:    sha256-gaYvQOX8nvMIMHENHB0+uklyLcmeXT5gjGGcVC9TTtE=
error: 1 dependencies of derivation '/nix/store/m409z1rq40bwzvvndbnghrrxm000zd9v-p4-fusion.drv' failed to build
```

To update the hashes, we use `nix` tools to calculate the hashes of the unpacked archives, and convert the format to an SRI representation that includes the type of the hash.

Get the URL of the archive from the part of the `srcs` array that matches the OS and architecture of your local system.

**Example**

Replace `<url of archive>` with the value of the archive url from `srcs`

```bash
$ hash_type=sha256
$ hash_value=$(nix-prefetch-url --type "${hash_type}" --unpack <url of archive>)
$ nix --extra-experimental-features nix-command hash to-sri "${hash_type}:${hash_value}"
```

Copy the output from that sequence of commands and paste it into the value of the `hash` field in the `fetchzip` attribute set.

Since when one changes, they all probably change, here is an example of getting the updated hashes for all of the archives ([current archive URLs; double-check that they are still correct](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/dev/nix/p4-fusion.nix))

```bash
hash_type=sha256
for url in \
"https://cdist2.perforce.com/perforce/r22.2/bin.macosx12arm64/p4api-openssl3.tgz" \
"https://cdist2.perforce.com/perforce/r22.2/bin.macosx12x86_64/p4api-openssl3.tgz" \
"https://cdist2.perforce.com/perforce/r22.2/bin.linux26x86_64/p4api-glibc2.3-openssl3.tgz"
do
  echo "${url}"
  hash_value=$(nix-prefetch-url --type "${hash_type}" --unpack "${url}")
  nix --extra-experimental-features nix-command hash to-sri "${hash_type}:${hash_value}"
  echo
done
```

# Manual

Although building p4-fusion manually can take some effort, it is possible on macOS (may require 13+) and Linux. Windows has not been researched.

## Build tools

In addition to the compiler, which can be installed with the `build-essential` package on Debian
You will need `cmake` to build `p4-fusion`. The compiler it uses

## Dependencies

- `openssl@3.0.8`

- the P4 API for your OS + platform:
  - [macOS ARM 64-bit](https://filehost.perforce.com/perforce/r22.2/bin.macosx12arm64/p4api-openssl3.tgz)
  - [macOS Intel 64-bit](https://filehost.perforce.com/perforce/r22.2/bin.macosx12x86_64/p4api-openssl3.tgz)
  - [Linux Intel 64-bit](https://filehost.perforce.com/perforce/r22.2/bin.linux26x86_64/p4api-glibc2.3-openssl3.tgz)
  - If you need a different one, browse the parent directory of one of the above to see if it's available.

## Source code

`git clone` the [p4-fusion repository](https://github/salesforce/p4-fusion) or download a zip archive of the source. Or do the same on your chosen fork/branch.

## Setup

1. Set the `OPENSSL_ROOT_DIR` environment variable to point to the OpenSSL install location - the directory that contains OpenSSL's `bin`, `include`, and `lib` directories.

    Note that we're building with dynamic linking, so the OpenSSL libraries need to stay there for `p4-fusion` to be able to run.
You might be able to fiddle with that using `LD_LIBRARY_PATH`, but it's safest to leave them there.

2. Unpack the P4 API archives into the `p4-fusion` source directory tree, into the `vendor/helix-core-api/{OS}` directory, where `{OS}` is either `mac` or `linux`, depending on your OS. You'll need the `include` and `lib` dirs from the P4 API archive.

## Build

Now that everything is in place, from within the `p4-fusion` source directory, run the following:

```bash
./generate_cache.sh Debug
./build.sh
```

## Use

If it all worked out as planned, the executable will be `${PWD}/build/p4-fusion/p4-fusion`. Use it there, or copy it to somewhere in your `PATH`.

## Examples

### macOS

Here's a sample shell script for macOS that follows all of the steps outlined above to build a Debug binary of the `p4-fusion` `1.12` release using OpenSSL 3.0.8 and the `22.2` P4 API.

```bash
brew install cmake openssl@3.0
export OPENSSL_ROOT_DIR=$(brew --prefix openssl@3.0)
mkdir p4-fusion 2>/dev/null
cd p4-fusion
[ -s "v1.12.tar.gz" ] || {
  curl -sSLO https://github.com/salesforce/p4-fusion/archive/refs/tags/v1.12.tar.gz
  tar xzf v1.12.tar.gz
}
cd p4-fusion-1.12
[ -f "p4api-openssl3.tgz" ] || {
  arch=x86_64
  [[ "$(arch)" = "arm64" ]] && arch=arm64
  curl -sSLO https://cdist2.perforce.com/perforce/r22.2/bin.macosx12${arch}/p4api-openssl3.tgz
  tar xzf p4api-openssl3.tgz
}
rm -rf vendor/helix-core-api/mac
mkdir -p vendor/helix-core-api/mac
mv -f p4api-2022.2.2407422/include p4api-2022.2.2407422/lib vendor/helix-core-api/mac
rm -rf p4api-2022.2.2407422
./generate_cache.sh Debug
./build.sh
ls -l "${PWD}/build/p4-fusion/p4-fusion"
```

### Linux

Here's a sample shell script for Debian with OpenSSL 3.0.8.
Debian 11 installs with OpenSSL 1.1, so make OpenSSL 3.0.8 from source (which takes awhile).
Note that P4 API has only Intel/AMD for Linux, not ARM.

```
sudo apt-get update
apt install build-essential checkinstall zlib1g-dev curl cmake

mkdir openssl
cd openssl
curl -sSLO https://www.openssl.org/source/openssl-3.0.8.tar.gz
tar xzf openssl-3.0.8.tar.gz
cd openssl-3.0.8
sudo ./config --prefix=/usr/local/ssl --openssldir=/usr/local/ssl shared zlib
sudo make && sudo make test && sudo make install
cd ../../
export OPENSSL_ROOT_DIR=/usr/local/ssl
mkdir p4-fusion
cd p4-fusion
[ -s "v1.12.tar.gz" ] || {
  wget https://github.com/salesforce/p4-fusion/archive/refs/tags/v1.12.tar.gz
  tar xzf v1.12.tar.gz
}
cd p4-fusion-1.12
curl -sSLO https://filehost.perforce.com/perforce/r22.2/bin.linux26x86_64/p4api-glibc2.3-openssl3.tgz
tar xzf p4api-glibc2.3-openssl3.tgz
rm -rf vendor/helix-core-api/linux
mkdir -p vendor/helix-core-api/linux
mv -f p4api-2022.2.2407422/include p4api-2022.2.2407422/lib vendor/helix-core-api/linux
rm -rf p4api-2022.2.2407422
ln -s /usr/local/ssl/lib64 /usr/local/ssl/lib
./generate_cache.sh Debug
./build.sh
ls -l "${PWD}/build/p4-fusion/p4-fusion"
```
