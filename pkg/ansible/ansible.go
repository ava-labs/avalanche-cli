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

	sdkHost "github.com/ava-labs/avalanche-tooling-sdk-go/host"
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
				ansibleInstanceID, err := HostCloudIDToAnsibleID(cloudService, instanceID)
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
			ansibleInstanceID, err := HostCloudIDToAnsibleID(cloudService, instanceID)
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
	inventoryContent += fmt.Sprintf(" ansible_ssh_common_args='%s'", constants.AnsibleSSHUseAgentParams)
	if _, err := inventoryFile.WriteString(inventoryContent + "\n"); err != nil {
		return err
	}
	return nil
}

// WriteNodeConfigsToAnsibleInventory writes node configs to ansible inventory file
func WriteNodeConfigsToAnsibleInventory(inventoryDirPath string, nc []models.NodeConfig) error {
	inventoryHostsFilePath := filepath.Join(inventoryDirPath, constants.AnsibleHostInventoryFileName)
	if err := os.MkdirAll(inventoryDirPath, os.ModePerm); err != nil {
		return err
	}
	inventoryFile, err := os.OpenFile(inventoryHostsFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, constants.WriteReadReadPerms)
	if err != nil {
		return err
	}
	defer inventoryFile.Close()
	for _, nodeConfig := range nc {
		nodeID, err := HostCloudIDToAnsibleID(nodeConfig.CloudService, nodeConfig.NodeID)
		if err != nil {
			return err
		}
		if err := writeToInventoryFile(inventoryFile, nodeID, nodeConfig.ElasticIP, nodeConfig.CertPath); err != nil {
			return err
		}
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

func GetInventoryFromAnsibleInventoryFile(inventoryDirPath string) ([]*sdkHost.Host, error) {
	inventory := []*sdkHost.Host{}
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
		commonArgs, err := CommonArgsToMap(parsedHost["ansible_ssh_common_args"])
		if err != nil {
			return nil, err
		}
		host := &sdkHost.Host{
			NodeID: strings.Split(scanner.Text(), " ")[0],
			IP:     parsedHost["ansible_host"],
			SSHConfig: sdkHost.SSHConfig{
				User:           parsedHost["ansible_user"],
				PrivateKeyPath: parsedHost["ansible_ssh_private_key_file"],
				Params:         commonArgs,
			},
		}
		inventory = append(inventory, host)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return inventory, nil
}

func GetHostByNodeID(nodeID string, inventoryDirPath string) (*sdkHost.Host, error) {
	allHosts, err := GetInventoryFromAnsibleInventoryFile(inventoryDirPath)
	if err != nil {
		return nil, err
	} else {
		hosts := utils.Filter(allHosts, func(h *sdkHost.Host) bool { return h.NodeID == nodeID })
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

func GetHostMapfromAnsibleInventory(inventoryDirPath string) (map[string]*sdkHost.Host, error) {
	hostMap := map[string]*sdkHost.Host{}
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
		_, nodeID, err := HostAnsibleIDToCloudID(host)
		if err != nil {
			return err
		}
		_, ok := nodesWithDynamicIP[nodeID]
		if !ok {
			if _, err = inventoryFile.WriteString(GetHostInventoryRecord(ansibleHostContent) + "\n"); err != nil {
				return err
			}
		} else {
			ansibleHostContent.IP = nodesWithDynamicIP[nodeID]
			if _, err = inventoryFile.WriteString(GetHostInventoryRecord(ansibleHostContent) + "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

// HostAnsibleIDToCloudID converts ansible host ID to cloud ID
func HostCloudIDToAnsibleID(cloudService string, hostCloudID string) (string, error) {
	switch cloudService {
	case constants.GCPCloudService:
		return fmt.Sprintf("%s_%s", constants.GCPNodeAnsiblePrefix, hostCloudID), nil
	case constants.AWSCloudService:
		return fmt.Sprintf("%s_%s", constants.AWSNodeAnsiblePrefix, hostCloudID), nil
	case constants.E2EDocker:
		return fmt.Sprintf("%s_%s", constants.E2EDocker, hostCloudID), nil
	}
	return "", fmt.Errorf("unknown cloud service %s", cloudService)
}

// HostAnsibleIDToCloudID converts a host Ansible ID to a cloud ID.
func HostAnsibleIDToCloudID(hostAnsibleID string) (string, string, error) {
	var cloudService, cloudIDPrefix string
	switch {
	case strings.HasPrefix(hostAnsibleID, constants.AWSNodeAnsiblePrefix):
		cloudService = constants.AWSCloudService
		cloudIDPrefix = strings.TrimPrefix(hostAnsibleID, constants.AWSNodeAnsiblePrefix+"_")
	case strings.HasPrefix(hostAnsibleID, constants.GCPNodeAnsiblePrefix):
		cloudService = constants.GCPCloudService
		cloudIDPrefix = strings.TrimPrefix(hostAnsibleID, constants.GCPNodeAnsiblePrefix+"_")
	case strings.HasPrefix(hostAnsibleID, constants.E2EDocker):
		cloudService = constants.E2EDocker
		cloudIDPrefix = strings.TrimPrefix(hostAnsibleID, constants.E2EDocker+"_")
	default:
		return "", "", fmt.Errorf("unknown cloud service prefix in %s", hostAnsibleID)
	}
	return cloudService, cloudIDPrefix, nil
}

// GetHostInventoryRecord returns the ansible inventory record for a host
func GetHostInventoryRecord(h *sdkHost.Host) string {
	return strings.Join([]string{
		h.NodeID,
		fmt.Sprintf("ansible_host=%s", h.IP),
		fmt.Sprintf("ansible_user=%s", h.SSHConfig.User),
		fmt.Sprintf("ansible_ssh_private_key_file=%s", h.SSHConfig.PrivateKeyPath),
		fmt.Sprintf("ansible_ssh_common_args='%s'", ParamsMapToCommonArgs(h.SSHConfig.Params)),
	}, " ")
}

// CommonArgsToMap converts a comma separated string of key value pairs to a map
func CommonArgsToMap(commaSeparatedString string) (map[string]string, error) {
	args := strings.Split(commaSeparatedString, ",")
	argsMap := map[string]string{}
	for _, arg := range args {
		kv := strings.Split(arg, "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid key value pair %s", arg)
		}
		argsMap[kv[0]] = kv[1]
	}
	return argsMap, nil
}

// ParamsMapToCommonArgs converts a map of key value pairs to a comma separated string
func ParamsMapToCommonArgs(params map[string]string) string {
	args := []string{}
	for k, v := range params {
		args = append(args, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(args, ",")
}
