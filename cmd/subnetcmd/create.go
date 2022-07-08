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
)

var (
	forceCreate  bool
	useSubnetEvm bool
	filename     string
	useCustom    bool

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

Currently, the tool supports using the Subnet-EVM as your base genesis
template. You can also provide a custom, user-generated genesis by inputing
a file path with the --file flag. As more subnets reach maturity, you'll be
able to use this tool to generate additional VM templates, such as the
SpacesVM.

By default, running the command with a subnetName that already exists will
cause the command to fail. If you’d like to overwrite an existing
configuration, pass the -f flag.`,
		Args: cobra.ExactArgs(1),
		RunE: createGenesis,
	}
	cmd.Flags().StringVar(&filename, "file", "", "file path of genesis to use instead of the wizard")
	cmd.Flags().BoolVar(&useSubnetEvm, "evm", false, "use the SubnetEVM as the base template")
	cmd.Flags().BoolVar(&useCustom, "custom", false, "use a custom VM template")
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

func createGenesis(cmd *cobra.Command, args []string) error {
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

	if filename == "" {
		var subnetType models.VMType
		var err error
		subnetType = getVMFromFlag()

		if subnetType == "" {
			subnetTypeStr, err := app.Prompt.CaptureList(
				"Choose your VM",
				[]string{subnetEvm, customVM},
			)
			if err != nil {
				return err
			}
			subnetType = models.VMTypeFromString(subnetTypeStr)
		}

		var (
			genesisBytes []byte
			sc           *models.Sidecar
		)

		switch subnetType {
		case subnetEvm:
			genesisBytes, sc, err = vm.CreateEvmGenesis(subnetName, app)
			if err != nil {
				return err
			}
			if err = app.CreateSidecar(sc); err != nil {
				return err
			}
		case customVM:
			genesisBytes, sc, err = vm.CreateCustomGenesis(subnetName, app)
			if err != nil {
				return err
			}
			if err = app.CreateSidecar(sc); err != nil {
				return err
			}
		default:
			return errors.New("not implemented")
		}

		if err = app.WriteGenesisFile(subnetName, genesisBytes); err != nil {
			return err
		}
		ux.Logger.PrintToUser("Successfully created genesis")
	} else {
		ux.Logger.PrintToUser("Using specified genesis")
		err := app.CopyGenesisFile(filename, subnetName)
		if err != nil {
			return err
		}

		var subnetType models.VMType
		subnetType = getVMFromFlag()

		if subnetType == "" {
			subnetTypeStr, err := app.Prompt.CaptureList(
				"What VM does your genesis use?",
				[]string{subnetEvm, customVM},
			)
			if err != nil {
				return err
			}
			subnetType = models.VMTypeFromString(subnetTypeStr)
		}
		sc := &models.Sidecar{
			Name:      subnetName,
			VM:        subnetType,
			Subnet:    subnetName,
			TokenName: "",
		}

		if err = app.CreateSidecar(sc); err != nil {
			return err
		}
		ux.Logger.PrintToUser("Successfully created genesis")
	}
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
