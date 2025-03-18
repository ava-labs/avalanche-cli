// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package flags

import (
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/spf13/cobra"
)

const (
	rpcURLFLag                      = "rpc"
	aggregatorLogLevelFlag          = "aggregator-log-level"
	aggregatorLogToStdoutFlag       = "aggregator-log-to-stdout"
	aggregatorExtraEndpointsFlag    = "aggregator-extra-endpoints"
	aggregatorAllowPrivatePeersFlag = "aggregator-allow-private-peers"
)

type ValidatorManagerFlags struct {
	RPC         string
	SigAggFlags SignatureAggregatorFlags
}

type SignatureAggregatorFlags struct {
	AggregatorLogLevel    string
	AggregatorLogToStdout bool
	//AggregatorExtraEndpoints    []string
	//AggregatorAllowPrivatePeers bool
}

func AddValidatorManagerFlagsToCmd(cmd *cobra.Command, flags *ValidatorManagerFlags, addRPCFlag bool) {
	cmd.Flags().StringVar(&flags.SigAggFlags.AggregatorLogLevel, aggregatorLogLevelFlag, constants.DefaultAggregatorLogLevel, "log level to use with signature aggregator")
	cmd.Flags().BoolVar(&flags.SigAggFlags.AggregatorLogToStdout, aggregatorLogToStdoutFlag, false, "use stdout for signature aggregator logs")
	//cmd.Flags().StringSliceVar(&flags.SigAggFlags.AggregatorExtraEndpoints, aggregatorExtraEndpointsFlag, nil, "endpoints for extra nodes that are needed in signature aggregation")
	//cmd.Flags().BoolVar(&flags.SigAggFlags.AggregatorAllowPrivatePeers, aggregatorAllowPrivatePeersFlag, true, "allow the signature aggregator to connect to peers with private IP")
	if addRPCFlag {
		cmd.Flags().StringVar(&flags.RPC, rpcURLFLag, "", "connect to validator manager at the given rpc endpoint")
	}
}
