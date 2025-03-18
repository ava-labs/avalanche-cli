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

type ValidatorManagerFlags struct {
	RPC         string
	SigAggFlags SignatureAggregatorFlags
}

type SignatureAggregatorFlags struct {
	AggregatorLogLevel    string
	AggregatorLogToStdout bool
}

func AddValidatorManagerFlagsToCmd(cmd *cobra.Command, flags *ValidatorManagerFlags, addRPCFlag bool) {
	cmd.Flags().StringVar(&flags.SigAggFlags.AggregatorLogLevel, aggregatorLogLevelFlag, constants.DefaultAggregatorLogLevel, "log level to use with signature aggregator")
	cmd.Flags().BoolVar(&flags.SigAggFlags.AggregatorLogToStdout, aggregatorLogToStdoutFlag, false, "use stdout for signature aggregator logs")
	if addRPCFlag {
		cmd.Flags().StringVar(&flags.RPC, rpcURLFLag, "", "connect to validator manager at the given rpc endpoint")
	}
}
