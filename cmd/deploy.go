// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
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
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/spf13/cobra"
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy your subnet to a network",
	Long: `Deploy your subnet to a network. Currently supports local network only. 
Starts an avalanche-network-runner in the background and deploys your subnet there.`,
	RunE: deploySubnet,
	Args: cobra.ExactArgs(1),
}

var (
	deployLocal bool
	force       bool
)

func getChainsInSubnet(subnetName string) ([]string, error) {
	files, err := ioutil.ReadDir(baseDir)
	if err != nil {
		return []string{}, fmt.Errorf("failed to read baseDir :%w", err)
	}

	chains := []string{}

	for _, f := range files {
		if strings.Contains(f.Name(), sidecar_suffix) {
			// read in sidecar file
			path := filepath.Join(baseDir, f.Name())
			jsonBytes, err := os.ReadFile(path)
			if err != nil {
				return []string{}, fmt.Errorf("failed reading file %s: %w", path, err)
			}

			var sc models.Sidecar
			err = json.Unmarshal(jsonBytes, &sc)
			if err != nil {
				return []string{}, fmt.Errorf("failed unmarshaling file %s: %w", path, err)
			}
			if sc.Subnet == subnetName {
				chains = append(chains, sc.Name)
			}
		}
	}
	return chains, nil
}

// deploySubnet is the cobra command run for deploying subnets
func deploySubnet(cmd *cobra.Command, args []string) error {
	// Check subnet exists
	// TODO create a file that lists chains by subnet for fast querying
	chains, err := getChainsInSubnet(args[0])
	if err != nil {
		return fmt.Errorf("failed to getChainsInSubnet: %w", err)
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

	ux.PrintToUser(fmt.Sprintf("Deploying %s to %s", chains, network.String()), log)
	// TODO
	switch network {
	case models.Local:
		log.Debug("Deploy local")
		// TODO: Add signal management here. If we Ctrl-C this guy it can leave
		// the gRPC server is a weird state. Should kill that too
		deployer := subnet.NewLocalSubnetDeployer(log, baseDir)
		chain := chains[0]
		chain_genesis := filepath.Join(baseDir, fmt.Sprintf("%s_genesis.json", chain))
		return deployer.DeployToLocalNetwork(chain, chain_genesis)
	default:
		return errors.New("Not implemented")
	}
}
