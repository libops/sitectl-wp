package cmd

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	sitectlplugin "github.com/libops/sitectl/pkg/plugin"
	"github.com/spf13/cobra"
)

const (
	wordpressService = "wp"
	wordpressPath    = "/var/www/bedrock/web/wp"
	wordpressTmpDir  = "/tmp"
)

var (
	wordpressPackageSlugPattern        = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	wordpressComposerConstraintPattern = regexp.MustCompile(`^[A-Za-z0-9.*^~<>=|,_-]+$`)
)

func registerWordPressCommands(s *sitectlplugin.SDK) {
	s.AddCommand(wpCLICommand(s))
	s.AddCommand(wpComposerCommand(s))
	s.AddCommand(wpPluginCommand(s))
	s.AddCommand(wpThemeCommand(s))
	s.AddCommand(wpCoreCommand(s))
	s.AddCommand(wpCacheCommand(s))
	s.AddCommand(wpDBCommand(s))
}

func wpCLICommand(s *sitectlplugin.SDK) *cobra.Command {
	return &cobra.Command{
		Use:                "cli [WP-CLI args...]",
		Short:              "Run WP-CLI in the active WordPress stack",
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rejectComposerManagedWPCLIMutation(args); err != nil {
				return err
			}
			return runWPCLI(s, cmd, args...)
		},
	}
}

func wpComposerCommand(s *sitectlplugin.SDK) *cobra.Command {
	return &cobra.Command{
		Use:                "composer [Composer args...]",
		Short:              "Run Composer against the active WordPress checkout",
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"install", "--no-interaction"}
			}
			return runWordPressComposer(s, cmd, args...)
		},
	}
}

func wpPluginCommand(s *sitectlplugin.SDK) *cobra.Command {
	root := &cobra.Command{
		Use:   "plugin",
		Short: "Inspect WordPress plugins and update their Composer packages",
	}
	root.AddCommand(wpPassthroughCommand(s, "list [WP-CLI args...]", "List WordPress plugins", []string{"plugin", "list"}))
	root.AddCommand(wpPassthroughCommand(s, "status [WP-CLI args...]", "Show WordPress plugin status", []string{"plugin", "status"}))
	root.AddCommand(wpComposerManagedUpdateCommand(s, "plugin"))
	return root
}

func wpThemeCommand(s *sitectlplugin.SDK) *cobra.Command {
	root := &cobra.Command{
		Use:   "theme",
		Short: "Inspect WordPress themes and update their Composer packages",
	}
	root.AddCommand(wpPassthroughCommand(s, "list [WP-CLI args...]", "List WordPress themes", []string{"theme", "list"}))
	root.AddCommand(wpPassthroughCommand(s, "status [WP-CLI args...]", "Show WordPress theme status", []string{"theme", "status"}))
	root.AddCommand(wpComposerManagedUpdateCommand(s, "theme"))
	return root
}

func wpCoreCommand(s *sitectlplugin.SDK) *cobra.Command {
	root := &cobra.Command{
		Use:   "core",
		Short: "Run WordPress core maintenance helpers",
	}
	root.AddCommand(&cobra.Command{
		Use:   "update-db",
		Short: "Run WordPress database updates",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWPCLI(s, cmd, "core", "update-db")
		},
	})
	root.AddCommand(wpPassthroughCommand(s, "version [WP-CLI args...]", "Show the WordPress core version", []string{"core", "version"}))
	root.AddCommand(wpComposerManagedUpdateCommand(s, "core"))
	return root
}

func wpComposerManagedUpdateCommand(s *sitectlplugin.SDK, kind string) *cobra.Command {
	var use, short string
	switch kind {
	case "plugin":
		use = "update PLUGIN:CONSTRAINT..."
		short = "Update Composer-managed WordPress plugin packages to explicit constraints"
	case "theme":
		use = "update THEME:CONSTRAINT..."
		short = "Update Composer-managed WordPress theme packages to explicit constraints"
	default:
		use = "update CONSTRAINT"
		short = "Update Composer-managed WordPress core to an explicit constraint"
	}
	return &cobra.Command{
		Use:                use,
		Short:              short,
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			composerArgs, err := wordpressComposerUpdateArgs(kind, args)
			if err != nil {
				return err
			}
			if err := runWordPressComposer(s, cmd, composerArgs...); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Composer manifest/lock update completed; review the diff, then rebuild and deploy.")
			return err
		},
	}
}

func wordpressComposerUpdateArgs(kind string, args []string) ([]string, error) {
	if kind == "core" {
		if len(args) != 1 || strings.HasPrefix(args[0], "-") || !wordpressComposerConstraintPattern.MatchString(args[0]) {
			return nil, fmt.Errorf("provide one explicit WordPress core Composer constraint, for example: sitectl wp core update 7.0.2")
		}
		return []string{"require", "roots/wordpress:" + args[0], "--with-all-dependencies", "--no-interaction"}, nil
	}

	prefix := ""
	switch kind {
	case "plugin":
		prefix = "wpackagist-plugin/"
	case "theme":
		prefix = "wpackagist-theme/"
	default:
		return nil, fmt.Errorf("unsupported Composer-managed WordPress package kind %q", kind)
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("provide at least one %s:constraint pair", kind)
	}

	composerArgs := []string{"require"}
	for _, target := range args {
		if strings.HasPrefix(target, "-") {
			return nil, fmt.Errorf("%s update requires explicit package constraints, not WP-CLI option %q; use sitectl wp composer for advanced Composer options", kind, target)
		}
		slug, constraint, found := strings.Cut(target, ":")
		if !found || strings.TrimSpace(constraint) == "" {
			return nil, fmt.Errorf("provide %s %q with an explicit Composer constraint as %s:CONSTRAINT", kind, target, strings.ToUpper(kind))
		}
		if !wordpressPackageSlugPattern.MatchString(slug) {
			return nil, fmt.Errorf("invalid WordPress %s slug %q", kind, slug)
		}
		if !wordpressComposerConstraintPattern.MatchString(constraint) {
			return nil, fmt.Errorf("invalid Composer constraint %q for WordPress %s %q", constraint, kind, slug)
		}
		composerArgs = append(composerArgs, prefix+slug+":"+constraint)
	}
	composerArgs = append(composerArgs, "--with-all-dependencies", "--no-interaction")
	return composerArgs, nil
}

func rejectComposerManagedWPCLIMutation(args []string) error {
	positionals := make([]string, 0, 2)
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if trimmed == "" || strings.HasPrefix(trimmed, "-") {
			continue
		}
		positionals = append(positionals, strings.ToLower(trimmed))
		if len(positionals) == 2 {
			break
		}
	}
	if len(positionals) < 2 {
		return nil
	}
	resource, action := positionals[0], positionals[1]
	switch resource {
	case "core":
		if action == "update" || action == "download" {
			return fmt.Errorf("WP-CLI cannot mutate Composer-managed WordPress core; use sitectl wp core update or sitectl wp composer")
		}
	case "plugin", "theme":
		if action == "install" || action == "update" || action == "delete" {
			return fmt.Errorf("WP-CLI cannot %s Composer-managed WordPress %s code; use sitectl wp %s update or sitectl wp composer", action, resource, resource)
		}
	}
	return nil
}

func wpCacheCommand(s *sitectlplugin.SDK) *cobra.Command {
	root := &cobra.Command{
		Use:   "cache",
		Short: "Run WordPress cache helpers",
	}
	root.AddCommand(&cobra.Command{
		Use:   "flush",
		Short: "Flush the WordPress object cache",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWPCLI(s, cmd, "cache", "flush")
		},
	})
	return root
}

func wpDBCommand(s *sitectlplugin.SDK) *cobra.Command {
	root := &cobra.Command{
		Use:   "db",
		Short: "Run WordPress database helpers",
	}
	root.AddCommand(&cobra.Command{
		Use:   "update",
		Short: "Run WordPress database updates",
		Aliases: []string{
			"update-db",
		},
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWPCLI(s, cmd, "core", "update-db")
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "export PATH",
		Short: "Export the WordPress database to a local path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			commands, err := wordpressDBExportCommands(args[0])
			if err != nil {
				return err
			}
			return s.RunActiveComposeProjectArgvList(cmd, commands)
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "import PATH",
		Short: "Import a local SQL dump into the WordPress database",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			commands, err := wordpressDBImportCommands(args[0])
			if err != nil {
				return err
			}
			return s.RunActiveComposeProjectArgvList(cmd, commands)
		},
	})
	return root
}

func wpPassthroughCommand(s *sitectlplugin.SDK, use, short string, prefix []string) *cobra.Command {
	return &cobra.Command{
		Use:                use,
		Short:              short,
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cliArgs := append([]string{}, prefix...)
			cliArgs = append(cliArgs, args...)
			return runWPCLI(s, cmd, cliArgs...)
		},
	}
}

func runWPCLI(s *sitectlplugin.SDK, cmd *cobra.Command, args ...string) error {
	cliArgs := []string{"wp", "--allow-root", "--path=" + wordpressPath}
	cliArgs = append(cliArgs, args...)
	return runWordPressExec(s, cmd, cliArgs...)
}

func wordpressDBExportCommands(localPath string) ([][]string, error) {
	localPath, _, remotePath, err := wordpressDBPaths(localPath)
	if err != nil {
		return nil, err
	}
	return [][]string{
		wordpressWPCLIArgv("db", "export", remotePath),
		{"docker", "compose", "cp", wordpressService + ":" + remotePath, localPath},
	}, nil
}

func wordpressDBImportCommands(localPath string) ([][]string, error) {
	localPath, _, remotePath, err := wordpressDBPaths(localPath)
	if err != nil {
		return nil, err
	}
	return [][]string{
		{"docker", "compose", "cp", localPath, wordpressService + ":" + remotePath},
		wordpressWPCLIArgv("db", "import", remotePath),
	}, nil
}

func wordpressDBPaths(localPath string) (string, string, string, error) {
	localPath = strings.TrimSpace(localPath)
	if localPath == "" {
		return "", "", "", fmt.Errorf("path is required")
	}
	base := filepath.Base(localPath)
	if base == "" || base == "." || base == "/" {
		return "", "", "", fmt.Errorf("path must include a file name")
	}
	localDir := filepath.Dir(localPath)
	if strings.TrimSpace(localDir) == "" {
		localDir = "."
	}
	return localPath, localDir, path.Join(wordpressTmpDir, base), nil
}

func wordpressWPCLIArgv(args ...string) []string {
	cliArgs := []string{"wp", "--allow-root", "--path=" + wordpressPath}
	cliArgs = append(cliArgs, args...)
	return sitectlplugin.DockerComposeExecArgv(wordpressService, cliArgs...)
}

// The running app is image-backed, so Composer must mutate a bind-mounted
// checkout rather than the container filesystem that disappears on rebuild.
func wordpressComposerArgv(host sitectlplugin.ComposeProjectHost, args ...string) []string {
	argv := []string{
		"docker", "compose", "run", "--rm", "--no-deps",
	}
	if host.HasNumericIdentity {
		argv = append(argv, "--user", host.UID+":"+host.GID)
	}
	argv = append(argv,
		"--volume", host.ProjectDir+":/workspace:z",
		"--workdir", "/workspace",
		"--entrypoint", "composer",
		wordpressService,
	)
	return append(argv, args...)
}

func runWordPressComposer(s *sitectlplugin.SDK, cmd *cobra.Command, args ...string) error {
	return s.RunActiveComposeProjectHostArgv(cmd, func(host sitectlplugin.ComposeProjectHost) []string {
		return wordpressComposerArgv(host, args...)
	})
}

func runWordPressExec(s *sitectlplugin.SDK, cmd *cobra.Command, args ...string) error {
	return s.RunActiveComposeProjectArgv(cmd, sitectlplugin.DockerComposeExecArgv(wordpressService, args...))
}
