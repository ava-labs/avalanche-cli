// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package signatureAggregatorCmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/signatureaggregator"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

var stopNetworkOptions = []networkoptions.NetworkOption{
	networkoptions.Local,
	networkoptions.Fuji,
}

type StopFlags struct {
	Network networkoptions.NetworkFlags
}

var stopFlags StopFlags

// avalanche interchain relayer stop
func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "stops signature aggregator",
		Long:  `Stops locally run signature aggregator for the specified network.`,
		RunE:  stop,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &stopFlags.Network, true, stopNetworkOptions)
	return cmd
}

func stop(_ *cobra.Command, args []string) error {
	return CallStop(args, stopFlags, models.UndefinedNetwork)
}

func CallStop(_ []string, flags StopFlags, network models.Network) error {
	var err error
	if network == models.UndefinedNetwork {
		network, err = networkoptions.GetNetworkFromCmdLineFlags(
			app,
			"",
			flags.Network,
			false,
			false,
			stopNetworkOptions,
			"",
		)
		if err != nil {
			return err
		}
	}
	// Clean up signature aggregator
	if err := signatureaggregator.SignatureAggregatorCleanup(app, network); err != nil {
		return err
	}
	ux.Logger.GreenCheckmarkToUser("Local Signature Aggregator successfully stopped for %s", network.Kind)
	return nil
}
