// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"encoding/json"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Config struct{}

func New() *Config {
	return &Config{}
}

func (*Config) SetConfig(log logging.Logger, s string) {
	viper.SetConfigType("json")
	d := filepath.Dir(s)
	viper.AddConfigPath(d)
	viper.SetConfigFile(s)
	viper.AutomaticEnv() // read in environment variables that match
	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		log.Info("Using config file", zap.String("config-file", s))
	} else {
		log.Info("No log file found")
	}
}

func (*Config) MergeConfig(log logging.Logger, s string) {
	prevS := viper.ConfigFileUsed()
	viper.SetConfigFile(s)
	log.Info("Merging configuration file", zap.String("config-file", s))
	if err := viper.MergeInConfig(); err != nil {
		log.Info("Error loading configuration file", zap.String("config-file", s))
	}
	viper.SetConfigFile(prevS)
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
	err := viper.WriteConfig()
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
