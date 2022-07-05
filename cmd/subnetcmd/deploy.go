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

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var (
	deployLocal bool
	keyName     string
)

// avalanche subnet deploy
func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [subnetName]",
		Short: "Deploys a subnet configuration with clean state",
		Long: `The subnet deploy command deploys your subnet configuration locally, to
Fuji Testnet, or to Mainnet. Currently, the beta release only support
local deploys.

At the end of the call, the command will print the RPC URL you can use
to interact with the subnet.

Subsequent calls of deploy using the same subnet configuration will
redeploy the subnet and reset the chain state to genesis.`,
		SilenceUsage: true,
		RunE:         deploySubnet,
		Args:         cobra.ExactArgs(1),
	}
	cmd.Flags().BoolVarP(&deployLocal, "local", "l", false, "deploy to a local network")
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use")
	return cmd
}

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

	// get the network to deploy to
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

	// deploy based on chosen network
	ux.Logger.PrintToUser("Deploying %s to %s", chains, network.String())
	chain := chains[0]
	chain_genesis := filepath.Join(app.GetBaseDir(), fmt.Sprintf("%s_genesis.json", chain))

	switch network {
	case models.Local:
		app.Log.Debug("Deploy local")
		deployer := subnet.NewLocalSubnetDeployer(app)
		if err := deployer.DeployToLocalNetwork(chain, chain_genesis); err != nil {
			if deployer.BackendStartedHere() {
				if innerErr := binutils.KillgRPCServerProcess(app); innerErr != nil {
					app.Log.Warn("tried to kill the gRPC server process but it failed: %w", innerErr)
				}
			}
		}
		return err
	case models.Fuji: // just make the switch pass
		if keyName == "" {
			return errors.New("this command requires the name of the private key")
		}

	case models.Mainnet: // just make the switch pass, fuij/main implementation is the same (for now)
	default:
		return errors.New("not implemented")
	}

	// prompt for control keys
	controlKeys, cancelled, err := getControlKeys()
	if err != nil {
		return err
	}
	if cancelled {
		ux.Logger.PrintToUser("User cancelled. No subnet deployed")
		return nil
	}

	// prompt for threshold
	var threshold uint32

	if len(controlKeys) > 0 {
		threshold, err = getThreshold(uint64(len(controlKeys)))
		if err != nil {
			return err
		}
	}

	// deploy to public network
	deployer := subnet.NewPublicSubnetDeployer(app, app.GetKeyPath(keyName), network)
	subnetID, blockchainID, err := deployer.Deploy(controlKeys, threshold, chain, chain_genesis)
	if err != nil {
		return err
	}

	// update sidecar
	// TODO: need to do something for backwards compatibility?
	sidecar, err := app.LoadSidecar(chain)
	if err != nil {
		return err
	}
	sidecar.SubnetID = subnetID
	sidecar.BlockchainID = blockchainID
	return app.UpdateSidecar(&sidecar)
}

func getControlKeys() ([]string, bool, error) {
	controlKeysPrompt := "Configure which addresses allow to add new validators"
	// TODO: is this text ok?
	noControlKeysPrompt := "You did not add any control key. This means anyone can add validators to your subnet. " +
		"This is considered unsafe. Do you really want to continue?"

	for {
		// ask in a loop so that if some condition is not met we can keep asking
		controlKeys, cancelled, err := controlKeysLoop(controlKeysPrompt)
		if err != nil {
			return nil, false, err
		}
		if cancelled {
			return nil, cancelled, nil
		}
		if len(controlKeys) == 0 {
			// we want to prompt if the user indeed wants to proceed without any control keys
			yes, err := prompts.CaptureNoYes(noControlKeysPrompt)
			if err != nil {
				return nil, false, err
			}
			// the user confirms no control keys
			if yes {
				return controlKeys, false, nil
			}
			// otherwise we go back to the loop for asking
		} else {
			return controlKeys, false, nil
		}
	}
}

// controlKeysLoop asks as many controlkeys the user requires, until Done or Cancel is selected
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

// getThreshold prompts for the threshold of addresses as a number
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
