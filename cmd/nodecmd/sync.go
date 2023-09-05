// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"golang.org/x/exp/slices"

	"github.com/ava-labs/avalanche-cli/pkg/constants"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync [clusterName] [subnetName]",
		Short: "(ALPHA Warning) Sync nodes in a cluster with a subnet",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node sync command enables all nodes in a cluster to be bootstrapped to a Subnet. 
You can check the subnet bootstrap status by calling avalanche node status <clusterName> --subnet <subnetName>`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE:         syncSubnet,
	}

	return cmd
}

func syncSubnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	subnetName := args[1]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	if err := setupAnsible(); err != nil {
		return err
	}
	if _, err := subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName}); err != nil {
		return err
	}
	notBootstrappedNodes, err := checkClusterIsBootstrapped(clusterName, false)
	if err != nil {
		return err
	}
	if len(notBootstrappedNodes) > 0 {
		return fmt.Errorf("node(s) %s are not bootstrapped yet, please try again later", notBootstrappedNodes)
	}
	incompatibleNodes, err := checkAvalancheGoVersionCompatible(clusterName, subnetName)
	if err != nil {
		return err
	}
	if len(incompatibleNodes) > 0 {
		ux.Logger.PrintToUser("Either modify your Avalanche Go version or modify your Subnet-EVM version")
		ux.Logger.PrintToUser("To modify your Avalanche Go version: https://docs.avax.network/nodes/maintain/upgrade-your-avalanchego-node")
		ux.Logger.PrintToUser("To modify your Subnet-EVM version: https://docs.avax.network/build/subnet/upgrade/upgrade-subnet-vm")
		return fmt.Errorf("the Avalanche Go version of node(s) %s is incompatible with Subnet EVM RPC version of %s", incompatibleNodes, subnetName)
	}
	untrackedNodes, err := trackSubnet(clusterName, subnetName, models.Fuji)
	if err != nil {
		return err
	}
	if len(untrackedNodes) > 0 {
		return fmt.Errorf("node(s) %s failed to sync with subnet %s", untrackedNodes, subnetName)
	}
	ux.Logger.PrintToUser("Node(s) successfully started syncing with Subnet!")
	ux.Logger.PrintToUser(fmt.Sprintf("Check node subnet syncing status with avalanche node status %s --subnet %s", clusterName, subnetName))
	return nil
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

func checkAvalancheGoVersionCompatible(clusterName, subnetName string) ([]string, error) {
	if err := app.CreateAnsibleDir(); err != nil {
		return nil, err
	}
	hostAliases, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryFilePath(clusterName))
	if err != nil {
		return nil, err
	}
	ux.Logger.PrintToUser(fmt.Sprintf("Checking compatibility of avalanche go version in cluster %s with Subnet EVM RPC of subnet %s ...", clusterName, subnetName))
	compatibleVersions := []string{}
	incompatibleNodes := []string{}
	for _, host := range hostAliases {
		if err := app.CreateAnsibleStatusFile(app.GetAvalancheGoJSONFile()); err != nil {
			return nil, err
		}
		if err := ansible.RunAnsiblePlaybookCheckAvalancheGoVersion(app.GetAnsibleDir(), app.GetAvalancheGoJSONFile(), app.GetAnsibleInventoryDirPath(clusterName), host); err != nil {
			return nil, err
		}
		avalancheGoVersion, err := parseAvalancheGoOutput(app.GetAvalancheGoJSONFile())
		if err != nil {
			return nil, err
		}
		if err := app.RemoveAnsibleStatusDir(); err != nil {
			return nil, err
		}
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return nil, err
		}
		compatibleVersions, err = checkForCompatibleAvagoVersion(sc.RPCVersion)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(compatibleVersions, avalancheGoVersion) {
			incompatibleNodes = append(incompatibleNodes, host)
		}
	}
	if len(incompatibleNodes) > 0 {
		ux.Logger.PrintToUser(fmt.Sprintf("Compatible Avalanche Go versions are %s", strings.Join(compatibleVersions, ", ")))
	}
	return incompatibleNodes, nil
}

// trackSubnet exports deployed subnet in user's local machine to cloud server and calls node to
// start tracking the specified subnet (similar to avalanche subnet join <subnetName> command)
func trackSubnet(clusterName, subnetToTrack string, network models.Network) ([]string, error) {
	hostAliases, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryFilePath(clusterName))
	if err != nil {
		return nil, err
	}
	subnetPath := "/tmp/" + subnetName + constants.ExportSubnetSuffix
	untrackedNodes := []string{}
	for _, host := range hostAliases {
		if err = subnetcmd.CallExportSubnet(subnetToTrack, subnetPath, network); err != nil {
			return nil, err
		}
		if err = ansible.RunAnsiblePlaybookExportSubnet(app.GetAnsibleDir(), app.GetAnsibleInventoryDirPath(clusterName), subnetPath, "/tmp", host); err != nil {
			return nil, err
		}
		// runs avalanche join subnet command
		if err = ansible.RunAnsiblePlaybookTrackSubnet(app.GetAnsibleDir(), subnetToTrack, subnetPath, app.GetAnsibleInventoryDirPath(clusterName), host); err != nil {
			untrackedNodes = append(untrackedNodes, host)
		}
	}
	return untrackedNodes, nil
}
