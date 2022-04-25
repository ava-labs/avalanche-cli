/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/spf13/cobra"
)

var filename string
var fast bool

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
	Run:  createGenesis,
}

func init() {
	subnetCmd.AddCommand(createCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// createCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	createCmd.Flags().StringVarP(&filename, "filename", "f", "", "filepath of genesis to use")
	createCmd.Flags().BoolVarP(&fast, "fast", "z", false, "use default values to minimize configuration")
}

const BaseDir = ".avalanche-cli"

func writeGenesisFile(subnetName string, genesisBytes []byte) error {
	usr, _ := user.Current()
	genesisPath := filepath.Join(usr.HomeDir, BaseDir, subnetName+genesis_suffix)
	err := os.WriteFile(genesisPath, genesisBytes, 0644)
	return err
}

func createGenesis(cmd *cobra.Command, args []string) {
	if filename == "" {
		const subnetEvm = "SubnetEVM"
		const spacesVm = "Spaces VM"
		const blobVm = "Blob VM"
		const timestampVm = "Timestamp VM"
		const customVm = "Custom"
		subnetType, err := captureList(
			"Choose your VM",
			[]string{subnetEvm, spacesVm, blobVm, timestampVm, customVm},
		)
		if err != nil {
			fmt.Println(err)
			return
		}

		var genesisBytes []byte

		switch subnetType {
		case subnetEvm:
			genesisBytes, err = createEvmGenesis(args[0])
			if err != nil {
				fmt.Println(err)
				return
			}
		default:
			fmt.Println("Not implemented")
			return
		}

		err = writeGenesisFile(args[0], genesisBytes)
		if err != nil {
			fmt.Println(err)
			return
		}
	} else {
		fmt.Println("Using specified genesis")
		// TODO copy file
	}
}
