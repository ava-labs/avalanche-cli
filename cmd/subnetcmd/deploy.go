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
		Short: "Deploys a subnet configuration",
		Long: `The subnet deploy command deploys your subnet configuration locally, to
Fuji Testnet, or to Mainnet. Currently, the beta release only supports
local and Fuji deploys.

At the end of the call, the command will print the RPC URL you can use
to interact with the subnet.

Subnets may only be deployed once. Subsequent calls of deploy to the
same network (local, Fuji, Mainnet) are not allowed. If you'd like to
redeploy a subnet locally for testing, you must first call avalanche
network clean to reset all deployed chain state. Subsequent local
deploys will redeploy the chain with fresh state. The same subnet can
be deployed to multiple networks, so you can take your locally tested
subnet and deploy it on Fuji or Mainnet.`,
		SilenceUsage: true,
		RunE:         deploySubnet,
		Args:         cobra.ExactArgs(1),
	}
	cmd.Flags().BoolVarP(&deployLocal, "local", "l", false, "deploy to a local network")
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use for fuji deploys")
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
	chains, err := validateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}

	// get the network to deploy to
	var network models.Network
	if deployLocal {
		network = models.Local
	} else {
		networkStr, err := app.Prompt.CaptureList(
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
	chainGenesis, err := app.LoadRawGenesis(chain)
	if err != nil {
		return err
	}

	genesisPath := app.GetGenesisPath(chain)

	switch network {
	case models.Local:
		app.Log.Debug("Deploy local")
		sc, err := app.LoadSidecar(chain)
		if err != nil {
			return fmt.Errorf("failed to load sidecar for later update: %w", err)
		}
		deployer := subnet.NewLocalSubnetDeployer(app)
		subnetID, blockchainID, err := deployer.DeployToLocalNetwork(chain, chainGenesis, genesisPath)
		if err != nil {
			if deployer.BackendStartedHere() {
				if innerErr := binutils.KillgRPCServerProcess(app); innerErr != nil {
					app.Log.Warn("tried to kill the gRPC server process but it failed: %w", innerErr)
				}
			}
			return err
		}
		if sc.Networks == nil {
			sc.Networks = make(map[string]models.NetworkData)
		}
		sc.Networks[models.Local.String()] = models.NetworkData{
			SubnetID:     subnetID,
			BlockchainID: blockchainID,
		}
		if err := app.UpdateSidecar(&sc); err != nil {
			return fmt.Errorf("creation of chains and subnet was successful, but failed to update sidecar: %w", err)
		}
		return nil

	case models.Fuji: // just make the switch pass
		if keyName == "" {
			keyName, err = captureKeyName()
			if err != nil {
				return err
			}
		}

	case models.Mainnet: // just make the switch pass, fuij/main implementation is the same (for now)
	default:
		return errors.New("not implemented")
	}

	// from here on we are assuming a public deploy

	// prompt for control keys
	controlKeys, cancelled, err := getControlKeys(network)
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
	deployer := subnet.NewPublicDeployer(app, app.GetKeyPath(keyName), network)
	subnetID, blockchainID, err := deployer.Deploy(controlKeys, threshold, chain, chainGenesis)
	if err != nil {
		return err
	}

	// update sidecar
	// TODO: need to do something for backwards compatibility?
	sidecar, err := app.LoadSidecar(chain)
	if err != nil {
		return err
	}
	nets := sidecar.Networks
	if nets == nil {
		nets = make(map[string]models.NetworkData)
	}
	nets[network.String()] = models.NetworkData{
		SubnetID:     subnetID,
		BlockchainID: blockchainID,
	}
	sidecar.Networks = nets
	return app.UpdateSidecar(&sidecar)
}

func getControlKeys(network models.Network) ([]string, bool, error) {
	controlKeysInitialPrompt := "Configure which addresses may add new validators to the subnet.\n" +
		"These addresses are known as your control keys. You will also\n" +
		"set how many control keys are required to add a validator."
	controlKeysPrompt := "Set control keys"

	ux.Logger.PrintToUser(controlKeysInitialPrompt)
	for {
		// ask in a loop so that if some condition is not met we can keep asking
		controlKeys, cancelled, err := controlKeysLoop(controlKeysPrompt, network)
		if err != nil {
			return nil, false, err
		}
		if cancelled {
			return nil, cancelled, nil
		}
		if len(controlKeys) == 0 {
			ux.Logger.PrintToUser("This tool does not allow to proceed without any control key set")
		} else {
			return controlKeys, false, nil
		}
	}
}

// controlKeysLoop asks as many controlkeys the user requires, until Done or Cancel is selected
func controlKeysLoop(controlKeysPrompt string, network models.Network) ([]string, bool, error) {
	label := "Control key"
	info := "Control keys are P-Chain addresses which have admin rights on the subnet.\n" +
		"Only private keys which control such addresses are allowed to make changes on the subnet"
	arg := network
	list, canceled, err := app.Prompt.CaptureListDecision(
		app.Prompt, // we need this to be able to mock test
		controlKeysPrompt,
		app.Prompt.CapturePChainAddress,
		"Enter P-Chain address (Example: P-...)",
		label,
		info,
		arg,
	)

	ctrlKeys := make([]string, len(list))
	var (
		key string
		ok  bool
	)
	for i, k := range list {
		if key, ok = k.(string); !ok {
			return nil, false, fmt.Errorf("expected string but got %T", key)
		}
		ctrlKeys[i] = key
	}
	return ctrlKeys, canceled, err
}

// getThreshold prompts for the threshold of addresses as a number
func getThreshold(maxLen uint64) (uint32, error) {
	threshold, err := app.Prompt.CaptureUint64("Enter required number of control key signatures to add a validator")
	if err != nil {
		return 0, err
	}
	if threshold > maxLen {
		return 0, fmt.Errorf("the threshold can't be bigger than the number of control keys")
	}
	return uint32(threshold), err
}

func validateSubnetNameAndGetChains(args []string) ([]string, error) {
	// this should not be necessary but some bright guy might just be creating
	// the genesis by hand or something...
	if err := checkInvalidSubnetNames(args[0]); err != nil {
		return nil, fmt.Errorf("subnet name %s is invalid: %s", args[0], err)
	}
	// Check subnet exists
	// TODO create a file that lists chains by subnet for fast querying
	chains, err := getChainsInSubnet(args[0])
	if err != nil {
		return nil, fmt.Errorf("failed to getChainsInSubnet: %w", err)
	}

	if len(chains) == 0 {
		return nil, errors.New("Invalid subnet " + args[0])
	}

	return chains, nil
}
