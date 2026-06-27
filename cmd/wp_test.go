package cmd

import (
	"reflect"
	"testing"
)

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
