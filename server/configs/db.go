package configs

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// ConfigurePostgres should be run after viper has read the config file.
func ConfigurePostgres() error {
	if os.Getenv("PGHOST") == "" {
		if err := os.Setenv("PGHOST", viper.GetString("database.host")); err != nil {
			return fmt.Errorf("error setting PGHOST env var: %w", err)
		}
	}
	if os.Getenv("PGPORT") == "" {
		if err := os.Setenv("PGPORT", viper.GetString("database.port")); err != nil {
			return fmt.Errorf("error setting PGPORT env var: %w", err)
		}
	}
	if os.Getenv("PGDATABASE") == "" {
		if err := os.Setenv("PGDATABASE", viper.GetString("database.name")); err != nil {
			return fmt.Errorf("error setting PGDATABASE env var: %w", err)
		}
	}
	return nil
}
