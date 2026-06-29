package cmd

import "github.com/libops/sitectl/pkg/plugin"

var wordpressHealthcheckRunner = plugin.StandardComposeWebHealthcheck(plugin.StandardComposeWebHealthcheckOptions{
	AppService:              "wp",
	HTTPName:                "http:wp",
	DefaultScheme:           "http",
	DefaultDomain:           "localhost",
	DatabaseService:         "mariadb",
	CheckDatabaseDependency: true,
})
