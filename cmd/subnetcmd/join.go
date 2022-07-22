// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/spf13/cobra"
)

var (
	// path to avalanchego config file
	avagoConfigPath string
	// if true, print the manual instructions to screen
	printManual bool
)

// avalanche subnet deploy
func newJoinCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "join [subnetName]",
		Short: "Instruct a validator node to begin validating a new subnet.",
		Long: `Instruct a validator node to begin validating a new subnet.
Either it prints the necessary instructions to screen or attempts to edit/generate a config file automatically.
The NodeID of that validator node must have been whitelisted by one of the 
subnet's control keys for this to work.

The node also needs to be restarted.
If --avalanchego-config is provided, this command tries to edit the config file at that path
(requires the file to be readable and writable).`,
		RunE: joinCmd,
		Args: cobra.ExactArgs(1),
	}
	cmd.Flags().StringVar(&avagoConfigPath, "avalanchego-config", "", "file path of the avalanchego config file")
	cmd.Flags().BoolVar(&printManual, "print", false, "if true, print the manual config without prompting")
	return cmd
}

func joinCmd(cmd *cobra.Command, args []string) error {
	if printManual && avagoConfigPath != "" {
		return errors.New("--print and --avalanchego-config simultaneously is not supported")
	}

	chains, err := validateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}
	subnetName := chains[0]

	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}

	var network models.Network
	networkStr, err := prompts.CaptureList(
		"Choose a network to deploy on (this command only supports public networks)",
		[]string{models.Fuji.String(), models.Mainnet.String()},
	)
	if err != nil {
		return err
	}
	network = models.NetworkFromString(networkStr)
	networkLower := strings.ToLower(network.String())

	subnetID := sc.Networks[network.String()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}
	subnetIDStr := subnetID.String()

	ask := "Would you like to check if your node is already whitelisted to join this subnet?"
	yes, err := prompts.CaptureYesNo(ask)
	if err != nil {
		return err
	}
	if yes {
		isValidating, err := isNodeValidatingSubnet(subnetID, network)
		if err != nil {
			return err
		}
		if !isValidating {
			ux.Logger.PrintToUser(`The node is not whitelisted to validate this subnet. 
You can continue with this command, generating a config file or printing the whitelisting configuration,
but until the node is whitelisted, it will not be able to validate this subnet.`)
			y, err := prompts.CaptureYesNo("Do you wish to continue")
			if err != nil {
				return err
			}
			if !y {
				return nil
			}
		}
	}

	if printManual {
		printJoinCmd(subnetIDStr, networkLower)
		return nil
	}

	if avagoConfigPath == "" {
		const (
			choiceManual    = "Manual"
			choiceAutomatic = "Automatic"
		)
		choice, err := prompts.CaptureList(
			"How would you like to update the avalanchego config?",
			[]string{choiceManual, choiceAutomatic},
		)
		if err != nil {
			return err
		}
		switch choice {
		case choiceManual:
			printJoinCmd(subnetIDStr, networkLower)
			return nil
		case choiceAutomatic:
			avagoConfigPath, err = prompts.CaptureString("Path to your existing config file (or where it will be generated)")
			if err != nil {
				return err
			}
		}
		// if choice is automatic, we just pass through this block,
		// so we don't need another else if the the config path is not set
	}
	if err := editConfigFile(subnetIDStr, networkLower, avagoConfigPath); err != nil {
		return err
	}
	return nil
}

func isNodeValidatingSubnet(subnetID ids.ID, network models.Network) (bool, error) {
	promptStr := "Please enter your node's ID (NodeID-...)"
	nodeID, err := prompts.CaptureNodeID(promptStr)
	if err != nil {
		return false, err
	}

	var api string
	switch network {
	case models.Fuji:
		api = constants.FujiAPIEndpoint
	case models.Mainnet:
		api = constants.MainnetAPIEndpoint
	default:
		return false, fmt.Errorf("network not supported")
	}
	ctx := context.Background()
	nodeIDs := []ids.NodeID{nodeID}

	pClient := platformvm.NewClient(api)
	vals, err := pClient.GetCurrentValidators(ctx, subnetID, nodeIDs)
	if err != nil {
		return false, err
	}
	for _, v := range vals {
		if v.NodeID == nodeID {
			return true, nil
		}
	}
	return false, nil
}

func editConfigFile(subnetID string, networkID string, configFile string) error {
	warn := "WARNING: This will edit your existing config file if there is any, are you sure?"
	yes, err := prompts.CaptureYesNo(warn)
	if err != nil {
		return err
	}
	if !yes {
		ux.Logger.PrintToUser("Canceled by user")
		return nil
	}
	fileBytes, err := os.ReadFile(configFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to load avalanchego config file %s: %w", configFile, err)
	}
	if fileBytes == nil {
		fileBytes = []byte("{}")
	}
	var avagoConfig map[string]interface{}
	if err := json.Unmarshal(fileBytes, &avagoConfig); err != nil {
		return fmt.Errorf("failed to unpack the config file %s to JSON: %w", configFile, err)
	}

	// check the old entries in the config file for whitelisted subnets
	oldVal := avagoConfig["whitelisted-subnets"]
	newVal := ""
	if oldVal != nil {
		// if an entry already exists, we check if the subnetID already is part
		// of the whitelisted-subnets...
		exists := false
		var oldValStr string
		var ok bool
		if oldValStr, ok = oldVal.(string); !ok {
			return fmt.Errorf("expected a string value, but got %T", oldVal)
		}
		elems := strings.Split(oldValStr, ",")
		for _, s := range elems {
			if s == subnetID {
				// ...if it is, we just don't need to update the value...
				newVal = oldVal.(string)
				exists = true
			}
		}
		// ...but if it is not, we concatenate the new subnet to the existing ones
		if !exists {
			newVal = strings.Join([]string{oldVal.(string), subnetID}, ",")
		}
	} else {
		// there were no entries yet, so add this subnet as its new value
		newVal = subnetID
	}
	avagoConfig["whitelisted-subnets"] = newVal
	avagoConfig["network-id"] = networkID

	writeBytes, err := json.MarshalIndent(avagoConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to pack JSON to bytes for the config file: %w", err)
	}
	if err := os.WriteFile(avagoConfigPath, writeBytes, constants.DefaultPerms755); err != nil {
		return fmt.Errorf("failed to write JSON config file, check permissions? %w", err)
	}
	msg := `The config file has been edited. To use it, make sure to start the node with the '--config-file' option, e.g.

./build/avalanchego --config-file %s

(using your binary location). The node has to be restarted for the changes to take effect.`
	ux.Logger.PrintToUser(msg, avagoConfigPath)
	return nil
}

func printJoinCmd(subnetID string, networkID string) {
	msg := `
If you start your node from the command line WITHOUT a config file (e.g. via command line or systemd script),
add the following flag to your node's startup command:

--whitelisted-subnets=%s
(if the node already has a whitelisted-subnets config, just add the new value to it).

For example:
./build/avalanchego --network-id=%s --whitelisted-subnets=%s

If you start the node via a JSON config file, add this to your config file:
whitelisted-subnets: %s

TIP: Try this command with the --avalanchego-config flag pointing to your config file,
this tool will try to update the file automatically (make sure it can write to it).`

	ux.Logger.PrintToUser(msg, subnetID, networkID, subnetID, subnetID)
}
