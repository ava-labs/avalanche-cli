// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ssh

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
)

//go:embed shell/*
var shell embed.FS

// CreateAnsibleHostInventory creates inventory file to be used for Ansible playbook commands
// specifies the ip address of the cloud server and the corresponding ssh cert path for the cloud server
// if publicIPs is empty, that means that user is not using elastic IP and we are using publicIPMap
// to get the host IP
func CreateHostInventory(inventoryDirPath, certFilePath string, publicIPMap map[string]string) error {
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
		inventoryContent += " ansible_user=ubuntu "
		inventoryContent += fmt.Sprintf("ansible_ssh_private_key_file=%s", certFilePath)
		inventoryContent += " ansible_ssh_common_args='-o StrictHostKeyChecking=no'"
		if _, err = inventoryFile.WriteString(inventoryContent + "\n"); err != nil {
			return err
		}
	}
	return nil
}

// GetAnsibleHostsFromInventory gets alias of all hosts in an inventory file
func GetHostsFromInventory(inventoryDirPath string) ([]string, error) {
	ansibleHostIDs := []string{}
	inventory, err := GetInventoryFromFile(inventoryDirPath)
	if err != nil {
		return nil, err
	}
	for _, host := range inventory {
		ansibleHostIDs = append(ansibleHostIDs, host.NodeID)
	}
	return ansibleHostIDs, nil
}

func GetInventoryFromFile(inventoryDirPath string) ([]models.Host, error) {
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
		parsedHost := strings.Split(scanner.Text(), " ")
		host := models.Host{
			NodeID:            parsedHost[0],
			IP:                strings.Split(parsedHost[1], "=")[1],
			SSHUser:           strings.Split(parsedHost[2], "=")[1],
			SSHPrivateKeyPath: strings.Split(parsedHost[3], "=")[1],
			SSHCommonArgs:     strings.Split(parsedHost[4], "=")[1],
		}
		inventory = append(inventory, host)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return inventory, nil
}

func GetHostMapfromInventoryFile(inventoryDirPath string) (map[string]models.Host, error) {
	hostMap := map[string]models.Host{}
	inventory, err := GetInventoryFromFile(inventoryDirPath)
	if err != nil {
		return nil, err
	}
	for _, host := range inventory {
		hostMap[host.NodeID] = host
	}
	return hostMap, nil
}

// getInventoryHostMap creates a map with nodeID as key and its corresponding ansible inventory host information as value
func getInventoryHostMap(inventoryDirPath string) (map[string]string, error) {
	inventoryHostsFile := filepath.Join(inventoryDirPath, constants.AnsibleHostInventoryFileName)
	ansibleInventoryHostMap := make(map[string]string)
	file, err := os.Open(inventoryHostsFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// host alias is first element in each line of host inventory file
		// host alias has name format "aws_node_<nodeID>"
		ansibleHostID := strings.Split(scanner.Text(), " ")[0]
		ansibleHostIDSplit := strings.Split(ansibleHostID, "_")
		if len(ansibleHostIDSplit) > 2 {
			ansibleInventoryHostMap[ansibleHostIDSplit[2]] = scanner.Text()
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return ansibleInventoryHostMap, nil
}

// UpdateInventoryHostPublicIP first maps existing ansible inventory host file content
// then it deletes the inventory file and regenerates a new ansible inventory file where it will fetch public IP
// of nodes without elastic IP and update its value in the new ansible inventory file
func UpdateInventoryHostPublicIP(inventoryDirPath string, nodesWoEIP map[string]string) error {
	ansibleHostMap, err := getInventoryHostMap(inventoryDirPath)
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
	for nodeID, ansibleHostContent := range ansibleHostMap {
		_, ok := nodesWoEIP[nodeID]
		if !ok {
			if _, err = inventoryFile.WriteString(ansibleHostContent + "\n"); err != nil {
				return err
			}
		} else {
			ansibleHostInfo := strings.Split(ansibleHostContent, " ")
			ansiblePublicIP := "ansible_host=" + nodesWoEIP[nodeID]
			newAnsibleHostInfo := []string{}
			if len(ansibleHostInfo) > 2 {
				newAnsibleHostInfo = append(newAnsibleHostInfo, ansibleHostInfo[0])
				newAnsibleHostInfo = append(newAnsibleHostInfo, ansiblePublicIP)
				newAnsibleHostInfo = append(newAnsibleHostInfo, ansibleHostInfo[2:]...)
			}
			if _, err = inventoryFile.WriteString(strings.Join(newAnsibleHostInfo, " ") + "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}
