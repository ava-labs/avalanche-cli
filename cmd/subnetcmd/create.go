// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"fmt"
	"unicode"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

var (
	forceCreate      bool
	useSubnetEvm     bool
	useSpacesVM      bool
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
		Long: `The subnet create command builds a new genesis file to configure your subnet.
The command is structured as an interactive wizard. It will walk you through
all the steps you need to create your first subnet.

Currently, the tool supports deploying Subnet-EVM and Subnet-EVM forks. You
can create a custom, user-generated genesis with a custom vm by providing
the path to your genesis and vm binarires with the --genesis and --vm flags.
As more subnets reach maturity, you'll be able to use this tool to generate
additional VM templates, such as the SpacesVM.

By default, running the command with a subnetName that already exists will
cause the command to fail. If you’d like to overwrite an existing
configuration, pass the -f flag.`,
		Args: cobra.ExactArgs(1),
		RunE: createSubnetConfig,
	}
	cmd.Flags().StringVar(&genesisFile, "genesis", "", "file path of genesis to use")
	cmd.Flags().StringVar(&vmFile, "vm", "", "file path of custom vm to use")
	cmd.Flags().BoolVar(&useSubnetEvm, "evm", false, "use the SubnetEVM as the base template")
	cmd.Flags().BoolVar(&useSpacesVM, "spacesvm", false, "use the SpacesVM as the base template")
	cmd.Flags().StringVar(&vmVersion, "vm-version", "", "version of vm template to use")
	cmd.Flags().BoolVar(&useCustom, "custom", false, "use a custom VM template")
	cmd.Flags().BoolVar(&useLatestVersion, "latest", false, "use latest VM version, takes precedence over --vm-version")
	cmd.Flags().BoolVarP(&forceCreate, forceFlag, "f", false, "overwrite the existing configuration if one exists")
	return cmd
}

func moreThanOneVMSelected() bool {
	vmVars := []bool{useSubnetEvm, useSpacesVM, useCustom}
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
	if useSpacesVM {
		return models.SpacesVM
	}
	if useCustom {
		return models.CustomVM
	}
	return ""
}

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
			[]string{subnetEvm, spacesVM, customVM},
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
		vmVersion = "latest"
	}

	if vmVersion != "latest" && vmVersion != "" && !semver.IsValid(vmVersion) {
		return fmt.Errorf("invalid version string, should be semantic version (ex: v1.1.1): %s", vmVersion)
	}

	switch subnetType {
	case models.SubnetEvm:
		genesisBytes, sc, err = vm.CreateEvmSubnetConfig(app, subnetName, genesisFile, vmVersion)
		if err != nil {
			return err
		}
	case models.SpacesVM:
		genesisBytes, sc, err = vm.CreateSpacesVMSubnetConfig(app, subnetName, genesisFile, vmVersion)
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

	if err = app.CreateSidecar(sc); err != nil {
		return err
	}

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
