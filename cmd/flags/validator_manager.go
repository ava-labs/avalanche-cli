// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package flags

import (
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/spf13/cobra"
)

const (
	rpcURLFLag                = "rpc"
	aggregatorLogLevelFlag    = "aggregator-log-level"
	aggregatorLogToStdoutFlag = "aggregator-log-to-stdout"
)

var (
	RPC         string
	SigAggFlags SignatureAggregatorFlags
)

type SignatureAggregatorFlags struct {
	AggregatorLogLevel    string
	AggregatorLogToStdout bool
}

func AddRPCFlagToCmd(cmd *cobra.Command) {
	cmd.Flags().StringVar(&RPC, rpcURLFLag, "", "blockchain rpc endpoint")
}

func AddSignatureAggregatorFlagsToCmd(cmd *cobra.Command) {
	cmd.Flags().StringVar(&SigAggFlags.AggregatorLogLevel, aggregatorLogLevelFlag, constants.DefaultAggregatorLogLevel, "log level to use with signature aggregator")
	cmd.Flags().BoolVar(&SigAggFlags.AggregatorLogToStdout, aggregatorLogToStdoutFlag, false, "use stdout for signature aggregator logs")
}
