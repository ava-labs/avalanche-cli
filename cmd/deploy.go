/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
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
	RunE: deploySubnet,
	Args: cobra.ExactArgs(1),
}

var deployLocal bool
var force bool

func getChainsInSubnet(subnetName string) ([]string, error) {
	files, err := ioutil.ReadDir(baseDir)
	if err != nil {
		return []string{}, err
	}

	chains := []string{}

	for _, f := range files {
		if strings.Contains(f.Name(), sidecar_suffix) {
			// read in sidecar file
			path := filepath.Join(baseDir, f.Name())
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

func deploySubnet(cmd *cobra.Command, args []string) error {
	// Check subnet exists
	// TODO create a file that lists chains by subnet for fast querying
	chains, err := getChainsInSubnet(args[0])
	if err != nil {
		return err
	}

	if len(chains) == 0 {
		return errors.New("Invalid subnet " + args[0])
	}

	var network models.Network
	if deployLocal {
		network = models.Local
	} else {
		networkStr, err := prompts.CaptureList(
			"Choose a network to deploy on",
			[]string{models.Local.String(), models.Fuji.String(), models.Mainnet.String()},
		)
		if err != nil {
			return err
		}
		network = models.NetworkFromString(networkStr)
	}

	fmt.Println("Deploying", chains, "to", network.String())
	// TODO
	switch network {
	case models.Local:
		// WRITE CODE HERE
		fmt.Println("Deploy local")
	default:
		fmt.Println("Not implemented")
	}
	return nil
}
