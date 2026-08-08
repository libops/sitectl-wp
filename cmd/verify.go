package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/libops/sitectl/pkg/config"
	"github.com/libops/sitectl/pkg/docker"
	"github.com/libops/sitectl/pkg/plugin"
	sitevalidate "github.com/libops/sitectl/pkg/validate"
	"github.com/spf13/cobra"
)

const (
	wordpressRoot            = "/var/www/bedrock"
	wordpressRuntimeUser     = "nginx"
	wordpressLockedCoreProbe = `$lock = json_decode(file_get_contents("/var/www/bedrock/composer.lock"), true, 512, JSON_THROW_ON_ERROR); foreach (array_merge($lock["packages"] ?? [], $lock["packages-dev"] ?? []) as $package) { if (($package["name"] ?? "") === "roots/wordpress") { echo ltrim($package["version"] ?? "", "v"); exit(0); } } fwrite(STDERR, "roots/wordpress is absent from composer.lock\n"); exit(2);`
	wordpressRuntimeProbe    = `$uploads = wp_upload_dir(null, false); echo wp_json_encode(["home" => home_url(), "siteurl" => site_url(), "uploads" => $uploads["basedir"], "writable" => wp_is_writable($uploads["basedir"])]);`
	wordpressMediaRoundTrip  = `tmp=/tmp/sitectl-verify-$$.png; attachment=; wpv() { wp --path=/var/www/bedrock/web/wp "$@"; }; cleanup() { if [ -n "$attachment" ]; then wpv post delete "$attachment" --force >/dev/null 2>&1 || true; fi; rm -f -- "$tmp"; }; trap cleanup EXIT INT TERM; php -r 'file_put_contents($argv[1], base64_decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="));' "$tmp"; attachment=$(wpv media import "$tmp" --porcelain); case "$attachment" in ''|*[!0-9]*) echo "media import returned an invalid attachment ID" >&2; exit 3 ;; esac; test "$(wpv post get "$attachment" --field=ID)" = "$attachment"; wpv post delete "$attachment" --force >/dev/null; attachment=; cleanup; trap - EXIT INT TERM; printf '%s\n' 'media round trip complete'`
)

type wordpressVerifyRuntime interface {
	ExecCapture(context.Context, []string) (string, error)
}

type dockerWordPressVerifyRuntime struct {
	client    *docker.DockerClient
	container string
}

func (r dockerWordPressVerifyRuntime) ExecCapture(ctx context.Context, argv []string) (string, error) {
	return docker.ExecCapture(ctx, r.client, r.container, wordpressRoot, argv)
}

type wordpressVerifyRunner struct {
	sdk        *plugin.SDK
	disposable bool
}

func (r *wordpressVerifyRunner) BindFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&r.disposable, "disposable", false, "Import, read, and delete a media fixture. Use only for a disposable CI site, never a retained site.")
}

func (r *wordpressVerifyRunner) Run(cmd *cobra.Command, _ *config.Context) ([]sitevalidate.Result, error) {
	if r.sdk == nil {
		return nil, fmt.Errorf("WordPress verifier SDK is not initialized")
	}
	verifyContext, err := r.sdk.GetContext()
	if err != nil {
		return nil, err
	}
	client, err := r.sdk.GetDockerClient()
	if err != nil {
		return nil, fmt.Errorf("connect to Docker for WordPress verification: %w", err)
	}
	defer func() { _ = client.Close() }()
	container, err := client.GetContainerNameContext(cmd.Context(), verifyContext, wordpressService)
	if err != nil {
		return nil, fmt.Errorf("find running WordPress container: %w", err)
	}
	return runWordPressVerifyChecks(cmd.Context(), dockerWordPressVerifyRuntime{client: client, container: container}, r.disposable), nil
}

func runWordPressVerifyChecks(ctx context.Context, runtime wordpressVerifyRuntime, disposable bool) []sitevalidate.Result {
	capacity := 6
	if disposable {
		capacity++
	}
	results := make([]sitevalidate.Result, 0, capacity)

	coreOutput, coreErr := runtime.ExecCapture(ctx, wordpressWPArgv("core", "version", "--skip-plugins", "--skip-themes"))
	lockOutput, lockErr := runtime.ExecCapture(ctx, []string{"php", "-r", wordpressLockedCoreProbe})
	results = append(results, wordpressVersionResult(coreOutput, lockOutput, coreErr, lockErr))

	databaseOutput, databaseErr := runtime.ExecCapture(ctx, wordpressWPArgv("db", "query", "SELECT CURRENT_USER();", "--skip-column-names", "--skip-plugins", "--skip-themes"))
	results = append(results, wordpressDatabaseResult(databaseOutput, databaseErr))

	restOutput, restErr := runtime.ExecCapture(ctx, []string{"curl", "--connect-timeout", "2", "--max-time", "30", "-fsS", "-H", "Accept: application/json", "http://127.0.0.1/wp-json/"})
	results = append(results, wordpressRESTResult(restOutput, restErr))

	cronOutput, cronErr := runtime.ExecCapture(ctx, wordpressWPArgv("cron", "event", "list", "--fields=hook,next_run_gmt", "--format=json", "--skip-plugins", "--skip-themes"))
	results = append(results, wordpressCronResult(cronOutput, cronErr))

	runtimeOutput, runtimeErr := runtime.ExecCapture(ctx, wordpressWPArgv("eval", wordpressRuntimeProbe, "--skip-plugins", "--skip-themes"))
	results = append(results, wordpressRuntimeResult(runtimeOutput, runtimeErr))

	adminOutput, adminErr := runtime.ExecCapture(ctx, wordpressWPArgv("user", "get", "admin", "--field=ID", "--skip-plugins", "--skip-themes"))
	results = append(results, wordpressAdminResult(adminOutput, adminErr))

	if disposable {
		_, mediaErr := runtime.ExecCapture(ctx, []string{"s6-setuidgid", wordpressRuntimeUser, "sh", "-ec", wordpressMediaRoundTrip})
		if mediaErr != nil {
			results = append(results, wordpressVerifyFailed("verify:wordpress:media-round-trip", mediaErr.Error(), "inspect uploads ownership and WordPress media handling"))
		} else {
			results = append(results, wordpressVerifyOK("verify:wordpress:media-round-trip", "media fixture was imported, read, and deleted as the service account"))
		}
	}

	return results
}

func wordpressWPArgv(args ...string) []string {
	argv := []string{"s6-setuidgid", wordpressRuntimeUser, "wp", "--path=" + wordpressPath}
	return append(argv, args...)
}

func wordpressVersionResult(coreOutput, lockOutput string, coreErr, lockErr error) sitevalidate.Result {
	if coreErr != nil {
		return wordpressVerifyFailed("verify:wordpress:core-lock", coreErr.Error(), "confirm WordPress core is installed from the image")
	}
	if lockErr != nil {
		return wordpressVerifyFailed("verify:wordpress:core-lock", lockErr.Error(), "restore roots/wordpress in composer.lock and rebuild")
	}
	coreVersion := strings.TrimPrefix(strings.TrimSpace(coreOutput), "v")
	lockVersion := strings.TrimPrefix(strings.TrimSpace(lockOutput), "v")
	if coreVersion == "" || lockVersion == "" {
		return wordpressVerifyFailed("verify:wordpress:core-lock", "runtime or locked WordPress version is empty", "restore the Composer lock and rebuild the application image")
	}
	if coreVersion != lockVersion {
		return wordpressVerifyFailed("verify:wordpress:core-lock", fmt.Sprintf("runtime core %s differs from composer.lock %s", coreVersion, lockVersion), "rebuild and deploy from the committed composer.lock")
	}
	return wordpressVerifyOK("verify:wordpress:core-lock", fmt.Sprintf("runtime and composer.lock both provide WordPress %s", coreVersion))
}

func wordpressDatabaseResult(output string, commandErr error) sitevalidate.Result {
	if commandErr != nil {
		return wordpressVerifyFailed("verify:wordpress:database-identity", commandErr.Error(), "check the scoped WordPress database secret and MariaDB connectivity")
	}
	identity := strings.TrimSpace(output)
	if identity == "" {
		return wordpressVerifyFailed("verify:wordpress:database-identity", "database returned no current user", "check the scoped WordPress database secret")
	}
	username, _, _ := strings.Cut(identity, "@")
	if strings.EqualFold(username, "root") {
		return wordpressVerifyFailed("verify:wordpress:database-identity", "WordPress is connected as the MariaDB root user", "configure WordPress with its scoped application database user")
	}
	return wordpressVerifyOK("verify:wordpress:database-identity", identity)
}

func wordpressRESTResult(output string, commandErr error) sitevalidate.Result {
	if commandErr != nil {
		return wordpressVerifyFailed("verify:wordpress:rest", commandErr.Error(), "confirm the internal WordPress REST route is reachable")
	}
	var index struct {
		Name       string   `json:"name"`
		Namespaces []string `json:"namespaces"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &index); err != nil {
		return wordpressVerifyFailed("verify:wordpress:rest", fmt.Sprintf("decode REST index: %v", err), "inspect the WordPress REST route")
	}
	if strings.TrimSpace(index.Name) == "" {
		return wordpressVerifyFailed("verify:wordpress:rest", "REST index omitted the site name", "complete WordPress installation")
	}
	var hasV2 bool
	for _, namespace := range index.Namespaces {
		if namespace == "wp/v2" {
			hasV2 = true
			break
		}
	}
	if !hasV2 {
		return wordpressVerifyFailed("verify:wordpress:rest", "REST index omitted the wp/v2 namespace", "inspect REST API policy and plugin compatibility")
	}
	return wordpressVerifyOK("verify:wordpress:rest", fmt.Sprintf("wp/v2 is available for %q", index.Name))
}

func wordpressCronResult(output string, commandErr error) sitevalidate.Result {
	if commandErr != nil {
		return wordpressVerifyFailed("verify:wordpress:cron", commandErr.Error(), "inspect WP-Cron configuration and database state")
	}
	var events []struct {
		Hook       string `json:"hook"`
		NextRunGMT string `json:"next_run_gmt"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &events); err != nil {
		return wordpressVerifyFailed("verify:wordpress:cron", fmt.Sprintf("decode cron schedule: %v", err), "inspect WP-Cron with wp cron event list")
	}
	if len(events) == 0 {
		return wordpressVerifyFailed("verify:wordpress:cron", "WP-Cron has no scheduled events", "restore the core cron schedule and confirm recurring execution ownership")
	}
	for index, event := range events {
		if strings.TrimSpace(event.Hook) == "" || strings.TrimSpace(event.NextRunGMT) == "" {
			return wordpressVerifyFailed("verify:wordpress:cron", fmt.Sprintf("cron event %d omitted hook or next run", index), "inspect WP-Cron schedule integrity")
		}
	}
	return wordpressVerifyOK("verify:wordpress:cron", fmt.Sprintf("%d scheduled event(s) visible", len(events)))
}

func wordpressRuntimeResult(output string, commandErr error) sitevalidate.Result {
	if commandErr != nil {
		return wordpressVerifyFailed("verify:wordpress:runtime-config", commandErr.Error(), "inspect Bedrock URL and uploads configuration")
	}
	var probe struct {
		Home     string `json:"home"`
		SiteURL  string `json:"siteurl"`
		Uploads  string `json:"uploads"`
		Writable bool   `json:"writable"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &probe); err != nil {
		return wordpressVerifyFailed("verify:wordpress:runtime-config", fmt.Sprintf("decode runtime probe: %v", err), "inspect Bedrock URL and uploads configuration")
	}
	home, homeErr := url.ParseRequestURI(probe.Home)
	siteURL, siteErr := url.ParseRequestURI(probe.SiteURL)
	if homeErr != nil || siteErr != nil || home.Host == "" || siteURL.Host == "" ||
		(home.Scheme != "http" && home.Scheme != "https") || siteURL.Scheme != home.Scheme ||
		home.Host != siteURL.Host || home.User != nil || siteURL.User != nil {
		return wordpressVerifyFailed("verify:wordpress:runtime-config", fmt.Sprintf("inconsistent home=%q siteurl=%q", probe.Home, probe.SiteURL), "reconcile ingress-managed WP_HOME and WP_SITEURL")
	}
	wantSitePath := strings.TrimRight(home.Path, "/") + "/wp"
	if strings.TrimRight(siteURL.Path, "/") != wantSitePath {
		return wordpressVerifyFailed("verify:wordpress:runtime-config", fmt.Sprintf("siteurl path %q does not match Bedrock path %q", siteURL.Path, wantSitePath), "restore the managed Bedrock WP_SITEURL")
	}
	if probe.Uploads != "/var/www/bedrock/web/app/uploads" || !probe.Writable {
		return wordpressVerifyFailed("verify:wordpress:runtime-config", fmt.Sprintf("uploads=%q writable=%t", probe.Uploads, probe.Writable), "repair the wordpress-uploads volume and service-account ownership")
	}
	return wordpressVerifyOK("verify:wordpress:runtime-config", fmt.Sprintf("home %s; uploads writable", probe.Home))
}

func wordpressAdminResult(output string, commandErr error) sitevalidate.Result {
	if commandErr != nil {
		return wordpressVerifyFailed("verify:wordpress:admin", commandErr.Error(), "confirm the managed break-glass administrator exists")
	}
	identifier := strings.TrimSpace(output)
	if identifier == "" {
		return wordpressVerifyFailed("verify:wordpress:admin", "administrator lookup returned no ID", "restore the managed break-glass administrator")
	}
	for _, char := range identifier {
		if char < '0' || char > '9' {
			return wordpressVerifyFailed("verify:wordpress:admin", fmt.Sprintf("administrator returned invalid ID %q", identifier), "inspect WordPress user state")
		}
	}
	return wordpressVerifyOK("verify:wordpress:admin", "managed administrator exists")
}

func wordpressVerifyOK(name, detail string) sitevalidate.Result {
	return sitevalidate.Result{Name: name, Status: sitevalidate.StatusOK, Detail: detail}
}

func wordpressVerifyFailed(name, detail, fix string) sitevalidate.Result {
	return sitevalidate.Result{Name: name, Status: sitevalidate.StatusFailed, Detail: detail, FixHint: fix}
}

var _ plugin.VerifyRunner = (*wordpressVerifyRunner)(nil)
