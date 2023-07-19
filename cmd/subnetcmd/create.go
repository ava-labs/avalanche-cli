// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"fmt"
	"unicode"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

const (
	forceFlag = "force"
	latest    = "latest"
)

var (
	forceCreate      bool
	useSubnetEvm     bool
	genesisFile      string
	vmFile           string
	useCustom        bool
	vmVersion        string
	useLatestVersion bool

	errIllegalNameCharacter = errors.New(
		"illegal name character: only letters, no special characters allowed")
)

// avalanche subnet create
func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [subnetName]",
		Short: "Create a new subnet configuration",
		Long: `The subnet create command builds a new genesis file to configure your Subnet.
By default, the command runs an interactive wizard. It walks you through
all the steps you need to create your first Subnet.

The tool supports deploying Subnet-EVM, and custom VMs. You
can create a custom, user-generated genesis with a custom VM by providing
the path to your genesis and VM binaries with the --genesis and --vm flags.

By default, running the command with a subnetName that already exists
causes the command to fail. If you’d like to overwrite an existing
configuration, pass the -f flag.`,
		SilenceUsage:      true,
		Args:              cobra.ExactArgs(1),
		RunE:              createSubnetConfig,
		PersistentPostRun: handlePostRun,
	}
	cmd.Flags().StringVar(&genesisFile, "genesis", "", "file path of genesis to use")
	cmd.Flags().StringVar(&vmFile, "vm", "", "file path of custom vm to use")
	cmd.Flags().BoolVar(&useSubnetEvm, "evm", false, "use the Subnet-EVM as the base template")
	cmd.Flags().StringVar(&vmVersion, "vm-version", "", "version of vm template to use")
	cmd.Flags().BoolVar(&useCustom, "custom", false, "use a custom VM template")
	cmd.Flags().BoolVar(&useLatestVersion, latest, false, "use latest VM version, takes precedence over --vm-version")
	cmd.Flags().BoolVarP(&forceCreate, forceFlag, "f", false, "overwrite the existing configuration if one exists")
	return cmd
}

func moreThanOneVMSelected() bool {
	vmVars := []bool{useSubnetEvm, useCustom}
	firstSelect := false
	for _, val := range vmVars {
		if firstSelect && val {
			return true
		} else if val {
			firstSelect = true
		}
	}
	return false
}

func getVMFromFlag() models.VMType {
	if useSubnetEvm {
		return models.SubnetEvm
	}
	if useCustom {
		return models.CustomVM
	}
	return ""
}

// override postrun function from root.go, so that we don't double send metrics for the same command
func handlePostRun(_ *cobra.Command, _ []string) {}

func createSubnetConfig(cmd *cobra.Command, args []string) error {
	subnetName := args[0]
	if app.GenesisExists(subnetName) && !forceCreate {
		return errors.New("configuration already exists. Use --" + forceFlag + " parameter to overwrite")
	}

	if err := checkInvalidSubnetNames(subnetName); err != nil {
		return fmt.Errorf("subnet name %q is invalid: %w", subnetName, err)
	}

	if moreThanOneVMSelected() {
		return errors.New("too many VMs selected. Provide at most one VM selection flag")
	}

	subnetType := getVMFromFlag()

	if subnetType == "" {
		subnetTypeStr, err := app.Prompt.CaptureList(
			"Choose your VM",
			[]string{models.SubnetEvm, models.CustomVM},
		)
		if err != nil {
			return err
		}
		subnetType = models.VMTypeFromString(subnetTypeStr)
	}

	var (
		genesisBytes []byte
		sc           *models.Sidecar
		err          error
	)

	if useLatestVersion {
		vmVersion = latest
	}

	if vmVersion != latest && vmVersion != "" && !semver.IsValid(vmVersion) {
		return fmt.Errorf("invalid version string, should be semantic version (ex: v1.1.1): %s", vmVersion)
	}

	switch subnetType {
	case models.SubnetEvm:
		genesisBytes, sc, err = vm.CreateEvmSubnetConfig(app, subnetName, genesisFile, vmVersion)
		if err != nil {
			return err
		}
	case models.CustomVM:
		genesisBytes, sc, err = vm.CreateCustomSubnetConfig(app, subnetName, genesisFile, vmFile)
		if err != nil {
			return err
		}
	default:
		return errors.New("not implemented")
	}

	if err = app.WriteGenesisFile(subnetName, genesisBytes); err != nil {
		return err
	}

	sc.ImportedFromAPM = false
	if err = app.CreateSidecar(sc); err != nil {
		return err
	}
	flags := make(map[string]string)
	flags[constants.SubnetType] = subnetType.RepoName()
	//config.GenesisPrecompiles[nativeminter.ConfigKey]

	utils.HandleTracking(cmd, app, flags)
	ux.Logger.PrintToUser("Successfully created subnet configuration")
	return nil
}

func checkInvalidSubnetNames(name string) error {
	// this is currently exactly the same code as in avalanchego/vms/platformvm/create_chain_tx.go
	for _, r := range name {
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == ' ') {
			return errIllegalNameCharacter
		}
	}

	return nil
}
