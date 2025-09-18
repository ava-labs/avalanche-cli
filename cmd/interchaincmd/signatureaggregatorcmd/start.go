// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package signatureaggregatorcmd

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/signatureaggregator"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

var startNetworkOptions = []networkoptions.NetworkOption{
	networkoptions.Local,
	networkoptions.Fuji,
	networkoptions.Mainnet,
}

type StartFlags struct {
	Network     networkoptions.NetworkFlags
	SigAggFlags flags.SignatureAggregatorFlags
}

var startFlags StartFlags

// avalanche interchain signatureAggregator start
func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "starts signature aggregator",
		Long:  `Starts locally run signature aggregator for the specified network'`,
		RunE:  start,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &startFlags.Network, true, startNetworkOptions)
	sigAggGroup := flags.AddSignatureAggregatorFlagsToCmd(cmd, &startFlags.SigAggFlags)
	cmd.SetHelpFunc(flags.WithGroupedHelp([]flags.GroupedFlags{sigAggGroup}))
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
	signatureaggregatorExists, err := isThereExistingSignatureAggregator(network)
	if err != nil {
		return err
	}

	if signatureaggregatorExists {
		ux.Logger.PrintToUser("There is already a running signature aggregator instance locally for %s", network.Name())
		ux.Logger.PrintToUser("To create a new signature aggregator instance, stop it first by calling `avalanche interchain signatureAggregator stop` command and run `avalanche interchain signatureAggregator start` again")
		return nil
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
	err = signatureaggregator.CreateSignatureAggregatorInstance(app, network, aggregatorLogger, startFlags.SigAggFlags)
	if err != nil {
		return err
	}
	return nil
}

func isThereExistingSignatureAggregator(network models.Network) (bool, error) {
	// first we check if the local config file for signature aggregator exists
	// if it doesn't exist, then there is no running signature aggregator instance
	runFilePath := app.GetLocalSignatureAggregatorRunPath(network.Kind)
	_, err := os.ReadFile(runFilePath)
	if os.IsNotExist(err) {
		return false, nil
	}
	// next, we check if the ports mentioned in local config file for signature aggregator are used
	// if they are both not available, that means that there is a running signature aggregator instance
	runFile, err := signatureaggregator.GetCurrentSignatureAggregatorProcessDetails(app, network)
	if err != nil {
		return false, fmt.Errorf("failed to get process details: %w", err)
	}
	if !signatureaggregator.IsPortAvailable(runFile.APIPort) && !signatureaggregator.IsPortAvailable(runFile.MetricsPort) {
		return true, nil
	}
	return false, nil
}
