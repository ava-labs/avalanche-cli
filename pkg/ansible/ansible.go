// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ansible

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
)

// CreateAnsibleHostInventory creates inventory file for ansible
// specifies the ip address of the cloud server and the corresponding ssh cert path for the cloud server
func CreateAnsibleHostInventory(inventoryDirPath, certFilePath, cloudService string, publicIPMap map[string]string, cloudConfigMap models.CloudConfig) error {
	if err := os.MkdirAll(inventoryDirPath, os.ModePerm); err != nil {
		return err
	}
	inventoryHostsFilePath := filepath.Join(inventoryDirPath, constants.AnsibleHostInventoryFileName)
	inventoryFile, err := os.OpenFile(inventoryHostsFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, constants.WriteReadReadPerms)
	if err != nil {
		return err
	}
	defer inventoryFile.Close()
	if cloudConfigMap != nil {
		for _, cloudConfig := range cloudConfigMap {
			for _, instanceID := range cloudConfig.InstanceIDs {
				ansibleInstanceID, err := models.HostCloudIDToAnsibleID(cloudService, instanceID)
				if err != nil {
					return err
				}
				if err = writeToInventoryFile(inventoryFile, ansibleInstanceID, publicIPMap[instanceID], cloudConfig.CertFilePath); err != nil {
					return err
				}
			}
		}
	} else {
		for instanceID := range publicIPMap {
			ansibleInstanceID, err := models.HostCloudIDToAnsibleID(cloudService, instanceID)
			if err != nil {
				return err
			}
			if err = writeToInventoryFile(inventoryFile, ansibleInstanceID, publicIPMap[instanceID], certFilePath); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeToInventoryFile(inventoryFile *os.File, ansibleInstanceID, publicIP, certFilePath string) error {
	inventoryContent := ansibleInstanceID
	inventoryContent += " ansible_host="
	inventoryContent += publicIP
	inventoryContent += " ansible_user=ubuntu"
	inventoryContent += fmt.Sprintf(" ansible_ssh_private_key_file=%s", certFilePath)
	inventoryContent += fmt.Sprintf(" ansible_ssh_common_args='%s'", constants.AnsibleSSHInventoryParams)
	if _, err := inventoryFile.WriteString(inventoryContent + "\n"); err != nil {
		return err
	}
	return nil
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

func GetInventoryFromAnsibleInventoryFile(inventoryDirPath string) ([]*models.Host, error) {
	inventory := []*models.Host{}
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
		host := &models.Host{
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

func GetHostByNodeID(nodeID string, inventoryDirPath string) (*models.Host, error) {
	allHosts, err := GetInventoryFromAnsibleInventoryFile(inventoryDirPath)
	if err != nil {
		return nil, err
	} else {
		hosts := utils.Filter(allHosts, func(h *models.Host) bool { return h.NodeID == nodeID })
		switch len(hosts) {
		case 1:
			return hosts[0], nil
		case 0:
			return nil, errors.New("host not found")
		default:
			return nil, errors.New("multiple hosts found")
		}
	}
}

func GetHostMapfromAnsibleInventory(inventoryDirPath string) (map[string]*models.Host, error) {
	hostMap := map[string]*models.Host{}
	inventory, err := GetInventoryFromAnsibleInventoryFile(inventoryDirPath)
	if err != nil {
		return nil, err
	}
	for _, host := range inventory {
		hostMap[host.NodeID] = host
	}
	return hostMap, nil
}

// UpdateInventoryHostPublicIP first maps existing ansible inventory host file content
// then it deletes the inventory file and regenerates a new ansible inventory file where it will fetch public IP
// of nodes without elastic IP and update its value in the new ansible inventory file
func UpdateInventoryHostPublicIP(inventoryDirPath string, nodesWithDynamicIP map[string]string) error {
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
		_, nodeID, err := models.HostAnsibleIDToCloudID(host)
		if err != nil {
			return err
		}
		_, ok := nodesWithDynamicIP[nodeID]
		if !ok {
			if _, err = inventoryFile.WriteString(ansibleHostContent.GetAnsibleInventoryRecord() + "\n"); err != nil {
				return err
			}
		} else {
			ansibleHostContent.IP = nodesWithDynamicIP[nodeID]
			if _, err = inventoryFile.WriteString(ansibleHostContent.GetAnsibleInventoryRecord() + "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}
