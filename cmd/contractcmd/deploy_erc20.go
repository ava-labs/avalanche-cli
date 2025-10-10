// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contractcmd

import (
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-tooling-sdk-go/evm"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/libevm/common"

	"github.com/spf13/cobra"
)

type DeployERC20Flags struct {
	Network         networkoptions.NetworkFlags
	PrivateKeyFlags contract.PrivateKeyFlags
	chainFlags      contract.ChainSpec
	symbol          string
	funded          string
	supply          uint64
	rpcEndpoint     string
}

var deployERC20Flags DeployERC20Flags

// avalanche contract deploy erc20
func newDeployERC20Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "erc20",
		Short: "Deploy an ERC20 token into a given Network and Blockchain",
		Long:  "Deploy an ERC20 token into a given Network and Blockchain",
		RunE:  deployERC20,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &deployERC20Flags.Network, true, networkoptions.DefaultSupportedNetworkOptions)
	deployERC20Flags.PrivateKeyFlags.AddToCmd(cmd, "as contract deployer")
	// enabling blockchain names, C-Chain and blockchain IDs
	deployERC20Flags.chainFlags.SetEnabled(true, true, false, false, true)
	deployERC20Flags.chainFlags.AddToCmd(cmd, "deploy the ERC20 contract into %s")
	cmd.Flags().StringVar(&deployERC20Flags.symbol, "symbol", "", "set the token symbol")
	cmd.Flags().Uint64Var(&deployERC20Flags.supply, "supply", 0, "set the token supply")
	cmd.Flags().StringVar(&deployERC20Flags.funded, "funded", "", "set the funded address")
	cmd.Flags().StringVar(&deployERC20Flags.rpcEndpoint, "rpc", "", "deploy the contract into the given rpc endpoint")
	return cmd
}

func deployERC20(_ *cobra.Command, _ []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		deployERC20Flags.Network,
		true,
		false,
		networkoptions.DefaultSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	if err := deployERC20Flags.chainFlags.CheckMutuallyExclusiveFields(); err != nil {
		return err
	}
	if !deployERC20Flags.chainFlags.Defined() {
		prompt := "Where do you want to Deploy the ERC-20 Token?"
		if cancel, err := contract.PromptChain(
			app,
			network,
			prompt,
			"",
			&deployERC20Flags.chainFlags,
		); cancel || err != nil {
			return err
		}
	}
	if deployERC20Flags.rpcEndpoint == "" {
		deployERC20Flags.rpcEndpoint, _, err = contract.GetBlockchainEndpoints(
			app,
			network,
			deployERC20Flags.chainFlags,
			true,
			false,
		)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser(logging.Yellow.Wrap("RPC Endpoint: %s"), deployERC20Flags.rpcEndpoint)
	}
	genesisAddress, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
		app,
		network,
		deployERC20Flags.chainFlags,
	)
	if err != nil {
		return err
	}
	privateKey, err := deployERC20Flags.PrivateKeyFlags.GetPrivateKey(app, genesisPrivateKey)
	if err != nil {
		return err
	}
	if privateKey == "" {
		ux.Logger.PrintToUser("A private key is needed to pay for the contract deploy fees.")
		ux.Logger.PrintToUser("It will also be considered the owner address of the contract, beign able to call")
		ux.Logger.PrintToUser("the contract methods only available to owners.")
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
		ux.Logger.PrintToUser("Which is the token symbol?")
		deployERC20Flags.symbol, err = app.Prompt.CaptureString("Token symbol")
		if err != nil {
			return err
		}
	}
	supply := new(big.Int).SetUint64(deployERC20Flags.supply)
	if deployERC20Flags.supply == 0 {
		ux.Logger.PrintToUser("Which is the total token supply?")
		supply, err = app.Prompt.CapturePositiveBigInt("Token supply")
		if err != nil {
			return err
		}
	}
	if deployERC20Flags.funded == "" {
		ux.Logger.PrintToUser("Which address should receive the supply?")
		deployERC20Flags.funded, err = prompts.PromptAddress(
			app.Prompt,
			"receive the total token supply",
			app.GetKeyDir(),
			app.GetKey,
			genesisAddress,
			network,
			prompts.EVMFormat,
			"Address",
		)
		if err != nil {
			return err
		}
	}
	signer, err := evm.NewSignerFromPrivateKey(privateKey)
	if err != nil {
		return err
	}
	address, _, _, err := contract.DeployERC20(
		deployERC20Flags.rpcEndpoint,
		signer,
		deployERC20Flags.symbol,
		common.HexToAddress(deployERC20Flags.funded),
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
