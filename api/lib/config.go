package lib

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Env    Environment  `json:"env"`
	Server ServerConfig `mapstructure:"server"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
}

func Load() (*Config, error) {

	v := viper.New()

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.SetEnvPrefix("APP")
	v.AutomaticEnv()

	v.SetConfigName("base")
	v.SetConfigType("yaml")
	v.AddConfigPath("configs")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read base config: %w", err)
	}

	env := Detect()

	v.SetConfigName(env.String())

	if err := v.MergeInConfig(); err != nil {
		fmt.Printf("Warning: environment config for '%s' not found, relying on base config\n", env)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cfg.Env = env

	return &cfg, nil
}
