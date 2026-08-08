package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	sitevalidate "github.com/libops/sitectl/pkg/validate"
)

type fakeWordPressVerifyRuntime struct {
	run func([]string) (string, error)
}

func (f fakeWordPressVerifyRuntime) ExecCapture(_ context.Context, argv []string) (string, error) {
	return f.run(argv)
}

func TestWordPressVerifyChecksApplicationAndLockBehavior(t *testing.T) {
	t.Parallel()

	runtime := fakeWordPressVerifyRuntime{run: func(argv []string) (string, error) {
		joined := strings.Join(argv, " ")
		switch {
		case strings.HasPrefix(joined, "test -r "):
			return "", nil
		case strings.Contains(joined, "core version"):
			return "7.0.2", nil
		case strings.Contains(joined, wordpressLockedCoreScript):
			return "7.0.2", nil
		case strings.Contains(joined, "SELECT CURRENT_USER()"):
			return "wordpress@%", nil
		case strings.Contains(joined, "/wp-json/"):
			return `{"name":"Library","namespaces":["wp/v2"]}`, nil
		case strings.Contains(joined, "cron event list"):
			return `[{"hook":"wp_version_check","next_run_gmt":"2026-08-08 12:00:00"}]`, nil
		case strings.Contains(joined, wordpressRuntimeStateScript):
			return `{"home":"https://wp.example.org","siteurl":"https://wp.example.org/wp","uploads":"/var/www/bedrock/web/app/uploads","writable":true}`, nil
		case strings.Contains(joined, "user get admin"):
			return "1", nil
		default:
			return "", errors.New("unexpected command: " + joined)
		}
	}}

	results := runWordPressVerifyChecks(context.Background(), runtime, false)
	assertAllWordPressVerifyOK(t, results, 6)
}

func TestWordPressVerifyFailsWhenRuntimeDiffersFromComposerLock(t *testing.T) {
	t.Parallel()

	result := wordpressVersionResult("7.0.1", "7.0.2", nil, nil)
	if result.Status != sitevalidate.StatusFailed || !strings.Contains(result.Detail, "7.0.1") || !strings.Contains(result.Detail, "7.0.2") {
		t.Fatalf("runtime/lock drift was not rejected: %+v", result)
	}
}

func TestWordPressVerifyDisposableModeRunsMediaRoundTrip(t *testing.T) {
	t.Parallel()

	var mediaCommand string
	runtime := fakeWordPressVerifyRuntime{run: func(argv []string) (string, error) {
		joined := strings.Join(argv, " ")
		switch {
		case strings.HasPrefix(joined, "test -r "):
			return "", nil
		case strings.Contains(joined, "core version"):
			return "7.0.2", nil
		case strings.Contains(joined, wordpressLockedCoreScript):
			return "7.0.2", nil
		case strings.Contains(joined, "SELECT CURRENT_USER()"):
			return "wordpress@%", nil
		case strings.Contains(joined, "/wp-json/"):
			return `{"name":"Library","namespaces":["wp/v2"]}`, nil
		case strings.Contains(joined, "cron event list"):
			return `[{"hook":"wp_version_check","next_run_gmt":"2026-08-08 12:00:00"}]`, nil
		case strings.Contains(joined, wordpressRuntimeStateScript):
			return `{"home":"http://localhost","siteurl":"http://localhost/wp","uploads":"/var/www/bedrock/web/app/uploads","writable":true}`, nil
		case strings.Contains(joined, "user get admin"):
			return "1", nil
		case strings.Contains(joined, wordpressMediaRoundTripScript):
			mediaCommand = joined
			return "media round trip complete", nil
		default:
			return "", errors.New("unexpected command: " + joined)
		}
	}}

	results := runWordPressVerifyChecks(context.Background(), runtime, true)
	assertAllWordPressVerifyOK(t, results, 7)
	for _, required := range []string{"s6-setuidgid nginx sh", wordpressMediaRoundTripScript} {
		if !strings.Contains(mediaCommand, required) {
			t.Fatalf("disposable media probe missing %q: %s", required, mediaCommand)
		}
	}
}

func TestWordPressVerifyRunsWPCLIAsServiceAccount(t *testing.T) {
	t.Parallel()

	argv := wordpressWPArgv("core", "version")
	joined := strings.Join(argv, " ")
	if !strings.HasPrefix(joined, "s6-setuidgid nginx wp ") {
		t.Fatalf("WordPress verification does not run as the service account: %s", joined)
	}
	if strings.Contains(joined, "--allow-root") {
		t.Fatalf("WordPress verification still requests root execution: %s", joined)
	}
}

func TestWordPressRuntimeProbesExecuteMountedFiles(t *testing.T) {
	t.Parallel()

	for name, script := range map[string]string{
		"locked core":      wordpressLockedCoreScript,
		"runtime state":    wordpressRuntimeStateScript,
		"media fixture":    wordpressMediaFixtureScript,
		"media round trip": wordpressMediaRoundTripScript,
		"wait installed":   wordpressWaitInstalledScript,
	} {
		if !strings.HasPrefix(script, "/usr/local/lib/sitectl/") {
			t.Fatalf("%s probe is not a mounted script path: %s", name, script)
		}
	}
}

func TestWordPressVerifyExplainsMissingTemplatePrograms(t *testing.T) {
	t.Parallel()

	runtime := fakeWordPressVerifyRuntime{run: func(argv []string) (string, error) {
		if strings.Contains(strings.Join(argv, " "), wordpressRuntimeStateScript) {
			return "", errors.New("not found")
		}
		return "", nil
	}}
	results := runWordPressVerifyChecks(context.Background(), runtime, false)
	if len(results) != 1 || results[0].Status != sitevalidate.StatusFailed || !strings.Contains(results[0].FixHint, "migrate the site checkout") {
		t.Fatalf("missing template program did not produce a migration diagnostic: %+v", results)
	}
}

func TestWordPressVerifyRejectsEmptyCronSchedule(t *testing.T) {
	t.Parallel()

	result := wordpressCronResult(`[]`, nil)
	if result.Status != sitevalidate.StatusFailed {
		t.Fatalf("empty cron schedule was accepted: %+v", result)
	}
}

func TestWordPressVerifyRejectsRootDatabaseAndMalformedREST(t *testing.T) {
	t.Parallel()

	if result := wordpressDatabaseResult("root@localhost", nil); result.Status != sitevalidate.StatusFailed {
		t.Fatalf("root database identity was accepted: %+v", result)
	}
	if result := wordpressRESTResult(`{"name":"Library","namespaces":[]}`, nil); result.Status != sitevalidate.StatusFailed {
		t.Fatalf("REST index without wp/v2 was accepted: %+v", result)
	}
}

func TestWordPressVerifyRejectsNonHTTPOrCredentialedCanonicalURLs(t *testing.T) {
	t.Parallel()

	for _, output := range []string{
		`{"home":"ftp://wp.example.org","siteurl":"ftp://wp.example.org/wp","uploads":"/var/www/bedrock/web/app/uploads","writable":true}`,
		`{"home":"https://user:password@wp.example.org","siteurl":"https://user:password@wp.example.org/wp","uploads":"/var/www/bedrock/web/app/uploads","writable":true}`,
	} {
		if result := wordpressRuntimeResult(output, nil); result.Status != sitevalidate.StatusFailed {
			t.Fatalf("unsafe canonical URL was accepted: %+v", result)
		}
	}
}

func assertAllWordPressVerifyOK(t *testing.T, results []sitevalidate.Result, want int) {
	t.Helper()
	if len(results) != want {
		t.Fatalf("verification results = %d, want %d: %+v", len(results), want, results)
	}
	for _, result := range results {
		if result.Status != sitevalidate.StatusOK {
			t.Fatalf("verification result is not OK: %+v", result)
		}
	}
}
