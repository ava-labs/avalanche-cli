// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

// avalanche subnet deploy
var deployCmd = &cobra.Command{
	Use:   "deploy [subnetName]",
	Short: "Deploys a subnet configuration with clean state",
	Long: `The subnet deploy command deploys your subnet configuration locally, to
Fuji Testnet, or to Mainnet. Currently, the beta release only support
local deploys.

At the end of the call, the command will print the RPC URL you can use
to interact with the subnet.

Subsequent calls of deploy using the same subnet configuration will
redeploy the subnet and reset the chain state to genesis.`,
	RunE: deploySubnet,
	Args: cobra.ExactArgs(1),
}

var deployLocal bool

func getChainsInSubnet(subnetName string) ([]string, error) {
	files, err := os.ReadDir(app.GetBaseDir())
	if err != nil {
		return []string{}, fmt.Errorf("failed to read baseDir :%w", err)
	}

	chains := []string{}

	for _, f := range files {
		if strings.Contains(f.Name(), constants.SidecarSuffix) {
			// read in sidecar file
			path := filepath.Join(app.GetBaseDir(), f.Name())
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
	// this should not be necessary but some bright guy might just be creating
	// the genesis by hand or something...
	if err := checkInvalidSubnetNames(args[0]); err != nil {
		return fmt.Errorf("subnet name %s is invalid: %s", args[0], err)
	}
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

	ux.Logger.PrintToUser("Deploying %s to %s", chains, network.String())
	// TODO
	switch network {
	case models.Local:
		app.Log.Debug("Deploy local")
		// TODO: Add signal management here. If we Ctrl-C this guy it can leave
		// the gRPC server is a weird state. Should kill that too
		deployer := subnet.NewLocalDeployer(app)
		chain := chains[0]
		chainGenesis := filepath.Join(app.GetBaseDir(), fmt.Sprintf("%s_genesis.json", chain))
		err := deployer.DeployToLocalNetwork(chain, chainGenesis)
		if err != nil {
			if deployer.BackendStartedHere() {
				if innerErr := binutils.KillgRPCServerProcess(); innerErr != nil {
					app.Log.Warn("tried to kill the gRPC server process but it failed: %w", innerErr)
				}
			}
		}
		return err
	default:
		return errors.New("not implemented")
	}
}
