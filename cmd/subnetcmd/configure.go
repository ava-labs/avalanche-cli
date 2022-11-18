// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var (
	subnetConf string
	chainConf  string
)

// avalanche subnet configure
func newConfigureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configure [subnetName]",
		Short: "Adds additional config files for the avalanchego nodes",
		Long: `Avalanchego nodes can be configured at different levels. 
For example, subnets have their own subnet config (applies to all chains/VMs in the subnet).
And each chain or VM can have its own specific chain config file.
This command allows to set both config files.`,
		SilenceUsage: true,
		RunE:         configure,
		Args:         cobra.ExactArgs(1),
	}

	cmd.Flags().StringVar(&subnetConf, "subnet-config", "", "path to the subnet configuration")
	cmd.Flags().StringVar(&chainConf, "chain-config", "", "path to the chain configuration")
	return cmd
}

func configure(cmd *cobra.Command, args []string) error {
	chains, err := validateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}
	subnetName := chains[0]

	const (
		chainLabel  = constants.ChainConfigFileName
		subnetLabel = constants.SubnetConfigFileName
	)
	configsToLoad := map[string]string{}

	if subnetConf != "" {
		configsToLoad[subnetLabel] = subnetConf
	}
	if chainConf != "" {
		configsToLoad[chainLabel] = chainConf
	}

	// no flags provided
	if len(configsToLoad) == 0 {
		options := []string{chainLabel, subnetLabel}
		selected, err := app.Prompt.CaptureList("Which configuration file would you like to provide?", options)
		if err != nil {
			return err
		}
		configsToLoad[selected], err = app.Prompt.CaptureExistingFilepath("Enter the path to your configuration file")
		if err != nil {
			return err
		}
		var other string
		if selected == chainLabel {
			other = subnetLabel
		} else {
			other = chainLabel
		}
		yes, err := app.Prompt.CaptureNoYes(fmt.Sprintf("Would you like to provide the %s file as well?", other))
		if err != nil {
			return err
		}
		if yes {
			configsToLoad[other], err = app.Prompt.CaptureExistingFilepath("Enter the path to your configuration file")
			if err != nil {
				return err
			}
		}
	}

	// load each provided file
	for filename, configPath := range configsToLoad {
		if err = updateConf(subnetName, configPath, filename); err != nil {
			return err
		}
	}

	return nil
}

func updateConf(subnet, path, filename string) error {
	fileBytes, err := utils.ValidateJSON(path)
	if err != nil {
		return err
	}
	subnetDir := filepath.Join(app.GetSubnetDir(), subnet)
	if err := os.MkdirAll(subnetDir, constants.DefaultPerms755); err != nil {
		return err
	}
	fileName := filepath.Join(subnetDir, filename)
	if err := os.WriteFile(fileName, fileBytes, constants.DefaultPerms755); err != nil {
		return err
	}
	ux.Logger.PrintToUser("File %s successfully written", fileName)

	return nil
}
