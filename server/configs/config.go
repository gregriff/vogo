// Package config contains the logic to obtain app configuration from a file or the environment
package configs

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	_ "embed" // used to embed the default application config file.

	"github.com/adrg/xdg"
	"github.com/spf13/viper"
)

//go:embed vogo-server.toml
var defaultConfigFile []byte

// InitConfig initializes the app config with Viper from the environment, a specified file, or a default file.
func InitConfig(file string) {
	if file == "" {
		panic("dev error, InitConfig should always be passed a valid config filepath")
	}
	viper.SetConfigName("vogo-server")
	viper.SetConfigType("toml")

	// allow env vars to override config file
	viper.SetEnvPrefix("vogo-server")
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

// func getConfigDir() string {
// 	appConfigDir := filepath.Join(xdg.ConfigHome, "vogo-server")
// 	if err := os.MkdirAll(appConfigDir, 0o750); err != nil {
// 		log.Fatalf("Error creating application config file at this location: %s", appConfigDir)
// 	}
// 	return appConfigDir
// }

// GetConfigDir obtains the configuration directory in a cross-platform manner,
// always respecting the XDG_CONFIG_HOME env var, using standard defaults on all OS's,
// but overriding to ~/.config on macOS
func GetConfigDir() string {
	var xdgConfigHome string
	if envVar := os.Getenv("XDG_CONFIG_HOME"); envVar != "" {
		xdgConfigHome = envVar
	} else if runtime.GOOS == "darwin" {
		home, _ := os.UserHomeDir()
		xdgConfigHome = filepath.Join(home, ".config") // override for mac
	} else {
		xdgConfigHome = xdg.ConfigHome
	}

	appConfigDir := filepath.Join(xdgConfigHome, "vogo")
	if err := os.MkdirAll(appConfigDir, 0o750); err != nil {
		log.Fatalf("Error creating application config directory (%s): %v", appConfigDir, err)
	}
	return appConfigDir
}
