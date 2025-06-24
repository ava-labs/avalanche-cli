// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package flags

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/spf13/cobra"
)

const (
	aggregatorLogLevelFlag    = "aggregator-log-level"
	aggregatorLogToStdoutFlag = "aggregator-log-to-stdout"
)

type SignatureAggregatorFlags struct {
	AggregatorLogLevel          string
	AggregatorLogToStdout       bool
	SignatureAggregatorEndpoint string
}

func validateSignatureAggregatorFlags(sigAggFlags SignatureAggregatorFlags) error {
	if _, err := logging.ToLevel(sigAggFlags.AggregatorLogLevel); err != nil {
		return fmt.Errorf(
			"invalid log level: %q. Available values: %s, %s, %s, %s, %s, %s, %s, %s",
			sigAggFlags.AggregatorLogLevel,
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

func AddSignatureAggregatorFlagsToCmd(cmd *cobra.Command, sigAggFlags *SignatureAggregatorFlags) GroupedFlags {
	return RegisterFlagGroup(cmd, "Signature Aggregator Flags", "show-signature-aggregator-flags", false, func(set *pflag.FlagSet) {
		set.StringVar(&sigAggFlags.AggregatorLogLevel, aggregatorLogLevelFlag, constants.DefaultAggregatorLogLevel, "log level to use with signature aggregator")
		sigAggPreRun := func(_ *cobra.Command, _ []string) error {
			if err := validateSignatureAggregatorFlags(*sigAggFlags); err != nil {
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
		set.BoolVar(&sigAggFlags.AggregatorLogToStdout, aggregatorLogToStdoutFlag, false, "use stdout for signature aggregator logs")
		set.StringVar(&sigAggFlags.SignatureAggregatorEndpoint, "signature-aggregator-endpoint", "", "signature-aggregator-endpoint (uses locally run signature aggregator if empty)")
	})
}
