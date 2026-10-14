{
  description = "grower development shell";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { nixpkgs, ... }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" ];
      forAll = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in {
      devShells = forAll (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go_1_26
            nodejs_24
            gnumake
            golangci-lint
            imagemagick # regenerates PWA icons (web/scripts/icons.sh)
            playwright-driver.browsers
          ];

          # Use nixpkgs' Playwright browsers; npm-downloaded ones don't run on NixOS.
          # Keep @playwright/test in web/package.json at the same version as playwright-driver.
          PLAYWRIGHT_BROWSERS_PATH = "${pkgs.playwright-driver.browsers}";
          PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD = "1";
          PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS = "true";

          shellHook = ''
            echo "grower dev shell: go $(go env GOVERSION), node $(node --version), playwright ${pkgs.playwright-driver.version}"
          '';
        };
      });
    };
}
