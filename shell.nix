# Experimental support for developing in nix. Please reach out to @keegan or @noah if
# you encounter any issues.
#
# Things it does differently:
#
# - Runs postgres under ~/.sourcegraph with a unix socket. No need to manage a
#   service. Must remember to run "pg_ctl stop" if you want to stop it.
#
# Status: everything works on linux & darwin.
{ pkgs
, buildFHSEnv
, mkShell
, hostPlatform
, lib
, writeShellScriptBin
}:
let
  # On darwin, we let bazelisk manage the bazel version since we actually need to run two
  # different versions thanks to aspect. Additionally bazelisk allows us to do
  # things like "bazel configure". So we just install a script called bazel
  # which calls bazelisk.
  #
  # Additionally bazel seems to break when CC and CXX is set to a nix managed
  # compiler on darwin. So the script unsets those.
  bazel-wrapper = writeShellScriptBin "bazel" (if hostPlatform.isMacOS then ''
    export CC=/usr/bin/clang
    exec ${pkgs.bazelisk}/bin/bazelisk "$@"
  '' else ''
    unset TMPDIR TMP
    exec ${pkgs.bazel_7}/bin/bazel "$@"
  '');
  bazel-watcher = writeShellScriptBin "ibazel" ''
    ${lib.optionalString hostPlatform.isMacOS "export CC=/usr/bin/clang"}
    exec ${pkgs.bazel-watcher}/bin/ibazel \
      ${lib.optionalString hostPlatform.isLinux "-bazel_path=${bazel-fhs}/bin/bazel"} "$@"
  '';
  bazel-fhs = buildFHSEnv {
    name = "bazel";
    runScript = "bazel";
    targetPkgs = pkgs: (with pkgs; [
      bazel-wrapper
      zlib.dev
    ]);
    # unsharePid required to preserve bazel server between bazel invocations,
    # the rest are disabled just in case
    unsharePid = false;
    unshareUser = false;
    unshareIpc = false;
    unshareNet = false;
    unshareUts = false;
    unshareCgroup = false;
  };

  # pkgs.universal-ctags installs the binary as "ctags", not "universal-ctags"
  # like zoekt expects.
  universal-ctags = pkgs.writeScriptBin "universal-ctags" ''
    #!${pkgs.stdenv.shell}
    exec ${pkgs.universal-ctags}/bin/ctags "$@"
  '';

  # We have scripts which use gsed on darwin since that is what homebrew calls
  # the binary for GNU sed.
  gsed = pkgs.writeShellScriptBin "gsed" ''exec ${pkgs.gnused}/bin/sed "$@"'';
in
mkShell.override { stdenv = if hostPlatform.isMacOS then pkgs.clang11Stdenv else pkgs.stdenv; } {
  name = "sourcegraph-dev";

  # The packages in the `buildInputs` list will be added to the PATH in our shell
  nativeBuildInputs = with pkgs; [
    bashInteractive
    zip

    # nix language server.
    nil

    # Our core DB.
    postgresql_13

    # Cache and some store data.
    redis

    # Used by symbols and zoekt-git-index to extract symbols from sourcecode.
    universal-ctags

    # Build our backend. Sometimes newer :^)
    go_1_22

    # Lots of our tooling and go tests rely on git et al.
    comby
    git
    git-lfs
    gsed
    nssTools
    parallel

    # CI lint tools you need locally.
    shfmt
    shellcheck

    # Web tools.
    nodejs-20_x
    nodejs-20_x.pkgs.pnpm
    nodejs-20_x.pkgs.typescript
    nodejs-20_x.pkgs.typescript-language-server

    # Rust utils for syntax-highlighter service, currently not pinned to the same versions.
    cargo
    rustc
    rustfmt
    libiconv
    clippy

    bazel-buildtools
  ] ++ lib.optional hostPlatform.isLinux (with pkgs; [
    # bazel via nix is broken on MacOS for us. Lets just rely on bazelisk from brew.
    # special sauce bazel stuff.
    bazelisk # needed to please sg, but not used directly by us
    bazel-fhs
    bazel-watcher
  ]) ++ lib.optional hostPlatform.isMacOS [ bazel-wrapper ];

  # Startup postgres, redis & set nixos specific stuff
  shellHook = ''
    set -h # command hashmap is not guaranteed to be enabled, but required by sg
    . ./dev/nix/shell-hook.sh
  '';

  # Fix for using Delve https://github.com/sourcegraph/sourcegraph/pull/35885
  hardeningDisable = [ "fortify" ];

  # By explicitly setting this environment variable we avoid starting up
  # universal-ctags via docker.
  CTAGS_COMMAND = "${universal-ctags}/bin/universal-ctags";

  RUST_SRC_PATH = "${pkgs.rust.packages.stable.rustPlatform.rustLibSrc}";

  # Some of the bazel actions require some tools assumed to be in the PATH defined by the "strict action env" that we enable
  # through --incompatible_strict_action_env. We can poke a custom PATH through with --action_env=PATH=$BAZEL_ACTION_PATH.
  # See https://sourcegraph.com/github.com/bazelbuild/bazel@6.1.2/-/blob/src/main/java/com/google/devtools/build/lib/bazel/rules/BazelRuleClassProvider.java?L532-547
  BAZEL_ACTION_PATH = with pkgs; lib.makeBinPath [ file hexdump bashInteractive stdenv.cc coreutils unzip zip curl gzip gnutar gnugrep gnused git patch openssh findutils perl python39 which postgresql_13 docker-credential-helpers ];

  # bazel complains when the bazel version differs even by a patch version to whats defined in .bazelversion,
  # so we tell it to h*ck off here.
  # https://sourcegraph.com/github.com/bazelbuild/bazel@1a4da7f331c753c92e2c91efcad434dc29d10d43/-/blob/scripts/packages/bazel.sh?L23-28
  USE_BAZEL_VERSION = if hostPlatform.isMacOS then "" else pkgs.bazel_7.version;

  LIBTOOL = if hostPlatform.isMacOS then "/usr/bin/libtool" else "";
}
