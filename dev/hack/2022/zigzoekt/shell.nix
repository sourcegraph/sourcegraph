let
  pkgs = import
    (fetchTarball {
      url =
        "https://github.com/NixOS/nixpkgs/archive/350731a856a1d901b0d26f6c9892785a63f48e17.tar.gz";
      sha256 = "1pbr62q57pcbfr9pnkr02p134ljrachp704j4f9qa86i08njfy0j";
    }){};

in
pkgs.mkShell {
  name = "zoekt-zig-dev";

  nativeBuildInputs = with pkgs; [
    zig
    zls
  ];
}
