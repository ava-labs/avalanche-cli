// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ansible

import (
	"bytes"
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
	if err != nil {
		return err
	}
	return nil
}

func RunAnsibleSetUpNodePlaybook(inventoryPath string) error {
	cmd := exec.Command(constants.AnsiblePlaybook, constants.SetUpNodePlaybook, constants.AnsibleInventoryFlag, inventoryPath, constants.AnsibleExtraArgsIdentitiesOnlyFlag) //nolint:gosec
	utils.SetUpMultiWrite(cmd)
	return cmd.Run()
}

func CheckIsInstalled() error {
	if err := exec.Command(constants.AnsiblePlaybook).Run(); errors.Is(err, exec.ErrNotFound) { //nolint:gosec
		ux.Logger.PrintToUser("Ansible tool is not available. It is a needed dependency for CLI to set up a remote node.")
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Please follow install instructions at https://docs.ansible.com/ansible/latest/installation_guide/intro_installation.html and try again")
		ux.Logger.PrintToUser("")
		return err
	}
	return nil
}
