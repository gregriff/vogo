// Package configs contains the logic to obtain app configuration from a file or the environment
package configs

import (
	_ "embed" // used to embed the default application config file.

	"github.com/gregriff/vogo/shared/config"
)

//go:embed vogo.toml
var defaultConfigFile []byte

func Init(name, file string) {
	config.Init(name, file, defaultConfigFile)
}

func Dir(name string) string {
	return config.Dir(name)
}

// PersistCredentialsToConfig updates the vogo config file with the username
// given by the server and the entered password (plaintext)
// TODO: this may be used in the future to update the config file
// func PersistCredentialsToConfig(filename, username, friendCode string) error {
// 	var config map[string]any

// 	data, err := os.ReadFile(filename)
// 	if err != nil {
// 		return fmt.Errorf("config file not found! developer error")
// 	}

// 	// loads entire config
// 	toml.Unmarshal(data, &config)
// 	config["username"] = username
// 	config["friend-code"] = friendCode

// 	data, err = toml.Marshal(config)
// 	if err != nil {
// 		return fmt.Errorf("marshaling error: %w", err)
// 	}

// 	return os.WriteFile(filename, data, 0644)
// }
