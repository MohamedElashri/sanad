package main

import (
	"strings"
	"testing"
)

func TestUpdateFlake(t *testing.T) {
	checksums := map[string]string{
		"sanad_1.2.3_Linux_x86_64.tar.gz":  "43a80a7f0a351355984d995f941ea3b9bbd08883abc82341545df252f38fd4b0",
		"sanad_1.2.3_Linux_arm64.tar.gz":   "7a1b0c86aa9ab1ac0471c2d53c1bfe33a703ab25e82b62863aa8bc8c1be74277",
		"sanad_1.2.3_Darwin_x86_64.tar.gz": "0741cbe898422b13568b0a8a8fe2b9033af7e5985f1cfe1d8a455cffef6f1afc",
		"sanad_1.2.3_Darwin_arm64.tar.gz":  "77227ba33cf7584b0e8e33026b1c02be0a6996e49abce2b191713f71d6bcebcc",
	}

	got, err := updateFlake(sampleFlake, "1.2.3", checksums)
	if err != nil {
		t.Fatalf("updateFlake returned error: %v", err)
	}

	wants := []string{
		`version = "1.2.3";`,
		`hash = "sha256-Q6gKfwo1E1WYTZlflB6jubvQiIOryCNBVF3yUvOP1LA=";`,
		`hash = "sha256-ehsMhqqasawEccLVPBv+M6cDqyXoK2KGOqi8jBvnQnc=";`,
		`hash = "sha256-B0HL6JhCKxNWiwqKj+K5Azr35ZhfHP4dikVc/+9vGvw=";`,
		`hash = "sha256-dyJ7ozz3WEsOjjMCaxwCvgppluSavOKxkXE/cda868w=";`,
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("updated flake missing %q:\n%s", want, got)
		}
	}
}

func TestUpdateFlakeRequiresChecksums(t *testing.T) {
	_, err := updateFlake(sampleFlake, "1.2.3", map[string]string{})
	if err == nil {
		t.Fatal("updateFlake returned nil error for missing checksums")
	}
}

const sampleFlake = `{
  description = "Pin and update GitHub Actions dependencies to immutable commit SHAs";

  outputs = { self, nixpkgs }:
    let
      version = "0.1.0";
      releaseSources = {
        x86_64-linux = {
          artifact = "sanad_${version}_Linux_x86_64.tar.gz";
          hash = "sha256-old=";
        };
        aarch64-linux = {
          artifact = "sanad_${version}_Linux_arm64.tar.gz";
          hash = "sha256-old=";
        };
        x86_64-darwin = {
          artifact = "sanad_${version}_Darwin_x86_64.tar.gz";
          hash = "sha256-old=";
        };
        aarch64-darwin = {
          artifact = "sanad_${version}_Darwin_arm64.tar.gz";
          hash = "sha256-old=";
        };
      };
    in
    {};
}
`
