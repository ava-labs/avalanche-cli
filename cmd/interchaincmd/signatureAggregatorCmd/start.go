// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package signatureAggregatorCmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/signatureaggregator"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

var startNetworkOptions = []networkoptions.NetworkOption{
	networkoptions.Local,
	networkoptions.Fuji,
}

type StartFlags struct {
	Network networkoptions.NetworkFlags
}

var startFlags StartFlags

// avalanche interchain signatureAggregator start
func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "starts signature aggregator",
		Long:  `Starts locally run signature aggregator for the specified network with the L1's aggregator peers'`,
		RunE:  start,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &startFlags.Network, true, startNetworkOptions)
	return cmd
}

func start(_ *cobra.Command, _ []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		startFlags.Network,
		false,
		false,
		startNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	// Clean up signature aggregator
	if err := signatureaggregator.SignatureAggregatorCleanup(app, network); err != nil {
		return err
	}
	ux.Logger.GreenCheckmarkToUser("Local Signature Aggregator successfully stopped for %s", network.Kind)
	return nil
}
