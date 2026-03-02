// Package configs contains the logic to obtain server configuration from a file or the environment
package configs

import (
	_ "embed" // used to embed the default application config file.

	"github.com/gregriff/vogo/shared/config"
)

//go:embed vogo-server.toml
var defaultConfigFile []byte

func Init(name, file string) {
	config.Init(name, file, defaultConfigFile)
}

func Dir(name string) string {
	return config.Dir(name)
}
