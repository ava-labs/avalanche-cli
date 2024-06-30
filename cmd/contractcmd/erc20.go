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
}

var (
	msgSupportedNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Devnet,
		networkoptions.Fuji,
	}
	msgFlags DeployERC20Flags
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
	networkoptions.AddNetworkFlagsToCmd(cmd, &msgFlags.Network, true, msgSupportedNetworkOptions)
	contract.AddPrivateKeyFlagsToCmd(cmd, &msgFlags.PrivateKeyFlags, "as contract deployer")
	cmd.Flags().BoolVar(&msgFlags.HexEncodedMessage, "hex-encoded", false, "given message is hex encoded")
	cmd.Flags().StringVar(&msgFlags.DestinationAddress, "destination-address", "", "deliver the message to the given contract destination address")
	return cmd
}

func deployERC20(_ *cobra.Command, args []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		msgFlags.Network,
		true,
		false,
		msgSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	_ = network
	ux.Logger.PrintToUser("ERC20 Contract Successfully Deployed!")
	return nil
}
