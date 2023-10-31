// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ansible

import (
	"bufio"
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
	"github.com/ava-labs/avalanche-cli/pkg/models"
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
func CreateAnsibleHostInventory(inventoryDirPath, certFilePath, cloudService string, publicIPMap map[string]string) error {
	if err := os.MkdirAll(inventoryDirPath, os.ModePerm); err != nil {
		return err
	}
	inventoryHostsFilePath := filepath.Join(inventoryDirPath, constants.AnsibleHostInventoryFileName)
	inventoryFile, err := os.OpenFile(inventoryHostsFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer inventoryFile.Close()
	for instanceID := range publicIPMap {
		inventoryContent := fmt.Sprintf("%s_%s", constants.AWSNodeAnsiblePrefix, instanceID)
		if cloudService == constants.GCPCloudService {
			inventoryContent = fmt.Sprintf("%s_%s", constants.GCPNodeAnsiblePrefix, instanceID)
		}
		inventoryContent += " ansible_host="
		inventoryContent += publicIPMap[instanceID]
		inventoryContent += " ansible_user=ubuntu"
		inventoryContent += fmt.Sprintf(" ansible_ssh_private_key_file=%s", certFilePath)
		inventoryContent += fmt.Sprintf(" ansible_ssh_common_args='%s'", constants.AnsibleSSHInventoryParams)
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
	ansibleHostIDs := []string{}
	inventory, err := GetInventoryFromAnsibleInventoryFile(inventoryDirPath)
	if err != nil {
		return nil, err
	}
	for _, host := range inventory {
		ansibleHostIDs = append(ansibleHostIDs, host.NodeID)
	}
	return ansibleHostIDs, nil
}

func GetInventoryFromAnsibleInventoryFile(inventoryDirPath string) ([]models.Host, error) {
	inventory := []models.Host{}
	inventoryHostsFile := filepath.Join(inventoryDirPath, constants.AnsibleHostInventoryFileName)
	file, err := os.Open(inventoryHostsFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// host alias is first element in each line of host inventory file
		parsedHost, err := utils.SplitKeyValueStringToMap(scanner.Text(), " ")
		if err != nil {
			return nil, err
		}
		host := models.Host{
			NodeID:            strings.Split(scanner.Text(), " ")[0],
			IP:                parsedHost["ansible_host"],
			SSHUser:           parsedHost["ansible_user"],
			SSHPrivateKeyPath: parsedHost["ansible_ssh_private_key_file"],
			SSHCommonArgs:     parsedHost["ansible_ssh_common_args"],
		}
		inventory = append(inventory, host)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return inventory, nil
}

func GetHostMapfromAnsibleInventory(inventoryDirPath string) (map[string]models.Host, error) {
	hostMap := map[string]models.Host{}
	inventory, err := GetInventoryFromAnsibleInventoryFile(inventoryDirPath)
	if err != nil {
		return nil, err
	}
	for _, host := range inventory {
		hostMap[host.NodeID] = host
	}
	return hostMap, nil
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
// targets all hosts in ansible inventory file
func RunAnsiblePlaybookSetupNode(configPath, ansibleDir, inventoryPath, avalancheGoVersion, ansibleHostIDs string) error {
	playbookInputs := "target=" + ansibleHostIDs + " configFilePath=" + configPath + " avalancheGoVersion=" + avalancheGoVersion
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
// targets a specific host ansibleHostID in ansible inventory file
func RunAnsiblePlaybookCopyStakingFiles(ansibleDir, ansibleHostID, nodeInstanceDirPath, inventoryPath string) error {
	playbookInputs := "target=" + ansibleHostID + " nodeInstanceDirPath=" + nodeInstanceDirPath + "/"
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
// targets a specific host ansibleHostID in ansible inventory file
func RunAnsiblePlaybookExportSubnet(ansibleDir, inventoryPath, exportPath, cloudServerSubnetPath, ansibleHostID string) error {
	playbookInputs := "target=" + ansibleHostID + " originSubnetPath=" + exportPath + " destSubnetPath=" + cloudServerSubnetPath
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
// targets a specific host ansibleHostID in ansible inventory file
func RunAnsiblePlaybookTrackSubnet(ansibleDir, subnetName, importPath, inventoryPath, ansibleHostID string) error {
	playbookInputs := "target=" + ansibleHostID + " subnetExportFileName=" + importPath + " subnetName=" + subnetName
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

// RunAnsiblePlaybookUpdateSubnet runs avalanche subnet join <subnetName> in cloud server using update subnet info
func RunAnsiblePlaybookUpdateSubnet(ansibleDir, subnetName, importPath, inventoryPath, ansibleHostID string) error {
	playbookInputs := "target=" + ansibleHostID + " subnetExportFileName=" + importPath + " subnetName=" + subnetName
	cmd := exec.Command(constants.AnsiblePlaybook, constants.UpdateSubnetPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
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
		if strings.Contains(line, "FAILED") || strings.Contains(line, "UNREACHABLE") {
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

// RunAnsiblePlaybookCheckAvalancheGoVersion checks if node is bootstrapped to primary network
// targets a specific host ansibleHostID in ansible inventory file
func RunAnsiblePlaybookCheckAvalancheGoVersion(ansibleDir, avalancheGoPath, inventoryPath, ansibleHostID string) error {
	playbookInput := "target=" + ansibleHostID + " avalancheGoJsonPath=" + avalancheGoPath
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
// targets a specific host ansibleHostID in ansible inventory file
func RunAnsiblePlaybookCheckBootstrapped(ansibleDir, isBootstrappedPath, inventoryPath, ansibleHostID string) error {
	playbookInputs := "target=" + ansibleHostID + " isBootstrappedJsonPath=" + isBootstrappedPath
	cmd := exec.Command(constants.AnsiblePlaybook, constants.IsBootstrappedPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
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
// targets a specific host ansibleHostID in ansible inventory file
func RunAnsiblePlaybookGetNodeID(ansibleDir, nodeIDPath, inventoryPath, ansibleHostID string) error {
	playbookInputs := "target=" + ansibleHostID + " nodeIDJsonPath=" + nodeIDPath
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
// targets a specific host ansibleHostID in ansible inventory file
func RunAnsiblePlaybookSubnetSyncStatus(ansibleDir, subnetSyncPath, blockchainID, inventoryPath, ansibleHostID string) error {
	playbookInputs := "target=" + ansibleHostID + " blockchainID=" + blockchainID + " subnetSyncPath=" + subnetSyncPath
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
func RunAnsiblePlaybookSetupBuildEnv(ansibleDir, inventoryPath, ansibleHostID string) error {
	playbookInputs := "target=" + ansibleHostID + " goVersion=" + constants.BuildEnvGolangVersion
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
func RunAnsiblePlaybookSetupCLIFromSource(ansibleDir, inventoryPath, cliBranch, ansibleHostID string) error {
	playbookInputs := "target=" + ansibleHostID + " cliBranch=" + cliBranch
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

// UpdateInventoryHostPublicIP first maps existing ansible inventory host file content
// then it deletes the inventory file and regenerates a new ansible inventory file where it will fetch public IP
// of nodes without elastic IP and update its value in the new ansible inventory file
func UpdateInventoryHostPublicIP(inventoryDirPath string, nodesWoEIP map[string]string) error {
	inventory, err := GetHostMapfromAnsibleInventory(inventoryDirPath)
	if err != nil {
		return err
	}
	inventoryHostsFilePath := filepath.Join(inventoryDirPath, constants.AnsibleHostInventoryFileName)
	if err = os.Remove(inventoryHostsFilePath); err != nil {
		return err
	}
	inventoryFile, err := os.Create(inventoryHostsFilePath)
	if err != nil {
		return err
	}
	for host, ansibleHostContent := range inventory {
		// trim prefix aws_node / gcp_node
		splitNodeName := strings.Split(host, "_")
		nodeID := splitNodeName[len(splitNodeName)-1]
		_, ok := nodesWoEIP[nodeID]
		if ok {
			ansibleHostContent.IP = nodesWoEIP[nodeID]
		}
		if _, err = inventoryFile.WriteString(host + " " + ansibleHostContent.GetAnsibleParams() + "\n"); err != nil {
			return err
		}
	}
	return nil
}

// RunAnsiblePlaybookUpgradeAvalancheGo upgrades avalanche go version of node
// targets a specific host ansibleHostID in ansible inventory file
func RunAnsiblePlaybookUpgradeAvalancheGo(ansibleDir, inventoryPath, ansibleHostID string) error {
	playbookInputs := "target=" + ansibleHostID
	cmd := exec.Command(constants.AnsiblePlaybook, constants.UpgradeAvalancheGoPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
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

// RunAnsiblePlaybookUpgradeSubnetEVM upgrades subnetEVM version of node
// targets a specific host ansibleHostID in ansible inventory file
func RunAnsiblePlaybookUpgradeSubnetEVM(ansibleDir, subnetEVMBinaryPaths, inventoryPath, ansibleHostID string) error {
	playbookInputs := "target=" + ansibleHostID + " subnetEVMBinaryPath=" + subnetEVMBinaryPaths
	cmd := exec.Command(constants.AnsiblePlaybook, constants.UpgradeSubnetEVMPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
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

// RunAnsiblePlaybookGetNewSubnetEVM downloads and unzips new subnetEVM version
// targets a specific host ansibleHostID in ansible inventory file
func RunAnsiblePlaybookGetNewSubnetEVM(ansibleDir, subnetEVMReleaseURL, subnetEVMArchive, inventoryPath, ansibleHostID string) error {
	playbookInputs := "target=" + ansibleHostID + " subnetEVMReleaseURL=" + subnetEVMReleaseURL + " subnetEVMArchive=" + subnetEVMArchive
	cmd := exec.Command(constants.AnsiblePlaybook, constants.GetNewSubnetEVMPlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraVarsFlag, playbookInputs, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
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
