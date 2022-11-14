// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/spf13/cobra"
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
	return cmd
}

func configure(cmd *cobra.Command, args []string) error {
	chains, err := validateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}
	subnetName := chains[0]

	const (
		subnetConf = "Subnet config"
		chainConf  = "Chain config"
	)

	ux.Logger.PrintToUser("The " + logging.Cyan.Wrap("subnet") + logging.Reset.Wrap(" config file applies to *all* VMs in a subnet"))
	ux.Logger.PrintToUser("The " + logging.Cyan.Wrap("chain") + logging.Reset.Wrap(" config file applies to a *specific* VM in a subnet"))

	options := []string{subnetConf, chainConf}
	selected, err := app.Prompt.CaptureList("Which configuration file would you like to update?", options)
	if err != nil {
		return err
	}
	switch selected {
	case subnetConf:
		err = updateConf(subnetName, constants.SubnetConfigFileName)
	case chainConf:
		err = updateConf(subnetName, constants.ChainConfigFileName)
	}
	if err != nil {
		return err
	}

	return nil
}

func updateConf(subnet, filename string) error {
	path, err := app.Prompt.CaptureExistingFilepath("Enter the path to your configuration file")
	if err != nil {
		return err
	}
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
