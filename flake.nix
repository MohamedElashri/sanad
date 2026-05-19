{
  description = "Pin and update GitHub Actions dependencies to immutable commit SHAs";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      version = "0.1.2";
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
      releaseSources = {
        x86_64-linux = {
          artifact = "sanad_${version}_Linux_x86_64.tar.gz";
          hash = "sha256-PSaWguN9FGXg4ElFN5UQ3zY7hoVnNklUQ9fEENbPE7c=";
        };
        aarch64-linux = {
          artifact = "sanad_${version}_Linux_arm64.tar.gz";
          hash = "sha256-1i6lDjFIYCvWNxy8rtOJ8ZH9CjS/7EL0D72GaDqgE3Q=";
        };
        x86_64-darwin = {
          artifact = "sanad_${version}_Darwin_x86_64.tar.gz";
          hash = "sha256-Uzajbhi+AlGF07h1+OkLAidLM4VRZSK+lL5le9zl3yQ=";
        };
        aarch64-darwin = {
          artifact = "sanad_${version}_Darwin_arm64.tar.gz";
          hash = "sha256-bFmMYqbiYbqLJcDMg4UzbYkg3BWJTG1sECQAfYtbd4U=";
        };
      };
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
          source = releaseSources.${system};
          sanad = pkgs.stdenvNoCC.mkDerivation {
            pname = "sanad";
            inherit version;

            src = pkgs.fetchurl {
              url = "https://github.com/MohamedElashri/sanad/releases/download/v${version}/${source.artifact}";
              hash = source.hash;
            };

            sourceRoot = ".";

            installPhase = ''
              runHook preInstall
              install -Dm755 sanad "$out/bin/sanad"
              runHook postInstall
            '';

            meta = with pkgs.lib; {
              description = "Pin and update GitHub Actions dependencies to immutable commit SHAs";
              homepage = "https://github.com/MohamedElashri/sanad";
              license = licenses.mit;
              mainProgram = "sanad";
              platforms = systems;
            };
          };
        in
        {
          inherit sanad;
          default = sanad;
        });

      apps = forAllSystems (system: {
        sanad = {
          type = "app";
          program = "${self.packages.${system}.sanad}/bin/sanad";
        };
        default = self.apps.${system}.sanad;
      });
    };
}
