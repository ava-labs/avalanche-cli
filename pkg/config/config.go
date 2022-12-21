// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"encoding/json"

	"github.com/spf13/viper"
)

type Config struct{}

func New() *Config {
	return &Config{}
}

func (*Config) LoadNodeConfig() (string, error) {
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
