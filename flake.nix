{
  description = "reel-life — AI-powered chatops agent for media curation";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
      pkgsFor = system: import nixpkgs {
        inherit system;
        overlays = [ self.overlays.default ];
      };
    in
    {
      overlays.default = final: prev: {
        reel-life = final.callPackage ./nix/package.nix { };
      };

      packages = forAllSystems (system: let
        pkgs = pkgsFor system;
      in {
        default = pkgs.reel-life;
        reel-life = pkgs.reel-life;
      });

      checks = forAllSystems (system:
        let pkgs = pkgsFor system;
        in pkgs.lib.optionalAttrs pkgs.stdenv.isLinux {
          outcome-report = import ./nix/tests/outcome-report.nix { inherit pkgs; };
        });

      devShells = forAllSystems (system: let
        pkgs = pkgsFor system;
      in {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            golangci-lint
            gh
            git
          ];
        };
      });

      nixosModules.default = { ... }: {
        imports = [ ./nix/module.nix ];
        nixpkgs.overlays = [ self.overlays.default ];
      };
    };
}
