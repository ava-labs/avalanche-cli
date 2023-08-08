// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ansible

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/ava-labs/avalanche-cli/pkg/utils"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func CreateAnsibleHostInventory(inventoryPath, elasticIP, certFilePath string) error {
	if err := os.MkdirAll(inventoryPath, os.ModePerm); err != nil {
		return err
	}
	inventoryHostsFile := inventoryPath + "/hosts"
	inventoryFile, err := os.Create(inventoryHostsFile)
	if err != nil {
		return err
	}
	alias := "aws-node "
	// terraform output has "" in the first and last characters, we need to remove them
	elasticIPToUse := elasticIP[1 : len(elasticIP)-2]
	alias += "ansible_host="
	alias += elasticIPToUse
	alias += " ansible_user=ubuntu "
	alias += fmt.Sprintf("ansible_ssh_private_key_file=%s", certFilePath)
	alias += " ansible_ssh_common_args='-o StrictHostKeyChecking=no'"
	_, err = inventoryFile.WriteString(alias + "\n")
	return err
}

func RunAnsibleSetupNodePlaybook(configPath, inventoryPath, avalancheGoVersion string) error {
	configDirVar := "configDir=" + configPath + " avalancheGoVersion=" + avalancheGoVersion
	cmd := exec.Command(constants.AnsiblePlaybook, constants.SetUpNodePlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, configDirVar, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	utils.SetupRealtimeCLIOutput(cmd)
	return cmd.Run()
}

func RunAnsibleCopyStakingFilesPlaybook(nodeInstanceDirPath, inventoryPath string) error {
	nodeInstanceDirPathVar := "nodeInstanceDirPath=" + nodeInstanceDirPath + "/"
	cmd := exec.Command(constants.AnsiblePlaybook, constants.CopyStakingFilesPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, nodeInstanceDirPathVar, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	utils.SetUpMultiWrite(cmd)
	return cmd.Run()
}

func RunAnsiblePlaybookExportSubnet(subnetName, inventoryPath string) error {
	exportOutput := "/tmp/" + subnetName + "-export.dat"
	exportedSubnet := "exportedSubnet=" + exportOutput
	cmd := exec.Command(constants.AnsiblePlaybook, constants.ExportSubnetPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, exportedSubnet, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	utils.SetUpMultiWrite(cmd)
	return cmd.Run()
}

func RunAnsiblePlaybookTrackSubnet(subnetName, inventoryPath string) error {
	importedFileName := "/tmp/" + subnetName + "-export.dat"
	importedSubnet := "subnetExportFileName=" + importedFileName + " subnetName=" + subnetName
	cmd := exec.Command(constants.AnsiblePlaybook, constants.TrackSubnetPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, importedSubnet, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	utils.SetUpMultiWrite(cmd)
	return cmd.Run()
}

func RunAnsiblePlaybookCheckBootstrapped(inventoryPath string) error {
	cmd := exec.Command(constants.AnsiblePlaybook, constants.IsBootstrappedPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	return cmd.Run()
}

func RunAnsiblePlaybookGetNodeID(inventoryPath string) error {
	cmd := exec.Command(constants.AnsiblePlaybook, constants.GetNodeIDPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	return cmd.Run()
}

func RunAnsiblePlaybookSubnetSyncStatus(blockchainID, inventoryPath string) error {
	blockchainIDArg := "blockchainID=" + blockchainID
	cmd := exec.Command(constants.AnsiblePlaybook, constants.IsSubnetSyncedPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, blockchainIDArg, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	return cmd.Run()
}

func CheckIsInstalled() error {
	if err := exec.Command(constants.AnsiblePlaybook).Run(); errors.Is(err, exec.ErrNotFound) { //nolint:gosec
		ux.Logger.PrintToUser("Ansible tool is not available. It is a necessary dependency for CLI to set up a remote node.")
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Please follow install instructions at https://docs.ansible.com/ansible/latest/installation_guide/intro_installation.html and try again")
		ux.Logger.PrintToUser("")
		return err
	}
	return nil
}
