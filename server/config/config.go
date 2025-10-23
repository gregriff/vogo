// Package config contains the logic to obtain app configuration from a file or the environment
package config

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "embed" // used to embed the default application config file.

	"github.com/adrg/xdg"
	"github.com/spf13/viper"
)

//go:embed vogo-server.toml
var defaultConfigFile []byte

// InitConfig initializes the app config with Viper from the environment, a specified file, or a default file.
func InitConfig(file string) {
	viper.SetConfigName("vogo-server")
	viper.SetConfigType("toml")
	viper.AddConfigPath(getConfigDir()) // $XDG_HOME_CONFIG takes precedence over config in repo dir
	viper.AddConfigPath("./config")     // in the repo

	// allow env vars to override config file
	viper.SetEnvPrefix("vogo-server")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	if file != "" {
		viper.SetConfigFile(file)
	}

	var configErr error
	if configErr = viper.ReadInConfig(); configErr == nil {
		return
	}

	if _, ok := configErr.(viper.ConfigFileNotFoundError); ok {
		// create config file from embedded default file
		if err := viper.ReadConfig(bytes.NewBuffer(defaultConfigFile)); err != nil {
			log.Fatalf("Error reading default config file at path: %s", defaultConfigFile)
		}
		configPath := filepath.Join(getConfigDir(), "vogo-server.toml")
		if err := os.WriteFile(configPath, defaultConfigFile, 0o600); err != nil {
			log.Fatalf("Error writing default config: %v", err)
		}
	} else {
		log.Fatalf("Error reading config file: %v", configErr)
	}

}

func getConfigDir() string {
	appConfigDir := filepath.Join(xdg.ConfigHome, "vogo-server")
	if err := os.MkdirAll(appConfigDir, 0o750); err != nil {
		log.Fatalf("Error creating application config file at this location: %s", appConfigDir)
	}
	return appConfigDir
}
