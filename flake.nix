{
  description = "episki — official command-line interface";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  inputs.flake-utils.url = "github:numtide/flake-utils";

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let pkgs = import nixpkgs { inherit system; };
      in {
        packages.default = pkgs.buildGoModule {
          pname = "episki";
          version = "1.0.0"; # x-release-please-version
          src = ./.;
          # Dependencies are vendored (vendor/ is committed), so no fetch
          # hash is needed and the build is reproducible as-is.
          vendorHash = null;
          subPackages = [ "cmd/episki" ];
          ldflags = [ "-s" "-w" "-X main.Version=1.0.0" ]; # x-release-please-version
          meta = {
            description = "episki CLI";
            homepage = "https://github.com/episki/episki-cli";
            mainProgram = "episki";
          };
        };

        devShells.default = pkgs.mkShell {
          packages = [ pkgs.go pkgs.goreleaser ];
        };
      });
}
