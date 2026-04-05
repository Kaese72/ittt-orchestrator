package config

import (
	"os"
	"strings"

	log "github.com/Kaese72/huemie-lib/logging"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

func (c DatabaseConfig) Validate() error {
	if c.Host == "" {
		return errors.New("must supply database host")
	}
	if c.User == "" {
		return errors.New("must supply database user")
	}
	if c.Password == "" {
		return errors.New("must supply database password")
	}
	return nil
}

type EventConfig struct {
	DeviceUpdatesTopic string `mapstructure:"device-updates"`
	ConnectionString   string `mapstructure:"connectionString"`
}

func (c EventConfig) Validate() error {
	if c.DeviceUpdatesTopic == "" {
		return errors.New("must supply event device updates topic")
	}
	if c.ConnectionString == "" {
		return errors.New("must supply event connection string")
	}
	return nil
}

type DeviceStoreConfig struct {
	URL string `mapstructure:"url"`
}

func (c DeviceStoreConfig) Validate() error {
	if c.URL == "" {
		return errors.New("must supply device store URL")
	}
	return nil
}

type Config struct {
	Database    DatabaseConfig    `mapstructure:"database"`
	Event       EventConfig       `mapstructure:"event"`
	DeviceStore DeviceStoreConfig `mapstructure:"device-store"`
}

func (c Config) Validate() error {
	if err := c.Database.Validate(); err != nil {
		return err
	}
	if err := c.Event.Validate(); err != nil {
		return err
	}
	if err := c.DeviceStore.Validate(); err != nil {
		return err
	}
	return nil
}

var Loaded Config

func init() {
	// We have elected not to use AutomaticEnv() because of https://github.com/spf13/viper/issues/584
	// Set replaces to allow keys like "database.host" → DATABASE_HOST
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// Database
	viper.BindEnv("database.host")
	viper.BindEnv("database.port")
	viper.SetDefault("database.port", 3306)
	viper.BindEnv("database.user")
	viper.BindEnv("database.password")
	viper.BindEnv("database.database")
	viper.SetDefault("database.database", "itttorchestrator")

	// Event streaming
	viper.BindEnv("event.device-updates")
	viper.SetDefault("event.device-updates", "deviceUpdates")
	viper.BindEnv("event.connectionstring")

	// Device store
	viper.BindEnv("device-store.url")
	viper.SetDefault("device-store.url", "http://device-store:8080")

	if err := viper.Unmarshal(&Loaded); err != nil {
		log.Error(err.Error(), map[string]interface{}{})
		os.Exit(1)
	}
}
