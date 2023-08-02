// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ansible

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
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
	if err != nil {
		return err
	}
	return nil
}

func RunAnsibleSetUpNodePlaybook(configPath, inventoryPath string) error {
	configDirVar := "configDir=" + configPath
	var stdBuffer bytes.Buffer
	cmd := exec.Command(constants.AnsiblePlaybook, constants.SetUpNodePlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, configDirVar, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mw
	return cmd.Run()
}

func RunAnsibleCopyStakingFilesPlaybook(nodeInstanceDirPath, inventoryPath string) error {
	nodeInstanceDirPathVar := "nodeInstanceDirPath=" + nodeInstanceDirPath + "/"
	fmt.Printf("nodeInstanceDirPathVar %s \n", nodeInstanceDirPathVar)
	var stdBuffer bytes.Buffer
	cmd := exec.Command(constants.AnsiblePlaybook, constants.CopyStakingFilesPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, nodeInstanceDirPathVar, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mw
	return cmd.Run()
}

func RunAnsiblePlaybookExportSubnet(subnetName, inventoryPath string) error {
	var stdBuffer bytes.Buffer
	exportOutput := "/tmp/" + subnetName + "-export.dat"
	exportedSubnet := "exportedSubnet=" + exportOutput
	cmd := exec.Command(constants.AnsiblePlaybook, constants.ExportSubnetPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, exportedSubnet, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mw
	return cmd.Run()
}

func RunAnsiblePlaybookTrackSubnet(subnetName, inventoryPath string) error {
	var stdBuffer bytes.Buffer
	importedFileName := "/tmp/" + subnetName + "-export.dat"
	importedSubnet := "subnetExportFileName=" + importedFileName + " subnetName=" + subnetName
	cmd := exec.Command(constants.AnsiblePlaybook, constants.TrackSubnetPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, importedSubnet, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mw
	return cmd.Run()
}

func RunAnsiblePlaybookCheckBootstrapped(inventoryPath string) error {
	var stdBuffer bytes.Buffer
	cmd := exec.Command(constants.AnsiblePlaybook, constants.IsBootstrappedPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mw
	return cmd.Run()
}

func RunAnsiblePlaybookGetNodeID(inventoryPath string) error {
	var stdBuffer bytes.Buffer
	cmd := exec.Command(constants.AnsiblePlaybook, constants.GetNodeIDPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mw
	return cmd.Run()
}

func RunAnsiblePlaybookSubnetSyncStatus(blockchainID, inventoryPath string) error {
	blockchainIDArg := "blockchainID=" + blockchainID
	var stdBuffer bytes.Buffer
	cmd := exec.Command(constants.AnsiblePlaybook, constants.IsSubnetSyncedPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, blockchainIDArg, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mw
	return cmd.Run()
}
