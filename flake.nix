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
          # Bumped automatically by release-please.
          version = "0.0.0";
          src = ./.;
          # Replaced by `nix build` once go.sum exists; until then run
          # `nix build --update-input nixpkgs` after `go mod tidy`.
          vendorHash = null;
          subPackages = [ "cmd/episki" ];
          ldflags = [ "-s" "-w" "-X main.Version=0.0.0" ];
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
