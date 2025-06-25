// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package signatureAggregatorCmd

import (
	"fmt"
	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/blockchain"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/signatureaggregator"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
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
	subnetIDStr, err := getSubnetID(network)
	if err != nil {
		return err
	}
	clusterName := ""
	// get extra aggregator peers automatically if we have l1 name
	if startFlags.l1 != "" {
		clusterName = localnet.LocalClusterName(network, startFlags.l1)
		if _, err := os.Stat(app.GetLocalClusterDir(clusterName)); err != nil && os.IsNotExist(err) {
			clusterName = ""
		}
	}
	if err = createLocalSignatureAggregator(clusterName, subnetIDStr, network); err != nil {
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
	ux.Logger.PrintToUser("  Signature Aggregator %s Network API Endpoint: %s", network.Name(), fmt.Sprintf("http://localhost:%d/aggregate-signatures", runFile.APIPort))
	ux.Logger.PrintToUser("  Signature Aggregator %s Network API Port: %d", network.Name(), runFile.APIPort)
	ux.Logger.PrintToUser("  Signature Aggregator %s Network Metrics Port: %d", network.Name(), runFile.MetricsPort)
	ux.Logger.GreenCheckmarkToUser("Local Signature Aggregator successfully started for %s", network.Kind)
	return nil
}

func createLocalSignatureAggregator(clusterName string, subnetIDStr string, network models.Network) error {
	aggregatorLogger, err := signatureaggregator.NewSignatureAggregatorLogger(
		startFlags.SigAggFlags.AggregatorLogLevel,
		startFlags.SigAggFlags.AggregatorLogToStdout,
		app.GetAggregatorLogDir(clusterName),
	)
	if err != nil {
		return err
	}
	extraAggregatorPeers, err := blockchain.GetAggregatorExtraPeers(app, clusterName)
	if err != nil {
		return err
	}
	fmt.Printf("signatureaggregator start clusterName %s \n", clusterName)
	fmt.Printf("signatureaggregator start peers %s \n", extraAggregatorPeers)
	err = signatureaggregator.CreateSignatureAggregatorInstance(app, subnetIDStr, network, extraAggregatorPeers, aggregatorLogger, "latest")
	if err != nil {
		return err
	}
	if _, err = signatureaggregator.GetSignatureAggregatorEndpoint(app, network); err != nil {
		return err
	}
	return nil
}

func getSubnetID(network models.Network) (string, error) {
	var err error
	l1ListOption := "I will choose from the L1 list created by Avalanche-CLI in my local machine"
	subnetIDOption := "I know the L1 Subnet ID"
	cancelOption := "Cancel"
	option := l1ListOption
	options := []string{l1ListOption, subnetIDOption, cancelOption}
	option, err = app.Prompt.CaptureList(
		"How do you want to specify the L1 that you want to use the signature aggregator on",
		options,
	)
	if err != nil {
		return "", err
	}
	switch option {
	case l1ListOption:
		deployedSubnets, err := subnet.GetDeployedSubnetsFromFile(app, network.Name())
		if err != nil {
			return "", fmt.Errorf("unable to read deployed subnets: %w", err)
		}
		startFlags.l1, err = app.Prompt.CaptureList(
			"What L1 do you want to the signature aggregator to be on?",
			deployedSubnets,
		)
		sc, err := app.LoadSidecar(startFlags.l1)
		if err != nil {
			return "", fmt.Errorf("failed to load sidecar: %w", err)
		}
		return sc.Networks[network.Name()].SubnetID.String(), nil

	case subnetIDOption:
		subnetID, err := app.Prompt.CaptureID("What is the Subnet ID that you want to use the signature aggregator on?")
		if err != nil {
			return "", err
		}
		return subnetID.String(), nil

	case cancelOption:
		return "", nil
	}
	return "", nil
}
