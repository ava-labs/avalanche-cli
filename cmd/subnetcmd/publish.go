// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"

	"github.com/ava-labs/apm/types"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/version"
	"gopkg.in/yaml.v3"
)

var (
	alias          string
	repoURL        string
	vmDescPath     string
	subnetDescPath string
	localOnly      bool
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
	cmd.Flags().StringVar(&alias, "alias", "", "We publish to a remote repo, but identiy the repo locally under a user-provided alias (e.g. myrepo).")
	cmd.Flags().StringVar(&repoURL, "repo-url", "", "The URL of the repo where we are publishing")
	cmd.Flags().StringVar(&vmDescPath, "vm-file-path", "", "Path to the VM description file. If not given, a prompting sequence will be initiated.")
	cmd.Flags().StringVar(&subnetDescPath, "subnet-file-path", "", "Path to the Subnet description file. If not given, a prompting sequence will be initiated.")
	cmd.Flags().BoolVar(&forceWrite, forceFlag, false, "If true, ignores if the subnet has been published in the past, and attempts a forced publish.")
	cmd.Flags().BoolVar(&localOnly, "--local-only", false, "If true, we actually don't push to a remote repo, just create a local repo structure and commit to it.")
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

	if !forceWrite {
		// if forceWrite is present, we don't need to check if it has been previously published, we just do
		published, err := isAlreadyPublished(subnetName)
		if err != nil {
			return err
		}
		if published {
			ux.Logger.PrintToUser("It appears this subnet has already been published, while no force flag has been detected.")
			return errors.New("aborted")
		}
	}

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

	if localOnly {
		ux.Logger.PrintToUser("`--local-only` is true. Created the local repo but did not push")
		return nil
	}

	// TODO: if not published? New commit? Etc...
	if err = publisher.Publish(repo, subnetName, vm.Alias, subnetYAML, vmYAML); err != nil {
		return err
	}

	ux.Logger.PrintToUser("Successfully published")
	return nil
}

// current simplistic approach:
// just search any folder names `subnetName` inside the reposDir's `subnets` folder
func isAlreadyPublished(subnetName string) (bool, error) {
	reposDir := app.GetReposDir()

	found := false

	if err := filepath.WalkDir(reposDir, func(path string, d fs.DirEntry, err error) error {
		if err == nil {
			if filepath.Base(path) == constants.VMDir {
				return filepath.SkipDir
			}
			if !d.IsDir() && d.Name() == subnetName {
				found = true
			}
		}
		return nil
	}); err != nil {
		return false, err
	}
	return found, nil
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
	return app.Prompt.CaptureString("Provide an alias for the repository we are going to use")
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
				// NOTE: supporting only one remote for now
				remotes[i] = r.URLs[0]
				i++
			}
			repoURL, err = app.Prompt.CaptureList("Which is the remote URL for this repo?", remotes)
			if err != nil {
				// should never happen
				return err
			}
			return nil
		}
		repoURL, err = app.Prompt.CaptureString("Provide the repository URL")
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

	desc, err := app.Prompt.CaptureEmpty("Provide a free-text description of the Subnet", nil)
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

	subnet := &types.Subnet{
		ID:          sc.Networks[models.Fuji.String()].SubnetID.String(),
		Alias:       sc.Name,
		Homepage:    homepage.(string),
		Description: desc.(string),
		Maintainers: strMaintrs,
		VMs:         []string{sc.Subnet},
	}

	return subnet, nil
}

func getVMInfo(sc *models.Sidecar) (*types.VM, error) {
	var (
		vmID, desc any
		strMaintrs []string
		err        error
	)

	switch {
	case sc.VM == models.CustomVM:
		vmID, err = app.Prompt.CaptureEmpty("What is the ID of this VM?", nil)
		if err != nil {
			return nil, err
		}
		desc, err = app.Prompt.CaptureEmpty("Provide a description for this VM", nil)
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

		strMaintrs = make([]string, len(maintrs))
		for i, m := range maintrs {
			strMaintrs[i] = m.(string)
		}

	case sc.VM == models.SpacesVM:
		vmID = models.SpacesVM
		desc = "Authenticated, hierarchical storage of arbitrary keys/values using any EIP-712 compatible wallet."
		strMaintrs = []string{"ava-labs"}
	case sc.VM == models.SubnetEvm:
		vmID = models.SubnetEvm
		desc = "Subnet EVM is a simplified version of Coreth VM (C-Chain). It implements the Ethereum Virtual Machine and supports Solidity smart contracts as well as most other Ethereum client functionality"
		strMaintrs = []string{"ava-labs"}
	default:
		return nil, fmt.Errorf("unexpected error: unsupported VM type: %s", sc.VM)
	}

	scr, err := app.Prompt.CaptureEmpty("What scripts needs to run to install this VM? Needs to be an executable command to build the VM.", nil)
	if err != nil {
		return nil, err
	}

	bin, err := app.Prompt.CaptureEmpty("What is the binary path? (This is the output of the build command)", nil)
	if err != nil {
		return nil, err
	}

	url, err := app.Prompt.CaptureEmpty("Tell us the URL to download the source. Needs to be a fixed version, not `latest`.", nil)
	if err != nil {
		return nil, err
	}

	sha, err := app.Prompt.CaptureEmpty("For integrity checks, provide the sha256 commit for the used version", nil)
	if err != nil {
		return nil, err
	}

	strVer, err := app.Prompt.CaptureVersion("This is the last question! What is the version being used? Use semantic versioning (v1.2.3)")
	if err != nil {
		return nil, err
	}
	ver, err := version.Parse(strVer)
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
