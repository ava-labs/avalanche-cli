// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contractcmd

import (
	"fmt"
	"math/big"

	cmdflags "github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

type DeployERC20Flags struct {
	Network         networkoptions.NetworkFlags
	PrivateKeyFlags contract.PrivateKeyFlags
	chainFlags      contract.ChainFlags
	symbol          string
	supply          uint64
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
	cmd.Flags().StringVar(&deployERC20Flags.symbol, "symbol", "", "set the ERC20 Token Symbol")
	cmd.Flags().Uint64Var(&deployERC20Flags.supply, "supply", 0, "set the ERC20 Token Supply")
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
	// flags exclusiveness
	if !cmdflags.EnsureMutuallyExclusive([]bool{
		deployERC20Flags.chainFlags.SubnetName != "",
		deployERC20Flags.chainFlags.CChain,
	}) {
		return fmt.Errorf("--subnet and --c-chain are mutually exclusive flags")
	}
	if deployERC20Flags.chainFlags.SubnetName == "" && !deployERC20Flags.chainFlags.CChain {
		subnetNames, err := app.GetSubnetNamesOnNetwork(network)
		if err != nil {
			return err
		}
		prompt := "Where do you want to Deploy the ERC-20 Token?"
		cancel, _, _, cChain, subnetName, err := prompts.PromptChain(
			app.Prompt,
			prompt,
			subnetNames,
			true,
			true,
			false,
			"",
		)
		if cancel {
			return nil
		}
		if err == nil {
			deployERC20Flags.chainFlags.SubnetName = subnetName
			deployERC20Flags.chainFlags.CChain = cChain
		}
	}
	genesisAddress, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
		app,
		network,
		deployERC20Flags.chainFlags.SubnetName,
		deployERC20Flags.chainFlags.CChain,
		"",
	)
	if err != nil {
		return err
	}
	privateKey, err := contract.GetPrivateKeyFromFlags(
		app,
		deployERC20Flags.PrivateKeyFlags,
		genesisPrivateKey,
	)
	if err != nil {
		return err
	}
	if privateKey == "" {
		privateKey, err = prompts.PromptPrivateKey(
			app.Prompt,
			"deploy the contract",
			app.GetKeyDir(),
			app.GetKey,
			genesisAddress,
			genesisPrivateKey,
		)
		if err != nil {
			return err
		}
	}
	if deployERC20Flags.symbol == "" {
		ux.Logger.PrintToUser("Select a symbol for the ERC20 Token")
		deployERC20Flags.symbol, err = app.Prompt.CaptureString("Token symbol")
		if err != nil {
			return err
		}
	}
	supply := new(big.Int).SetUint64(deployERC20Flags.supply)
	if deployERC20Flags.supply == 0 {
		ux.Logger.PrintToUser("Select the total available supply for the ERC20 Token")
		supply, err = app.Prompt.CapturePositiveBigInt("Token supply")
		if err != nil {
			return err
		}
	}
	rpcURL, err := contract.GetRPCURL(
		app,
		network,
		deployERC20Flags.chainFlags.SubnetName,
		deployERC20Flags.chainFlags.CChain,
	)
	if err != nil {
		return err
	}
	address, err := contract.DeployERC20(
		rpcURL,
		privateKey,
		deployERC20Flags.symbol,
		supply,
	)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Token Address: %s", address.Hex())
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("ERC20 Contract Successfully Deployed!")
	return nil
}
