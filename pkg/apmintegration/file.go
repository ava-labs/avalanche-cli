package apmintegration

import (
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
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

func getVMsInSubnet(app *application.Avalanche, repoAlias, subnetName string) ([]string, error) {
	// TODO
	// Start here then fill out sidecar
	subnetYamlPath := filepath.Join(app.ApmDir, "repositories", repoAlias, "subnets", subnetName)

	return []string{}, nil
}
