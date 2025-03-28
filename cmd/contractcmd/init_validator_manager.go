// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contractcmd

import (
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/blockchain"

	"github.com/ava-labs/avalanche-cli/cmd/blockchaincmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	blockchainSDK "github.com/ava-labs/avalanche-cli/sdk/blockchain"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

type ValidatorManagerFlags struct {
	Network                     networkoptions.NetworkFlags
	PrivateKeyFlags             contract.PrivateKeyFlags
	rpcEndpoint                 string
	aggregatorLogLevel          string
	aggregatorLogToStdout       bool
	aggregatorExtraEndpoints    []string
	aggregatorAllowPrivatePeers bool
}

type POSManagerSpecFlags struct {
	rewardCalculatorAddress string
	minimumStakeAmount      uint64 // big.Int
	maximumStakeAmount      uint64 // big.Int
	minimumStakeDuration    uint64
	minimumDelegationFee    uint16
	maximumStakeMultiplier  uint8
	weightToValueFactor     uint64 // big.Int
}

var (
	validatorManagerFlags   ValidatorManagerFlags
	initPOSManagerFlags     POSManagerSpecFlags
	validatorManagerAddress string
)

// avalanche contract initValidatorManager
func newInitValidatorManagerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "initValidatorManager blockchainName",
		Short: "Initializes Proof of Authority(PoA) or Proof of Stake(PoS) Validator Manager on a given Network and Blockchain",
		Long:  "Initializes Proof of Authority(PoA) or Proof of Stake(PoS)Validator Manager contract on a Blockchain and sets up initial validator set on the Blockchain. For more info on Validator Manager, please head to https://github.com/ava-labs/icm-contracts/tree/main/contracts/validator-manager",
		RunE:  initValidatorManager,
		Args:  cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &validatorManagerFlags.Network, true, networkoptions.DefaultSupportedNetworkOptions)
	validatorManagerFlags.PrivateKeyFlags.AddToCmd(cmd, "as contract deployer")
	cmd.Flags().StringVar(&validatorManagerFlags.rpcEndpoint, "rpc", "", "deploy the contract into the given rpc endpoint")
	cmd.Flags().StringSliceVar(&validatorManagerFlags.aggregatorExtraEndpoints, "aggregator-extra-endpoints", nil, "endpoints for extra nodes that are needed in signature aggregation")
	cmd.Flags().BoolVar(&validatorManagerFlags.aggregatorAllowPrivatePeers, "aggregator-allow-private-peers", true, "allow the signature aggregator to connect to peers with private IP")
	cmd.Flags().StringVar(&validatorManagerFlags.aggregatorLogLevel, "aggregator-log-level", constants.DefaultAggregatorLogLevel, "log level to use with signature aggregator")
	cmd.Flags().BoolVar(&validatorManagerFlags.aggregatorLogToStdout, "aggregator-log-to-stdout", false, "dump signature aggregator logs to stdout")

	cmd.Flags().StringVar(&initPOSManagerFlags.rewardCalculatorAddress, "pos-reward-calculator-address", "", "(PoS only) initialize the ValidatorManager with reward calculator address")
	cmd.Flags().Uint64Var(&initPOSManagerFlags.minimumStakeAmount, "pos-minimum-stake-amount", 1, "(PoS only) minimum stake amount")
	cmd.Flags().Uint64Var(&initPOSManagerFlags.maximumStakeAmount, "pos-maximum-stake-amount", 1000, "(PoS only) maximum stake amount")
	cmd.Flags().Uint64Var(&initPOSManagerFlags.minimumStakeDuration, "pos-minimum-stake-duration", constants.PoSL1MinimumStakeDurationSeconds, "(PoS only) minimum stake duration (in seconds)")
	cmd.Flags().Uint16Var(&initPOSManagerFlags.minimumDelegationFee, "pos-minimum-delegation-fee", 1, "(PoS only) minimum delegation fee")
	cmd.Flags().Uint8Var(&initPOSManagerFlags.maximumStakeMultiplier, "pos-maximum-stake-multiplier", 1, "(PoS only )maximum stake multiplier")
	cmd.Flags().Uint64Var(&initPOSManagerFlags.weightToValueFactor, "pos-weight-to-value-factor", 1, "(PoS only) weight to value factor")
	return cmd
}

func initValidatorManager(_ *cobra.Command, args []string) error {
	blockchainName := args[0]
	chainSpec := contract.ChainSpec{
		BlockchainName: blockchainName,
	}
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		validatorManagerFlags.Network,
		true,
		false,
		networkoptions.DefaultSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	if network.ClusterName != "" {
		network = models.ConvertClusterToNetwork(network)
	}
	if validatorManagerFlags.rpcEndpoint == "" {
		validatorManagerFlags.rpcEndpoint, _, err = contract.GetBlockchainEndpoints(
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
	ux.Logger.PrintToUser(logging.Yellow.Wrap("RPC Endpoint: %s"), validatorManagerFlags.rpcEndpoint)
	genesisAddress, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return err
	}
	privateKey, err := validatorManagerFlags.PrivateKeyFlags.GetPrivateKey(app, genesisPrivateKey)
	if err != nil {
		return err
	}
	if privateKey == "" {
		privateKey, err = prompts.PromptPrivateKey(
			app.Prompt,
			"pay for initializing Proof of Authority Validator Manager contract? (Uses Blockchain gas token)",
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
	if sc.Networks[network.Name()].ValidatorManagerAddress == "" {
		return fmt.Errorf("unable to find Validator Manager address")
	}
	validatorManagerAddress = sc.Networks[network.Name()].ValidatorManagerAddress
	scNetwork := sc.Networks[network.Name()]
	if scNetwork.BlockchainID == ids.Empty {
		return fmt.Errorf("blockchain has not been deployed to %s", network.Name())
	}
	bootstrapValidators := scNetwork.BootstrapValidators
	avaGoBootstrapValidators, err := blockchaincmd.ConvertToAvalancheGoSubnetValidator(bootstrapValidators)
	if err != nil {
		return err
	}
	clusterName := scNetwork.ClusterName
	extraAggregatorPeers, err := blockchain.GetAggregatorExtraPeers(app, clusterName, validatorManagerFlags.aggregatorExtraEndpoints)
	if err != nil {
		return err
	}
	aggregatorLogger, err := utils.NewLogger(
		constants.SignatureAggregatorLogName,
		validatorManagerFlags.aggregatorLogLevel,
		constants.DefaultAggregatorLogLevel,
		app.GetAggregatorLogDir(clusterName),
		validatorManagerFlags.aggregatorLogToStdout,
		ux.Logger.PrintToUser,
	)
	if err != nil {
		return err
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
		RPC:                 validatorManagerFlags.rpcEndpoint,
	}
	aggregatorCtx, aggregatorCancel := sdkutils.GetTimedContext(constants.SignatureAggregatorTimeout)
	defer aggregatorCancel()
	switch {
	case sc.PoA(): // PoA
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Initializing Proof of Authority Validator Manager contract on blockchain %s"), blockchainName)
		if err := validatormanager.SetupPoA(
			aggregatorCtx,
			app.Log,
			subnetSDK,
			network,
			privateKey,
			extraAggregatorPeers,
			validatorManagerFlags.aggregatorAllowPrivatePeers,
			aggregatorLogger,
			validatorManagerAddress,
			sc.UseACP99,
		); err != nil {
			return err
		}
		ux.Logger.GreenCheckmarkToUser("Proof of Authority Validator Manager contract successfully initialized on blockchain %s", blockchainName)
	case sc.PoS(): // PoS
		deployed, err := validatormanager.ProxyHasValidatorManagerSet(validatorManagerFlags.rpcEndpoint)
		if err != nil {
			return err
		}
		if !deployed {
			// it is not in genesis
			ux.Logger.PrintToUser("Deploying Proof of Stake Validator Manager contract on blockchain %s ...", blockchainName)
			proxyOwnerPrivateKey, err := blockchaincmd.GetProxyOwnerPrivateKey(
				app,
				network,
				sc.ProxyContractOwner,
				ux.Logger.PrintToUser,
			)
			if err != nil {
				return err
			}
			if _, err := validatormanager.DeployAndRegisterPoSValidatorManagerContrac(
				validatorManagerFlags.rpcEndpoint,
				genesisPrivateKey,
				proxyOwnerPrivateKey,
			); err != nil {
				return err
			}
		}
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Initializing Proof of Stake Validator Manager contract on blockchain %s"), blockchainName)
		if initPOSManagerFlags.rewardCalculatorAddress == "" {
			initPOSManagerFlags.rewardCalculatorAddress = validatorManagerSDK.RewardCalculatorAddress
		}
		if err := validatormanager.SetupPoS(
			aggregatorCtx,
			app.Log,
			subnetSDK,
			network,
			privateKey,
			extraAggregatorPeers,
			validatorManagerFlags.aggregatorAllowPrivatePeers,
			aggregatorLogger,
			validatorManagerSDK.PoSParams{
				MinimumStakeAmount:      big.NewInt(int64(initPOSManagerFlags.minimumStakeAmount)),
				MaximumStakeAmount:      big.NewInt(int64(initPOSManagerFlags.maximumStakeAmount)),
				MinimumStakeDuration:    initPOSManagerFlags.minimumStakeDuration,
				MinimumDelegationFee:    initPOSManagerFlags.minimumDelegationFee,
				MaximumStakeMultiplier:  initPOSManagerFlags.maximumStakeMultiplier,
				WeightToValueFactor:     big.NewInt(int64(initPOSManagerFlags.weightToValueFactor)),
				RewardCalculatorAddress: initPOSManagerFlags.rewardCalculatorAddress,
				UptimeBlockchainID:      blockchainID,
			},
			validatorManagerAddress,
		); err != nil {
			return err
		}
		ux.Logger.GreenCheckmarkToUser("Native Token Proof of Stake Validator Manager contract successfully initialized on blockchain %s", blockchainName)
	default: // unsupported
		return fmt.Errorf("only PoA and PoS supported")
	}
	return nil
}
