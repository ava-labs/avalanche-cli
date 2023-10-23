// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"io"
	"os"
	"strings"
)

func newUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "(ALPHA Warning) Update avalanchego or VM version for all node in a cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node update command suite provides a collection of commands for nodes to update
their avalanchego or VM version.

You can check the status after update by calling avalanche node status`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         upgrade,
	}

	return cmd
}

func upgrade(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	toUpgradeNodes, err := getNodesToBeUpgraded(clusterName)
	if err != nil {
		return err
	}
	for node, toUpgrade := range toUpgradeNodes {
		// toUpgradeNodes either has value of avalancheGo or subnetEvm or avalancheGo,subnetEvm
		toUpgradeItems := strings.Split(toUpgrade, ",")
		for _, toUpGradeItem := range toUpgradeItems {
			// toUpGradeItem either has value format of avalancheGo=<versionNum> or subnetEvm=<versionNum>
			upgradeInfo := strings.Split(toUpGradeItem, "=")
			if strings.Contains(toUpGradeItem, constants.AvalancheGoRepoName) {
				if err = upgradeAvalancheGo(clusterName, node, upgradeInfo[1]); err != nil {
					return err
				}
			} else if strings.Contains(toUpGradeItem, constants.SubnetEVMRepoName) {
				// subnetEVM version has value format of n8Anw9kErmgk7KHviddYtecCmziLZTphDwfL1V2DfnFjWZXbE:<versionNum> or subnetEvm=<versionNum>
				subnetEVMVersionInfo := strings.Split(upgradeInfo[1], ":")
				subnetEMVersionToUpgradeTo := subnetEVMVersionInfo[1]
				subnetEMVersionToUpgradeToWoPrefix := strings.TrimPrefix(subnetEMVersionToUpgradeTo, "v")
				subnetEVMReleaseURL := fmt.Sprintf("https://github.com/ava-labs/subnet-evm/releases/download/%s/subnet-evm_%s_linux_amd64.tar.gz", subnetEMVersionToUpgradeTo, subnetEMVersionToUpgradeToWoPrefix)
				subnetEVMArchive := fmt.Sprintf("subnet-evm_%s_linux_amd64.tar.gz", subnetEMVersionToUpgradeToWoPrefix)
				subnetEVMBinaryPath := fmt.Sprintf("/home/ubuntu/.avalanchego/plugins/%s", subnetEVMVersionInfo[0])
				if err = upgradeSubnetEVM(clusterName, subnetEVMReleaseURL, subnetEVMArchive, subnetEVMBinaryPath, node, subnetEMVersionToUpgradeTo); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func upgradeAvalancheGo(clusterName, ansibleNodeID, avaGoVersionToUpdateTo string) error {
	ux.Logger.PrintToUser("Upgrading Avalanche Go version of node %s to version %s ...", ansibleNodeID, avaGoVersionToUpdateTo)
	if err := ansible.RunAnsiblePlaybookUpgradeAvalancheGo(app.GetAnsibleDir(), app.GetAnsibleInventoryDirPath(clusterName), ansibleNodeID); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Successfully upgraded Avalanche Go version of node %s!", ansibleNodeID)
	ux.Logger.PrintToUser("======================================")
	return nil

}

func upgradeSubnetEVM(clusterName, subnetEVMReleaseURL, subnetEVMArchive, subnetEVMBinaryPath, ansibleNodeID, subnetEVMVersion string) error {
	ux.Logger.PrintToUser("Upgrading SubnetEVM version of node %s to version %s ...", ansibleNodeID, subnetEVMVersion)
	if err := ansible.RunAnsiblePlaybookUpgradeSubnetEVM(app.GetAnsibleDir(), subnetEVMReleaseURL, subnetEVMArchive, subnetEVMBinaryPath, app.GetAnsibleInventoryDirPath(clusterName), ansibleNodeID); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Successfully upgraded SubnetEVM version of node %s!", ansibleNodeID)
	ux.Logger.PrintToUser("======================================")
	return nil
}

func getNodesToBeUpgraded(clusterName string) (map[string]string, error) {
	latestAvagoVersion, err := app.Downloader.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
		constants.AvaLabsOrg,
		constants.AvalancheGoRepoName,
	))
	if err != nil {
		return nil, err
	}
	fmt.Printf("latest avalanchego version %s \n", latestAvagoVersion)
	latestSubnetEVMVersion, err := app.Downloader.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
		constants.AvaLabsOrg,
		constants.SubnetEVMRepoName,
	))
	if err != nil {
		return nil, err
	}
	fmt.Printf("latestSubnetEVMVersion version %s \n", latestSubnetEVMVersion)
	rpcVersion, err := vm.GetRPCProtocolVersion(app, models.SubnetEvm, latestSubnetEVMVersion)
	if err != nil {
		return nil, err
	}
	fmt.Printf("rpcVersion version %s \n", rpcVersion)
	ansibleNodeIDs, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return nil, err
	}
	failedNodes := []string{}
	nodeErrors := []error{}
	nodesToUpgrade := make(map[string]string)
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
		vmVersions, err := parseNodeVersionOutput(app.GetAvalancheGoJSONFile())
		if err != nil {
			failedNodes = append(failedNodes, host)
			nodeErrors = append(nodeErrors, err)
			continue
		}
		currentAvalancheGoVersion := vmVersions[constants.PlatformKeyName]
		var avalancheGoVersionToUpdateTo string
		var subnetEVMVersionToUpdateTo string
		for vmName, vmVersion := range vmVersions {
			// when calling info.getNodeVersion, this is what we get
			// "vmVersions":{"avm":"v1.10.12","evm":"v0.12.5","n8Anw9kErmgk7KHviddYtecCmziLZTphDwfL1V2DfnFjWZXbE":"v0.5.6","platform":"v1.10.12"}},
			// we need to get the VM ID of the subnets that the node is currently validating, in the example above it is n8Anw9kErmgk7KHviddYtecCmziLZTphDwfL1V2DfnFjWZXbE
			if vmName != constants.PlatformKeyName && vmName != constants.EVMKeyName && vmName != constants.AVMKeyName {
				//vms[vmName] = vmVersion
				if vmVersion != latestSubnetEVMVersion {
					// update subnet EVM version
					ux.Logger.PrintToUser("Upgrading Subnet EVM version for node %s from version to version %s", host, vmVersion, latestSubnetEVMVersion)
					// check if highest avalanche go version compatible with current highest rpc,
					isCompatible, err := checkIfAvaGoSubnetEVMCompatible(latestAvagoVersion, rpcVersion)
					if err != nil {
						failedNodes = append(failedNodes, host)
						nodeErrors = append(nodeErrors, err)
					}
					avalancheGoVersionToUpdateTo = latestAvagoVersion
					subnetEVMVersionToUpdateTo = latestSubnetEVMVersion
					if !isCompatible {
						// if highest avalanche go version not compatible with current highest rpc,
						// find the highest version of avalanche go that is still compatible with current highest rpc
						avalancheGoVersionToUpdateTo, err = GetLatestAvagoVersionForRPC(rpcVersion)
						if err != nil {
							failedNodes = append(failedNodes, host)
							nodeErrors = append(nodeErrors, err)
						}
					}
					// check if new Subnet EVM is compatible with newAvalanchego
				}
				// check if current subnetEVM is compatible with newAvalanchego
			}
		}
		if currentAvalancheGoVersion != avalancheGoVersionToUpdateTo {
			if avalancheGoVersionToUpdateTo == "" {
				nodesToUpgrade[host] = constants.AvalancheGoRepoName + "=" + latestAvagoVersion
			} else {
				nodesToUpgrade[host] = constants.AvalancheGoRepoName + "=" + avalancheGoVersionToUpdateTo
			}
		}
		if subnetEVMVersionToUpdateTo != "" {
			nodesToUpgrade[host] += constants.SubnetEVMRepoName + "=" + subnetEVMVersionToUpdateTo
		}
		if err := app.RemoveAnsibleStatusDir(); err != nil {
			failedNodes = append(failedNodes, host)
			nodeErrors = append(nodeErrors, err)
			continue
		}

	}
	return nil, nil
}

func checkIfAvaGoSubnetEVMCompatible(avalancheGoVersion string, rpcVersion int) (bool, error) {
	compatibleVersions, err := checkForCompatibleAvagoVersion(rpcVersion)
	if err != nil {
		return false, err
	}
	if !slices.Contains(compatibleVersions, avalancheGoVersion) {
		return false, errors.New("incompatible avalancheGoVersion")
	}
	return true, nil
}

func parseNodeVersionOutput(fileName string) (map[string]string, error) {
	jsonFile, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)

	var result map[string]interface{}
	if err = json.Unmarshal(byteValue, &result); err != nil {
		return nil, err
	}
	nodeIDInterface, ok := result["result"].(map[string]interface{})
	if ok {
		vmVersions, ok := nodeIDInterface["vmVersions"].(map[string]string)
		if ok {
			return vmVersions, nil
		}
	}
	return nil, nil
}
