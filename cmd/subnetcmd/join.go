// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var avagoConfigPath string

// avalanche subnet deploy
func newJoinCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "join [subnetName]",
		Short: "Instruct a validator node to begin validating a new subnet",
		Long: `Instruct a validator node to begin validating a new subnet.
The NodeID of that validator node must have been whitelisted by one of the 
subnet's control keys for this to work.

The node also needs to be restarted.
If --avalanchego-config is provided, this command tries to edit the config file at that path
(consider correct permissions).`,
		RunE: joinCmd,
		Args: cobra.ExactArgs(1),
	}
	cmd.Flags().StringVar(&avagoConfigPath, "avalanchego-config", "", "file path of the avalanchego config file")
	return cmd
}

func joinCmd(cmd *cobra.Command, args []string) error {
	/*
		chains, err := validateSubnetName(args)
		if err != nil {
			return err
		}
		subnetName := chains[0]

		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return err
		}

		subnetID := sc.SubnetID
		if subnetID == ids.Empty {
			return errNoSubnetID
		}
	*/
	subnetIDstr := "29Zd5yhP7Yb2cTebBbUVKUjHHNviHzgj1y9kKJsvMn2dyWnkpG"
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

	if avagoConfigPath == "" {
		printJoinCmd(subnetIDstr, networkLower)
	} else {
		if err := editConfigFile(subnetIDstr, networkLower); err != nil {
			return err
		}
	}
	return nil
}

func editConfigFile(subnetID string, networkID string) error {
	warn := "WARNING: This will overwrite your existing config file if there is any, are you sure?"
	yes, err := prompts.CaptureYesNo(warn)
	if err != nil {
		return err
	}
	if !yes {
		ux.Logger.PrintToUser("Canceled by user")
		return nil
	}
	fileBytes, err := os.ReadFile(avagoConfigPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to load avalanchego config file %s: %w", avagoConfigPath, err)
	}
	if fileBytes == nil {
		fileBytes = []byte("{}")
	}
	var avagoConfig map[string]interface{}
	if err := json.Unmarshal(fileBytes, &avagoConfig); err != nil {
		return fmt.Errorf("failed to unpack the config file %s to JSON: %w", avagoConfigPath, err)
	}
	avagoConfig["whitelisted-subnets"] = subnetID
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

(using your binary location)`
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

	ux.Logger.PrintToUser(msg, subnetID, networkID)
}
