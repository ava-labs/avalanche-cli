// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package signatureaggregatorcmd

import (
	"fmt"
	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/signatureaggregator"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"os"

	"github.com/spf13/cobra"
)

var startNetworkOptions = []networkoptions.NetworkOption{
	networkoptions.Local,
	networkoptions.Fuji,
}

type StartFlags struct {
	Network     networkoptions.NetworkFlags
	SigAggFlags flags.SignatureAggregatorFlags
	l1          string
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
	cmd.Flags().StringVar(&startFlags.l1, "l1", "", "name of L1")
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
	if network.Kind == models.Local {
		isRunning, err := localnet.IsLocalNetworkRunning(app)
		if err != nil {
			return err
		}
		if !isRunning {
			return fmt.Errorf("unable to start local signature aggregator for local network as local network is not running. Run avalanche network start to start local networl")
		}
	}
	if err = createLocalSignatureAggregator(network); err != nil {
		return err
	}
	runFilePath := app.GetLocalSignatureAggregatorRunPath(network.Kind)
	// Check if run file exists and read ports from it
	_, err = os.Stat(runFilePath)
	if err != nil {
		return err
	}
	// File exists, get process details
	runFile, err := signatureaggregator.GetCurrentSignatureAggregatorProcessDetails(app, network)
	if err != nil {
		return fmt.Errorf("failed to get process details: %w", err)
	}
	signatureAggregatorEndpoint, err := signatureaggregator.GetSignatureAggregatorEndpoint(app, network)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("  Signature Aggregator %s Network API Endpoint: %s", network.Name(), signatureAggregatorEndpoint)
	ux.Logger.PrintToUser("  Signature Aggregator %s Network API Port: %d", network.Name(), runFile.APIPort)
	ux.Logger.PrintToUser("  Signature Aggregator %s Network Metrics Port: %d", network.Name(), runFile.MetricsPort)
	ux.Logger.GreenCheckmarkToUser("Local Signature Aggregator successfully started for %s", network.Kind)
	return nil
}

func createLocalSignatureAggregator(network models.Network) error {
	aggregatorLogger, err := signatureaggregator.NewSignatureAggregatorLogger(
		startFlags.SigAggFlags.AggregatorLogLevel,
		startFlags.SigAggFlags.AggregatorLogToStdout,
		app.GetAggregatorLogDir(""),
	)
	if err != nil {
		return err
	}
	err = signatureaggregator.CreateSignatureAggregatorInstance(app, network, aggregatorLogger, "latest")
	if err != nil {
		return err
	}
	return nil
}
