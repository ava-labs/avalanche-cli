// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package configcmd

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var MigrateOutput string

// avalanche transaction sign
func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "migrate",
		Short:        "migrate depricated ~/.avalanche-cli.json configuration",
		Long:         `migrate command migrate depricated ~/.avalanche-cli.json to /.avalanche-cli/config..`,
		RunE:         migrateConfig,
		SilenceUsage: true,
	}
	return cmd
}

func migrateConfig(_ *cobra.Command, _ []string) error {
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)
	oldConfigFilename := fmt.Sprintf("%s/%s.%s", home, constants.OldConfigFileName, constants.DefaultConfigFileType)
	if !app.ConfigFileExists(oldConfigFilename) {
		return fmt.Errorf("depricated configuration file %s does not exist", oldConfigFilename)
	} else {
		// load old config
		viper.SetConfigFile(oldConfigFilename)
		if err := viper.MergeInConfig(); err != nil {
			return err
		}
		viper.SetConfigFile(fmt.Sprintf("%s/%s/%s", home, constants.BaseDirName, constants.DefaultConfigFileName))
		if err := viper.SafeWriteConfig(); err != nil {
			return err
		}
		ux.Logger.PrintToUser("Configuration migrated to %s", viper.ConfigFileUsed())
		ux.Logger.PrintToUser("Depricated configuration file %s removed", oldConfigFilename)
		// remove old configuration file
		if err := os.Remove(oldConfigFilename); err != nil {
			return fmt.Errorf("failed to remove depricated configuration file %s", oldConfigFilename)
		}
		return nil
	}
}
