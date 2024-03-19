{ pkgs
, pkgsStatic
, lib
, stdenv
, fetchzip
, fetchFromGitHub
  # nativeBuildInputs only, the rest we fetch from pkgsStatic
, cmake
, patchelf
, pkg-config
, darwin
, hostPlatform
}:
let
  inherit (import ./util.nix { inherit lib; }) mkStatic unNixifyDylibs;
  http-parser-static = ((mkStatic pkgsStatic.http-parser).overrideAttrs (oldAttrs: {
    # http-parser makefile is a bit incomplete, so fill in the gaps here
    # to move the static object and header files to the right location
    # https://github.com/nodejs/http-parser/issues/310
    buildFlags = [ "package" ];
    installTargets = "package";
    postInstall = ''
      install -D libhttp_parser.a $out/lib/libhttp_parser.a
      install -D  http_parser.h $out/include/http_parser.h
      ls -la $out/lib $out/include
    '';
  }));
  libiconv-static = mkStatic pkgsStatic.libiconv;
  openssl-static = (mkStatic pkgsStatic.openssl).dev;
  pcre-static = (mkStatic pkgsStatic.pcre).dev;
  # pkgsStatic.zlib.static doesn't exist on linux, but does on macos
  zlib-static = (pkgsStatic.zlib.static or pkgsStatic.zlib);
in
unNixifyDylibs { inherit pkgs; } (pkgsStatic.gccStdenv.mkDerivation rec {
  name = "p4-fusion";
  version = "v1.12";

  srcs = [
    (fetchFromGitHub {
      inherit name;
      owner = "salesforce";
      repo = "p4-fusion";
      rev = "3ee482466464c18e6a635ff4f09cd75a2e1bfe0f";
      hash = "sha256-rUXuBoXuOUanWxutd7dNgjn2vLFvHQ0IgCIn9vG5dgs=";
    })
    (
      if hostPlatform.isMacOS then
        if hostPlatform.isAarch64 then
          fetchzip
            {
              name = "helix-core-api";
              url = "https://filehost.perforce.com/perforce/r22.2/bin.macosx12arm64/p4api-openssl3.tgz";
              hash = "sha256-6Bz26+1RixsK+zq6qeU3hbT8w+mNT7KBvqZ3Gnrkke4=";
            }
        else
          fetchzip {
            name = "helix-core-api";
            url = "https://filehost.perforce.com/perforce/r22.2/bin.macosx12x86_64/p4api-openssl3.tgz";
            hash = "sha256-/Ia9R+H95Yx4Sx7+Grke0d3QskuZ2YtH4LZOS7vRMZc=";
          }
      else if hostPlatform.isLinux then
        fetchzip
          {
            name = "helix-core-api";
            url = "https://filehost.perforce.com/perforce/r22.2/bin.linux26x86_64/p4api-glibc2.3-openssl3.tgz";
            hash = "sha256-tqWhdQQdOVAiGa6HiRajw4emoYRRRgZf6pZVEIf1qqU=";
          }
      else throw "unsupported platform ${stdenv.hostPlatform.parsed.kernel.name}"
    )
  ];

  sourceRoot = name;

  nativeBuildInputs = [
    patchelf
    pkg-config
    cmake
  ];

  buildInputs = [
    zlib-static
    http-parser-static
    pcre-static
    openssl-static
  ] ++ lib.optional hostPlatform.isMacOS [
    # iconv is bundled with glibc and apparently only needed for osx
    # https://sourcegraph.com/github.com/salesforce/p4-fusion@3ee482466464c18e6a635ff4f09cd75a2e1bfe0f/-/blob/vendor/libgit2/README.md?L178:3
    libiconv-static
    darwin.apple_sdk.frameworks.CFNetwork
    darwin.apple_sdk.frameworks.Cocoa
  ];

  # copy helix-core-api stuff into the expected directories, and statically link libstdc++
  preBuild = let dir = if hostPlatform.isMacOS then "mac" else "linux"; in
    ''
      mkdir -p $NIX_BUILD_TOP/$sourceRoot/vendor/helix-core-api/${dir}
      cp -R $NIX_BUILD_TOP/helix-core-api/* $NIX_BUILD_TOP/$sourceRoot/vendor/helix-core-api/${dir}

      sed -i "s/target_link_libraries(p4-fusion PUBLIC/target_link_libraries(p4-fusion PUBLIC -static-libstdc++/" \
        $NIX_BUILD_TOP/$sourceRoot/p4-fusion/CMakeLists.txt
    '';

  cmakeFlags = [
    # we want to statically link
    "-DBUILD_SHARED_LIBS=OFF"
    # Copied from upstream, where relevant
    # https://sourcegraph.com/github.com/salesforce/p4-fusion@3ee482466464c18e6a635ff4f09cd75a2e1bfe0f/-/blob/generate_cache.sh?L7-21
    "-DUSE_SSH=OFF"
    "-DUSE_HTTPS=OFF"
    "-DBUILD_CLAR=OFF"
    # salesforce don't link against GSSAPI in CI, so I won't either
    "-DUSE_GSSAPI=OFF"
    # prefer nix-provided http-parser instead of bundled
    "-DUSE_HTTP_PARSER=system"
  ];

  postInstall = ''
    mkdir -p "$out/bin"
    cp p4-fusion/p4-fusion "$out/bin/p4-fusion"
  '';

  meta = {
    homepage = "https://github.com/salesforce/p4-fusion";
    platforms = [ "x86_64-darwin" "aarch64-darwin" "x86_64-linux" ];
    license = lib.licenses.bsd3;
  };
})
