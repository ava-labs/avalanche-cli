// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/plugins"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/kardianos/osext"
	"github.com/shirou/gopsutil/process"
	"github.com/spf13/cobra"
)

var (
	// path to avalanchego config file
	avagoConfigPath string
	// path to avalanchego plugin dir
	pluginDir string
	// if true, print the manual instructions to screen
	printManual bool
	// skipWhitelistCheck if true doesn't prompt, skipping the test
	skipWhitelistCheck bool
	// forceWhitelistCheck if true doesn't prompt, doing the test
	forceWhitelistCheck bool
	// failIfNotValidating
	failIfNotValidating bool
	// if true, doesn't ask for overwriting the config file
	forceWrite bool
	// a list of directories to scan for potential location
	// of avalanchego configs
	scanConfigDirs = []string{}
	// env var for avalanchego data dir
	defaultUnexpandedDataDir = "$" + config.AvalancheGoDataDirVar
	// expected file name for the config
	// TODO should other file names be supported? e.g. conf.json, etc.
	defaultConfigFileName = "config.json"
	// expected name of the plugins dir
	defaultPluginDir = "plugins"
	// default dir where the binary is usually found
	defaultAvalanchegoBuildDir = filepath.Join("go", "src", "github.com", constants.AvaLabsOrg, constants.AvalancheGoRepoName, "build")
)

// this init is partly "borrowed" from avalanchego/config/config.go
func init() {
	folderPath, err := osext.ExecutableFolder()
	if err == nil {
		scanConfigDirs = append(scanConfigDirs, folderPath)
		scanConfigDirs = append(scanConfigDirs, filepath.Dir(folderPath))
	}
	wd, err := os.Getwd()
	if err != nil {
		// really this shouldn't happen, and we could just os.Exit,
		// but it's bit bad to hide an os.Exit here
		fmt.Println("Warning: failed to get current directory")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// really this shouldn't happen, and we could just os.Exit,
		// but it's bit bad to hide an os.Exit here
		fmt.Println("Warning: failed to get user home dir")
	}
	// TODO: Any other dirs we want to scan?
	scanConfigDirs = append(scanConfigDirs,
		filepath.Join("/", "etc", constants.AvalancheGoRepoName),
		filepath.Join("/", "usr", "local", "lib", constants.AvalancheGoRepoName),
		wd,
		home,
		filepath.Join(home, constants.AvalancheGoRepoName),
		filepath.Join(home, defaultAvalanchegoBuildDir),
		filepath.Join(home, ".avalanchego"),
		defaultUnexpandedDataDir,
	)
}

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
	cmd.Flags().BoolVar(&deployTestnet, "fuji", false, "join on `fuji` (alias for `testnet`)")
	cmd.Flags().BoolVar(&deployTestnet, "testnet", false, "join on `testnet` (alias for `fuji`)")
	cmd.Flags().BoolVar(&deployMainnet, "mainnet", false, "join on `mainnet`")
	cmd.Flags().BoolVar(&printManual, "print", false, "if true, print the manual config without prompting")
	cmd.Flags().BoolVar(&skipWhitelistCheck, "skip-whitelist-check", false, "if true, skip the whitelist test")
	cmd.Flags().BoolVar(&forceWhitelistCheck, "force-whitelist-check", false, "if true, force the whitelist test")
	cmd.Flags().BoolVar(&failIfNotValidating, "fail-if-not-validating", false, "fail if whitelist check fails")
	cmd.Flags().StringVar(&nodeIDStr, "nodeID", "", "set the NodeID of the validator to check")
	cmd.Flags().BoolVar(&forceWrite, "force-write", false, "if true, skip to prompt to overwrite the config file")
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

	if err := checkMutuallyExclusive(deployTestnet, deployMainnet, false); err != nil {
		return errors.New("--fuji and --mainnet are mutually exclusive")
	}

	var network models.Network
	switch {
	case deployTestnet:
		network = models.Fuji
	case deployMainnet:
		network = models.Mainnet
	}

	if network == models.Undefined {
		networkStr, err := app.Prompt.CaptureList(
			"Choose a network to validate on (this command only supports public networks)",
			[]string{models.Fuji.String(), models.Mainnet.String()},
		)
		if err != nil {
			return err
		}
		// flag provided
		networkStr = strings.Title(networkStr)
		// as we are allowing a flag, we need to check if a supported network has been provided
		if !(networkStr == models.Fuji.String() || networkStr == models.Mainnet.String()) {
			return errors.New("unsupported network")
		}
		network = models.NetworkFromString(networkStr)
	}

	// used in E2E to simulate public network execution paths on a local network
	if os.Getenv(constants.SimulatePublicNetwork) != "" {
		network = models.Local
	}

	networkLower := strings.ToLower(network.String())

	subnetID := sc.Networks[network.String()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}
	subnetIDStr := subnetID.String()

	if !skipWhitelistCheck {
        yes := true
        if !forceWhitelistCheck {
            ask := "Would you like to check if your node is allowed to join this subnet?\n" +
                "If not, the subnet's control key holder must call avalanche subnet\n" +
                "addValidator with your NodeID."
            ux.Logger.PrintToUser(ask)
            yes, err = app.Prompt.CaptureYesNo("Check whitelist?")
            if err != nil {
                return err
            }
        }
		if yes {
			isValidating, err := isNodeValidatingSubnet(subnetID, network)
			if err != nil {
				return err
			}
			if !isValidating {
                if failIfNotValidating {
                    ux.Logger.PrintToUser("The node is not whitelisted to validate this subnet.")
                    return nil
                }
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
			ux.Logger.PrintToUser("The node is already whitelisted! You are good to go.")
		}
	}

	if printManual {
		pluginDir = app.GetTmpPluginDir()
		vmPath, err := plugins.CreatePlugin(app, sc.Name, pluginDir)
		if err != nil {
			return err
		}
		printJoinCmd(subnetIDStr, networkLower, vmPath)
		return nil
	}

	// if **both** flags were set, nothing special needs to be done
	// just check the following blocks
	if avagoConfigPath == "" && pluginDir == "" {
		// both flags are NOT set
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
			vmPath, err := plugins.CreatePlugin(app, sc.Name, pluginDir)
			if err != nil {
				return err
			}
			printJoinCmd(subnetIDStr, networkLower, vmPath)
			return nil
		}
	}

	// if choice is automatic, we just pass through this block
	// or, pluginDir was set but not avagoConfigPath
	// if **both** flags were set, this will be skipped...
	if avagoConfigPath == "" {
		avagoConfigPath = findAvagoConfigPath()
		if avagoConfigPath != "" {
			ux.Logger.PrintToUser(logging.Bold.Wrap(logging.Green.Wrap("Found a config file at %s")), avagoConfigPath)
			yes, err := app.Prompt.CaptureYesNo("Is this the file we should update?")
			if err != nil {
				return err
			}
			if yes {
				ux.Logger.PrintToUser("Will use file at path %s to update the configuration", avagoConfigPath)
			} else {
				avagoConfigPath = ""
			}
		}
		if avagoConfigPath == "" {
			avagoConfigPath, err = app.Prompt.CaptureString("Path to your existing config file (or where it will be generated)")
			if err != nil {
				return err
			}
		}
	}

	// ...but not this
	avagoConfigPath, err := sanitizePath(avagoConfigPath)
	if err != nil {
		return err
	}

	// avagoConfigPath was set but not pluginDir
	// if **both** flags were set, this will be skipped...
	if pluginDir == "" {
		pluginDir = findPluginDir()
		if pluginDir != "" {
			ux.Logger.PrintToUser(logging.Bold.Wrap(logging.Green.Wrap("Found the VM plugin directory at %s")), pluginDir)
			yes, err := app.Prompt.CaptureYesNo("Is this where we should install the VM?")
			if err != nil {
				return err
			}
			if yes {
				ux.Logger.PrintToUser("Will use plugin directory at %s to install the VM", pluginDir)
			} else {
				pluginDir = ""
			}
		}
		if pluginDir == "" {
			pluginDir, err = app.Prompt.CaptureString("Path to your avalanchego plugin dir (likely avalanchego/build/plugins)")
			if err != nil {
				return err
			}
		}
	}

	// ...but not this
	pluginDir, err := sanitizePath(pluginDir)
	if err != nil {
		return err
	}

	vmPath, err := plugins.CreatePlugin(app, sc.Name, pluginDir)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("VM binary written to %s", vmPath)

	if err := plugins.EditConfigFile(app, subnetIDStr, networkLower, avagoConfigPath, forceWrite); err != nil {
		return err
	}
	return nil
}

func findAvagoConfigPath() string {
	ux.Logger.PrintToUser(logging.Yellow.Wrap("Scanning your system for existing files..."))
	var path string
	// Attempt 1: Try the admin API
	if path = findByRunningProcesses(constants.AvalancheGoRepoName, config.ConfigFileKey); path != "" {
		return path
	}
	// Attempt 2: find looking at some usual dirs
	if path = findByCommonDirs(defaultConfigFileName, scanConfigDirs); path != "" {
		return path
	}
	ux.Logger.PrintToUser(logging.Yellow.Wrap("No config file has been found on your system"))
	return ""
}

func findByCommonDirs(filename string, scanDirs []string) string {
	for _, d := range scanDirs {
		if d == defaultUnexpandedDataDir {
			d = os.ExpandEnv(d)
		}
		path := filepath.Join(d, filename)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func findByRunningProcesses(procName, key string) string {
	procs, err := process.Processes()
	if err != nil {
		return ""
	}
	regex, err := regexp.Compile(procName + ".*" + key)
	if err != nil {
		return ""
	}
	for _, p := range procs {
		name, err := p.Cmdline()
		if err != nil {
			// ignore errors for processes that just died (macos implementation)
			continue
		}
		if regex.MatchString(name) {
			// truncate at end of `--config-file` + 1 (ignores if = or space)
			trunc := name[strings.Index(name, key)+len(key)+1:]
			// there might be other params after the config file entry, so split those away
			// first entry is the value of configFileKey
			return strings.Split(trunc, " ")[0]
		}
	}
	return ""
}

func findPluginDir() string {
	ux.Logger.PrintToUser(logging.Yellow.Wrap("Scanning your system for the plugin directory..."))
	dir := findByCommonDirs(defaultPluginDir, scanConfigDirs)
	if dir != "" {
		return dir
	}
	ux.Logger.PrintToUser(logging.Yellow.Wrap("No plugin directory found on your system"))
	return ""
}

func isNodeValidatingSubnet(subnetID ids.ID, network models.Network) (bool, error) {
    var (
        nodeID ids.NodeID
        err error
    )
    if nodeIDStr != "" {
        ux.Logger.PrintToUser("Next, we need the NodeID of the validator you want to whitelist.")
        ux.Logger.PrintToUser("")
        ux.Logger.PrintToUser("Check https://docs.avax.network/apis/avalanchego/apis/info#infogetnodeid for instructions about how to query the NodeID from your node")
        ux.Logger.PrintToUser("(Edit host IP address and port to match your deployment, if needed).")

        promptStr := "What is the NodeID of the validator you'd like to whitelist?"
        nodeID, err = app.Prompt.CaptureNodeID(promptStr)
        if err != nil {
            return false, err
        }
    } else {
		nodeID, err = ids.NodeIDFromString(nodeIDStr)
		if err != nil {
			return false, err
		}
    }

	var api string
	switch network {
	case models.Fuji:
		api = constants.FujiAPIEndpoint
	case models.Mainnet:
		api = constants.MainnetAPIEndpoint
	case models.Local:
		api = constants.LocalAPIEndpoint
	default:
		return false, fmt.Errorf("network not supported")
	}

	pClient := platformvm.NewClient(api)

	return checkIsValidating(subnetID, nodeID, pClient)
}

func checkIsValidating(subnetID ids.ID, nodeID ids.NodeID, pClient platformvm.Client) (bool, error) {
	// first check if the node is already an accepted validator on the subnet
	ctx := context.Background()
	nodeIDs := []ids.NodeID{nodeID}
	vals, err := pClient.GetCurrentValidators(ctx, subnetID, nodeIDs)
	if err != nil {
		return false, err
	}
	for _, v := range vals {
		// strictly this is not needed, as we are providing the nodeID as param
		// just a double check
		if v.NodeID == nodeID {
			return true, nil
		}
	}

	// if not, also check the pending validator set
	pVals, _, err := pClient.GetPendingValidators(ctx, subnetID, nodeIDs)
	if err != nil {
		return false, err
	}
	// pVals is an array of interfaces as it can be of different types
	// but it's content is a JSON map[string]interface{}
	for _, iv := range pVals {
		if v, ok := iv.(map[string]interface{}); ok {
			// strictly this is not needed, as we are providing the nodeID as param
			// just a double check
			if v["nodeID"] == nodeID.String() {
				return true, nil
			}
		}
	}

	return false, nil
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

func sanitizePath(path string) (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	homeDir := usr.HomeDir
	if path == "~" {
		// In case of "~", which won't be caught by the "else if"
		path = homeDir
	} else if strings.HasPrefix(path, "~/") {
		path = filepath.Join(homeDir, path[2:])
	}
	return path, nil
}
