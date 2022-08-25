// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ava-labs/apm/types"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/version"
	"gopkg.in/yaml.v3"
)

var (
	repoURL        string
	vmDescPath     string
	subnetDescPath string
)

// avalanche subnet deploy
func newPublishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "publish [subnetName]",
		Short:        "Publish the subnet's VM to the package manager.",
		Long:         ``,
		SilenceUsage: true,
		RunE:         publish,
		Args:         cobra.ExactArgs(1),
	}
	cmd.Flags().StringVar(&repoURL, "repo-url", "", "The URL of the repo where we are publishing")
	cmd.Flags().StringVar(&vmDescPath, "vm-file-path", "", "Path to the VM description file. If not given, a prompting sequence will be initiated.")
	cmd.Flags().StringVar(&subnetDescPath, "subnet-file-path", "", "Path to the Subnet description file. If not given, a prompting sequence will be initiated.")
	return cmd
}

func publish(cmd *cobra.Command, args []string) error {
	chains, err := validateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}
	subnetName := chains[0]
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}

	if repoURL == "" {
		repoURL, err = app.Prompt.CaptureString("Please provide the repository URL")
		if err != nil {
			return err
		}
	}

	var (
		tsubnet *types.Subnet
		vm      *types.VM
	)

	if subnetDescPath == "" {
		tsubnet, err = getSubnetInfo(sc)
	} else {
		err = loadYAMLFile(subnetDescPath, tsubnet)
	}
	if err != nil {
		return err
	}

	if vmDescPath == "" {
		vm, err = getVMInfo(sc)
	} else {
		err = loadYAMLFile(vmDescPath, vm)
	}
	if err != nil {
		return err
	}

	subnetYAML, err := yaml.Marshal(tsubnet)
	if err != nil {
		return err
	}

	vmYAML, err := yaml.Marshal(vm)
	if err != nil {
		return err
	}

	// TODO Create a helper method on app
	repoDir := filepath.Join(app.GetBaseDir(), "repos")
	publisher := subnet.NewPublisher(repoDir)
	// TODO: only if repo does not exist
	repo, err := publisher.AddRepo(repoURL)
	if err != nil {
		return err
	}

	return publisher.Publish(repoURL, repo, subnetName, vm.Alias, subnetYAML, vmYAML)
}

// loadYAMLFile loads a YAML file from disk into a concrete types.Definition object
// using generics. It's role really is solely to verify that the YAML content is valid.
func loadYAMLFile[T types.Definition](path string, defType T) error {
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(fileBytes, defType)
}

func getSubnetInfo(sc models.Sidecar) (*types.Subnet, error) {
	homepage, err := app.Prompt.CaptureEmpty("What is the homepage of the Subnet project?", nil)
	if err != nil {
		return nil, err
	}

	desc, err := app.Prompt.CaptureEmpty("Please provide a free-text description of the Subnet", nil)
	if err != nil {
		return nil, err
	}

	maintrs, canceled, err := app.Prompt.CaptureListDecision(
		app.Prompt,
		"Who are the maintainers of the Subnet?",
		app.Prompt.CaptureEmail,
		"Provide a maintainer",
		"Maintainer",
		"",
		nil,
	)
	if err != nil {
		return nil, err
	}
	if canceled {
		ux.Logger.PrintToUser("Publishing aborted")
		return nil, errors.New("Canceled by user")
	}

	strMaintrs := make([]string, len(maintrs))
	for i, m := range maintrs {
		strMaintrs[i] = m.(string)
	}

	vms, canceled, err := app.Prompt.CaptureListDecision(
		app.Prompt,
		"Provide a list of VMs this Subnet is running",
		app.Prompt.CaptureEmpty,
		"Provide a VM",
		"VM",
		"VMs are instances of blockchains a given Subnet is running.",
		nil,
	)
	if err != nil {
		return nil, err
	}
	if canceled {
		ux.Logger.PrintToUser("Publishing aborted")
		return nil, errors.New("Canceled by user")
	}

	strVMs := make([]string, len(vms))
	for i, v := range vms {
		strVMs[i] = v.(string)
	}

	subnet := &types.Subnet{
		ID:          sc.Networks[models.Fuji.String()].SubnetID.String(),
		Alias:       sc.Name,
		Homepage:    homepage.(string),
		Description: desc.(string),
		Maintainers: strMaintrs,
		VMs:         strVMs,
	}

	return subnet, nil
}

func getVMInfo(sc models.Sidecar) (*types.VM, error) {
	vm := &types.VM{
		ID:            sc.ChainID,                                // This needs to change
		Alias:         sc.Networks["Fuji"].BlockchainID.String(), // Set to something meaningful
		Homepage:      "",
		Description:   "",
		Maintainers:   []string{},
		InstallScript: "",
		BinaryPath:    "",
		URL:           "",
		SHA256:        "",
		Version: version.Semantic{
			Major: 0,
			Minor: 0,
			Patch: 0,
		},
	}

	return vm, nil
}
