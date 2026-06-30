package cmd

import (
	corecomponent "github.com/libops/sitectl/pkg/component"
	"github.com/libops/sitectl/pkg/plugin"
	coredevmode "github.com/libops/sitectl/pkg/services/devmode"
	coretraefik "github.com/libops/sitectl/pkg/services/traefik"
)

const (
	createRepo   = "https://github.com/libops/wp"
	createBranch = "main"
	pluginName   = "wp"
	defaultPath  = "./wp"
)

func createDefinition() plugin.CreateSpec {
	return plugin.CreateSpec{
		Name:                "default",
		Description:         "Create a WordPress stack",
		Default:             true,
		MinCPUCores:         2,
		MinMemory:           "4 GiB",
		MinDiskSpace:        "20 GiB",
		DockerComposeRepo:   createRepo,
		DockerComposeBranch: createBranch,
		DockerComposeBuild: []string{
			"docker compose pull --ignore-buildable",
			"docker compose build",
		},
		Images: []plugin.ComposeImageSpec{
			{Service: "wp", Image: "libops/wp:nginx-1.30.3-php84", BuildPolicy: plugin.BuildPolicyIfNotPresent},
		},
		DockerComposeInit: []string{
			"docker compose run --rm init",
		},
		InitArtifacts: []plugin.InitArtifact{
			{Path: "secrets/DB_ROOT_PASSWORD"},
			{Path: "secrets/WORDPRESS_DB_PASSWORD"},
			{Path: "secrets/WORDPRESS_ADMIN_PASSWORD"},
			{Path: "secrets/WORDPRESS_AUTH_KEY"},
			{Path: "secrets/WORDPRESS_SECURE_AUTH_KEY"},
			{Path: "secrets/WORDPRESS_LOGGED_IN_KEY"},
			{Path: "secrets/WORDPRESS_NONCE_KEY"},
			{Path: "secrets/WORDPRESS_AUTH_SALT"},
			{Path: "secrets/WORDPRESS_SECURE_AUTH_SALT"},
			{Path: "secrets/WORDPRESS_LOGGED_IN_SALT"},
			{Path: "secrets/WORDPRESS_NONCE_SALT"},
		},
		InitVolumes: []plugin.InitVolume{
			{Name: "mariadb-data"},
			{Name: "wordpress-uploads"},
		},
		DockerComposeUp: []string{
			"docker compose up --remove-orphans -d",
		},
		DockerComposeDown: []string{"docker compose down"},
		DockerComposeRollout: []string{
			"docker compose pull --ignore-buildable --quiet || docker compose pull --ignore-buildable || true",
			"docker compose build --pull",
			"docker compose run --rm init",
			"docker compose up --remove-orphans --wait --pull missing --quiet-pull -d",
			"docker compose exec -T wp wp --allow-root --path=/var/www/bedrock/web/wp core update-db || echo \"WordPress database update skipped or failed\"",
			"docker compose exec -T wp wp --allow-root --path=/var/www/bedrock/web/wp cache flush || true",
			"docker compose up --remove-orphans --wait --pull missing --quiet-pull -d",
		},
	}
}

// RegisterCommands registers WordPress commands with the plugin SDK.
func RegisterCommands(s *plugin.SDK) {
	s.SetComposeProjectDiscovery(plugin.ComposeProjectDiscovery{
		RequiredServices: []string{"wp"},
		Reason:           "wp service",
	})
	s.RegisterComposeTemplateCreateRunner(createDefinition(), plugin.ComposeTemplateCreateOptions{
		DefaultPath:   defaultPath,
		DefaultPlugin: pluginName,
		ReadyMessage:  "WordPress is ready for use through sitectl.",
	})
	registerApplicationComponents(s, "WordPress", "wp")
	s.RegisterHealthcheckRunner(wordpressHealthcheckRunner)
	s.RegisterIngressRouteProvider(plugin.StandardComposeWebIngressRoutesWithOptions(plugin.StandardComposeWebIngressOptions{
		AppService:     "wp",
		Router:         "wordpress-web",
		URLVariables:   []string{"WORDPRESS_HOME"},
		HTTPSVariables: []string{"WORDPRESS_ENABLE_HTTPS"},
	}))
	registerWordPressCommands(s)
}

func registerApplicationComponents(s *plugin.SDK, displayName, appService string) {
	ingress, err := coretraefik.Ingress(coretraefik.IngressOptions{
		AppService:      appService,
		HTTPEntrypoint:  "web",
		HTTPSEntrypoint: "websecure",
		ServiceEnvTemplates: map[string]map[string]string{
			appService: {
				"WORDPRESS_ENABLE_HTTPS": "{https_enabled}",
				"WORDPRESS_HOME":         "{base_url}",
				"WORDPRESS_SITEURL":      "{base_url}/wp",
			},
		},
	})
	if err != nil {
		panic(err)
	}
	devMode, err := coredevmode.Component(coredevmode.Options{
		AppService: appService,
		Volumes: []string{
			"./web/app/plugins:/var/www/bedrock/web/app/plugins:z,rw",
			"./web/app/themes:/var/www/bedrock/web/app/themes:z,rw",
		},
	})
	if err != nil {
		panic(err)
	}
	s.RegisterServiceComponents(plugin.ServiceComponentRegistryOptions{
		DisplayName: displayName,
		Components:  []corecomponent.ComposeServiceComponent{ingress, devMode},
	})
}
