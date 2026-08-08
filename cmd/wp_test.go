package cmd

import (
	"reflect"
	"strings"
	"testing"
)

func TestComposerManagedUpdateArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind string
		args []string
		want []string
	}{
		{name: "one plugin", kind: "plugin", args: []string{"akismet:5.8"}, want: []string{"require", "wpackagist-plugin/akismet:5.8", "--with-all-dependencies", "--no-interaction"}},
		{name: "two themes", kind: "theme", args: []string{"twentytwentyfour:^1.6", "twentytwentythree:1.7"}, want: []string{"require", "wpackagist-theme/twentytwentyfour:^1.6", "wpackagist-theme/twentytwentythree:1.7", "--with-all-dependencies", "--no-interaction"}},
		{name: "core", kind: "core", args: []string{"7.1.0"}, want: []string{"require", "roots/wordpress:7.1.0", "--with-all-dependencies", "--no-interaction"}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := wordpressComposerUpdateArgs(test.kind, test.args)
			if err != nil {
				t.Fatalf("wordpressComposerUpdateArgs() error = %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("wordpressComposerUpdateArgs() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestComposerManagedUpdateArgumentsRejectWPCLIFlagsAndInvalidSlugs(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{}, {"--minor"}, {"../../plugin:1.0"}, {"--all"}, {"akismet"}, {"akismet:not a constraint"}} {
		if _, err := wordpressComposerUpdateArgs("plugin", args); err == nil {
			t.Fatalf("wordpressComposerUpdateArgs(plugin, %q) error = nil", args)
		}
	}
	for _, args := range [][]string{{}, {"--minor"}, {"7.0.2", "7.1.0"}} {
		if _, err := wordpressComposerUpdateArgs("core", args); err == nil {
			t.Fatalf("wordpressComposerUpdateArgs(core, %q) error = nil", args)
		}
	}
}

func TestRawWPCLIRejectsComposerOwnedCodeMutation(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"plugin", "update", "--all"},
		{"--url=https://example.org", "theme", "install", "example"},
		{"plugin", "--quiet", "update", "akismet"},
		{"theme", "--url=https://example.org", "delete", "example"},
		{"core", "update"},
		{"core", "--quiet", "download"},
		{"plugin", "delete", "akismet"},
	} {
		if err := rejectComposerManagedWPCLIMutation(args); err == nil || !strings.Contains(err.Error(), "sitectl wp composer") {
			t.Fatalf("rejectComposerManagedWPCLIMutation(%q) = %v, want Composer guidance", args, err)
		}
	}
	for _, args := range [][]string{
		{"plugin", "list"},
		{"plugin", "activate", "akismet"},
		{"core", "update-db"},
		{"option", "get", "plugin", "update"},
		{"eval", `echo "plugin update";`},
	} {
		if err := rejectComposerManagedWPCLIMutation(args); err != nil {
			t.Fatalf("read/state WP-CLI command %q rejected: %v", args, err)
		}
	}
}

func TestWordPressComposerCommandPersistsIntoCheckout(t *testing.T) {
	t.Parallel()

	got := wordpressComposerCommand("require", "vendor/package:^1.2")
	want := `docker compose run --rm --no-deps --user "$(id -u):$(id -g)" --volume "$PWD:/workspace:z" --workdir /workspace --entrypoint composer wp 'require' 'vendor/package:^1.2'`
	if got != want {
		t.Fatalf("wordpressComposerCommand() = %q, want %q", got, want)
	}
	for _, required := range []string{"docker compose run", `--volume "$PWD:/workspace:z"`, "--workdir /workspace", "--entrypoint composer", "'require' 'vendor/package:^1.2'"} {
		if !strings.Contains(got, required) {
			t.Fatalf("persistent Composer command is missing %q: %s", required, got)
		}
	}
	if strings.Contains(got, "docker compose exec") {
		t.Fatalf("Composer mutation must not target the disposable running image: %s", got)
	}
}

func TestWordPressDBExportCommands(t *testing.T) {
	t.Parallel()

	got, err := wordpressDBExportCommands("./backups/site.sql")
	if err != nil {
		t.Fatalf("wordpressDBExportCommands() error = %v", err)
	}
	want := []string{
		"'mkdir' '-p' 'backups'",
		"'docker' 'compose' 'exec' '-T' 'wp' 'wp' '--allow-root' '--path=/var/www/bedrock/web/wp' 'db' 'export' '/tmp/site.sql'",
		"'docker' 'compose' 'cp' 'wp:/tmp/site.sql' './backups/site.sql'",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wordpressDBExportCommands() = %#v, want %#v", got, want)
	}
}

func TestWordPressDBImportCommands(t *testing.T) {
	t.Parallel()

	got, err := wordpressDBImportCommands("./backups/site.sql")
	if err != nil {
		t.Fatalf("wordpressDBImportCommands() error = %v", err)
	}
	want := []string{
		"'test' '-f' './backups/site.sql'",
		"'docker' 'compose' 'cp' './backups/site.sql' 'wp:/tmp/site.sql'",
		"'docker' 'compose' 'exec' '-T' 'wp' 'wp' '--allow-root' '--path=/var/www/bedrock/web/wp' 'db' 'import' '/tmp/site.sql'",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wordpressDBImportCommands() = %#v, want %#v", got, want)
	}
}

func TestWordPressDBPathsRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	if _, _, _, err := wordpressDBPaths(" "); err == nil {
		t.Fatal("wordpressDBPaths() error = nil, want error")
	}
}
