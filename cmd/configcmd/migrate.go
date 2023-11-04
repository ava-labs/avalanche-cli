// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package configcmd

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
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
		Long:         `migrate command migrates old ~/.avalanche-cli.json and ~/.avalanche-cli/config to /.avalanche-cli/config.json..`,
		RunE:         migrateConfig,
		SilenceUsage: true,
	}
	return cmd
}

func migrateConfig(_ *cobra.Command, _ []string) error {
	oldConfigFilename := utils.UserHomePath(constants.OldConfigFileName)
	metricConfigFilename := utils.UserHomePath(constants.MetricsConfigFileName)
	viperConfigFilename := fmt.Sprintf("%s.%s", utils.UserHomePath(constants.BaseDirName, constants.DefaultConfigFileName), constants.DefaultConfigFileType)
	if utils.FileExists(viperConfigFilename) {
		ux.Logger.PrintToUser("Configuration file %s already exists. Configuration migration is not required.", viperConfigFilename)
		return nil
	}
	if !utils.FileExists(oldConfigFilename) && !utils.FileExists(metricConfigFilename) {
		ux.Logger.PrintToUser("Old configuration file %s or %s not found. Configuration migration is not required.", oldConfigFilename, metricConfigFilename)
		return nil
	} else {
		// load old config
		if utils.FileExists(oldConfigFilename) {
			viper.SetConfigFile(oldConfigFilename)
			if err := viper.MergeInConfig(); err != nil {
				return err
			}
		}
		if utils.FileExists(metricConfigFilename) {
			viper.SetConfigFile(metricConfigFilename)
			if err := viper.MergeInConfig(); err != nil {
				return err
			}
		}
		viper.SetConfigFile(constants.DefaultConfigFileName)
		if err := viper.SafeWriteConfig(); err != nil {
			return err
		}
		ux.Logger.PrintToUser("Configuration migrated to %s", viperConfigFilename)
		// remove old configuration file
		if utils.FileExists(oldConfigFilename) {
			if err := os.Remove(oldConfigFilename); err != nil {
				return fmt.Errorf("failed to remove old configuration file %s", oldConfigFilename)
			}
			ux.Logger.PrintToUser("Old configuration file %s removed", oldConfigFilename)
		}
		if utils.FileExists(metricConfigFilename) {
			if err := os.Remove(metricConfigFilename); err != nil {
				return fmt.Errorf("failed to remove old configuration file %s", metricConfigFilename)
			}
			ux.Logger.PrintToUser("Old configuration file %s removed", metricConfigFilename)
		}
		return nil
	}
}
