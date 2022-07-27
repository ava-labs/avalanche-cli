// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/spf13/cobra"
)

var (
	// path to avalanchego config file
	avagoConfigPath string
	// path to avalanchego plugin dir
	pluginDir string
	// if true, print the manual instructions to screen
	printManual bool
)

// avalanche subnet deploy
func newJoinCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "join [subnetName]",
		Short: "Configure your validator node to begin validating a new subnet",
		Long: `The subnet join command configures your validator node to begin validating
a new subnet.

To complete this process, you must have access to the machine running your
validator. If the CLI is running on the same machine as your validator,
it can generate or update your node's config file automatically.
Alternatively, the command can print the necessary instructions to
update your node manually. To complete the validation process, the
NodeID of your validator node must have been whitelisted by one of the
subnet's control keys.

After you update your validator's config, you will need to restart your
validator manually. If the --avalanchego-config flag is provided, this
command attempts to edit the config file at that path (requires the file
to be readable and writable).

This command currently only supports subnets deployed on the Fuji testnet.`,
		RunE: joinCmd,
		Args: cobra.ExactArgs(1),
	}
	cmd.Flags().StringVar(&avagoConfigPath, "avalanchego-config", "", "file path of the avalanchego config file")
	cmd.Flags().StringVar(&pluginDir, "plugin-dir", "", "file path of avalanchego's plugin directory")
	cmd.Flags().BoolVar(&printManual, "print", false, "if true, print the manual config without prompting")
	return cmd
}

func joinCmd(cmd *cobra.Command, args []string) error {
	if printManual && (avagoConfigPath != "" || pluginDir != "") {
		return errors.New("--print cannot be used with --avalanchego-config or --plugin-dir")
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
	networkStr, err := app.Prompt.CaptureList(
		"Choose a network to validate on (this command only supports public networks)",
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

	ask := "Would you like to check if your node is allowed to join this subnet?\n" +
		"If not, the subnet's control key holder must call avalanche subnet\n" +
		"addValidator with your NodeID."
	ux.Logger.PrintToUser(ask)
	yes, err := app.Prompt.CaptureYesNo("Check whitelist?")
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
			y, err := app.Prompt.CaptureYesNo("Do you wish to continue")
			if err != nil {
				return err
			}
			if !y {
				return nil
			}
		}
	}

	if printManual {
		pluginDir = app.GetTmpPluginDir()
		vmPath, err := createPlugin(sc.Name, pluginDir)
		if err != nil {
			return err
		}
		printJoinCmd(subnetIDStr, networkLower, vmPath)
		return nil
	}

	if avagoConfigPath == "" && pluginDir == "" {
		const (
			choiceManual    = "Manual"
			choiceAutomatic = "Automatic"
		)
		choice, err := app.Prompt.CaptureList(
			"How would you like to update the avalanchego config?",
			[]string{choiceAutomatic, choiceManual},
		)
		if err != nil {
			return err
		}
		if choice == choiceManual {
			pluginDir = app.GetTmpPluginDir()
			vmPath, err := createPlugin(sc.Name, pluginDir)
			if err != nil {
				return err
			}
			printJoinCmd(subnetIDStr, networkLower, vmPath)
			return nil
		}
	}

	// if choice is automatic, we just pass through this block
	if avagoConfigPath == "" {
		avagoConfigPath, err = app.Prompt.CaptureString("Path to your existing config file (or where it will be generated)")
		if err != nil {
			return err
		}
	}

	if pluginDir == "" {
		pluginDir, err = app.Prompt.CaptureString("Path to your avalanchego plugin dir (likely avalanchego/build/plugins)")
		if err != nil {
			return err
		}
	}

	vmPath, err := createPlugin(sc.Name, pluginDir)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("VM binary written to %s", vmPath)

	if err := editConfigFile(subnetIDStr, networkLower, avagoConfigPath); err != nil {
		return err
	}
	return nil
}

func isNodeValidatingSubnet(subnetID ids.ID, network models.Network) (bool, error) {
	promptStr := "Please enter your node's ID (NodeID-...)"
	nodeID, err := app.Prompt.CaptureNodeID(promptStr)
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
	warn := "This will edit your existing config file. This edit is nondestructive,\n" +
		"but it's always good to have a backup."
	ux.Logger.PrintToUser(warn)
	yes, err := app.Prompt.CaptureYesNo("Proceed?")
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

func printJoinCmd(subnetID string, networkID string, vmPath string) {
	msg := `
To setup your node, you must do two things:

1. Add your VM binary to your node's plugin directory
2. Update your node config to start validating the subnet

To add the VM to your plugin directory, copy or scp from %s

If you installed avalanchego manually, your plugin directory is likely
avalanchego/build/plugins.

If you start your node from the command line WITHOUT a config file (e.g. via command
line or systemd script), add the following flag to your node's startup command:

--whitelisted-subnets=%s
(if the node already has a whitelisted-subnets config, append the new value by
comma-separating it).

For example:
./build/avalanchego --network-id=%s --whitelisted-subnets=%s

If you start the node via a JSON config file, add this to your config file:
whitelisted-subnets: %s

TIP: Try this command with the --avalanchego-config flag pointing to your config file,
this tool will try to update the file automatically (make sure it can write to it).

After you update your config, you will need to restart your node for the changes to
take effect.`

	ux.Logger.PrintToUser(msg, vmPath, subnetID, networkID, subnetID, subnetID)
}

func createPlugin(subnetName string, pluginDir string) (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	homeDir := usr.HomeDir
	if pluginDir == "~" {
		// In case of "~", which won't be caught by the "else if"
		pluginDir = homeDir
	} else if strings.HasPrefix(pluginDir, "~/") {
		pluginDir = filepath.Join(homeDir, pluginDir[2:])
	}

	fmt.Println("Plugin Dir", pluginDir)

	chainVMID, err := utils.VMID(subnetName)
	if err != nil {
		return "", fmt.Errorf("failed to create VM ID from %s: %w", subnetName, err)
	}

	downloader := binutils.NewPluginBinaryDownloader(app.Log)

	binDir := filepath.Join(app.GetBaseDir(), constants.AvalancheCliBinDir)
	if err := downloader.DownloadVM(chainVMID.String(), pluginDir, binDir); err != nil {
		return "", err
	}

	vmPath := filepath.Join(pluginDir, chainVMID.String())
	return vmPath, nil
}
