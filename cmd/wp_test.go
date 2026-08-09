package cmd

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	sitectlplugin "github.com/libops/sitectl/pkg/plugin"
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

func TestWordPressComposerArgvPersistsIntoCheckout(t *testing.T) {
	t.Parallel()

	host := sitectlplugin.ComposeProjectHost{
		UID:                "1000",
		GID:                "1001",
		ProjectDir:         "/Users/operator/Library/Application Support/libops/templates/wordpress",
		HasNumericIdentity: true,
	}
	got := wordpressComposerArgv(host, "require", "vendor/package:^1.2")
	want := []string{
		"docker", "compose", "run", "--rm", "--no-deps",
		"--user", "1000:1001",
		"--volume", "/Users/operator/Library/Application Support/libops/templates/wordpress:/workspace:z",
		"--workdir", "/workspace",
		"--entrypoint", "composer", "wp",
		"require", "vendor/package:^1.2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wordpressComposerArgv() = %#v, want %#v", got, want)
	}
	if got[2] != "run" {
		t.Fatalf("Composer mutation must use a one-off container, got %#v", got)
	}
}

func TestWordPressComposerArgvOmitsPOSIXUserOnNativeWindows(t *testing.T) {
	t.Parallel()

	host := sitectlplugin.ComposeProjectHost{ProjectDir: `C:\Users\operator\wordpress`}
	got := wordpressComposerArgv(host, "install", "--no-interaction")
	want := []string{
		"docker", "compose", "run", "--rm", "--no-deps",
		"--volume", `C:\Users\operator\wordpress:/workspace:z`,
		"--workdir", "/workspace",
		"--entrypoint", "composer", "wp",
		"install", "--no-interaction",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wordpressComposerArgv() = %#v, want %#v", got, want)
	}
	for index, argument := range got {
		if argument == "--user" {
			t.Fatalf("native Windows Composer argv includes unsupported numeric user at index %d: %#v", index, got)
		}
	}
}

func TestWordPressDBExportCommands(t *testing.T) {
	t.Parallel()

	got, err := wordpressDBExportCommands("./backups/site.sql")
	if err != nil {
		t.Fatalf("wordpressDBExportCommands() error = %v", err)
	}
	want := [][]string{
		{"docker", "compose", "exec", "-T", "wp", "wp", "--allow-root", "--path=/var/www/bedrock/web/wp", "db", "export", "/tmp/site.sql"},
		{"docker", "compose", "cp", "wp:/tmp/site.sql", "./backups/site.sql"},
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
	want := [][]string{
		{"docker", "compose", "cp", "./backups/site.sql", "wp:/tmp/site.sql"},
		{"docker", "compose", "exec", "-T", "wp", "wp", "--allow-root", "--path=/var/www/bedrock/web/wp", "db", "import", "/tmp/site.sql"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wordpressDBImportCommands() = %#v, want %#v", got, want)
	}
}

func TestWordPressDBCommandsUseOnlyCrossPlatformDockerArgv(t *testing.T) {
	t.Parallel()

	for _, build := range []func(string) ([][]string, error){wordpressDBExportCommands, wordpressDBImportCommands} {
		commands, err := build("backups/site.sql")
		if err != nil {
			t.Fatal(err)
		}
		for _, command := range commands {
			if len(command) == 0 || command[0] != "docker" {
				t.Fatalf("database command uses an operator-host-specific executable: %#v", command)
			}
		}
	}
	_, _, containerPath, err := wordpressDBPaths(filepath.Join("backups", "site.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if containerPath != "/tmp/site.sql" {
		t.Fatalf("container database path = %q, want POSIX /tmp/site.sql", containerPath)
	}
}

func TestWordPressDBPathsRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	if _, _, _, err := wordpressDBPaths(" "); err == nil {
		t.Fatal("wordpressDBPaths() error = nil, want error")
	}
}
