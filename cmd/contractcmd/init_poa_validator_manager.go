// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contractcmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/cmd/blockchaincmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"
)

type InitPOAManagerFlags struct {
	Network         networkoptions.NetworkFlags
	PrivateKeyFlags contract.PrivateKeyFlags
	rpcEndpoint     string
}

var (
	initPOAManagerSupportedNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Devnet,
		networkoptions.Fuji,
	}
	initPOAManagerFlags        InitPOAManagerFlags
	privateAggregatorEndpoints []string
)

// avalanche contract initpoamanager
func newInitPOAManagerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "initPoaManager",
		Short: "Initializes a Proof of Authority Validator Manager on a given Network and Blockchain",
		Long:  "Initializes Proof of Authority Validator Manager contract on a Blockchain and sets up initial validator set on the Blockchain. For more info on Validator Manager, please head to https://github.com/ava-labs/teleporter/tree/staking-contract/contracts/validator-manager",
		RunE:  initPOAManager,
		Args:  cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &initPOAManagerFlags.Network, true, initPOAManagerSupportedNetworkOptions)
	initPOAManagerFlags.PrivateKeyFlags.AddToCmd(cmd, "as contract deployer")
	cmd.Flags().StringVar(&initPOAManagerFlags.rpcEndpoint, "rpc", "", "deploy the contract into the given rpc endpoint")
	cmd.Flags().StringSliceVar(&privateAggregatorEndpoints, "private-aggregator-endpoints", nil, "endpoints for private nodes that are not available as network peers but are needed in signature aggregation")
	return cmd
}

func initPOAManager(_ *cobra.Command, args []string) error {
	blockchainName := args[0]
	chainSpec := contract.ChainSpec{
		BlockchainName: blockchainName,
	}
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		initPOAManagerFlags.Network,
		true,
		false,
		initPOAManagerSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	if initPOAManagerFlags.rpcEndpoint == "" {
		initPOAManagerFlags.rpcEndpoint, _, err = contract.GetBlockchainEndpoints(
			app,
			network,
			chainSpec,
			true,
			false,
		)
		if err != nil {
			return err
		}
	}
	ux.Logger.PrintToUser(logging.Yellow.Wrap("RPC Endpoint: %s"), initPOAManagerFlags.rpcEndpoint)
	genesisAddress, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return err
	}
	privateKey, err := initPOAManagerFlags.PrivateKeyFlags.GetPrivateKey(app, genesisPrivateKey)
	if err != nil {
		return err
	}
	if privateKey == "" {
		privateKey, err = prompts.PromptPrivateKey(
			app.Prompt,
			"Which key to you want to use to pay for initializing Proof of Authority Validator Manager contract? (Uses Blockchain gas token)",
			app.GetKeyDir(),
			app.GetKey,
			genesisAddress,
			genesisPrivateKey,
		)
		if err != nil {
			return err
		}
	}
	sc, err := app.LoadSidecar(chainSpec.BlockchainName)
	if err != nil {
		return fmt.Errorf("failed to load sidecar: %w", err)
	}
	if sc.Networks[network.Name()].BlockchainID == ids.Empty {
		return fmt.Errorf("blockchain has not been deployed to %s", network.Name())
	}
	bootstrapValidators := sc.Networks[network.Name()].BootstrapValidators
	avaGoBootstrapValidators, err := blockchaincmd.ConvertToAvalancheGoSubnetValidator(bootstrapValidators)
	if err != nil {
		return err
	}
	if len(privateAggregatorEndpoints) == 0 {
		privateAggregatorEndpoints, err = blockchaincmd.GetAggregatorExtraPeerEndpoints(network)
		if err != nil {
			return err
		}
	}
	if err := validatormanager.SetupPoA(
		app,
		network,
		initPOAManagerFlags.rpcEndpoint,
		chainSpec,
		privateKey,
		common.HexToAddress(sc.PoAValidatorManagerOwner),
		avaGoBootstrapValidators,
		privateAggregatorEndpoints,
	); err != nil {
		return err
	}
	ux.Logger.GreenCheckmarkToUser("Proof of Authority Validator Manager contract successfully initialized on blockchain %s", blockchainName)
	return nil
}
