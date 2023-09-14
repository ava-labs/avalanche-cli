// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ansible

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
func CreateAnsibleHostInventory(inventoryDirPath, certFilePath string, publicIPs, instanceIDs []string) error {
	if err := os.MkdirAll(inventoryDirPath, os.ModePerm); err != nil {
		return err
	}
	inventoryHostsFile := filepath.Join(inventoryDirPath, constants.AnsibleHostInventoryFileName)
	inventoryFile, err := os.Create(inventoryHostsFile)
	if err != nil {
		return err
	}
	for i, instanceID := range instanceIDs {
		inventoryContent := fmt.Sprintf("aws_node_%s", instanceID)
		inventoryContent += " ansible_host="
		inventoryContent += publicIPs[i]
		inventoryContent += " ansible_user=ubuntu "
		inventoryContent += fmt.Sprintf("ansible_ssh_private_key_file=%s", certFilePath)
		inventoryContent += " ansible_ssh_common_args='-o StrictHostKeyChecking=no'"
		if _, err = inventoryFile.WriteString(inventoryContent + "\n"); err != nil {
			return err
		}
	}
	return nil
}

func Setup(ansibleDir string) error {
	err := WriteCfgFile(ansibleDir)
	if err != nil {
		return err
	}
	return WritePlaybookFiles(ansibleDir)
}

// GetAnsibleHostsFromInventory gets alias of all hosts in an inventory file
func GetAnsibleHostsFromInventory(inventoryDirPath string) ([]string, error) {
	inventoryHostsFile := filepath.Join(inventoryDirPath, constants.AnsibleHostInventoryFileName)
	ansibleHostIDs := []string{}
	file, err := os.Open(inventoryHostsFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// host alias is first element in each line of host inventory file
		ansibleHostID := strings.Split(scanner.Text(), " ")[0]
		ansibleHostIDs = append(ansibleHostIDs, ansibleHostID)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return ansibleHostIDs, nil
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

// RunAnsibleSetupNodePlaybook installs avalanche go and avalanche-cli. It also copies the user's
// metric preferences in configFilePath from local machine to cloud server
// targets all hosts in ansible inventory file
func RunAnsibleSetupNodePlaybook(configPath, ansibleDir, inventoryPath, avalancheGoVersion string) error {
	playbookInputs := "configFilePath=" + configPath + " avalancheGoVersion=" + avalancheGoVersion
	cmd := exec.Command(constants.AnsiblePlaybook, constants.SetupNodePlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	utils.SetupRealtimeCLIOutput(cmd)
	return cmd.Run()
}

// RunAnsibleCopyStakingFilesPlaybook copies staker.crt and staker.key into local machine so users can back up their node
// these files are stored in .avalanche-cli/nodes/<nodeID> dir
// targets a specific host ansibleHostID in ansible inventory file
func RunAnsibleCopyStakingFilesPlaybook(ansibleDir, ansibleHostID, nodeInstanceDirPath, inventoryPath string) error {
	playbookInputs := "target=" + ansibleHostID + " nodeInstanceDirPath=" + nodeInstanceDirPath + "/"
	cmd := exec.Command(constants.AnsiblePlaybook, constants.CopyStakingFilesPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	utils.SetupRealtimeCLIOutput(cmd)
	return cmd.Run()
}

// RunAnsiblePlaybookExportSubnet exports deployed Subnet from local machine to cloud server
// targets a specific host ansibleHostID in ansible inventory file
func RunAnsiblePlaybookExportSubnet(ansibleDir, inventoryPath, exportPath, cloudServerSubnetPath, ansibleHostID string) error {
	playbookInputs := "target=" + ansibleHostID + " originSubnetPath=" + exportPath + " destSubnetPath=" + cloudServerSubnetPath
	cmd := exec.Command(constants.AnsiblePlaybook, constants.ExportSubnetPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	utils.SetupRealtimeCLIOutput(cmd)
	return cmd.Run()
}

// RunAnsiblePlaybookTrackSubnet runs avalanche subnet join <subnetName> in cloud server
// targets a specific host ansibleHostID in ansible inventory file
func RunAnsiblePlaybookTrackSubnet(ansibleDir, subnetName, importPath, inventoryPath, ansibleHostID string) error {
	playbookInputs := "target=" + ansibleHostID + " subnetExportFileName=" + importPath + " subnetName=" + subnetName
	cmd := exec.Command(constants.AnsiblePlaybook, constants.TrackSubnetPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	utils.SetupRealtimeCLIOutput(cmd)
	return cmd.Run()
}

// RunAnsiblePlaybookCheckAvalancheGoVersion checks if node is bootstrapped to primary network
// targets a specific host ansibleHostID in ansible inventory file
func RunAnsiblePlaybookCheckAvalancheGoVersion(ansibleDir, avalancheGoPath, inventoryPath, ansibleHostID string) error {
	playbookInput := "target=" + ansibleHostID + " avalancheGoJsonPath=" + avalancheGoPath
	cmd := exec.Command(constants.AnsiblePlaybook, constants.AvalancheGoVersionPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInput, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	return cmd.Run()
}

// RunAnsiblePlaybookCheckBootstrapped checks if node is bootstrapped to primary network
// targets a specific host ansibleHostID in ansible inventory file
func RunAnsiblePlaybookCheckBootstrapped(ansibleDir, isBootstrappedPath, inventoryPath, ansibleHostID string) error {
	playbookInputs := "target=" + ansibleHostID + " isBootstrappedJsonPath=" + isBootstrappedPath
	cmd := exec.Command(constants.AnsiblePlaybook, constants.IsBootstrappedPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	return cmd.Run()
}

// RunAnsiblePlaybookGetNodeID gets node ID of cloud server
// targets a specific host ansibleHostID in ansible inventory file
func RunAnsiblePlaybookGetNodeID(ansibleDir, nodeIDPath, inventoryPath, ansibleHostID string) error {
	playbookInputs := "target=" + ansibleHostID + " nodeIDJsonPath=" + nodeIDPath
	cmd := exec.Command(constants.AnsiblePlaybook, constants.GetNodeIDPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	return cmd.Run()
}

// RunAnsiblePlaybookSubnetSyncStatus checks if node is synced to subnet
// targets a specific host ansibleHostID in ansible inventory file
func RunAnsiblePlaybookSubnetSyncStatus(ansibleDir, subnetSyncPath, blockchainID, inventoryPath, ansibleHostID string) error {
	playbookInputs := "target=" + ansibleHostID + " blockchainID=" + blockchainID + " subnetSyncPath=" + subnetSyncPath
	cmd := exec.Command(constants.AnsiblePlaybook, constants.IsSubnetSyncedPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
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
