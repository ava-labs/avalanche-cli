// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package apmintegration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/apm/types"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"gopkg.in/yaml.v3"
)

func GetRepos(app *application.Avalanche) ([]string, error) {
	repositoryDir := filepath.Join(app.ApmDir, "repositories")
	orgs, err := os.ReadDir(repositoryDir)
	if err != nil {
		return []string{}, err
	}

	output := []string{}

	for _, org := range orgs {
		repoDir := filepath.Join(repositoryDir, org.Name())
		repos, err := os.ReadDir(repoDir)
		if err != nil {
			return []string{}, err
		}
		for _, repo := range repos {
			output = append(output, org.Name()+"/"+repo.Name())
		}
	}

	return output, nil
}

func GetSubnets(app *application.Avalanche, repoAlias string) ([]string, error) {
	subnetDir := filepath.Join(app.ApmDir, "repositories", repoAlias, "subnets")
	subnets, err := os.ReadDir(subnetDir)
	if err != nil {
		return []string{}, err
	}
	subnetOptions := make([]string, len(subnets))
	for i, subnet := range subnets {
		subnetOptions[i] = subnet.Name()[:len(subnet.Name())-5]
	}

	return subnetOptions, nil
}

type SubnetWrapper struct {
	Subnet types.Subnet `yaml:"subnet"`
}

type VMWrapper struct {
	VM types.VM `yaml:"vm"`
}

func LoadSubnetFile(app *application.Avalanche, subnetKey string) (types.Subnet, error) {
	splitSubnet := strings.Split(subnetKey, ":")
	if len(splitSubnet) != 2 {
		return types.Subnet{}, fmt.Errorf("invalid subnet key: %s", subnetKey)
	}
	repo := splitSubnet[0]
	subnetName := splitSubnet[1]

	subnetYamlPath := filepath.Join(app.ApmDir, "repositories", repo, "subnets", subnetName+".yaml")
	var subnetWrapper SubnetWrapper

	subnetYamlBytes, err := os.ReadFile(subnetYamlPath)
	if err != nil {
		return types.Subnet{}, err
	}

	err = yaml.Unmarshal(subnetYamlBytes, &subnetWrapper)
	if err != nil {
		return types.Subnet{}, err
	}

	return subnetWrapper.Subnet, nil
}

func LoadVMFile(app *application.Avalanche, repo, vm string) (types.VM, error) {
	vmYamlPath := filepath.Join(app.ApmDir, "repositories", repo, "vms", vm+".yaml")
	var vmWrapper VMWrapper

	vmYamlBytes, err := os.ReadFile(vmYamlPath)
	if err != nil {
		return types.VM{}, err
	}

	err = yaml.Unmarshal(vmYamlBytes, &vmWrapper)
	if err != nil {
		return types.VM{}, err
	}

	return vmWrapper.VM, nil
}

func getVMsInSubnet(app *application.Avalanche, subnetKey string) ([]string, error) {
	subnet, err := LoadSubnetFile(app, subnetKey)
	if err != nil {
		return []string{}, err
	}

	return subnet.VMs, nil
}
