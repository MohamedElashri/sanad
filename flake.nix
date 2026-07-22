{
  description = "Pin and update GitHub Actions dependencies to immutable commit SHAs";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      version = "0.2.0";
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
          hash = "sha256-Esqvj77BMQWiQZ+AiUvqYUrmoEJspccJcv+5YdMs9mY=";
        };
        aarch64-linux = {
          artifact = "sanad_${version}_Linux_arm64.tar.gz";
          hash = "sha256-svc96bL2TAWHaVpQcmUd5UKQTZR7MLcki6HFTmEHfsg=";
        };
        x86_64-darwin = {
          artifact = "sanad_${version}_Darwin_x86_64.tar.gz";
          hash = "sha256-z0paxbKnICF+MCJ09qs8bVjwyBBuOMKr/E4om9IGtwg=";
        };
        aarch64-darwin = {
          artifact = "sanad_${version}_Darwin_arm64.tar.gz";
          hash = "sha256-Z7MhtSPBYpJ+VlNb/huYIZ8Q5tsNiY8xqnWVVpGHA6w=";
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
