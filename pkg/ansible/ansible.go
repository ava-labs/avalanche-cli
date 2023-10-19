// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ansible

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/utils/logging"
)

// CreateAnsibleHostInventory creates inventory file to be used for Ansible playbook commands
// specifies the ip address of the cloud server and the corresponding ssh cert path for the cloud server
// if publicIPs is empty, that means that user is not using elastic IP and we are using publicIPMap
// to get the host IP
func CreateAnsibleHostInventory(inventoryDirPath, certFilePath string, publicIPMap map[string]string) error {
	if err := os.MkdirAll(inventoryDirPath, os.ModePerm); err != nil {
		return err
	}
	inventoryHostsFilePath := filepath.Join(inventoryDirPath, constants.AnsibleHostInventoryFileName)
	inventoryFile, err := os.Create(inventoryHostsFilePath)
	if err != nil {
		return err
	}
	for instanceID := range publicIPMap {
		inventoryContent := fmt.Sprintf("%s%s", constants.AnsibleAWSNodePrefix, instanceID)
		inventoryContent += " ansible_host="
		inventoryContent += publicIPMap[instanceID]
		inventoryContent += " ansible_user=ubuntu"
		inventoryContent += fmt.Sprintf(" ansible_ssh_private_key_file=%s", certFilePath)
		inventoryContent += fmt.Sprintf(" ansible_ssh_common_args='%s'", constants.AnsibleSSHParams)
		if _, err = inventoryFile.WriteString(inventoryContent + "\n"); err != nil {
			return err
		}
	}
	return nil
}

// GetAnsibleHostsFromInventory gets alias of all hosts in an inventory file
/*func GetAnsibleHostsFromInventory(inventoryDirPath string) ([]string, error) {
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
*/
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

func GetHostListFromAnsibleInventory(inventoryDirPath string) ([]string, error) {
	hosts,err :=GetHostMapfromAnsibleInventory(inventoryDirPath)
	if err != nil {
		return nil, err
	}
	var hostList []string
	for _, host := range hosts {
		hostList = append(hostList, host.NodeID)
	}
	return hostList, nil
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
	for node, ansibleHostContent := range inventory {
		nodeID := ansibleHostContent.ConvertToInstanceID(node)
		_, ok := nodesWoEIP[nodeID]
		if !ok {
			if _, err = inventoryFile.WriteString(node + " " + ansibleHostContent.GetAnsibleParams() + "\n"); err != nil {
				return err
			}
		} else {
			ansibleHostContent.IP = nodesWoEIP[nodeID]
			if _, err = inventoryFile.WriteString(node + " " + ansibleHostContent.GetAnsibleParams() + "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}
