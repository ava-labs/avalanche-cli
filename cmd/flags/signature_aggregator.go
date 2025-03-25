// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package flags

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/spf13/cobra"
)

const (
	aggregatorLogLevelFlag    = "aggregator-log-level"
	aggregatorLogToStdoutFlag = "aggregator-log-to-stdout"
)

var SigAggFlags SignatureAggregatorFlags

type SignatureAggregatorFlags struct {
	AggregatorLogLevel    string
	AggregatorLogToStdout bool
}

func validateSignatureAggregatorFlags() error {
	if _, err := logging.ToLevel(SigAggFlags.AggregatorLogLevel); err != nil {
		return fmt.Errorf(
			"invalid log level: %q. Available values: %s, %s, %s, %s, %s, %s, %s, %s",
			SigAggFlags.AggregatorLogLevel,
			logging.Info.LowerString(),
			logging.Warn.LowerString(),
			logging.Error.LowerString(),
			logging.Off.LowerString(),
			logging.Fatal.LowerString(),
			logging.Debug.LowerString(),
			logging.Trace.LowerString(),
			logging.Verbo.LowerString(),
		)
	}
	return nil
}

func AddSignatureAggregatorFlagsToCmd(cmd *cobra.Command) {
	cmd.Flags().StringVar(&SigAggFlags.AggregatorLogLevel, aggregatorLogLevelFlag, constants.DefaultAggregatorLogLevel, "log level to use with signature aggregator")
	sigAggPreRun := func(_ *cobra.Command, _ []string) error {
		if err := validateSignatureAggregatorFlags(); err != nil {
			return err
		}
		return nil
	}

	existingPreRunE := cmd.PreRunE
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if existingPreRunE != nil {
			if err := existingPreRunE(cmd, args); err != nil {
				return err
			}
		}
		return sigAggPreRun(cmd, args)
	}

	cmd.Flags().BoolVar(&SigAggFlags.AggregatorLogToStdout, aggregatorLogToStdoutFlag, false, "use stdout for signature aggregator logs")
}
