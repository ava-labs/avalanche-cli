/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/spf13/cobra"
	// "github.com/ava-labs/avalanche-network-runner/cmd/avalanche-network-runner/server"
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy your subnet to a network",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run:  deploySubnet,
	Args: cobra.ExactArgs(1),
}

var deployLocal *bool
var force *bool

func init() {
	subnetCmd.AddCommand(deployCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// deployCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	deployLocal = deployCmd.Flags().BoolP("local", "l", false, "Deploy subnet locally")
	force = deployCmd.Flags().BoolP("force", "f", false, "Deploy without asking for confirmation")
}

func getChainsInSubnet(subnetName string) ([]string, error) {
	usr, _ := user.Current()
	mainDir := filepath.Join(usr.HomeDir, BaseDir)
	files, err := ioutil.ReadDir(mainDir)
	if err != nil {
		return []string{}, err
	}

	chains := []string{}

	for _, f := range files {
		if strings.Contains(f.Name(), sidecar_suffix) {
			// read in sidecar file
			path := filepath.Join(mainDir, f.Name())
			jsonBytes, err := os.ReadFile(path)
			if err != nil {
				return []string{}, err
			}

			var sc models.Sidecar
			err = json.Unmarshal(jsonBytes, &sc)
			if err != nil {
				return []string{}, err
			}
			if sc.Subnet == subnetName {
				chains = append(chains, sc.Name)
			}
		}
	}
	return chains, nil
}

func deploySubnet(cmd *cobra.Command, args []string) {
	// Check subnet exists
	// TODO create a file that lists chains by subnet for fast querying
	chains, err := getChainsInSubnet(args[0])
	if err != nil {
		fmt.Println(err)
		return
	}

	if len(chains) == 0 {
		fmt.Println("Invalid subnet", args[0])
		return
	}

	var network models.Network
	if *deployLocal {
		network = models.Local
	} else {
		networkStr, err := prompts.CaptureList(
			"Choose a network to deploy on",
			[]string{models.Local.String(), models.Fuji.String(), models.Mainnet.String()},
		)
		if err != nil {
			fmt.Println(err)
			return
		}
		network = models.NetworkFromString(networkStr)
	}

	fmt.Println("Deploying", chains, "to", network.String())

}
