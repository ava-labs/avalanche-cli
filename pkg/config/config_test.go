// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func Test_LoadNodeConfig(t *testing.T) {
	require := require.New(t)
	cf := New()

	err := useViper("node-config-test")
	require.NoError(err)

	config, err := cf.LoadNodeConfig()
	require.NoError(err)
	fmt.Println("Config:", config)
	testVal := viper.GetString("var")
	fmt.Println("Test val", testVal)
	require.Equal("val", testVal)
}

func Test_LoadNodeConfig_EmptyConfig(t *testing.T) {
	require := require.New(t)
	cf := New()

	err := useViper("empty-config")
	require.NoError(err)

	config, err := cf.LoadNodeConfig()
	require.NoError(err)
	require.Empty(config)
}

func Test_LoadNodeConfig_NoConfig(t *testing.T) {
	require := require.New(t)
	cf := New()

	err := useViper("")
	// we want to make sure this errors and no config file is read
	require.Error(err)

	config, err := cf.LoadNodeConfig()
	require.NoError(err)
	require.Empty(config)
}

func useViper(configName string) error {
	viper.Reset()
	viper.SetConfigName(configName)
	viper.SetConfigType("json")
	viper.AddConfigPath(filepath.Join("..", "..", "tests", "assets"))

	return viper.ReadInConfig()
}
