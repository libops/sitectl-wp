package cmd

import "github.com/libops/sitectl/pkg/plugin"

const (
	createRepo   = "https://github.com/libops/wp"
	createBranch = "main"
	pluginName   = "wp"
	defaultPath  = "./wp"
	displayName  = "WordPress"
)

func createDefinition() plugin.CreateSpec {
	return plugin.CreateSpec{
		Name:                 "default",
		Description:          "Create a WordPress stack",
		Default:              true,
		MinCPUCores:          2,
		MinMemory:            "4 GiB",
		MinDiskSpace:         "20 GiB",
		DockerComposeRepo:    createRepo,
		DockerComposeBranch:  createBranch,
		DockerComposeBuild:   []string{"make build"},
		DockerComposeInit:    []string{"make init"},
		DockerComposeUp:      []string{"make up"},
		DockerComposeDown:    []string{"make down"},
		DockerComposeRollout: []string{"make rollout"},
	}
}

// RegisterCommands registers WordPress commands with the plugin SDK.
func RegisterCommands(s *plugin.SDK) {
	s.SetComposeProjectDiscovery(plugin.ComposeProjectDiscovery{
		RequiredServices: []string{"wp"},
		Reason:           "wp service",
	})
	s.RegisterStandardComposeTemplate(createDefinition(), plugin.StandardComposeTemplateOptions{
		DefaultPath:   defaultPath,
		DefaultPlugin: pluginName,
		ReadyMessage:  "WordPress is ready for use through sitectl.",
		DisplayName:   displayName,
	})
	registerWordPressCommands(s)
}
