// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package terraform

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// InitConf creates hclFile where we define all terraform configuration in hclFile.Body() and create .tf file where we save the content in
func InitConf() (*hclwrite.File, *hclwrite.Body, error) {
	hclFile := hclwrite.NewEmptyFile()
	rootBody := hclFile.Body()
	return hclFile, rootBody, nil
}

// SaveConf writes all terraform configuration defined in hclFile to tfFile
func SaveConf(terraformDir string, hclFile *hclwrite.File) error {
	tfFile, err := os.Create(filepath.Join(terraformDir, constants.TerraformNodeConfigFile))
	if err != nil {
		return err
	}
	_, err = tfFile.Write(hclFile.Bytes())
	return err
}

// RemoveDirectory remove terraform directory in .avalanche-cli. We need to call this before and after creating ec2 instance
func RemoveDirectory(terraformDir string) error {
	return os.RemoveAll(terraformDir)
}

func CheckIsInstalled() error {
	if err := exec.Command(constants.Terraform).Run(); errors.Is(err, exec.ErrNotFound) { //nolint:gosec
		ux.Logger.PrintToUser("Terraform tool is not available. It is a needed dependency for CLI to create a remote node.")
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Please follow install instructions at https://developer.hashicorp.com/terraform/downloads?product_intent=terraform and try again")
		ux.Logger.PrintToUser("")
		return err
	}
	return nil
}

func GetPublicIPs(terraformDir string) ([]string, error) {
	cmd := exec.Command(constants.Terraform, "output", "instance_ips") //nolint:gosec
	cmd.Env = os.Environ()
	cmd.Dir = terraformDir
	ipsOutput, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	publicIPs := []string{}
	ipsOutputWoSpace := strings.TrimSpace(string(ipsOutput))
	// ip and nodeID outputs are bounded by [ and ,] , we need to remove them
	trimmedPublicIPs := ipsOutputWoSpace[1 : len(ipsOutputWoSpace)-3]
	splitPublicIPs := strings.Split(trimmedPublicIPs, ",")
	for _, publicIP := range splitPublicIPs {
		publicIPWoSpace := strings.TrimSpace(publicIP)
		// ip and nodeID both are bounded by double quotation "", we need to remove them before they can be used
		publicIPs = append(publicIPs, publicIPWoSpace[1:len(publicIPWoSpace)-1])
	}
	return publicIPs, nil
}
