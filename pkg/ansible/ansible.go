// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ansible

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
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

func Setup(ansibleDir string) error {
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

// RunAnsiblePlaybookSetupNode installs avalanche go and avalanche-cli. It also copies the user's
// metric preferences in configFilePath from local machine to cloud server
func RunAnsiblePlaybookSetupNode(configPath, ansibleDir, inventoryPath, avalancheGoVersion string) error {
	playbookInputs := "configFilePath=" + configPath + " avalancheGoVersion=" + avalancheGoVersion
	cmd := exec.Command(constants.AnsiblePlaybook, constants.SetupNodePlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	stdoutBuffer, stderrBuffer := utils.SetupRealtimeCLIOutput(cmd, true, true)
	cmdErr := cmd.Run()
	if err := displayErrMsg(stdoutBuffer); err != nil {
		return err
	}
	if err := displayErrMsg(stderrBuffer); err != nil {
		return err
	}
	return cmdErr
}

// RunAnsiblePlaybookCopyStakingFiles copies staker.crt and staker.key into local machine so users can back up their node
// these files are stored in .avalanche-cli/nodes/<nodeID> dir
func RunAnsiblePlaybookCopyStakingFiles(ansibleDir, nodeInstanceDirPath, inventoryPath string) error {
	playbookInputs := "nodeInstanceDirPath=" + nodeInstanceDirPath + "/"
	cmd := exec.Command(constants.AnsiblePlaybook, constants.CopyStakingFilesPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	stdoutBuffer, stderrBuffer := utils.SetupRealtimeCLIOutput(cmd, true, true)
	cmdErr := cmd.Run()
	if err := displayErrMsg(stdoutBuffer); err != nil {
		return err
	}
	if err := displayErrMsg(stderrBuffer); err != nil {
		return err
	}
	return cmdErr
}

// RunAnsiblePlaybookExportSubnet exports deployed Subnet from local machine to cloud server
func RunAnsiblePlaybookExportSubnet(ansibleDir, inventoryPath, exportPath, cloudServerSubnetPath string) error {
	playbookInputs := "originSubnetPath=" + exportPath + " destSubnetPath=" + cloudServerSubnetPath
	cmd := exec.Command(constants.AnsiblePlaybook, constants.ExportSubnetPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	stdoutBuffer, stderrBuffer := utils.SetupRealtimeCLIOutput(cmd, true, true)
	cmdErr := cmd.Run()
	if err := displayErrMsg(stdoutBuffer); err != nil {
		return err
	}
	if err := displayErrMsg(stderrBuffer); err != nil {
		return err
	}
	return cmdErr
}

// RunAnsiblePlaybookTrackSubnet runs avalanche subnet join <subnetName> in cloud server
func RunAnsiblePlaybookTrackSubnet(ansibleDir, subnetName, importPath, inventoryPath string) error {
	playbookInputs := "subnetExportFileName=" + importPath + " subnetName=" + subnetName
	cmd := exec.Command(constants.AnsiblePlaybook, constants.TrackSubnetPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	stdoutBuffer, stderrBuffer := utils.SetupRealtimeCLIOutput(cmd, true, true)
	cmdErr := cmd.Run()
	if err := displayErrMsg(stdoutBuffer); err != nil {
		return err
	}
	if err := displayErrMsg(stderrBuffer); err != nil {
		return err
	}
	return cmdErr
}

func displayErrMsg(buffer *bytes.Buffer) error {
	for _, line := range strings.Split(buffer.String(), "\n") {
		if strings.Contains(line, "FAILED") {
			i := strings.Index(line, "{")
			if i >= 0 {
				line = line[i:]
			}
			var jsonMap map[string]interface{}
			if err := json.Unmarshal([]byte(line), &jsonMap); err != nil {
				return err
			}
			toDump := []string{}
			stdoutLines, ok := jsonMap["stdout_lines"].([]interface{})
			if ok {
				toDump = append(toDump, getStringSeqFromISeq(stdoutLines)...)
			}
			stderrLines, ok := jsonMap["stderr_lines"].([]interface{})
			if ok {
				toDump = append(toDump, getStringSeqFromISeq(stderrLines)...)
			}
			msgLine, ok := jsonMap["msg"].(string)
			if ok {
				toDump = append(toDump, msgLine)
			}
			contentLine, ok := jsonMap["content"].(string)
			if ok {
				toDump = append(toDump, contentLine)
			}
			if len(toDump) > 0 {
				fmt.Println()
				fmt.Println(logging.Red.Wrap("Message from cloud node:"))
				for _, l := range toDump {
					fmt.Println("  " + logging.Red.Wrap(l))
				}
				fmt.Println()
			}
		}
	}
	return nil
}

func getStringSeqFromISeq(lines []interface{}) []string {
	seq := []string{}
	for _, lineI := range lines {
		line, ok := lineI.(string)
		if ok {
			if strings.Contains(line, "Usage:") {
				break
			}
			seq = append(seq, line)
		}
	}
	return seq
}

// RunAnsiblePlaybookCheckBootstrapped checks if node is bootstrapped to primary network
func RunAnsiblePlaybookCheckAvalancheGoVersion(ansibleDir, avalancheGoPath, inventoryPath string) error {
	playbookInput := "avalancheGoJsonPath=" + avalancheGoPath
	cmd := exec.Command(constants.AnsiblePlaybook, constants.AvalancheGoVersionPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInput, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	stdoutBuffer, stderrBuffer := utils.SetupRealtimeCLIOutput(cmd, false, false)
	cmdErr := cmd.Run()
	if err := displayErrMsg(stdoutBuffer); err != nil {
		return err
	}
	if err := displayErrMsg(stderrBuffer); err != nil {
		return err
	}
	return cmdErr
}

// RunAnsiblePlaybookCheckBootstrapped checks if node is bootstrapped to primary network
func RunAnsiblePlaybookCheckBootstrapped(ansibleDir, isBootstrappedPath, inventoryPath string) error {
	isBootstrappedJSONPath := "isBootstrappedJsonPath=" + isBootstrappedPath
	cmd := exec.Command(constants.AnsiblePlaybook, constants.IsBootstrappedPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, isBootstrappedJSONPath, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	stdoutBuffer, stderrBuffer := utils.SetupRealtimeCLIOutput(cmd, false, false)
	cmdErr := cmd.Run()
	if err := displayErrMsg(stdoutBuffer); err != nil {
		return err
	}
	if err := displayErrMsg(stderrBuffer); err != nil {
		return err
	}
	return cmdErr
}

// RunAnsiblePlaybookGetNodeID gets node ID of cloud server
func RunAnsiblePlaybookGetNodeID(ansibleDir, nodeIDPath, inventoryPath string) error {
	playbookInputs := "nodeIDJsonPath=" + nodeIDPath
	cmd := exec.Command(constants.AnsiblePlaybook, constants.GetNodeIDPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	stdoutBuffer, stderrBuffer := utils.SetupRealtimeCLIOutput(cmd, false, false)
	cmdErr := cmd.Run()
	if err := displayErrMsg(stdoutBuffer); err != nil {
		return err
	}
	if err := displayErrMsg(stderrBuffer); err != nil {
		return err
	}
	return cmdErr
}

// RunAnsiblePlaybookSubnetSyncStatus checks if node is synced to subnet
func RunAnsiblePlaybookSubnetSyncStatus(ansibleDir, subnetSyncPath, blockchainID, inventoryPath string) error {
	playbookInputs := "blockchainID=" + blockchainID + " subnetSyncPath=" + subnetSyncPath
	cmd := exec.Command(constants.AnsiblePlaybook, constants.IsSubnetSyncedPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	stdoutBuffer, stderrBuffer := utils.SetupRealtimeCLIOutput(cmd, false, false)
	cmdErr := cmd.Run()
	if err := displayErrMsg(stdoutBuffer); err != nil {
		return err
	}
	if err := displayErrMsg(stderrBuffer); err != nil {
		return err
	}
	return cmdErr
}

// RunAnsiblePlaybookSetupBuildEnv installs gcc, golang, rust
func RunAnsiblePlaybookSetupBuildEnv(ansibleDir, inventoryPath string) error {
	playbookInputs := "goVersion=" + constants.BuildEnvGolangVersion
	cmd := exec.Command(constants.AnsiblePlaybook, constants.SetupBuildEnvPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	stdoutBuffer, stderrBuffer := utils.SetupRealtimeCLIOutput(cmd, true, true)
	cmdErr := cmd.Run()
	if err := displayErrMsg(stdoutBuffer); err != nil {
		return err
	}
	if err := displayErrMsg(stderrBuffer); err != nil {
		return err
	}
	return cmdErr
}

// RunAnsiblePlaybookSetupCLIFromSource installs any CLI branch from source
func RunAnsiblePlaybookSetupCLIFromSource(ansibleDir, inventoryPath, cliBranch string) error {
	playbookInputs := "cliBranch=" + cliBranch
	cmd := exec.Command(constants.AnsiblePlaybook, constants.SetupCLIFromSourcePlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	cmd.Dir = ansibleDir
	stdoutBuffer, stderrBuffer := utils.SetupRealtimeCLIOutput(cmd, true, true)
	cmdErr := cmd.Run()
	if err := displayErrMsg(stdoutBuffer); err != nil {
		return err
	}
	if err := displayErrMsg(stderrBuffer); err != nil {
		return err
	}
	return cmdErr
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
