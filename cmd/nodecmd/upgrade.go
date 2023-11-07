// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

type NodeUpgradeInfo struct {
	AvalancheGoVersion    string   // avalanche go version to update to on cloud server
	SubnetEVMVersion      string   // subnet EVM version to update to on cloud server
	SubnetEVMIDsToUpgrade []string // list of ID of Subnet EVM to be upgraded to subnet EVM version to update to
}

func newUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "(ALPHA Warning) Update avalanchego or VM version for all node in a cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node update command suite provides a collection of commands for nodes to update
their avalanchego or VM version.

You can check the status after upgrade by calling avalanche node status`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         upgrade,
	}

	return cmd
}

func upgrade(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	if err := setupAnsible(clusterName); err != nil {
		return err
	}
	toUpgradeNodesMap, err := getNodesUpgradeInfo(clusterName)
	if err != nil {
		return err
	}
	for node, upgradeInfo := range toUpgradeNodesMap {
		if upgradeInfo.AvalancheGoVersion != "" {
			if err = upgradeAvalancheGo(clusterName, node, upgradeInfo.AvalancheGoVersion); err != nil {
				return err
			}
		}
		if upgradeInfo.SubnetEVMVersion != "" {
			subnetEVMVersionToUpgradeToWoPrefix := strings.TrimPrefix(upgradeInfo.SubnetEVMVersion, "v")
			subnetEVMArchive := fmt.Sprintf(constants.SubnetEVMArchive, subnetEVMVersionToUpgradeToWoPrefix)
			subnetEVMReleaseURL := fmt.Sprintf(constants.SubnetEVMReleaseURL, upgradeInfo.SubnetEVMVersion, subnetEVMArchive)
			if err = getNewSubnetEVMRelease(clusterName, subnetEVMReleaseURL, subnetEVMArchive, node, upgradeInfo.SubnetEVMVersion); err != nil {
				return err
			}
			if err = stopNode(clusterName, node); err != nil {
				return err
			}
			for _, vmID := range upgradeInfo.SubnetEVMIDsToUpgrade {
				subnetEVMBinaryPath := fmt.Sprintf(constants.SubnetEVMBinaryPath, vmID)
				if err = upgradeSubnetEVM(clusterName, subnetEVMBinaryPath, node, upgradeInfo.SubnetEVMVersion); err != nil {
					return err
				}
			}
			if err = startNode(clusterName, node); err != nil {
				return err
			}
		}
	}
	return nil
}

// getNodesUpgradeInfo gets the node versions of all nodes in cluster clusterName and checks which
// nodes needs to have Avalanche Go & SubnetEVM upgraded. It first checks the subnet EVM version -
// it will install the newest subnet EVM version and install the latest avalanche Go that is still compatible with the Subnet EVM version
// if the node is not tracking any subnet, it will just install latestAvagoVersion
func getNodesUpgradeInfo(clusterName string) (map[string]NodeUpgradeInfo, error) {
	latestAvagoVersion, err := app.Downloader.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
		constants.AvaLabsOrg,
		constants.AvalancheGoRepoName,
	))
	if err != nil {
		return nil, err
	}
	latestSubnetEVMVersion, err := app.Downloader.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
		constants.AvaLabsOrg,
		constants.SubnetEVMRepoName,
	))
	if err != nil {
		return nil, err
	}
	rpcVersion, err := vm.GetRPCProtocolVersion(app, models.SubnetEvm, latestSubnetEVMVersion)
	if err != nil {
		return nil, err
	}
	ansibleNodeIDs, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return nil, err
	}
	failedNodes := []string{}
	nodeErrors := []error{}
	nodesToUpgrade := make(map[string]NodeUpgradeInfo)
	for _, host := range ansibleNodeIDs {
		if err := app.CreateAnsibleStatusFile(app.GetAvalancheGoJSONFile()); err != nil {
			failedNodes = append(failedNodes, host)
			nodeErrors = append(nodeErrors, err)
			continue
		}
		if err := ansible.RunAnsiblePlaybookCheckAvalancheGoVersion(app.GetAnsibleDir(), app.GetAvalancheGoJSONFile(), app.GetAnsibleInventoryDirPath(clusterName), host); err != nil {
			failedNodes = append(failedNodes, host)
			nodeErrors = append(nodeErrors, err)
			continue
		}
		vmVersions, err := parseNodeVersionOutput(app.GetAvalancheGoJSONFile() + "." + host)
		if err != nil {
			failedNodes = append(failedNodes, host)
			nodeErrors = append(nodeErrors, err)
			continue
		}
		currentAvalancheGoVersion := vmVersions[constants.PlatformKeyName]
		avalancheGoVersionToUpdateTo := latestAvagoVersion
		nodeUpgradeInfo := NodeUpgradeInfo{}
		nodeUpgradeInfo.SubnetEVMIDsToUpgrade = []string{}
		for vmName, vmVersion := range vmVersions {
			// when calling info.getNodeVersion, this is what we get
			// "vmVersions":{"avm":"v1.10.12","evm":"v0.12.5","n8Anw9kErmgk7KHviddYtecCmziLZTphDwfL1V2DfnFjWZXbE":"v0.5.6","platform":"v1.10.12"}},
			// we need to get the VM ID of the subnets that the node is currently validating, in the example above it is n8Anw9kErmgk7KHviddYtecCmziLZTphDwfL1V2DfnFjWZXbE
			if !checkIfKeyIsStandardVMName(vmName) {
				if vmVersion != latestSubnetEVMVersion {
					// update subnet EVM version
					ux.Logger.PrintToUser("Upgrading Subnet EVM version for node %s from version %s to version %s", host, vmVersion, latestSubnetEVMVersion)
					nodeUpgradeInfo.SubnetEVMVersion = latestSubnetEVMVersion
					nodeUpgradeInfo.SubnetEVMIDsToUpgrade = append(nodeUpgradeInfo.SubnetEVMIDsToUpgrade, vmName)
				}
				// find the highest version of avalanche go that is still compatible with current highest rpc
				avalancheGoVersionToUpdateTo, err = GetLatestAvagoVersionForRPC(rpcVersion)
				if err != nil {
					failedNodes = append(failedNodes, host)
					nodeErrors = append(nodeErrors, err)
					continue
				}
			}
		}
		if slices.Contains(failedNodes, host) {
			continue
		}
		if currentAvalancheGoVersion != avalancheGoVersionToUpdateTo {
			ux.Logger.PrintToUser("Upgrading Avalanche Go version for node %s from version %s to version %s", host, currentAvalancheGoVersion, avalancheGoVersionToUpdateTo)
			nodeUpgradeInfo.AvalancheGoVersion = avalancheGoVersionToUpdateTo
		}
		nodesToUpgrade[host] = nodeUpgradeInfo
		if err := app.RemoveAnsibleStatusDir(); err != nil {
			failedNodes = append(failedNodes, host)
			nodeErrors = append(nodeErrors, err)
			continue
		}
	}
	if len(failedNodes) > 0 {
		ux.Logger.PrintToUser("Failed to upgrade nodes: ")
		for i, node := range failedNodes {
			ux.Logger.PrintToUser("node %s failed to upgrade due to %s", node, nodeErrors[i])
		}
		return nil, fmt.Errorf("failed to upgrade node(s) %s", failedNodes)
	}
	return nodesToUpgrade, nil
}

// checks if vmName is "avm", "evm" or "platform"
func checkIfKeyIsStandardVMName(vmName string) bool {
	standardVMNames := []string{constants.PlatformKeyName, constants.EVMKeyName, constants.AVMKeyName}
	return slices.Contains(standardVMNames, vmName)
}

func upgradeAvalancheGo(clusterName, ansibleNodeID, avaGoVersionToUpdateTo string) error {
	ux.Logger.PrintToUser("Upgrading Avalanche Go version of node %s to version %s ...", ansibleNodeID, avaGoVersionToUpdateTo)
	if err := ansible.RunAnsiblePlaybookUpgradeAvalancheGo(app.GetAnsibleDir(), app.GetAnsibleInventoryDirPath(clusterName), ansibleNodeID, avaGoVersionToUpdateTo); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Successfully upgraded Avalanche Go version of node %s!", ansibleNodeID)
	ux.Logger.PrintToUser("======================================")
	return nil
}

func stopNode(clusterName, ansibleNodeID string) error {
	if err := ansible.RunAnsiblePlaybookStopNode(app.GetAnsibleDir(), app.GetAnsibleInventoryDirPath(clusterName), ansibleNodeID); err != nil {
		return err
	}
	return nil
}

func startNode(clusterName, ansibleNodeID string) error {
	if err := ansible.RunAnsiblePlaybookStartNode(app.GetAnsibleDir(), app.GetAnsibleInventoryDirPath(clusterName), ansibleNodeID); err != nil {
		return err
	}
	return nil
}

func upgradeSubnetEVM(clusterName, subnetEVMBinaryPath, ansibleNodeID, subnetEVMVersion string) error {
	ux.Logger.PrintToUser("Upgrading SubnetEVM version of node %s to version %s ...", ansibleNodeID, subnetEVMVersion)
	if err := ansible.RunAnsiblePlaybookUpgradeSubnetEVM(app.GetAnsibleDir(), subnetEVMBinaryPath, app.GetAnsibleInventoryDirPath(clusterName), ansibleNodeID); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Successfully upgraded SubnetEVM version of node %s!", ansibleNodeID)
	ux.Logger.PrintToUser("======================================")
	return nil
}

func getNewSubnetEVMRelease(clusterName, subnetEVMReleaseURL, subnetEVMArchive, ansibleNodeID, subnetEVMVersion string) error {
	ux.Logger.PrintToUser("Getting new SubnetEVM version %s ...", subnetEVMVersion)
	if err := ansible.RunAnsiblePlaybookGetNewSubnetEVM(app.GetAnsibleDir(), subnetEVMReleaseURL, subnetEVMArchive, app.GetAnsibleInventoryDirPath(clusterName), ansibleNodeID); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Successfully downloaded SubnetEVM version for node %s!", ansibleNodeID)
	ux.Logger.PrintToUser("======================================")
	return nil
}

func parseNodeVersionOutput(fileName string) (map[string]interface{}, error) {
	jsonFile, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()
	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err = json.Unmarshal(byteValue, &result); err != nil {
		return nil, err
	}
	nodeIDInterface, ok := result["result"].(map[string]interface{})
	if ok {
		vmVersions, ok := nodeIDInterface["vmVersions"].(map[string]interface{})
		if ok {
			return vmVersions, nil
		}
	}
	return nil, nil
}
