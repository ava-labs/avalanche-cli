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

	"github.com/spf13/cobra"
)

type InitPOSManagerFlags struct {
	Network                  networkoptions.NetworkFlags
	PrivateKeyFlags          contract.PrivateKeyFlags
	rpcEndpoint              string
	aggregatorLogLevel       string
	aggregatorExtraEndpoints []string
}

var (
	initPOSManagerSupportedNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Devnet,
		networkoptions.Fuji,
	}
	initPOSManagerFlags InitPOSManagerFlags
)

// avalanche contract initposmanager
func newInitPOSManagerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "initPosManager blockchainName",
		Short: "Initializes a Native Proof of Stake Validator Manager on a given Network and Blockchain",
		Long:  "Initializes the Native Proof of Stake Validator Manager contract on a Blockchain and sets up initial validator set on the Blockchain. For more info on Validator Manager, please head to https://github.com/ava-labs/teleporter/tree/staking-contract/contracts/validator-manager",
		RunE:  initPOSManager,
		Args:  cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &initPOSManagerFlags.Network, true, initPOSManagerSupportedNetworkOptions)
	initPOSManagerFlags.PrivateKeyFlags.AddToCmd(cmd, "as contract deployer")
	cmd.Flags().StringVar(&initPOSManagerFlags.rpcEndpoint, "rpc", "", "deploy the contract into the given rpc endpoint")
	cmd.Flags().StringSliceVar(&initPOSManagerFlags.aggregatorExtraEndpoints, "aggregator-extra-endpoints", nil, "endpoints for extra nodes that are needed in signature aggregation")
	cmd.Flags().StringVar(&initPOSManagerFlags.aggregatorLogLevel, "aggregator-log-level", "Off", "log level to use with signature aggregator")
	return cmd
}

func initPOSManager(_ *cobra.Command, args []string) error {
	blockchainName := args[0]
	chainSpec := contract.ChainSpec{
		BlockchainName: blockchainName,
	}
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		initPOSManagerFlags.Network,
		true,
		false,
		initPOSManagerSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	if initPOSManagerFlags.rpcEndpoint == "" {
		initPOSManagerFlags.rpcEndpoint, _, err = contract.GetBlockchainEndpoints(
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
	ux.Logger.PrintToUser(logging.Yellow.Wrap("RPC Endpoint: %s"), initPOSManagerFlags.rpcEndpoint)
	genesisAddress, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return err
	}
	privateKey, err := initPOSManagerFlags.PrivateKeyFlags.GetPrivateKey(app, genesisPrivateKey)
	if err != nil {
		return err
	}
	if privateKey == "" {
		privateKey, err = prompts.PromptPrivateKey(
			app.Prompt,
			"pay for initializing Proof of Stake Validator Manager contract? (Uses Blockchain gas token)",
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
	if len(bootstrapValidators) == 0 {
		return fmt.Errorf("no bootstrap validators found for blockchain %s", blockchainName)
	}
	avaGoBootstrapValidators, err := blockchaincmd.ConvertToAvalancheGoSubnetValidator(bootstrapValidators)
	if err != nil {
		return err
	}
	extraAggregatorPeers, err := blockchaincmd.GetAggregatorExtraPeers(network, initPOSManagerFlags.aggregatorExtraEndpoints)
	if err != nil {
		return err
	}

	minimumStakeAmount, err := app.Prompt.CapturePositiveBigInt("Enter the minimum stake amount")
	if err != nil {
		return err
	}

	maximumStakeAmount, err := app.Prompt.CapturePositiveBigInt("Enter the maximum stake amount")
	if err != nil {
		return err
	}

	minimumStakeDuration, err := app.Prompt.CaptureUint64("Enter the minimum stake duration (in seconds)")
	if err != nil {
		return err
	}

	minimumDelegationFee, err := app.Prompt.CaptureUint16("Enter the minimum delegation fee")
	if err != nil {
		return err
	}

	maximumStakeMultiplier, err := app.Prompt.CaptureUint8("Enter the maximum stake multiplier")
	if err != nil {
		return err
	}

	weightToValueFactor, err := app.Prompt.CapturePositiveBigInt("Enter the weight to value factor")
	if err != nil {
		return err
	}
	if err := validatormanager.SetupPoS(
		app,
		network,
		initPOSManagerFlags.rpcEndpoint,
		chainSpec,
		privateKey,
		avaGoBootstrapValidators,
		extraAggregatorPeers,
		initPOSManagerFlags.aggregatorLogLevel,
		minimumStakeAmount,
		maximumStakeAmount,
		minimumStakeDuration,
		minimumDelegationFee,
		maximumStakeMultiplier,
		weightToValueFactor,
	); err != nil {
		return err
	}
	ux.Logger.GreenCheckmarkToUser("Native Token Proof of Stake Validator Manager contract successfully initialized on blockchain %s", blockchainName)
	return nil
}
