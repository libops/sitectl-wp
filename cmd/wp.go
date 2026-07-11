package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	sitectlplugin "github.com/libops/sitectl/pkg/plugin"
	"github.com/spf13/cobra"
)

const (
	wordpressService = "wp"
	wordpressPath    = "/var/www/bedrock/web/wp"
	wordpressTmpDir  = "/tmp"
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
			return s.RunActiveComposeProjectCommand(cmd, wordpressComposerCommand(args...))
		},
	}
}

func wpPluginCommand(s *sitectlplugin.SDK) *cobra.Command {
	root := &cobra.Command{
		Use:   "plugin",
		Short: "Manage WordPress plugins with WP-CLI",
	}
	root.AddCommand(wpPassthroughCommand(s, "list [WP-CLI args...]", "List WordPress plugins", []string{"plugin", "list"}))
	root.AddCommand(wpPassthroughCommand(s, "status [WP-CLI args...]", "Show WordPress plugin status", []string{"plugin", "status"}))
	root.AddCommand(wpPassthroughCommand(s, "update [PLUGIN...] [WP-CLI args...]", "Update WordPress plugins", []string{"plugin", "update"}))
	return root
}

func wpThemeCommand(s *sitectlplugin.SDK) *cobra.Command {
	root := &cobra.Command{
		Use:   "theme",
		Short: "Manage WordPress themes with WP-CLI",
	}
	root.AddCommand(wpPassthroughCommand(s, "list [WP-CLI args...]", "List WordPress themes", []string{"theme", "list"}))
	root.AddCommand(wpPassthroughCommand(s, "status [WP-CLI args...]", "Show WordPress theme status", []string{"theme", "status"}))
	root.AddCommand(wpPassthroughCommand(s, "update [THEME...] [WP-CLI args...]", "Update WordPress themes", []string{"theme", "update"}))
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
	return root
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
			return s.RunActiveComposeProjectCommandList(cmd, commands)
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
			return s.RunActiveComposeProjectCommandList(cmd, commands)
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

func wordpressDBExportCommands(localPath string) ([]string, error) {
	localPath, localDir, remotePath, err := wordpressDBPaths(localPath)
	if err != nil {
		return nil, err
	}
	return []string{
		sitectlplugin.ShellJoin([]string{"mkdir", "-p", localDir}),
		wordpressWPCLICommand("db", "export", remotePath),
		sitectlplugin.ShellJoin([]string{"docker", "compose", "cp", wordpressService + ":" + remotePath, localPath}),
	}, nil
}

func wordpressDBImportCommands(localPath string) ([]string, error) {
	localPath, _, remotePath, err := wordpressDBPaths(localPath)
	if err != nil {
		return nil, err
	}
	return []string{
		sitectlplugin.ShellJoin([]string{"test", "-f", localPath}),
		sitectlplugin.ShellJoin([]string{"docker", "compose", "cp", localPath, wordpressService + ":" + remotePath}),
		wordpressWPCLICommand("db", "import", remotePath),
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
	return localPath, localDir, filepath.Join(wordpressTmpDir, base), nil
}

func wordpressWPCLICommand(args ...string) string {
	cliArgs := []string{"wp", "--allow-root", "--path=" + wordpressPath}
	cliArgs = append(cliArgs, args...)
	return sitectlplugin.DockerComposeExecCommand(wordpressService, cliArgs...)
}

// The running app is image-backed, so Composer must mutate a bind-mounted
// checkout rather than the container filesystem that disappears on rebuild.
func wordpressComposerCommand(args ...string) string {
	return `docker compose run --rm --no-deps --user "$(id -u):$(id -g)" --volume "$PWD:/workspace:z" --workdir /workspace --entrypoint composer ` + wordpressService + " " + sitectlplugin.ShellJoin(args)
}

func runWordPressExec(s *sitectlplugin.SDK, cmd *cobra.Command, args ...string) error {
	return s.RunActiveComposeProjectCommand(cmd, sitectlplugin.DockerComposeExecCommand(wordpressService, args...))
}
