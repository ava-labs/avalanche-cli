// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func Test_LoadNodeConfig(t *testing.T) {
	assert := assert.New(t)
	cf := New()

	err := useViper("node-config-test")
	assert.NoError(err)

	config, err := cf.LoadNodeConfig()
	assert.NoError(err)
	fmt.Println("Config:", config)
	testVal := viper.GetString("var")
	fmt.Println("Test val", testVal)
	assert.Equal("val", testVal)
}

func Test_LoadNodeConfig_EmptyConfig(t *testing.T) {
	assert := assert.New(t)
	cf := New()

	err := useViper("empty-config")
	assert.NoError(err)

	config, err := cf.LoadNodeConfig()
	assert.NoError(err)
	assert.Empty(config)
}

func Test_LoadNodeConfig_NoConfig(t *testing.T) {
	assert := assert.New(t)
	cf := New()

	err := useViper("")
	// we want to make sure this errors and no config file is read
	assert.Error(err)

	config, err := cf.LoadNodeConfig()
	assert.NoError(err)
	assert.Empty(config)
}

func useViper(configName string) error {
	viper.Reset()
	viper.SetConfigName(configName)
	viper.SetConfigType("json")
	viper.AddConfigPath(filepath.Join("..", "..", "tests", "assets"))

	return viper.ReadInConfig()
}
