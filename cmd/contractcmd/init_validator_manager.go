// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contractcmd

import (
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/cmd/blockchaincmd"
	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/blockchain"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/signatureaggregator"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	blockchainSDK "github.com/ava-labs/avalanche-cli/sdk/blockchain"
	validatormanagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

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
	initPOSManagerFlags       POSManagerSpecFlags
	network                   networkoptions.NetworkFlags
	privateKeyFlags           contract.PrivateKeyFlags
	initValidatorManagerFlags ContractInitValidatorManagerFlags
)

type ContractInitValidatorManagerFlags struct {
	RPC         string
	SigAggFlags flags.SignatureAggregatorFlags
}

// avalanche contract initValidatorManager
func newInitValidatorManagerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "initValidatorManager blockchainName",
		Short:   "Initializes Proof of Authority(PoA) or Proof of Stake(PoS) Validator Manager on a given Network and Blockchain",
		Long:    "Initializes Proof of Authority(PoA) or Proof of Stake(PoS)Validator Manager contract on a Blockchain and sets up initial validator set on the Blockchain. For more info on Validator Manager, please head to https://github.com/ava-labs/icm-contracts/tree/main/contracts/validator-manager",
		RunE:    initValidatorManager,
		PreRunE: cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &network, true, networkoptions.DefaultSupportedNetworkOptions)
	privateKeyFlags.AddToCmd(cmd, "as contract deployer")
	flags.AddRPCFlagToCmd(cmd, app, &initValidatorManagerFlags.RPC)
	sigAggGroup := flags.AddSignatureAggregatorFlagsToCmd(cmd, &initValidatorManagerFlags.SigAggFlags)

	cmd.Flags().StringVar(&initPOSManagerFlags.rewardCalculatorAddress, "pos-reward-calculator-address", "", "(PoS only) initialize the ValidatorManager with reward calculator address")
	cmd.Flags().Uint64Var(&initPOSManagerFlags.minimumStakeAmount, "pos-minimum-stake-amount", 1, "(PoS only) minimum stake amount")
	cmd.Flags().Uint64Var(&initPOSManagerFlags.maximumStakeAmount, "pos-maximum-stake-amount", 1000, "(PoS only) maximum stake amount")
	cmd.Flags().Uint64Var(&initPOSManagerFlags.minimumStakeDuration, "pos-minimum-stake-duration", constants.PoSL1MinimumStakeDurationSeconds, "(PoS only) minimum stake duration (in seconds)")
	cmd.Flags().Uint16Var(&initPOSManagerFlags.minimumDelegationFee, "pos-minimum-delegation-fee", 1, "(PoS only) minimum delegation fee")
	cmd.Flags().Uint8Var(&initPOSManagerFlags.maximumStakeMultiplier, "pos-maximum-stake-multiplier", 1, "(PoS only )maximum stake multiplier")
	cmd.Flags().Uint64Var(&initPOSManagerFlags.weightToValueFactor, "pos-weight-to-value-factor", 1, "(PoS only) weight to value factor")
	cmd.SetHelpFunc(flags.WithGroupedHelp([]flags.GroupedFlags{sigAggGroup}))
	return cmd
}

func initValidatorManager(_ *cobra.Command, args []string) error {
	blockchainName := args[0]
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		network,
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

	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return fmt.Errorf("failed to load sidecar: %w", err)
	}

	blockchainID := sc.Networks[network.Name()].BlockchainID
	if blockchainID == ids.Empty {
		return fmt.Errorf("blockchain has not been deployed to %s", network.Name())
	}

	subnetID := sc.Networks[network.Name()].SubnetID
	if subnetID == ids.Empty {
		return fmt.Errorf("unable to find Subnet ID")
	}

	validatorManagerRPCEndpoint := sc.Networks[network.Name()].ValidatorManagerRPCEndpoint
	validatorManagerBlockchainID := sc.Networks[network.Name()].ValidatorManagerBlockchainID
	validatorManagerAddressStr := sc.Networks[network.Name()].ValidatorManagerAddress

	if validatorManagerBlockchainID == ids.Empty {
		return fmt.Errorf("unable to find Validator Manager blockchain ID")
	}
	if validatorManagerAddressStr == "" {
		return fmt.Errorf("unable to find Validator Manager address")
	}

	if initValidatorManagerFlags.RPC != "" {
		validatorManagerRPCEndpoint = initValidatorManagerFlags.RPC
	}

	if validatorManagerRPCEndpoint == "" {
		validatorManagerRPCEndpoint, err = blockchaincmd.GetValidatorManagerRPCEndpoint(
			network,
			blockchainName,
			blockchainID,
			validatorManagerBlockchainID,
		)
		if err != nil {
			return err
		}
	}
	ux.Logger.PrintToUser(logging.Yellow.Wrap("RPC Endpoint: %s"), validatorManagerRPCEndpoint)

	genesisAddress, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
		app,
		network,
		contract.ChainSpec{
			BlockchainID: validatorManagerBlockchainID.String(),
		},
	)
	if err != nil {
		return err
	}
	privateKey, err := privateKeyFlags.GetPrivateKey(app, genesisPrivateKey)
	if err != nil {
		return err
	}
	if privateKey == "" {
		privateKey, err = prompts.PromptPrivateKey(
			app.Prompt,
			"pay for initializing Validator Manager contract? (Uses Blockchain gas token)",
			app.GetKeyDir(),
			app.GetKey,
			genesisAddress,
			genesisPrivateKey,
		)
		if err != nil {
			return err
		}
	}

	var specializedValidatorManagerAddressStr string
	if sc.UseACP99 && sc.PoS() {
		if blockchainID == validatorManagerBlockchainID && validatorManagerAddressStr == validatormanagerSDK.ValidatorProxyContractAddress {
			// assumed to be managed by CLI
			specializedValidatorManagerAddressStr = validatormanagerSDK.SpecializationProxyContractAddress
		} else {
			specializedValidatorManagerAddress, err := app.Prompt.CaptureAddress("What is the address of the Specialized Validator Manager?")
			if err != nil {
				return err
			}
			specializedValidatorManagerAddressStr = specializedValidatorManagerAddress.String()
		}
	}

	validatorManagerAddress := common.HexToAddress(validatorManagerAddressStr)
	specializedValidatorManagerAddress := common.HexToAddress(specializedValidatorManagerAddressStr)

	validatorManagerOwnerAddressStr := sc.ValidatorManagerOwner
	validatorManagerOwnerAddress := common.HexToAddress(validatorManagerOwnerAddressStr)

	found, _, _, validatorManagerOwnerPrivateKey, err := contract.SearchForManagedKey(
		app,
		network,
		validatorManagerOwnerAddress,
		true,
	)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("could not find validator manager owner private key")
	}

	bootstrapValidators := sc.Networks[network.Name()].BootstrapValidators
	avaGoBootstrapValidators, err := blockchaincmd.ConvertToAvalancheGoSubnetValidator(bootstrapValidators)
	if err != nil {
		return err
	}

	clusterName := sc.Networks[network.Name()].ClusterName
	extraAggregatorPeers, err := blockchain.GetAggregatorExtraPeers(app, clusterName)
	if err != nil {
		return err
	}

	aggregatorLogger, err := signatureaggregator.NewSignatureAggregatorLogger(
		initValidatorManagerFlags.SigAggFlags.AggregatorLogLevel,
		initValidatorManagerFlags.SigAggFlags.AggregatorLogToStdout,
		app.GetAggregatorLogDir(clusterName),
	)
	if err != nil {
		return err
	}

	subnetSDK := blockchainSDK.Subnet{
		Network:                            network.SDKNetwork(),
		SubnetID:                           subnetID,
		ValidatorManagerRPC:                validatorManagerRPCEndpoint,
		ValidatorManagerBlockchainID:       validatorManagerBlockchainID,
		ValidatorManagerAddress:            &validatorManagerAddress,
		SpecializedValidatorManagerAddress: &specializedValidatorManagerAddress,
		ValidatorManagerOwnerAddress:       &validatorManagerOwnerAddress,
		ValidatorManagerOwnerPrivateKey:    validatorManagerOwnerPrivateKey,
		BootstrapValidators:                avaGoBootstrapValidators,
	}

	err = signatureaggregator.CreateSignatureAggregatorInstance(app, subnetID.String(), network, extraAggregatorPeers, aggregatorLogger, "latest")
	if err != nil {
		return err
	}
	signatureAggregatorEndpoint, err := signatureaggregator.GetSignatureAggregatorEndpoint(app, network)
	if err != nil {
		return err
	}

	switch {
	case sc.PoA(): // PoA
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Initializing Proof of Authority Validator Manager contract on blockchain %s"), blockchainName)
		if err := validatormanager.SetupPoA(
			app.Log,
			subnetSDK,
			privateKey,
			aggregatorLogger,
			sc.UseACP99,
			signatureAggregatorEndpoint,
		); err != nil {
			return err
		}
		ux.Logger.GreenCheckmarkToUser("Proof of Authority Validator Manager contract successfully initialized on blockchain %s", blockchainName)
	case sc.PoS(): // PoS
		if blockchainID == validatorManagerBlockchainID && validatorManagerAddressStr == validatormanagerSDK.ValidatorProxyContractAddress {
			// we assume it is fully CLI managed
			if err := blockchaincmd.CompleteValidatorManagerL1Deploy(
				network,
				blockchainName,
				validatorManagerRPCEndpoint,
				validatorManagerOwnerAddressStr,
				sc.PoS(),
				sc.UseACP99,
			); err != nil {
				return err
			}
		}

		ux.Logger.PrintToUser(logging.Yellow.Wrap("Initializing Proof of Stake Validator Manager contract on blockchain %s"), blockchainName)
		if initPOSManagerFlags.rewardCalculatorAddress == "" {
			initPOSManagerFlags.rewardCalculatorAddress = validatormanagerSDK.RewardCalculatorAddress
		}
		if err := validatormanager.SetupPoS(
			app.Log,
			subnetSDK,
			privateKey,
			aggregatorLogger,
			validatormanagerSDK.PoSParams{
				MinimumStakeAmount:      big.NewInt(int64(initPOSManagerFlags.minimumStakeAmount)),
				MaximumStakeAmount:      big.NewInt(int64(initPOSManagerFlags.maximumStakeAmount)),
				MinimumStakeDuration:    initPOSManagerFlags.minimumStakeDuration,
				MinimumDelegationFee:    initPOSManagerFlags.minimumDelegationFee,
				MaximumStakeMultiplier:  initPOSManagerFlags.maximumStakeMultiplier,
				WeightToValueFactor:     big.NewInt(int64(initPOSManagerFlags.weightToValueFactor)),
				RewardCalculatorAddress: initPOSManagerFlags.rewardCalculatorAddress,
				UptimeBlockchainID:      blockchainID,
			},
			sc.UseACP99,
			signatureAggregatorEndpoint,
		); err != nil {
			return err
		}
		ux.Logger.GreenCheckmarkToUser("Native Token Proof of Stake Validator Manager contract successfully initialized on blockchain %s", blockchainName)
	default: // unsupported
		return fmt.Errorf("only PoA and PoS supported")
	}

	sidecar, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}
	if specializedValidatorManagerAddress != (common.Address{}) {
		validatorManagerAddress = specializedValidatorManagerAddress
	}
	sidecar.UpdateValidatorManagerAddress(network.Name(), validatorManagerRPCEndpoint, validatorManagerBlockchainID, validatorManagerAddress.String())
	if err := app.UpdateSidecar(&sidecar); err != nil {
		return err
	}

	return nil
}
