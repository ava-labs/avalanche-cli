// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/spf13/cobra"
	"io"
	"os"
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
	latestAvagoVersion, err := app.Downloader.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
		constants.AvaLabsOrg,
		constants.AvalancheGoRepoName,
	))
	if err != nil {
		return err
	}
	fmt.Printf("latest avalanchego version %s \n", latestAvagoVersion)
	latestSubnetEVMVersion, err := app.Downloader.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
		constants.AvaLabsOrg,
		constants.SubnetEVMRepoName,
	))
	if err != nil {
		return err
	}
	fmt.Printf("latestSubnetEVMVersion version %s \n", latestSubnetEVMVersion)
	rpcVersion, err := vm.GetRPCProtocolVersion(app, models.SubnetEvm, "v0.5.5")
	if err != nil {
		return err
	}
	fmt.Printf("rpcVersion version %s \n", rpcVersion)

	ansibleNodeIDs, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	//compatibleVersions := []string{}
	//incompatibleNodes := []string{}
	failedNodes := []string{}
	nodeErrors := []error{}
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
		avalancheGoVersion := vmVersions[constants.PlatformKeyName]
		//subnetEVMVersion := ""
		//vms := make(map[string]string)
		for vmName, vmVersion := range vmVersions {
			if vmName != constants.PlatformKeyName && vmName != constants.EVMKeyName && vmName != constants.AVMKeyName {
				//vms[vmName] = vmVersion
			}
		}
		if err := app.RemoveAnsibleStatusDir(); err != nil {
			failedNodes = append(failedNodes, host)
			nodeErrors = append(nodeErrors, err)
			continue
		}
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return nil, err
		}
		//compatibleVersions, err = checkForCompatibleAvagoVersion(sc.RPCVersion)
		//if err != nil {
		//	return nil, err
		//}
		//if !slices.Contains(compatibleVersions, avalancheGoVersion) {
		//	incompatibleNodes = append(incompatibleNodes, host)
		//}
	}
	return nil
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
