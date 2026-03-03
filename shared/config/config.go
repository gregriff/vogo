package config

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/adrg/xdg"
	"github.com/spf13/viper"
)

// Init initializes the config with Viper from the environment, a specified file, or a default file.
// defaultConfigFile is expected to be an embedded file containing default configuration.
func Init(name, file string, defaultConfigFile []byte) {
	if file == "" {
		log.Fatal("error, no config file specified")
	}
	viper.SetConfigName(name)
	viper.SetConfigType("toml")

	// allow env vars to override config file
	viper.SetEnvPrefix(name)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
	viper.SetConfigFile(file)

	// if config file does not exist, create it with the embedded default config
	if _, err := os.Stat(file); err != nil {
		log.Printf("config file not found (%s)", file)
		if err := viper.ReadConfig(bytes.NewBuffer(defaultConfigFile)); err != nil {
			log.Fatal(fmt.Errorf("error reading default embedded config file (%s): %w", defaultConfigFile, err).Error())
		}
		log.Printf("writing new config file (%s)", file)
		if err := os.WriteFile(file, defaultConfigFile, 0o600); err != nil {
			log.Fatalf("error writing default config: %v", err)
		}
		return
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal(fmt.Errorf("error reading config file: %w", err).Error())
	}
}

// Dir obtains the configuration directory in a cross-platform manner,
// always respecting the XDG_CONFIG_HOME env var, using standard defaults on all OS's,
// but overriding to ~/.config on macOS
func Dir(name string) string {
	var configHome string
	if envVar := os.Getenv("XDG_CONFIG_HOME"); envVar != "" {
		configHome = envVar
	} else if runtime.GOOS == "darwin" {
		home, _ := os.UserHomeDir()
		configHome = filepath.Join(home, ".config") // override for mac
	} else {
		configHome = xdg.ConfigHome
	}

	configDir := filepath.Join(configHome, name)
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		log.Fatalf("error creating config directory (%s): %v", configDir, err)
	}
	return configDir
}
