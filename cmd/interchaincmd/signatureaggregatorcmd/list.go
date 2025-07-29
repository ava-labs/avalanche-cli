// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package signatureaggregatorcmd

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/signatureaggregator"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/spf13/cobra"
)

var listNetworkOptions = []networkoptions.NetworkOption{
	networkoptions.Local,
	networkoptions.Fuji,
	networkoptions.Mainnet,
}

type ListFlags struct {
	Network networkoptions.NetworkFlags
}

var listFlags ListFlags

// avalanche interchain signatureAggregator list
func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "lists signature aggregator endpoints",
		Long:  `Lists locally run signature aggregator API and metrics endpoints for the specified network.`,
		RunE:  list,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &listFlags.Network, true, listNetworkOptions)
	return cmd
}

func list(_ *cobra.Command, _ []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		listFlags.Network,
		false,
		false,
		listNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	runFilePath := app.GetLocalSignatureAggregatorRunPath(network.Kind)
	// Check if run file exists and read ports from it
	if _, err := os.Stat(runFilePath); err == nil {
		// File exists, get process details
		runFile, err := signatureaggregator.GetCurrentSignatureAggregatorProcessDetails(app, network)
		if err != nil {
			return fmt.Errorf("failed to get process details: %w", err)
		}
		ux.Logger.PrintToUser("  Signature Aggregator %s Network API Endpoint: %s", network.Name(), fmt.Sprintf("http://localhost:%d/aggregate-signatures", runFile.APIPort))
		ux.Logger.PrintToUser("  Signature Aggregator %s Network API Port: %d", network.Name(), runFile.APIPort)
		ux.Logger.PrintToUser("  Signature Aggregator %s Network Metrics Port: %d", network.Name(), runFile.MetricsPort)
	} else {
		ux.Logger.PrintToUser("No locally run signature aggregator found for %s", network.Name())
	}
	return nil
}
