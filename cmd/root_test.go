package cmd

import (
	"strings"
	"testing"

	"github.com/libops/sitectl/pkg/plugin"
)

func TestCreateDefinitionLifecycleContract(t *testing.T) {
	t.Parallel()
	spec := createDefinition()
	if len(spec.Images) != 1 || spec.Images[0].Image != "libops/wp:nginx-1.30.3-php84" || spec.Images[0].BuildPolicy != plugin.BuildPolicyAlways {
		t.Fatalf("unexpected WordPress image contract: %+v", spec.Images)
	}
	if len(spec.DockerComposeUp) != 1 || !strings.Contains(spec.DockerComposeUp[0], "--wait --wait-timeout 600") {
		t.Fatalf("create must wait for service health before reporting ready: %+v", spec.DockerComposeUp)
	}
	rollout := strings.Join(spec.DockerComposeRollout, "\n")
	if !strings.Contains(rollout, "core update-db") {
		t.Fatalf("WordPress schema migration must run and fail hard:\n%s", rollout)
	}
	foundMigration := false
	for index, command := range spec.DockerComposeRollout {
		if !strings.Contains(command, "core update-db") {
			continue
		}
		if strings.Contains(command, "||") || index < 2 || !strings.Contains(spec.DockerComposeRollout[index-1], "test -f /installed") || !strings.Contains(spec.DockerComposeRollout[index-1], "-ge 150") {
			t.Fatalf("WordPress migration must fail hard after bounded readiness: %+v", spec.DockerComposeRollout)
		}
		initialStart := spec.DockerComposeRollout[index-2]
		wantInitialStart := "docker compose up --remove-orphans --pull missing --quiet-pull -d wp"
		if initialStart != wantInitialStart ||
			!strings.HasSuffix(initialStart, " -d wp") ||
			strings.Contains(initialStart, "--wait") {
			t.Fatalf("initial rollout start must target only WordPress without waiting: %q", initialStart)
		}
		if index+2 >= len(spec.DockerComposeRollout) || !strings.Contains(spec.DockerComposeRollout[index+1], "cache flush") {
			t.Fatalf("cache flush and bounded fail-hard final health wait must follow migration: %+v", spec.DockerComposeRollout)
		}
		finalStart := spec.DockerComposeRollout[index+2]
		wantFinalStart := "docker compose up --remove-orphans --wait --wait-timeout 600 --pull missing --quiet-pull -d"
		if finalStart != wantFinalStart ||
			!strings.Contains(finalStart, "--wait --wait-timeout 600") ||
			!strings.HasSuffix(finalStart, " -d") ||
			strings.Contains(finalStart, "||") {
			t.Fatalf("final rollout start must wait for the full stack and fail hard: %q", finalStart)
		}
		foundMigration = true
		break
	}
	if !foundMigration {
		t.Fatalf("WordPress schema migration command not found: %+v", spec.DockerComposeRollout)
	}

	sdk := plugin.NewSDK(plugin.Metadata{Name: "wp"})
	RegisterCommands(sdk)
	var foundDevMode bool
	for _, definition := range sdk.LocalComponentDefinitions() {
		foundDevMode = foundDevMode || definition.Name == "dev-mode"
	}
	if !foundDevMode {
		t.Fatal("Composer-owned WordPress checkout must expose dev-mode")
	}
	volumes := strings.Join(wordpressDevModeVolumes, "\n")
	for _, want := range []string{
		"./web/app/mu-plugins:/var/www/bedrock/web/app/mu-plugins:z,rw",
		"./web/app/plugins:/var/www/bedrock/web/app/plugins:z,rw",
		"./web/app/themes:/var/www/bedrock/web/app/themes:z,rw",
	} {
		if !strings.Contains(volumes, want) {
			t.Fatalf("WordPress dev mode must mount Composer installer destination %q: %v", want, wordpressDevModeVolumes)
		}
	}
}
