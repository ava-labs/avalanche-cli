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
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/spf13/cobra"
)

// deployCmd represents the deploy command
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
	RunE:         deploySubnet,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
}

var deployLocal bool

func getChainsInSubnet(subnetName string) ([]string, error) {
	files, err := ioutil.ReadDir(app.GetBaseDir())
	if err != nil {
		return []string{}, fmt.Errorf("failed to read baseDir :%w", err)
	}

	chains := []string{}

	for _, f := range files {
		if strings.Contains(f.Name(), constants.Sidecar_suffix) {
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
	chain := chains[0]
	chain_genesis := filepath.Join(app.GetBaseDir(), fmt.Sprintf("%s_genesis.json", chain))
	// TODO
	switch network {
	case models.Local:
		app.Log.Debug("Deploy local")
		// TODO: Add signal management here. If we Ctrl-C this guy it can leave
		// the gRPC server is a weird state. Should kill that too
		deployer := subnet.NewLocalSubnetDeployer(app)
		err := deployer.DeployToLocalNetwork(chain, chain_genesis)
		if err != nil {
			if deployer.BackendStartedHere() {
				if innerErr := binutils.KillgRPCServerProcess(); innerErr != nil {
					app.Log.Warn("tried to kill the gRPC server process but it failed: %w", innerErr)
				}
			}
		}
		return err
	case models.Fuji: // just make the switch pass
	case models.Mainnet: // just make the switch pass
	default:
		return errors.New("Not implemented")
	}
	controlKeys, cancelled, err := getControlKeys()
	if err != nil {
		return err
	}
	if cancelled {
		ux.Logger.PrintToUser("User cancelled. No subnet deployed")
		return nil
	}

	var threshold uint32

	if len(controlKeys) > 0 {
		threshold, err = getThreshold(uint64(len(controlKeys)))
		if err != nil {
			return err
		}
	}
	deployer := subnet.NewPublicSubnetDeployer(app, privKeyPath, network)
	return deployer.Deploy(controlKeys, threshold, chain, chain_genesis)
}

func getControlKeys() ([]string, bool, error) {
	controlKeysPrompt := "Configure which addresses allow to add new validators"
	noControlKeysPrompt := "You did not add any control key. This means anyone can add validators to your subnet. " +
		"This is considered unsafe. Do you really want to continue?"

	for {
		controlKeys, cancelled, err := controlKeysLoop(controlKeysPrompt)
		if err != nil {
			return nil, false, err
		}
		if cancelled {
			return nil, cancelled, nil
		}
		if len(controlKeys) == 0 {
			yes, err := prompts.CaptureNoYes(noControlKeysPrompt)
			if err != nil {
				return nil, false, err
			}
			if yes {
				return controlKeys, false, nil
			}
		} else {
			return controlKeys, false, nil
		}
	}
}

func controlKeysLoop(controlKeysPrompt string) ([]string, bool, error) {
	const (
		addCtrlKey = "Add control key"
		doneMsg    = "Done"
		cancelMsg  = "Cancel"
	)

	var controlKeys []string

	for {
		listDecision, err := prompts.CaptureList(
			controlKeysPrompt, []string{addCtrlKey, doneMsg, cancelMsg},
		)
		if err != nil {
			return nil, false, err
		}

		switch listDecision {
		case addCtrlKey:
			controlKey, err := prompts.CapturePChainAddress(
				"Enter the addresses which control who can add validators to this subnet",
			)
			if err != nil {
				return nil, false, err
			}
			if contains(controlKeys, controlKey) {
				fmt.Println("Address already in list")
				continue
			}
			controlKeys = append(controlKeys, controlKey)
		case doneMsg:
			return controlKeys, false, nil
		case cancelMsg:
			return nil, true, nil
		default:
			return nil, false, errors.New("unexpected option")
		}
	}
}

func getThreshold(minLen uint64) (uint32, error) {
	threshold, err := prompts.CaptureUint64("Enter required number of control addresses to add validators")
	if err != nil {
		return 0, err
	}
	if threshold > minLen {
		return 0, fmt.Errorf("The threshold can't be bigger than the number of control addresses")
	}
	return uint32(threshold), err
}

func contains(list []string, element string) bool {
	for _, val := range list {
		if val == element {
			return true
		}
	}
	return false
}
