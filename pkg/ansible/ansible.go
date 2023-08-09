// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ansible

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/utils"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

//go:embed playbook/*
var playbook embed.FS

//go:embed ansible.cfg
var config []byte

// CreateAnsibleHostInventory creates inventory file to be used for Ansible playbook commands
// specifies the ip address of the cloud server and the corresponding ssh cert path for the cloud server
func CreateAnsibleHostInventory(inventoryPath, ip, certFilePath string) error {
	if err := os.MkdirAll(inventoryPath, os.ModePerm); err != nil {
		return err
	}
	inventoryHostsFile := inventoryPath + "/hosts"
	inventoryFile, err := os.Create(inventoryHostsFile)
	if err != nil {
		return err
	}
	alias := "aws-node "
	alias += "ansible_host="
	alias += ip
	alias += " ansible_user=ubuntu "
	alias += fmt.Sprintf("ansible_ssh_private_key_file=%s", certFilePath)
	alias += " ansible_ssh_common_args='-o StrictHostKeyChecking=no'"
	_, err = inventoryFile.WriteString(alias + "\n")
	return err
}

func SetUp(ansibleDir string) error {
	err := WriteCfgFile(ansibleDir)
	if err != nil {
		return err
	}
	return WritePlaybookFiles(ansibleDir)
}

func WritePlaybookFiles(ansibleDir string) error {
	playbookDir := filepath.Join(ansibleDir, "playbook")
	files, err := playbook.ReadDir("playbook")
	if err != nil {
		return err
	}

	for _, file := range files {
		fileContent, err := playbook.ReadFile(fmt.Sprintf("%s/%s", "playbook", file.Name()))
		if err != nil {
			return err
		}
		playbookFile, err := os.Create(filepath.Join(playbookDir, file.Name()))
		if err != nil {
			return err
		}
		_, err = playbookFile.Write(fileContent)
		if err != nil {
			return err
		}
	}
	return nil
}

func WriteCfgFile(ansibleDir string) error {
	cfgFile, err := os.Create(filepath.Join(ansibleDir, "ansible.cfg"))
	if err != nil {
		return err
	}
	_, err = cfgFile.Write(config)
	return err
}

func RunAnsibleSetupNodePlaybook(configPath, ansibleDir, inventoryPath, avalancheGoVersion string) error {
	configDirVar := "configDir=" + configPath + " avalancheGoVersion=" + avalancheGoVersion
	cmd := exec.Command(constants.AnsiblePlaybook, constants.SetupNodePlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, configDirVar, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	utils.SetupRealtimeCLIOutput(cmd)
	return cmd.Run()
}

// RunAnsibleCopyStakingFilesPlaybook copies staker.crt and staker.key into local machine so users can back up their node
// these files are stored in .avalanche-cli/nodes/<nodeID> dir
func RunAnsibleCopyStakingFilesPlaybook(ansibleDir, nodeInstanceDirPath, inventoryPath string) error {
	nodeInstanceDirPathVar := "nodeInstanceDirPath=" + nodeInstanceDirPath + "/"
	cmd := exec.Command(constants.AnsiblePlaybook, constants.CopyStakingFilesPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, nodeInstanceDirPathVar, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	utils.SetupRealtimeCLIOutput(cmd)
	return cmd.Run()
}

// RunAnsiblePlaybookExportSubnet exports deployed Subnet from local machine to cloud server
func RunAnsiblePlaybookExportSubnet(ansibleDir, subnetName, inventoryPath string) error {
	exportOutput := "/tmp/" + subnetName + "-export.dat"
	exportedSubnet := "exportedSubnet=" + exportOutput
	cmd := exec.Command(constants.AnsiblePlaybook, constants.ExportSubnetPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, exportedSubnet, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	utils.SetupRealtimeCLIOutput(cmd)
	return cmd.Run()
}

// RunAnsiblePlaybookTrackSubnet runs avalanche subnet join <subnetName> in cloud server
func RunAnsiblePlaybookTrackSubnet(ansibleDir, subnetName, inventoryPath string) error {
	importedFileName := "/tmp/" + subnetName + "-export.dat"
	importedSubnet := "subnetExportFileName=" + importedFileName + " subnetName=" + subnetName
	cmd := exec.Command(constants.AnsiblePlaybook, constants.TrackSubnetPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, importedSubnet, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	utils.SetupRealtimeCLIOutput(cmd)
	return cmd.Run()
}

// RunAnsiblePlaybookCheckBootstrapped checks if node is bootstrapped to primary network
func RunAnsiblePlaybookCheckBootstrapped(ansibleDir, inventoryPath string) error {
	cmd := exec.Command(constants.AnsiblePlaybook, constants.IsBootstrappedPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	return cmd.Run()
}

// RunAnsiblePlaybookGetNodeID gets node ID of cloud server
func RunAnsiblePlaybookGetNodeID(ansibleDir, inventoryPath string) error {
	cmd := exec.Command(constants.AnsiblePlaybook, constants.GetNodeIDPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	return cmd.Run()
}

// RunAnsiblePlaybookSubnetSyncStatus checks if node is synced to subnet
func RunAnsiblePlaybookSubnetSyncStatus(ansibleDir, blockchainID, inventoryPath string) error {
	blockchainIDArg := "blockchainID=" + blockchainID
	cmd := exec.Command(constants.AnsiblePlaybook, constants.IsSubnetSyncedPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, blockchainIDArg, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
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
