package config

import (
	"encoding/json"

	"github.com/spf13/viper"
)

type Config struct{}

func New() *Config {
	return &Config{}
}

func (c *Config) LoadNodeConfig() (string, error) {
	globalConfigs := viper.GetStringMap("node-config")
	if len(globalConfigs) == 0 {
		return "", nil
	}
	configStr, err := json.Marshal(globalConfigs)
	if err != nil {
		return "", err
	}
	return string(configStr), nil
}
