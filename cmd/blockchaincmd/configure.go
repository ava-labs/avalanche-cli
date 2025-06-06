// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/cmd/blockchaincmd/upgradecmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var (
	nodeConf         string
	subnetConf       string
	chainConf        string
	perNodeChainConf string
)

// avalanche blockchain configure
func newConfigureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configure [blockchainName]",
		Short: "Adds additional config files for the avalanchego nodes",
		Long: `AvalancheGo nodes support several different configuration files.
Each network (a Subnet or an L1) has their own config which applies to all blockchains/VMs in the network (see https://build.avax.network/docs/nodes/configure/avalanche-l1-configs)
Each blockchain within the network can have its own chain config (see https://build.avax.network/docs/nodes/chain-configs/c-chain https://github.com/ava-labs/subnet-evm/blob/master/plugin/evm/config/config.go for subnet-evm options).
A chain can also have special requirements for the AvalancheGo node configuration itself (see https://build.avax.network/docs/nodes/configure/configs-flags).
This command allows you to set all those files.`,
		RunE: configure,
		Args: cobrautils.ExactArgs(1),
	}

	cmd.Flags().StringVar(&nodeConf, "node-config", "", "path to avalanchego node configuration")
	cmd.Flags().StringVar(&subnetConf, "subnet-config", "", "path to the subnet configuration")
	cmd.Flags().StringVar(&chainConf, "chain-config", "", "path to the chain configuration")
	cmd.Flags().StringVar(&perNodeChainConf, "per-node-chain-config", "", "path to per node chain configuration for local network")
	return cmd
}

func CallConfigure(
	cmd *cobra.Command,
	blockchainName string,
	chainConfParam string,
	subnetConfParam string,
	nodeConfParam string,
) error {
	chainConf = chainConfParam
	subnetConf = subnetConfParam
	nodeConf = nodeConfParam
	return configure(cmd, []string{blockchainName})
}

func configure(_ *cobra.Command, args []string) error {
	chains, err := ValidateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}
	blockchainName := chains[0]

	const (
		chainLabel        = constants.ChainConfigFileName
		perNodeChainLabel = constants.PerNodeChainConfigFileName
		subnetLabel       = constants.SubnetConfigFileName
		nodeLabel         = constants.NodeConfigFileName
	)
	configsToLoad := map[string]string{}

	if nodeConf != "" {
		configsToLoad[nodeLabel] = nodeConf
	}
	if subnetConf != "" {
		configsToLoad[subnetLabel] = subnetConf
	}
	if chainConf != "" {
		configsToLoad[chainLabel] = chainConf
	}
	if perNodeChainConf != "" {
		configsToLoad[perNodeChainLabel] = perNodeChainConf
	}

	// no flags provided
	if len(configsToLoad) == 0 {
		options := []string{nodeLabel, chainLabel, subnetLabel, perNodeChainLabel}
		selected, err := app.Prompt.CaptureList("Which configuration file would you like to provide?", options)
		if err != nil {
			return err
		}
		configsToLoad[selected], err = app.Prompt.CaptureExistingFilepath("Enter the path to your configuration file")
		if err != nil {
			return err
		}
		var other string
		if selected == chainLabel || selected == perNodeChainLabel {
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
	for filename, path := range configsToLoad {
		if err = copyBlockchainConf(blockchainName, path, filename); err != nil {
			return err
		}
	}

	upgradecmd.PrintHowToApplyConfChangesMessage(blockchainName)

	return nil
}

func copyBlockchainConf(blockchainName, path, configFilename string) error {
	var (
		fileBytes []byte
		err       error
	)
	if strings.ToLower(filepath.Ext(configFilename)) == "json" {
		fileBytes, err = utils.ValidateJSON(path)
		if err != nil {
			return err
		}
	} else {
		fileBytes, err = os.ReadFile(path)
		if err != nil {
			return err
		}
	}
	return SetBlockchainConf(blockchainName, fileBytes, configFilename)
}

func SetBlockchainConf(
	blockchainName string,
	fileBytes []byte,
	configFilename string,
) error {
	blockchainDir := filepath.Join(app.GetSubnetDir(), blockchainName)
	if err := os.MkdirAll(blockchainDir, constants.DefaultPerms755); err != nil {
		return err
	}
	fileName := filepath.Join(blockchainDir, configFilename)
	_ = os.RemoveAll(fileName)
	if err := os.WriteFile(fileName, fileBytes, constants.WriteReadReadPerms); err != nil {
		return err
	}
	ux.Logger.PrintToUser("File %s successfully written", fileName)
	return nil
}
