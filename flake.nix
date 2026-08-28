{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
  };

  outputs =
    inputs@{
      self,
      nixpkgs,
      flake-parts,
      ...
    }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [ "x86_64-linux" ];

      perSystem = { pkgs, ... }: {
        packages.vita-grid = pkgs.buildGoModule {
          pname = "vita-grid";
          version = "0.1.0";
          src = ./.;
          vendorHash = null;
          subPackages = [
            "aggregator"
            "statusd"
          ];
          env.CGO_ENABLED = "0";
        };
        packages.default = self.packages.${pkgs.stdenv.hostPlatform.system}.vita-grid;
      };

      flake = {
        overlays.default = final: prev: {
          vita-grid = self.packages.${final.stdenv.hostPlatform.system}.vita-grid;
        };
        nixosModules.vitaGrid = import ./modules/vitaGrid.nix;
      };
    };
}
