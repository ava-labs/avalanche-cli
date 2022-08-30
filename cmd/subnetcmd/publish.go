// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"

	"github.com/ava-labs/apm/types"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"gopkg.in/yaml.v3"
)

var (
	alias          string
	repoURL        string
	vmDescPath     string
	subnetDescPath string
)

type newPublisherFunc func(string, string, string) subnet.Publisher

// avalanche subnet deploy
func newPublishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "publish [subnetName]",
		Short:        "Publish the subnet's VM to a repository",
		Long:         ``,
		SilenceUsage: true,
		RunE:         publish,
		Args:         cobra.ExactArgs(1),
	}
	cmd.Flags().StringVar(&alias, "alias", "", "An alias for the repo to publish to")
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
	return doPublish(&sc, subnetName, subnet.NewPublisher)
}

func doPublish(sc *models.Sidecar, subnetName string, publisherCreateFunc newPublisherFunc) (err error) {
	reposDir := app.GetReposDir()
	// iterate the reposDir to check what repos already exist locally
	// if nothing is found, prompt the user for an alias for a new repo
	if err = getAlias(reposDir); err != nil {
		return err
	}
	// get the URL for the repo
	if err = getRepoURL(reposDir); err != nil {
		return err
	}

	var (
		tsubnet *types.Subnet
		vm      *types.VM
	)
	// TODO: Do we need to check if it has already been published?
	// If yes, do we query the local repo, the remote, do we write into the sidecar, ...?
	if subnetDescPath == "" {
		tsubnet, err = getSubnetInfo(sc)
	} else {
		tsubnet = new(types.Subnet)
		err = loadYAMLFile(subnetDescPath, tsubnet)
	}
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("Nice! We got the subnet info. Let's now get the VM details")

	if vmDescPath == "" {
		vm, err = getVMInfo(sc)
	} else {
		vm = new(types.VM)
		err = loadYAMLFile(vmDescPath, vm)
	}
	if err != nil {
		return err
	}

	// TODO: Publishing exactly 1 subnet and 1 VM in this iteration
	tsubnet.VMs = []string{vm.Alias}

	subnetYAML, err := yaml.Marshal(tsubnet)
	if err != nil {
		return err
	}
	vmYAML, err := yaml.Marshal(vm)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("Thanks! We got all the bits and pieces. Trying to publish on the provided repo...")

	publisher := publisherCreateFunc(reposDir, repoURL, alias)
	repo, err := publisher.GetRepo()
	if err != nil {
		return err
	}

	// TODO: if not published? New commit? Etc...
	if err = publisher.Publish(repo, subnetName, vm.Alias, subnetYAML, vmYAML); err != nil {
		return err
	}

	ux.Logger.PrintToUser("Successfully published")
	return nil
}

// iterate the reposDir to check what repos already exist locally
// if nothing is found, prompt the user for an alias for a new repo
func getAlias(reposDir string) error {
	// have any aliases already been defined?
	if alias == "" {
		matches, err := os.ReadDir(reposDir)
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			// no aliases yet; just ask for a new one
			alias, err = getNewAlias()
			if err != nil {
				return err
			}
		} else {
			// there are already aliases, ask how to proceed
			options := []string{"Provide a new alias", "Pick from list"}
			choice, err := app.Prompt.CaptureList("Don't know which repo to publish to. How would you like to proceed?", options)
			if err != nil {
				return err
			}
			if choice == options[0] {
				// user chose to provide a new alias
				alias, err = getNewAlias()
				if err != nil {
					return err
				}
				// double-check: actually this path exists...
				if _, err := os.Stat(filepath.Join(reposDir, alias)); err == nil {
					ux.Logger.PrintToUser("The repository with the given alias already exists locally. You may have already published this subnet there (the other explanation is that a different subnet has been published there).")
					yes, err := app.Prompt.CaptureYesNo("Do you wish to continue?")
					if err != nil {
						return err
					}
					if !yes {
						ux.Logger.PrintToUser("User canceled, nothing got published.")
						return nil
					}
				}
			} else {
				aliases := make([]string, len(matches))
				for i, a := range matches {
					aliases[i] = a.Name()
				}
				alias, err = app.Prompt.CaptureList("Pick an alias", aliases)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ask for a new alias
func getNewAlias() (string, error) {
	return app.Prompt.CaptureString("Please provide an alias for the repository we are going to use")
}

func getRepoURL(reposDir string) error {
	if repoURL == "" {
		path := filepath.Join(reposDir, alias)
		repo, err := git.PlainOpen(path)
		if err != nil {
			// TODO: probably a debug log; the alias might not have been created yet,
			// so the repo might not exist, therefore just ignore it!
		} else {
			// there is a repo already for this alias, let's try to figure out the remote URL from there
			conf, err := repo.Config()
			if err != nil {
				// TODO Would we really want to abort here?
				return err
			}
			remotes := make([]string, len(conf.Remotes))
			i := 0
			for _, r := range conf.Remotes {
				// TODO: Usually there's only one URL for a remote, but more are supported - should we too?
				remotes[i] = r.URLs[0]
				i++
			}
			repoURL, err = app.Prompt.CaptureList("Which is the remote URL for this repo?", remotes)
			if err != nil {
				// TODO Would we really want to abort here?
				return err
			}
			return nil
		}
		repoURL, err = app.Prompt.CaptureString("Please provide the repository URL")
		if err != nil {
			return err
		}
	}
	return nil
}

// loadYAMLFile loads a YAML file from disk into a concrete types.Definition object
// using generics. It's role really is solely to verify that the YAML content is valid.
func loadYAMLFile[T types.Definition](path string, defType T) error {
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(fileBytes, &defType)
}

func getSubnetInfo(sc *models.Sidecar) (*types.Subnet, error) {
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
		return nil, errors.New("canceled by user")
	}

	strMaintrs := make([]string, len(maintrs))
	for i, m := range maintrs {
		strMaintrs[i] = m.(string)
	}

	// TODO: In this version, we are publishing 1 subnet and exactly 1 VM.
	// 1. Will the VM list has to be editale in the future?
	// 2. If yes, need to think about the whole process
	// 3. Can VMs later be added independently?
	/*
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
			return nil, errors.New("canceled by user")
		}

		strVMs := make([]string, len(vms))
		for i, v := range vms {
			strVMs[i] = v.(string)
		}
	*/

	subnet := &types.Subnet{
		ID:          sc.Networks[models.Fuji.String()].SubnetID.String(),
		Alias:       sc.Name,
		Homepage:    homepage.(string),
		Description: desc.(string),
		Maintainers: strMaintrs,
		// VMs:         strVMs,
		VMs: nil,
	}

	return subnet, nil
}

func getVMInfo(sc *models.Sidecar) (*types.VM, error) {
	vmID, err := app.Prompt.CaptureEmpty("What is the ID of this VM?", nil)
	if err != nil {
		return nil, err
	}

	desc, err := app.Prompt.CaptureEmpty("Provide a description for this VM please", nil)
	if err != nil {
		return nil, err
	}

	maintrs, canceled, err := app.Prompt.CaptureListDecision(
		app.Prompt,
		"Who are the maintainers of the VM?",
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
		return nil, errors.New("canceled by user")
	}

	strMaintrs := make([]string, len(maintrs))
	for i, m := range maintrs {
		strMaintrs[i] = m.(string)
	}

	scr, err := app.Prompt.CaptureEmpty("What scripts needs to run to install this VM?", nil)
	if err != nil {
		return nil, err
	}

	bin, err := app.Prompt.CaptureEmpty("What is the binary path?", nil)
	if err != nil {
		return nil, err
	}

	url, err := app.Prompt.CaptureEmpty("Tell us the URL to download the binary please", nil)
	if err != nil {
		return nil, err
	}

	sha, err := app.Prompt.CaptureEmpty("For integrity checks, please provide the sha256 commit for the used version", nil)
	if err != nil {
		return nil, err
	}

	ver, err := app.Prompt.CaptureSemanticVersion("This is the last question! What is the version being used? Use semantic versioning (v1.2.3)")
	if err != nil {
		return nil, err
	}

	vm := &types.VM{
		ID:            vmID.(string),                             // This needs to change
		Alias:         sc.Networks["Fuji"].BlockchainID.String(), // Set to something meaningful
		Homepage:      "",
		Description:   desc.(string),
		Maintainers:   strMaintrs,
		InstallScript: scr.(string),
		BinaryPath:    bin.(string),
		URL:           url.(string),
		SHA256:        sha.(string),
		Version:       *ver,
	}

	return vm, nil
}
