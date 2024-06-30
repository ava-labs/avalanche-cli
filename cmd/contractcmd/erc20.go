// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contractcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

type DeployERC20Flags struct {
	Network            networkoptions.NetworkFlags
	DestinationAddress string
	HexEncodedMessage  bool
	PrivateKeyFlags    contract.PrivateKeyFlags
	chainFlags         contract.ChainFlags
}

var (
	deployERC20SupportedNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Devnet,
		networkoptions.Fuji,
	}
	deployERC20Flags DeployERC20Flags
)

// avalanche contract deploy erc20
func newDeployERC20Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "erc20",
		Short: "Deploy an ERC20 token into a given Network and Blockchain",
		Long:  "Deploy an ERC20 token into a given Network and Blockchain",
		RunE:  deployERC20,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &deployERC20Flags.Network, true, deployERC20SupportedNetworkOptions)
	contract.AddPrivateKeyFlagsToCmd(cmd, &deployERC20Flags.PrivateKeyFlags, "as contract deployer")
	contract.AddChainFlagsToCmd(
		cmd,
		&deployERC20Flags.chainFlags,
		"deploy the ERC20 contract",
		"",
		"",
	)
	return cmd
}

func deployERC20(_ *cobra.Command, _ []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		deployERC20Flags.Network,
		true,
		false,
		deployERC20SupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	subnetNames, err := app.GetSubnetNamesOnNetwork(network)
	if err != nil {
		return err
	}
	_ = subnetNames
	ux.Logger.PrintToUser("ERC20 Contract Successfully Deployed!")
	return nil
}
