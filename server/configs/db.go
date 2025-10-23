package configs

import (
	"os"

	"github.com/spf13/viper"
)

// ConfigurePostgres should be run after viper has read the config file
func ConfigurePostgres() {
	if os.Getenv("PGHOST") == "" {
		os.Setenv("PGHOST", viper.GetString("database.host"))
	}
	if os.Getenv("PGPORT") == "" {
		os.Setenv("PGPORT", viper.GetString("database.port"))
	}
	if os.Getenv("PGDATABASE") == "" {
		os.Setenv("PGDATABASE", viper.GetString("database.name"))
	}
}
