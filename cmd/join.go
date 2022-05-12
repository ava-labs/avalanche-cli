/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/spf13/cobra"
)

var subnetGroupName *string

// listCmd represents the list command
var joinCmd = &cobra.Command{
	Use:   "join",
	Short: "Combine multiple chains into the same subnet",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run:  joinSubnets,
	Args: cobra.MinimumNArgs(2),
}

func init() {
	subnetCmd.AddCommand(joinCmd)

	subnetGroupName = joinCmd.Flags().StringP("name", "n", "",
		"specify a name for the subnet containing this group of chains")
}

func joinSubnets(cmd *cobra.Command, args []string) {
	// Check all subnets exist so that we don't do a partial modification
	for _, subnetName := range args {
		sidecar := filepath.Join(baseDir, subnetName+sidecar_suffix)
		if _, err := os.Stat(sidecar); err != nil {
			fmt.Println("Could not find subnet", subnetName)
			return
		}
	}
	// All subnets exist

	// Get subnet name
	if *subnetGroupName == "" {
		var err error
		*subnetGroupName, err = prompts.CaptureString("Choose a name for your subnet")
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	// Add chains to subnet
	for _, subnetName := range args {
		sidecar := filepath.Join(baseDir, subnetName+sidecar_suffix)

		// Read sidecar
		jsonBytes, err := os.ReadFile(sidecar)
		if err != nil {
			fmt.Println(err)
			return
		}

		var sc models.Sidecar
		err = json.Unmarshal(jsonBytes, &sc)
		if err != nil {
			fmt.Println(err)
			return
		}

		// Modify sidecar
		sc.Subnet = *subnetGroupName

		// Write sidecar
		scBytes, err := json.MarshalIndent(sc, "", "    ")
		if err != nil {
			fmt.Println(err)
			return
		}

		err = os.WriteFile(sidecar, scBytes, 0644)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

}
