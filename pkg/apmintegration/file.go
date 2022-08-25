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

func getVMsInSubnet(app *application.Avalanche, subnetKey string) ([]string, error) {
	// TODO
	// Start here then fill out sidecar
	splitSubnet := strings.Split(subnetKey, ":")
	if len(splitSubnet) != 2 {
		return []string{}, fmt.Errorf("invalid subnet key: %s", subnetKey)
	}
	repo := splitSubnet[0]
	subnetName := splitSubnet[1]

	subnetYamlPath := filepath.Join(app.ApmDir, "repositories", repo, "subnets", subnetName+".yaml")
	var subnetWrapper SubnetWrapper

	subnetYamlBytes, err := os.ReadFile(subnetYamlPath)
	if err != nil {
		return []string{}, err
	}

	fmt.Println("File:", string(subnetYamlBytes))

	err = yaml.Unmarshal(subnetYamlBytes, &subnetWrapper)
	if err != nil {
		return []string{}, err
	}

	subnet := subnetWrapper.Subnet

	fmt.Println("Subnet", subnet)
	fmt.Println("VMs", subnet.VMs)

	return subnet.VMs, nil
}
