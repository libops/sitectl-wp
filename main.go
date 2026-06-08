package main

import (
	"fmt"

	"github.com/libops/sitectl-wp/cmd"
	"github.com/libops/sitectl/pkg/plugin"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	sdk := plugin.NewSDK(plugin.Metadata{
		Name:         "wp",
		Version:      fmt.Sprintf("%s (Built on %s from Git SHA %s)", version, date, commit),
		Description:  "WordPress helpers",
		Author:       "libops",
		TemplateRepo: "https://github.com/libops/wp",
		Includes:     cmd.IncludedPlugins(),
	})

	cmd.RegisterCommands(sdk)
	sdk.Execute()
}
