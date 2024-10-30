// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contractcmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/cmd/blockchaincmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"
)

type GetConversionMessageFlags struct {
	Network                  networkoptions.NetworkFlags
	rpcEndpoint              string
	aggregatorLogLevel       string
	aggregatorExtraEndpoints []string
}

var (
	getConversionMessageSupportedNetworkFlags = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Devnet,
		networkoptions.Fuji,
	}
	getConversionMessageFlags GetConversionMessageFlags
)

// avalanche contract getValidatorManagerConversionMessage
func newGetConversionMessageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "getConversionMessage blockchainName",
		Short: "Fetches and signs the conversion message from a ValidatorManager for a given blockchain.",
		Long:  "Fetches the conversion message from a ValidatorManager for a given blockchain and signs it with the boot-strapped validator set.",
		RunE:  getConversionMessage,
		Args:  cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &getConversionMessageFlags.Network, true, getConversionMessageSupportedNetworkFlags)
	cmd.Flags().StringVar(&getConversionMessageFlags.rpcEndpoint, "rpc", "", "get conversion message from the given rpc endpoint")
	cmd.Flags().StringSliceVar(&getConversionMessageFlags.aggregatorExtraEndpoints, "aggregator-extra-endpoints", nil, "endpoints for extra nodes that are needed in signature aggregation")
	cmd.Flags().StringVar(&getConversionMessageFlags.aggregatorLogLevel, "aggregator-log-level", "Off", "log level to use with signature aggregator")
	return cmd
}

func getConversionMessage(_ *cobra.Command, args []string) error {
	blockchainName := args[0]
	chainSpec := contract.ChainSpec{
		BlockchainName: blockchainName,
	}
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		getConversionMessageFlags.Network,
		true,
		false,
		getConversionMessageSupportedNetworkFlags,
		"",
	)
	if err != nil {
		return err
	}
	if getConversionMessageFlags.rpcEndpoint == "" {
		getConversionMessageFlags.rpcEndpoint, _, err = contract.GetBlockchainEndpoints(
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
	ux.Logger.PrintToUser(logging.Yellow.Wrap("RPC Endpoint: %s"), getConversionMessageFlags.rpcEndpoint)

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
	aggregatorExtraPeerEndpoints, err := blockchaincmd.GetAggregatorExtraPeers(network, getConversionMessageFlags.aggregatorExtraEndpoints)
	if err != nil {
		return err
	}

	aggregatorLogLevel, err := logging.ToLevel(getConversionMessageFlags.aggregatorLogLevel)
	if err != nil {
		aggregatorLogLevel = logging.Off
	}

	subnetConversionSignedMessage, err := validatormanager.ValidatorManagerGetPChainSubnetConversionWarpMessage(
		network,
		aggregatorLogLevel,
		0,
		aggregatorExtraPeerEndpoints,
		sc.Networks[network.Name()].SubnetID,
		sc.Networks[network.Name()].BlockchainID,
		common.HexToAddress(validatormanager.ProxyContractAddress),
		avaGoBootstrapValidators,
	)
	if err != nil {
		return err
	}
	ux.Logger.GreenCheckmarkToUser("Message successfully signed %s", blockchainName)
	ux.Logger.PrintToUser(logging.Green.Wrap("Subnet Conversion Signed Message: %s"), subnetConversionSignedMessage)
	return nil
}
