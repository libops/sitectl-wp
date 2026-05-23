package cmd

import (
	"fmt"

	sitectlplugin "github.com/libops/sitectl/pkg/plugin"
	"github.com/spf13/cobra"
)

const (
	wordpressService = "wp"
	wordpressPath    = "/var/www/bedrock/web/wp"
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
		Short:              "Run Composer in the active WordPress stack",
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"install", "--no-interaction"}
			}
			return runWordPressExec(s, cmd, append([]string{"composer"}, args...)...)
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
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWPCLI(s, cmd, "core", "update-db")
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "export [PATH]",
		Short: "Export the WordPress database through the template Makefile",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/tmp/wp.sql"
			if len(args) == 1 {
				path = args[0]
			}
			return s.RunActiveComposeProjectCommand(cmd, fmt.Sprintf("make db-export DB_DUMP=%s", sitectlplugin.ShellQuote(path)))
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "import PATH",
		Short: "Import the WordPress database through the template Makefile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return s.RunActiveComposeProjectCommand(cmd, fmt.Sprintf("make db-import DB_DUMP=%s", sitectlplugin.ShellQuote(args[0])))
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

func runWordPressExec(s *sitectlplugin.SDK, cmd *cobra.Command, args ...string) error {
	return s.RunActiveComposeProjectCommand(cmd, sitectlplugin.DockerComposeExecCommand(wordpressService, args...))
}
