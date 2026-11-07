{
  description = "Adopt and update native GitHub Actions lockfiles";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      version = "1.0.2";
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
          hash = "sha256-IAuCItAH67VLyTlJGWT4Ow5GE78UIo+btazDeWahb/w=";
        };
        aarch64-linux = {
          artifact = "sanad_${version}_Linux_arm64.tar.gz";
          hash = "sha256-TgBYgwM4nGQJyHLVB4QBvRTqZvTWK+JvWdWH276I59M=";
        };
        x86_64-darwin = {
          artifact = "sanad_${version}_Darwin_x86_64.tar.gz";
          hash = "sha256-w7ne/PXytoDBikDU0+j6dWG74p2AWXmZYSmPUb3Y8uQ=";
        };
        aarch64-darwin = {
          artifact = "sanad_${version}_Darwin_arm64.tar.gz";
          hash = "sha256-xfe5w99wDfnB0t3aJSrZ0u9jnfy+osRia6e0Jb7whMg=";
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

            nativeBuildInputs = [
              pkgs.installShellFiles
            ];

            installPhase = ''
              runHook preInstall
              install -Dm755 sanad "$out/bin/sanad"
              if test -f gh-actions-lock; then
                install -Dm755 gh-actions-lock "$out/bin/gh-actions-lock"
              fi
              "$out/bin/sanad" completion bash > sanad.bash
              "$out/bin/sanad" completion zsh > _sanad
              "$out/bin/sanad" completion fish > sanad.fish
              installShellCompletion --cmd sanad \
                --bash sanad.bash \
                --zsh _sanad \
                --fish sanad.fish
              runHook postInstall
            '';

            meta = with pkgs.lib; {
              description = "Adopt and update native GitHub Actions lockfiles";
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
