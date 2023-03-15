// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	ANRclient "github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/server"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/precompile/contracts/txallowlist"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const (
	timestampFormat  = "20060102150405"
	tmpSnapshotInfix = "-tmp-"
)

var (
	ErrNetworkNotStartedOutput = "No local network running. Please start the network first."
	ErrSubnetNotDeployedOutput = "Looks like this subnet has not been deployed to this network yet."

	errSubnetNotYetDeployed       = errors.New("subnet not yet deployed")
	errInvalidPrecompiles         = errors.New("invalid precompiles")
	errNoBlockTimestamp           = errors.New("no blockTimestamp value set")
	errBlockTimestampInvalid      = errors.New("blockTimestamp is invalid")
	errNoPrecompiles              = errors.New("no precompiles present")
	errNoUpcomingUpgrades         = errors.New("no valid upcoming activation timestamp found")
	errNewUpgradesNotContainsLock = errors.New("the new upgrade file does not contain the content of the lock file")

	errUserAborted = errors.New("user aborted")

	avalanchegoChainConfigDirDefault = filepath.Join("$HOME", ".avalanchego", "chains")
	avalanchegoChainConfigFlag       = "avalanchego-chain-config-dir"
	avalanchegoChainConfigDir        string

	print bool
)

// avalanche subnet upgrade apply
func newUpgradeApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply [subnetName]",
		Short: "Apply upgrade bytes onto subnet nodes",
		Long: `Apply generated upgrade bytes to running subnet nodes to trigger a network upgrade. 

For public networks (fuji testnet or mainnet), to complete this process, 
you must have access to the machine running your validator.
If the CLI is running on the same machine as your validator, it can manipulate your node's
configuration automatically. Alternatively, the command can print the necessary instructions
to upgrade your node manually.

After you update your validator's configuration, you need to restart your validator manually. 
If you provide the --avalanchego-chain-config-dir flag, this command attempts to write the upgrade file at that path.
Refer to https://docs.avax.network/nodes/maintain/chain-config-flags#subnet-chain-configs for related documentation.`,
		RunE: applyCmd,
		Args: cobra.ExactArgs(1),
	}

	cmd.Flags().BoolVar(&useConfig, "config", false, "create upgrade config for future subnet deployments (same as generate)")
	cmd.Flags().BoolVar(&useLocal, "local", false, "apply upgrade existing `local` deployment")
	cmd.Flags().BoolVar(&useFuji, "fuji", false, "apply upgrade existing `fuji` deployment (alias for `testnet`)")
	cmd.Flags().BoolVar(&useFuji, "testnet", false, "apply upgrade existing `testnet` deployment (alias for `fuji`)")
	cmd.Flags().BoolVar(&useMainnet, "mainnet", false, "apply upgrade existing `mainnet` deployment")
	cmd.Flags().BoolVar(&print, "print", false, "if true, print the manual config without prompting (for public networks only)")
	cmd.Flags().BoolVar(&force, "force", false, "If true, don't prompt for confirmation of timestamps in the past")
	cmd.Flags().StringVar(&avalanchegoChainConfigDir, avalanchegoChainConfigFlag, os.ExpandEnv(avalanchegoChainConfigDirDefault), "avalanchego's chain config file directory")

	return cmd
}

func applyCmd(_ *cobra.Command, args []string) error {
	subnetName := args[0]

	if !app.SubnetConfigExists(subnetName) {
		return errors.New("subnet does not exist")
	}

	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return fmt.Errorf("unable to load sidecar: %w", err)
	}

	networkToUpgrade, err := selectNetworkToUpgrade(sc, []string{})
	if err != nil {
		return err
	}

	switch networkToUpgrade {
	// update a locally running network
	case localDeployment:
		return applyLocalNetworkUpgrade(subnetName, models.Local.String(), &sc)
	case fujiDeployment:
		return applyPublicNetworkUpgrade(subnetName, models.Fuji.String(), &sc)
	case mainnetDeployment:
		return applyPublicNetworkUpgrade(subnetName, models.Mainnet.String(), &sc)
	}

	return nil
}

// applyLocalNetworkUpgrade:
// * if subnet NOT deployed (`network status`):
// *   Stop the apply command and print a message suggesting to deploy first
// * if subnet deployed:
// *   if never upgraded before, apply
// *   if upgraded before, and this upgrade contains the same upgrade as before (.lock)
// *     if has new valid upgrade on top, apply
// *     if the same, print info and do nothing
// *   if upgraded before, but this upgrade is not cumulative (append-only)
// *     fail the apply, print message

// For a already deployed subnet, the supported scheme is to
// save a snapshot, and to load the snapshot with the upgrade
func applyLocalNetworkUpgrade(subnetName, networkKey string, sc *models.Sidecar) error {
	if print {
		ux.Logger.PrintToUser("The --print flag is ignored on local networks. Continuing.")
	}
	precmpUpgrades, strNetUpgrades, err := validateUpgrade(subnetName, networkKey, sc, force)
	if err != nil {
		return err
	}

	cli, err := binutils.NewGRPCClient()
	if err != nil {
		ux.Logger.PrintToUser(ErrNetworkNotStartedOutput)
		return err
	}
	ctx := binutils.GetAsyncContext()

	// first let's get the status
	status, err := cli.Status(ctx)
	if err != nil {
		if server.IsServerError(err, server.ErrNotBootstrapped) {
			ux.Logger.PrintToUser(ErrNetworkNotStartedOutput)
			return err
		}
		return err
	}

	// confirm in the status that the subnet actually is deployed and running
	deployed := false
	subnets := status.ClusterInfo.GetSubnets()
	for _, s := range subnets {
		if s == sc.Networks[networkKey].SubnetID.String() {
			deployed = true
			break
		}
	}

	if !deployed {
		return subnetNotYetDeployed()
	}

	// get the blockchainID from the sidecar
	blockchainID := sc.Networks[networkKey].BlockchainID
	if blockchainID == ids.Empty {
		return errors.New(
			"failed to find deployment information about this subnet in state - aborting")
	}

	// save a temporary snapshot
	snapName := subnetName + tmpSnapshotInfix + time.Now().Format(timestampFormat)
	app.Log.Debug("saving temporary snapshot for upgrade bytes", zap.String("snapshot-name", snapName))
	_, err = cli.SaveSnapshot(ctx, snapName)
	if err != nil {
		return err
	}
	app.Log.Debug(
		"network stopped and named temporary snapshot created. Now starting the network with given snapshot")

	netUpgradeConfs := map[string]string{
		blockchainID.String(): strNetUpgrades,
	}
	// restart the network setting the upgrade bytes file
	opts := ANRclient.WithUpgradeConfigs(netUpgradeConfs)
	_, err = cli.LoadSnapshot(ctx, snapName, opts)
	if err != nil {
		return err
	}

	clusterInfo, err := subnet.WaitForHealthy(ctx, cli)
	if err != nil {
		return fmt.Errorf("failed waiting for network to become healthy: %w", err)
	}

	fmt.Println()
	if subnet.HasEndpoints(clusterInfo) {
		ux.Logger.PrintToUser("Network restarted and ready to use. Upgrade bytes have been applied to running nodes at these endpoints.")

		nextUpgrade, err := getEarliestUpcomingTimestamp(precmpUpgrades)
		// this should not happen anymore at this point...
		if err != nil {
			app.Log.Warn("looks like the upgrade went well, but we failed getting the timestamp of the next upcoming upgrade: %w")
		}
		ux.Logger.PrintToUser("The next upgrade will go into effect %s", time.Unix(nextUpgrade, 0).Local().Format(constants.TimeParseLayout))
		ux.PrintTableEndpoints(clusterInfo)

		return writeLockFile(precmpUpgrades, subnetName)
	}

	return errors.New("unexpected network size of zero nodes")
}

// applyPublicNetworkUpgrade applies an upgrade file to a locally running validator
// for public networks (fuji, main)
// the validation of the upgrade file has many things to consider:
// * No upgrade file for <public net> can be found - do we copy the existing file in the prev stage?
// (for fuji: take the local, for main, take the fuji?)?
// * If not, we exit, but then force the user to create a fuji file? Can be quite annoying!
// * Do we validate that the fuji file is the same as local before applying? Or we just take whatever is there?
// For main, that it's the same as fuji and/or local or take whatever is there?
// * What if the local deployment has applied different stages of upgrades,
// but they were for development only and fuji/main is going to be different (start from scratch)?
// * What if someone isn't even doing local, just fuji and main...(or even just main...we may want to discourage that though...)
// * User probably would never use the exact same file for local as for Fuji, because you’d probably want to change the timestamps
//
// For public networks we therefore limit ourselves to just "apply" the upgrades
// This also means we are *ignoring* the lock file here!
func applyPublicNetworkUpgrade(subnetName, networkKey string, sc *models.Sidecar) error {
	if print {
		blockchainIDstr := "<your-blockchain-id>"
		if sc.Networks != nil &&
			sc.Networks[networkKey] != (models.NetworkData{}) &&
			sc.Networks[networkKey].BlockchainID != ids.Empty {
			blockchainIDstr = sc.Networks[networkKey].BlockchainID.String()
		}
		ux.Logger.PrintToUser("To install the upgrade file on your validator:")
		fmt.Println()
		ux.Logger.PrintToUser("1. Identify where your validator has the avalanchego chain config dir configured.")
		ux.Logger.PrintToUser("   The default is at $HOME/.avalanchego/chains (%s on this machine).", os.ExpandEnv(avalanchegoChainConfigDirDefault))
		ux.Logger.PrintToUser("   If you are using a different chain config dir for your node, use that one.")
		ux.Logger.PrintToUser("2. Create a directory with the blockchainID in the configured chain-config-dir (e.g. $HOME/.avalanchego/chains/%s) if doesn't already exist.", blockchainIDstr)
		ux.Logger.PrintToUser("3. Create an `upgrade.json` file in the blockchain directory with the content of your upgrade file.")
		upgr, err := app.ReadUpgradeFile(subnetName)
		if err == nil {
			var prettyJSON bytes.Buffer
			if err := json.Indent(&prettyJSON, upgr, "", "    "); err == nil {
				ux.Logger.PrintToUser("   This is the content of your upgrade file as configured in this tool:")
				fmt.Println(prettyJSON.String())
			}
		}
		fmt.Println()
		ux.Logger.PrintToUser("   *************************************************************************************************************")
		ux.Logger.PrintToUser("   * Upgrades are tricky. The syntactic correctness of the upgrade file is important.                          *")
		ux.Logger.PrintToUser("   * The sequence of upgrades must be strictly observed.                                                       *")
		ux.Logger.PrintToUser("   * Make sure you understand https://docs.avax.network/nodes/maintain/chain-config-flags#subnet-chain-configs *")
		ux.Logger.PrintToUser("   * before applying upgrades manually.                                                                        *")
		ux.Logger.PrintToUser("   *************************************************************************************************************")
		return nil
	}
	_, _, err := validateUpgrade(subnetName, networkKey, sc, force)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("The chain config dir avalanchego uses is set at %s", avalanchegoChainConfigDir)
	// give the user the chance to check if they indeed want to use the default
	if avalanchegoChainConfigDir == avalanchegoChainConfigDirDefault {
		useDefault, err := app.Prompt.CaptureYesNo("It is set to the default. Is that correct?")
		if err != nil {
			return err
		}
		if !useDefault {
			avalanchegoChainConfigDir, err = app.Prompt.CaptureExistingFilepath(
				"Enter the path to your custom chain config dir (*without* the blockchain ID, e.g /my/configs/dir)")
			if err != nil {
				return err
			}
		}
	}

	ux.Logger.PrintToUser("Trying to install the upgrade files at the provided %s path", avalanchegoChainConfigDir)
	chainDir := filepath.Join(avalanchegoChainConfigDir, sc.Networks[networkKey].BlockchainID.String())
	destPath := filepath.Join(chainDir, constants.UpgradeBytesFileName)
	if err = os.Mkdir(chainDir, constants.DefaultPerms755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create blockchain directory: %w", err)
	}

	if err := binutils.CopyFile(app.GetUpgradeBytesFilePath(subnetName), destPath); err != nil {
		return fmt.Errorf("failed to install the upgrades path at the provided destination: %w", err)
	}
	ux.Logger.PrintToUser("Successfully installed upgrade file")
	return nil
}

func validateUpgrade(subnetName, networkKey string, sc *models.Sidecar, skipPrompting bool) ([]params.PrecompileUpgrade, string, error) {
	// if there's no entry in the Sidecar, we assume there hasn't been a deploy yet
	if sc.Networks[networkKey] == (models.NetworkData{}) {
		return nil, "", subnetNotYetDeployed()
	}
	chainID := sc.Networks[networkKey].BlockchainID
	if chainID == ids.Empty {
		return nil, "", errors.New(ErrSubnetNotDeployedOutput)
	}
	// let's check update bytes actually exist
	netUpgradeBytes, err := app.ReadUpgradeFile(subnetName)
	if err != nil {
		if err == os.ErrNotExist {
			ux.Logger.PrintToUser("No file with upgrade specs for the given subnet has been found")
			ux.Logger.PrintToUser("You may need to first create it with the `avalanche subnet upgrade generate` command or import it")
			ux.Logger.PrintToUser("Aborting this command. No changes applied")
		}
		return nil, "", err
	}

	// read the lock file right away
	lockUpgradeBytes, err := app.ReadLockUpgradeFile(subnetName)
	if err != nil {
		// if the file doesn't exist, that's ok
		if !os.IsNotExist(err) {
			return nil, "", err
		}
	}

	// validate the upgrade bytes files
	upgrds, err := validateUpgradeBytes(netUpgradeBytes, lockUpgradeBytes, skipPrompting)
	if err != nil {
		return nil, "", err
	}

	// checks that adminAddress in precompile upgrade for TxAllowList has enough token balance
	for _, precmpUpgrade := range upgrds {
		allowListCfg, ok := precmpUpgrade.Config.(*txallowlist.Config)
		if !ok {
			return nil, "", fmt.Errorf("expected txallowlist.Config, got %T", allowListCfg)
		}
		if allowListCfg != nil {
			if err := ensureAdminsHaveBalance(allowListCfg.AdminAddresses, subnetName); err != nil {
				return nil, "", err
			}
		}
	}
	return upgrds, string(netUpgradeBytes), nil
}

func subnetNotYetDeployed() error {
	ux.Logger.PrintToUser(ErrSubnetNotDeployedOutput)
	ux.Logger.PrintToUser("Please deploy this network first.")
	return errSubnetNotYetDeployed
}

func writeLockFile(precmpUpgrades []params.PrecompileUpgrade, subnetName string) error {
	// it seems all went well this far, now we try to write/update the lock file
	// if this fails, we probably don't want to cause an error to the user?
	// so we are silently failing, just write a log entry
	wrapper := params.UpgradeConfig{
		PrecompileUpgrades: precmpUpgrades,
	}
	jsonBytes, err := json.Marshal(wrapper)
	if err != nil {
		app.Log.Debug("failed to marshaling upgrades lock file content", zap.Error(err))
	}
	if err := app.WriteLockUpgradeFile(subnetName, jsonBytes); err != nil {
		app.Log.Debug("failed to write upgrades lock file", zap.Error(err))
	}

	return nil
}

func validateUpgradeBytes(file, lockFile []byte, skipPrompting bool) ([]params.PrecompileUpgrade, error) {
	upgrades, err := getAllUpgrades(file)
	if err != nil {
		return nil, err
	}

	if len(lockFile) > 0 {
		lockUpgrades, err := getAllUpgrades(lockFile)
		if err != nil {
			return nil, err
		}
		match := 0
		for _, lu := range lockUpgrades {
			for _, u := range upgrades {
				if reflect.DeepEqual(u, lu) {
					match++
					break
				}
			}
		}
		if match != len(lockUpgrades) {
			return nil, errNewUpgradesNotContainsLock
		}
	}

	allTimestamps, err := getAllTimestamps(upgrades)
	if err != nil {
		return nil, err
	}

	if !skipPrompting {
		for _, ts := range allTimestamps {
			if time.Unix(ts, 0).Before(time.Now()) {
				ux.Logger.PrintToUser("Warning: one or more of your upgrades is set to happen in the past.")
				ux.Logger.PrintToUser(
					"If you've already upgraded your network, the configuration is likely correct and will not cause problems.")
				ux.Logger.PrintToUser(
					"If this is a new upgrade, this configuration could cause unpredictable behavior and irrecoverable damage to your Subnet.")
				ux.Logger.PrintToUser(
					"The config MUST be removed. Use caution before proceeding")
				yes, err := app.Prompt.CaptureYesNo("Do you want to continue (use --force to skip prompting)?")
				if err != nil {
					return nil, err
				}
				if !yes {
					ux.Logger.PrintToUser("No selected.")
					return nil, errUserAborted
				}
			}
		}
	}

	return upgrades, nil
}

func getAllTimestamps(upgrades []params.PrecompileUpgrade) ([]int64, error) {
	allTimestamps := []int64{}

	if len(upgrades) == 0 {
		return nil, errNoBlockTimestamp
	}
	for _, upgrade := range upgrades {
		ts, err := validateTimestamp(upgrade.Timestamp())
		if err != nil {
			return nil, err
		}
		allTimestamps = append(allTimestamps, ts)
	}
	if len(allTimestamps) == 0 {
		return nil, errNoBlockTimestamp
	}
	return allTimestamps, nil
}

func validateTimestamp(ts *big.Int) (int64, error) {
	if ts == nil {
		return 0, errNoBlockTimestamp
	}
	if !ts.IsInt64() {
		return 0, errBlockTimestampInvalid
	}
	val := ts.Int64()
	if val == int64(0) {
		return 0, errBlockTimestampInvalid
	}
	return val, nil
}

func getEarliestUpcomingTimestamp(upgrades []params.PrecompileUpgrade) (int64, error) {
	allTimestamps, err := getAllTimestamps(upgrades)
	if err != nil {
		return 0, err
	}

	earliest := int64(math.MaxInt64)

	for _, ts := range allTimestamps {
		// we may also not necessarily need to check
		// if after now, but to know if something is upcoming,
		// seems appropriate
		if ts < earliest && time.Unix(ts, 0).After(time.Now()) {
			earliest = ts
		}
	}

	// this should not happen as we have timestamp validation
	// but might be required if called in a different context
	if earliest == math.MaxInt64 {
		return earliest, errNoUpcomingUpgrades
	}

	return earliest, nil
}

func getAllUpgrades(file []byte) ([]params.PrecompileUpgrade, error) {
	var precompiles params.UpgradeConfig

	if err := json.Unmarshal(file, &precompiles); err != nil {
		cause := fmt.Errorf(err.Error(), errInvalidPrecompiles)
		return nil, fmt.Errorf("failed parsing JSON : %w", cause)
	}

	if len(precompiles.PrecompileUpgrades) == 0 {
		return nil, errNoPrecompiles
	}

	return precompiles.PrecompileUpgrades, nil
}
