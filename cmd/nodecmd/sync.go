// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"golang.org/x/exp/slices"
	"io"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync [clusterName]",
		Short: "(ALPHA Warning) Sync nodes in a cluster with a subnet",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node sync command enables all nodes in a cluster to be bootstrapped to a Subnet. 
You can check the subnet bootstrap status by calling avalanche node status --subnet`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         syncSubnet,
	}
	cmd.Flags().StringVar(&subnetName, "subnet", "", "specify the subnet the node is syncing with")

	return cmd
}

func syncSubnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	if subnetName == "" {
		ux.Logger.PrintToUser("Please provide the name of the subnet that the node will be validating with --subnet flag")
		return errors.New("no subnet provided")
	}
	if err := setupAnsible(); err != nil {
		return err
	}
	if _, err := subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName}); err != nil {
		return err
	}
	isBootstrapped, err := checkNodeIsBootstrapped(clusterName, false)
	if err != nil {
		return err
	}
	if !isBootstrapped {
		return errors.New("node is not bootstrapped yet, please try again later")
	}
	if err := checkAvalancheGoVersionCompatible(clusterName, subnetName); err != nil {
		return err
	}
	return trackSubnet(clusterName, subnetName, models.Fuji)
}

func parseAvalancheGoOutput(fileName string) (string, error) {
	jsonFile, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)

	var result map[string]interface{}
	if err = json.Unmarshal(byteValue, &result); err != nil {
		return "", err
	}
	nodeIDInterface, ok := result["result"].(map[string]interface{})
	if ok {
		vmVersions, ok := nodeIDInterface["vmVersions"].(map[string]interface{})
		if ok {
			avalancheGoVersion, ok := vmVersions["platform"].(string)
			if ok {
				return avalancheGoVersion, nil
			}
		}
	}
	return "", nil
}

func checkForCompatibleAvagoVersion(configuredRPCVersion int) ([]string, error) {
	compatibleAvagoVersions, err := vm.GetAvailableAvalancheGoVersions(
		app, configuredRPCVersion, constants.AvalancheGoCompatibilityURL)
	if err != nil {
		return nil, err
	}
	return compatibleAvagoVersions, nil
}

func checkAvalancheGoVersionCompatible(clusterName, subnetName string) error {
	ux.Logger.PrintToUser(fmt.Sprintf("Checking compatibility of avalanche go version in cluster %s with Subnet EVM RPC of subnet %s ...", clusterName, subnetName))
	err := app.CreateAnsibleDir()
	if err != nil {
		return err
	}
	if err := ansible.RunAnsiblePlaybookCheckAvalancheGoVersion(app.GetAnsibleDir(), app.GetAvalancheGoJSONFile(), app.GetAnsibleInventoryPath(clusterName)); err != nil {
		return err
	}
	avalancheGoVersion, err := parseAvalancheGoOutput(app.GetAvalancheGoJSONFile())
	if err != nil {
		return err
	}
	if err := app.RemoveAnsibleStatusDir(); err != nil {
		return err
	}
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}
	compatibleVersions, err := checkForCompatibleAvagoVersion(sc.RPCVersion)
	if err != nil {
		return err
	}
	if !slices.Contains(compatibleVersions, avalancheGoVersion) {
		ux.Logger.PrintToUser(fmt.Sprintf("Compatible Avalanche Go versions are %s", strings.Join(compatibleVersions, ", ")))
		ux.Logger.PrintToUser("Either modify your Avalanche Go version or modify your Subnet-EVM version")
		ux.Logger.PrintToUser("To modify your Avalanche Go version: https://docs.avax.network/nodes/maintain/upgrade-your-avalanchego-node")
		ux.Logger.PrintToUser("To modify your Subnet-EVM version: https://docs.avax.network/build/subnet/upgrade/upgrade-subnet-vm")
		return fmt.Errorf("the Avalanche Go version of cluster %s is incompatible with Subnet EVM RPC version of %s", clusterName, subnetName)
	}
	return nil
}

// trackSubnet exports deployed subnet in user's local machine to cloud server and calls node to
// start tracking the specified subnet (similar to avalanche subnet join <subnetName> command)
func trackSubnet(clusterName, subnetToTrack string, network models.Network) error {
	subnetPath := "/tmp/" + subnetName + constants.ExportSubnetSuffix
	if err := subnetcmd.CallExportSubnet(subnetToTrack, subnetPath, network); err != nil {
		return err
	}
	if err := ansible.RunAnsiblePlaybookExportSubnet(app.GetAnsibleDir(), app.GetAnsibleInventoryPath(clusterName), subnetPath, "/tmp"); err != nil {
		return err
	}
	// runs avalanche join subnet command
	if err := ansible.RunAnsiblePlaybookTrackSubnet(app.GetAnsibleDir(), subnetToTrack, subnetPath, app.GetAnsibleInventoryPath(clusterName)); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Node successfully started syncing with Subnet!")
	ux.Logger.PrintToUser(fmt.Sprintf("Check node subnet syncing status with avalanche node status %s --subnet %s", clusterName, subnetToTrack))
	return nil
}
