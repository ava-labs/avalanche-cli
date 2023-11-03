// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package configcmd

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var MigrateOutput string

// avalanche config metrics migrate
func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "migrate",
		Short:        "migrate ~/.avalanche-cli.json and ~/.avalanche-cli/config to new configuration location ~/.avalanche-cli/config.json",
		Long:         `migrate command migrates deprecated ~/.avalanche-cli.json and ~/.avalanche-cli/config to /.avalanche-cli/config.json..`,
		RunE:         migrateConfig,
		SilenceUsage: true,
	}
	return cmd
}

func migrateConfig(_ *cobra.Command, _ []string) error {
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)
	oldConfigFilename := fmt.Sprintf("%s/%s", home, constants.OldConfigFileName)
	metricConfigFilename := fmt.Sprintf("%s/%s", home, constants.MetricsConfigFileName)
	if !application.FileExists(oldConfigFilename) && !application.FileExists(metricConfigFilename) {
		ux.Logger.PrintToUser("Old configuration file not found. Configuration migration is not required.")
		return nil
	} else {
		// load old config
		viper.SetConfigFile(oldConfigFilename)
		if err := viper.MergeInConfig(); err != nil {
			return err
		}
		viper.SetConfigFile(metricConfigFilename)
		if err := viper.MergeInConfig(); err != nil {
			return err
		}
		viper.SetConfigFile(fmt.Sprintf("%s/%s/%s.%s", home, constants.BaseDirName, constants.DefaultConfigFileName, constants.DefaultConfigFileType))
		if err := viper.SafeWriteConfig(); err != nil {
			return err
		}
		ux.Logger.PrintToUser("Configuration migrated to %s", viper.ConfigFileUsed())
		// remove old configuration file
		if err := os.Remove(oldConfigFilename); err != nil {
			return fmt.Errorf("failed to remove old configuration file %s", oldConfigFilename)
		}
		ux.Logger.PrintToUser("Old configuration file %s removed", oldConfigFilename)
		if err := os.Remove(metricConfigFilename); err != nil {
			return fmt.Errorf("failed to remove old configuration file %s", metricConfigFilename)
		}
		ux.Logger.PrintToUser("Old configuration file %s removed", metricConfigFilename)
		return nil
	}
}
