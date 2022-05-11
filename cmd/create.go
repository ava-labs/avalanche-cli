/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/spf13/cobra"
)

var filename string

var forceCreate bool
var useSubnetEvm bool

// var useSpaces *bool
// var useBlob *bool
// var useTimestamp *bool
var useCustom bool

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create [subnetName]",
	Short: "Create a new subnet genesis",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Args: cobra.ExactArgs(1),
	RunE: createGenesis,
}

func moreThanOneVmSelected() bool {
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

func getVmFromFlag() models.VmType {
	if useSubnetEvm {
		return models.SubnetEvm
	}
	if useCustom {
		return models.CustomVm
	}
	return ""
}

func createGenesis(cmd *cobra.Command, args []string) error {
	if moreThanOneVmSelected() {
		return errors.New("Too many VMs selected. Provide at most one VM selection flag.")
	}

	if filename == "" {

		var subnetType models.VmType
		var err error
		subnetType = getVmFromFlag()

		if subnetType == "" {

			subnetTypeStr, err := prompts.CaptureList(
				"Choose your VM",
				[]string{subnetEvm, spacesVm, blobVm, timestampVm, customVm},
			)
			if err != nil {
				return err
			}
			subnetType = models.VmTypeFromString(subnetTypeStr)
		}

		var genesisBytes []byte

		switch subnetType {
		case subnetEvm:
			genesisBytes, err = vm.CreateEvmGenesis(args[0])
			if err != nil {
				return err
			}

			err = createSidecar(args[0], models.SubnetEvm)
			if err != nil {
				return err
			}
		case customVm:
			genesisBytes, err = vm.CreateCustomGenesis(args[0])
			if err != nil {
				return err
			}
			err = createSidecar(args[0], models.CustomVm)
			if err != nil {
				return err
			}
		default:
			return errors.New("Not implemented")
		}

		err = writeGenesisFile(args[0], genesisBytes)
		if err != nil {
			return err
		}
		fmt.Println("Successfully created genesis")
	} else {
		fmt.Println("Using specified genesis")
		err := copyGenesisFile(filename, args[0])
		if err != nil {
			return err
		}

		var subnetType models.VmType
		subnetType = getVmFromFlag()

		if subnetType == "" {
			subnetTypeStr, err := prompts.CaptureList(
				"What VM does your genesis use?",
				[]string{subnetEvm, spacesVm, blobVm, timestampVm, customVm},
			)
			if err != nil {
				return err
			}
			subnetType = models.VmTypeFromString(subnetTypeStr)
		}
		err = createSidecar(args[0], subnetType)
		if err != nil {
			return err
		}
		fmt.Println("Successfully created genesis")
	}
	return nil
}
