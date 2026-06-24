{
  description = "Pin and update GitHub Actions dependencies to immutable commit SHAs";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      version = "0.1.5";
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
          hash = "sha256-3YO5v24jM/oP9dqbXTUui/moAjAbc7OLz/C5NQFlCaI=";
        };
        aarch64-linux = {
          artifact = "sanad_${version}_Linux_arm64.tar.gz";
          hash = "sha256-V+gAiQjwhfo72gPixeuRLm7Yk3eDqz/Ry0ODiCRu4GQ=";
        };
        x86_64-darwin = {
          artifact = "sanad_${version}_Darwin_x86_64.tar.gz";
          hash = "sha256-lSd1qebFgL6BFuSqrTp+Z7WUWVE8pmbO9O5pC+Ol1OU=";
        };
        aarch64-darwin = {
          artifact = "sanad_${version}_Darwin_arm64.tar.gz";
          hash = "sha256-Yq4TZUwRF3Ug79Wc+A+FI7WSTnWrN5xfc8H8WbZ5GmA=";
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
