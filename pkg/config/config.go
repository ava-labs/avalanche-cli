// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"encoding/json"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"

	"github.com/spf13/viper"
)

type Config struct{}

func New() *Config {
	return &Config{}
}

func (*Config) GetConfigPath() string {
	return viper.ConfigFileUsed()
}

func (c *Config) ConfigFileExists() bool {
	return utils.FileExists(c.GetConfigPath())
}

// SetConfigValue sets the value of a configuration key.
func (*Config) SetConfigValue(key string, value interface{}) error {
	viper.Set(key, value)
	err := viper.SafeWriteConfig()
	return err
}

func (*Config) ConfigValueIsSet(key string) bool {
	return viper.IsSet(key)
}

func (*Config) GetConfigBoolValue(key string) bool {
	return viper.GetBool(key)
}

func (*Config) GetConfigStringValue(key string) string {
	return viper.GetString(key)
}

func (*Config) LoadNodeConfig() (string, error) {
	globalConfigs := viper.GetStringMap(constants.ConfigNodeConfigKey)
	if len(globalConfigs) == 0 {
		return "", nil
	}
	configStr, err := json.Marshal(globalConfigs)
	if err != nil {
		return "", err
	}
	return string(configStr), nil
}
