package config

import (
	"errors"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/loopholelabs/cmdutils/pkg/config"

	"github.com/loopholelabs/conduit/pkg/server"
	"github.com/loopholelabs/conduit/pkg/statsd"
	"github.com/loopholelabs/conduit/pkg/transit"
)

var _ config.Config = (*Config)(nil)

var (
	configFile string
	logFile    string
)

var (
	ErrFailedToParseTransitConfig    = errors.New("failed to parse transit config")
	ErrFailedToParseServerConfig     = errors.New("failed to parse server config")
	ErrFailedToUnmarshalConfig       = errors.New("failed to unmarshal config")
	ErrFailedToValidateTransitConfig = errors.New("failed to validate transit config")
	ErrFailedToValidateServerConfig  = errors.New("failed to validate server config")
	ErrFailedToParseStatsDConfig     = errors.New("failed to parse statsd config")
	ErrFailedToValidateStatsDConfig  = errors.New("failed to validate statsd config")
)

type Config struct {
	Transit *transit.Config `mapstructure:"transit_config"`
	Server  *server.Config  `mapstructure:"server_config"`
	StatsD  *statsd.Config  `mapstructure:"statsd_config"`
}

func New() *Config {
	return &Config{
		Transit: transit.DefaultConfig("00:00:00:00:00:00", "00:00:00:00:00:00"),
		Server:  server.NewConfig(),
		StatsD:  nil,
	}
}

func (c *Config) RootPersistentFlags(_ *pflag.FlagSet) {
	// Add flags here as needed
}

func (c *Config) GlobalRequiredFlags(_ *cobra.Command) error {
	return nil
}

func (c *Config) Parse() error {
	var errs error

	if err := c.Transit.Parse(); err != nil {
		errs = errors.Join(errs, ErrFailedToParseTransitConfig, err)
	}

	if err := c.Server.Parse(); err != nil {
		errs = errors.Join(errs, ErrFailedToParseServerConfig, err)
	}

	if c.StatsD != nil {
		if err := c.StatsD.Parse(); err != nil {
			errs = errors.Join(errs, ErrFailedToParseStatsDConfig, err)
		}
	}

	return errs
}

func (c *Config) Validate() error {
	var errs error
	if err := viper.Unmarshal(c); err != nil {
		return errors.Join(errs, ErrFailedToUnmarshalConfig, err)
	}

	if err := c.Transit.Validate(); err != nil {
		return errors.Join(errs, ErrFailedToValidateTransitConfig, err)
	}

	if err := c.Server.Validate(); err != nil {
		return errors.Join(errs, ErrFailedToValidateServerConfig, err)
	}

	if c.StatsD != nil {
		if err := c.StatsD.Validate(); err != nil {
			return errors.Join(errs, ErrFailedToValidateStatsDConfig, err)
		}
	}

	return nil
}

func (c *Config) DefaultConfigDir() (string, error) {
	return xdg.ConfigHome, nil
}

func (c *Config) DefaultConfigFile() string {
	return "architect-networking.yaml"
}

func (c *Config) DefaultLogDir() (string, error) {
	return xdg.StateHome, nil
}

func (c *Config) DefaultLogFile() string {
	return "architect-networking.log"
}

func (c *Config) SetConfigFile(file string) {
	configFile = file
}

func (c *Config) GetConfigFile() string {
	return configFile
}

func (c *Config) SetLogFile(file string) {
	logFile = file
}

func (c *Config) GetLogFile() string {
	return logFile
}
