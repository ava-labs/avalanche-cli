// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contractcmd

import (
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/cmd/blockchaincmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	blockchainSDK "github.com/ava-labs/avalanche-cli/sdk/blockchain"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"
)

type InitPOSManagerFlags struct {
	Network                  networkoptions.NetworkFlags
	PrivateKeyFlags          contract.PrivateKeyFlags
	rpcEndpoint              string
	rewardCalculatorAddress  string
	aggregatorLogLevel       string
	aggregatorExtraEndpoints []string
	minimumStakeAmount       uint64 // big.Int
	maximumStakeAmount       uint64 // big.Int
	minimumStakeDuration     uint64
	minimumDelegationFee     uint16
	maximumStakeMultiplier   uint8
	weightToValueFactor      uint64 // big.Int
}

var (
	initPOSManagerSupportedNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Devnet,
		networkoptions.EtnaDevnet,
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
	cmd.Flags().StringVar(&initPOSManagerFlags.rewardCalculatorAddress, "reward-calculator-address", "", "initialize the ValidatorManager with reward calculator address")
	cmd.Flags().StringSliceVar(&initPOSManagerFlags.aggregatorExtraEndpoints, "aggregator-extra-endpoints", nil, "endpoints for extra nodes that are needed in signature aggregation")
	cmd.Flags().StringVar(&initPOSManagerFlags.aggregatorLogLevel, "aggregator-log-level", "Off", "log level to use with signature aggregator")

	cmd.Flags().Uint64Var(&initPOSManagerFlags.minimumStakeAmount, "pos-minimum-stake-amount", 1, "(PoS only) minimum stake amount")
	cmd.Flags().Uint64Var(&initPOSManagerFlags.maximumStakeAmount, "pos-maximum-stake-amount", 1000, "(PoS only) maximum stake amount")
	cmd.Flags().Uint64Var(&initPOSManagerFlags.minimumStakeDuration, "pos-minimum-stake-duration", 100, "(PoS only) minimum stake duration")
	cmd.Flags().Uint16Var(&initPOSManagerFlags.minimumDelegationFee, "pos-minimum-delegation-fee", 1, "(PoS only) minimum delegation fee")
	cmd.Flags().Uint8Var(&initPOSManagerFlags.maximumStakeMultiplier, "pos-maximum-stake-multiplier", 1, "(PoS only )maximum stake multiplier")
	cmd.Flags().Uint64Var(&initPOSManagerFlags.weightToValueFactor, "pos-weight-to-value-factor", 1, "(PoS only) weight to value factor")
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
	if network.ClusterName != "" {
		clusterNameFlagValue = network.ClusterName
		network = models.ConvertClusterToNetwork(network)
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
	extraAggregatorPeers, err := blockchaincmd.GetAggregatorExtraPeers(clusterNameFlagValue, initPOSManagerFlags.aggregatorExtraEndpoints)
	if err != nil {
		return err
	}
	if initPOSManagerFlags.rewardCalculatorAddress == "" {
		initPOSManagerFlags.rewardCalculatorAddress = validatorManagerSDK.RewardCalculatorAddress
	}
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return err
	}
	blockchainID, err := contract.GetBlockchainID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return err
	}
	ownerAddress := common.HexToAddress(sc.ProxyContractOwner)
	subnetSDK := blockchainSDK.Subnet{
		SubnetID:            subnetID,
		BlockchainID:        blockchainID,
		BootstrapValidators: avaGoBootstrapValidators,
		OwnerAddress:        &ownerAddress,
		RPC:                 initPOSManagerFlags.rpcEndpoint,
	}
	ux.Logger.PrintToUser("Initializing native token Proof of Stake Validator Manager contract on blockchain %s", subnetSDK.RPC)
	if err := validatormanager.SetupPoS(
		subnetSDK,
		network,
		privateKey,
		extraAggregatorPeers,
		initPOSManagerFlags.aggregatorLogLevel,
		validatorManagerSDK.PoSParams{
			MinimumStakeAmount:      big.NewInt(int64(initPOSManagerFlags.minimumStakeAmount)),
			MaximumStakeAmount:      big.NewInt(int64(initPOSManagerFlags.maximumStakeAmount)),
			MinimumStakeDuration:    initPOSManagerFlags.minimumStakeDuration,
			MinimumDelegationFee:    initPOSManagerFlags.minimumDelegationFee,
			MaximumStakeMultiplier:  initPOSManagerFlags.maximumStakeMultiplier,
			WeightToValueFactor:     big.NewInt(int64(initPOSManagerFlags.weightToValueFactor)),
			RewardCalculatorAddress: initPOSManagerFlags.rewardCalculatorAddress,
		},
	); err != nil {
		return err
	}
	ux.Logger.GreenCheckmarkToUser("Native Token Proof of Stake Validator Manager contract successfully initialized on blockchain %s", blockchainName)
	return nil
}
