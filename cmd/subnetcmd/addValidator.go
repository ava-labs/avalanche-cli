// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/spf13/cobra"
)

var (
	nodeIDStr              string
	weight                 uint64
	startTimeStr           string
	duration               time.Duration
	defaultValidatorParams bool
	useDefaultStartTime    bool
	useDefaultDuration     bool
	useDefaultWeight       bool

	errNoSubnetID                       = errors.New("failed to find the subnet ID for this subnet, has it been deployed/created on this network?")
	errMutuallyExclusiveDurationOptions = errors.New("--use-default-duration/--use-default-validator-params and --staking-period are mutually exclusive")
	errMutuallyExclusiveStartOptions    = errors.New("--use-default-start-time/--use-default-validator-params and --start-time are mutually exclusive")
	errMutuallyExclusiveWeightOptions   = errors.New("--use-default-validator-params and --weight are mutually exclusive")
)

// avalanche subnet deploy
func newAddValidatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addValidator [subnetName]",
		Short: "Allow a validator to validate your subnet",
		Long: `The subnet addValidator command whitelists a primary network validator to
validate the provided deployed Subnet.

To add the validator to the Subnet's allow list, you first need to provide
the subnetName and the validator's unique NodeID. The command then prompts
for the validation start time, duration, and stake weight. You can bypass
these prompts by providing the values with flags.

This command currently only works on Subnets deployed to either the Fuji
Testnet or Mainnet.`,
		SilenceUsage: true,
		RunE:         addValidator,
		Args:         cobra.ExactArgs(1),
	}
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet only]")
	cmd.Flags().StringVar(&nodeIDStr, "nodeID", "", "set the NodeID of the validator to add")
	cmd.Flags().Uint64Var(&weight, "weight", 0, "set the staking weight of the validator to add")

	cmd.Flags().BoolVar(&useDefaultStartTime, "default-start-time", false, "use default start time for subnet validator (5 minutes later for fuji & mainnet, 30 seconds later for devnet)")
	cmd.Flags().StringVar(&startTimeStr, "start-time", "", "UTC start time when this validator starts validating, in 'YYYY-MM-DD HH:MM:SS' format")

	cmd.Flags().BoolVar(&useDefaultDuration, "default-duration", false, "set duration so as to validate until primary validator ends its period")
	cmd.Flags().DurationVar(&duration, "staking-period", 0, "how long this validator will be staking")

	cmd.Flags().BoolVar(&defaultValidatorParams, "default-validator-params", false, "use default weight/start/duration params for subnet validator")

	cmd.Flags().StringVar(&endpoint, "endpoint", "", "use the given endpoint for network operations")
	cmd.Flags().BoolVar(&deployLocal, "local", false, "add subnet validator on `local`")
	cmd.Flags().BoolVar(&deployDevnet, "devnet", false, "add subnet validator on `devnet`")
	cmd.Flags().BoolVar(&deployTestnet, "fuji", false, "add subnet validator on `fuji` (alias for `testnet`)")
	cmd.Flags().BoolVar(&deployTestnet, "testnet", false, "add subnet validator on `testnet` (alias for `fuji`)")
	cmd.Flags().BoolVar(&deployMainnet, "mainnet", false, "add subnet validator on `mainnet`")
	cmd.Flags().StringSliceVar(&subnetAuthKeys, "subnet-auth-keys", nil, "control keys that will be used to authenticate add validator tx")
	cmd.Flags().StringVar(&outputTxPath, "output-tx-path", "", "file path of the add validator tx")
	cmd.Flags().BoolVarP(&useEwoq, "ewoq", "e", false, "use ewoq key [fuji/devnet only]")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	return cmd
}

func addValidator(_ *cobra.Command, args []string) error {
	network, err := GetNetworkFromCmdLineFlags(
		deployLocal,
		deployDevnet,
		deployTestnet,
		deployMainnet,
		endpoint,
		true,
		[]models.NetworkKind{models.Local, models.Devnet, models.Fuji, models.Mainnet},
	)
	if err != nil {
		return err
	}
	fee := network.GenesisParams().AddSubnetValidatorFee
	kc, err := keychain.GetKeychainFromCmdLineFlags(
		app,
		constants.PayTxsFeesMsg,
		network,
		keyName,
		useEwoq,
		useLedger,
		ledgerAddresses,
		fee,
	)
	if err != nil {
		return err
	}
	network.HandlePublicNetworkSimulation()
	return CallAddValidator(network, kc, useLedger, args[0], nodeIDStr, defaultValidatorParams)
}

func CallAddValidator(
	network models.Network,
	kc *keychain.Keychain,
	useLedgerSetting bool,
	subnetName string,
	nodeIDStr string,
	defaultValidatorParamsSetting bool,
) error {
	var (
		nodeID ids.NodeID
		start  time.Time
		err    error
	)

	useLedger = useLedgerSetting
	defaultValidatorParams = defaultValidatorParamsSetting

	if defaultValidatorParams {
		useDefaultDuration = true
		useDefaultStartTime = true
		useDefaultWeight = true
	}

	if useDefaultDuration && duration != 0 {
		return errMutuallyExclusiveDurationOptions
	}
	if useDefaultStartTime && startTimeStr != "" {
		return errMutuallyExclusiveStartOptions
	}
	if useDefaultWeight && weight != 0 {
		return errMutuallyExclusiveWeightOptions
	}

	if outputTxPath != "" {
		if utils.FileExists(outputTxPath) {
			return fmt.Errorf("outputTxPath %q already exists", outputTxPath)
		}
	}

	_, err = ValidateSubnetNameAndGetChains([]string{subnetName})
	if err != nil {
		return err
	}

	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}

	subnetID := sc.Networks[network.Name()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	controlKeys, threshold, err := txutils.GetOwners(network, subnetID)
	if err != nil {
		return err
	}

	// add control keys to the keychain whenever possible
	if err := kc.AddAddresses(controlKeys); err != nil {
		return err
	}

	kcKeys, err := kc.PChainFormattedStrAddresses()
	if err != nil {
		return err
	}

	// get keys for add validator tx signing
	if subnetAuthKeys != nil {
		if err := prompts.CheckSubnetAuthKeys(kcKeys, subnetAuthKeys, controlKeys, threshold); err != nil {
			return err
		}
	} else {
		subnetAuthKeys, err = prompts.GetSubnetAuthKeys(app.Prompt, kcKeys, controlKeys, threshold)
		if err != nil {
			return err
		}
	}
	ux.Logger.PrintToUser("Your subnet auth keys for add validator tx creation: %s", subnetAuthKeys)

	if nodeIDStr == "" {
		nodeID, err = PromptNodeID()
		if err != nil {
			return err
		}
	} else {
		nodeID, err = ids.NodeIDFromString(nodeIDStr)
		if err != nil {
			return err
		}
	}

	selectedWeight, err := getWeight()
	if err != nil {
		return err
	}
	if selectedWeight < constants.MinStakeWeight {
		return fmt.Errorf("illegal weight, must be greater than or equal to %d: %d", constants.MinStakeWeight, selectedWeight)
	}

	start, selectedDuration, err := getTimeParameters(network, nodeID, true)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("NodeID: %s", nodeID.String())
	ux.Logger.PrintToUser("Network: %s", network.Name())
	ux.Logger.PrintToUser("Start time: %s", start.Format(constants.TimeParseLayout))
	ux.Logger.PrintToUser("End time: %s", start.Add(selectedDuration).Format(constants.TimeParseLayout))
	ux.Logger.PrintToUser("Weight: %d", selectedWeight)
	ux.Logger.PrintToUser("Inputs complete, issuing transaction to add the provided validator information...")

	deployer := subnet.NewPublicDeployer(app, kc, network)
	isFullySigned, tx, remainingSubnetAuthKeys, err := deployer.AddValidator(controlKeys, subnetAuthKeys, subnetID, nodeID, selectedWeight, start, selectedDuration)
	if err != nil {
		return err
	}
	if !isFullySigned {
		if err := SaveNotFullySignedTx(
			"Add Validator",
			tx,
			subnetName,
			subnetAuthKeys,
			remainingSubnetAuthKeys,
			outputTxPath,
			false,
		); err != nil {
			return err
		}
	}

	return err
}

func PromptDuration(start time.Time, network models.Network) (time.Duration, error) {
	for {
		txt := "How long should this validator be validating? Enter a duration, e.g. 8760h. Valid time units are \"ns\", \"us\" (or \"µs\"), \"ms\", \"s\", \"m\", \"h\""
		var d time.Duration
		var err error
		if network.Kind == models.Fuji {
			d, err = app.Prompt.CaptureFujiDuration(txt)
		} else {
			d, err = app.Prompt.CaptureMainnetDuration(txt)
		}
		if err != nil {
			return 0, err
		}
		end := start.Add(d)
		confirm := fmt.Sprintf("Your validator will finish staking by %s", end.Format(constants.TimeParseLayout))
		yes, err := app.Prompt.CaptureYesNo(confirm)
		if err != nil {
			return 0, err
		}
		if yes {
			return d, nil
		}
	}
}

func getMaxValidationTime(network models.Network, nodeID ids.NodeID, startTime time.Time) (time.Duration, error) {
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	platformCli := platformvm.NewClient(network.Endpoint)
	vs, err := platformCli.GetCurrentValidators(ctx, avagoconstants.PrimaryNetworkID, nil)
	cancel()
	if err != nil {
		return 0, err
	}
	for _, v := range vs {
		if v.NodeID == nodeID {
			return time.Unix(int64(v.EndTime), 0).Sub(startTime), nil
		}
	}
	return 0, errors.New("nodeID not found in validator set: " + nodeID.String())
}

func getTimeParameters(network models.Network, nodeID ids.NodeID, isValidator bool) (time.Time, time.Duration, error) {
	defaultStakingStartLeadTime := constants.StakingStartLeadTime
	if network.Kind == models.Devnet {
		defaultStakingStartLeadTime = constants.DevnetStakingStartLeadTime
	}

	const custom = "Custom"

	// this sets either the global var startTimeStr or useDefaultStartTime to enable repeated execution with
	// state keeping from node cmds
	if startTimeStr == "" && !useDefaultStartTime {
		if isValidator {
			ux.Logger.PrintToUser("When should your validator start validating?\n" +
				"If you validator is not ready by this time, subnet downtime can occur.")
		} else {
			ux.Logger.PrintToUser("When do you want to start delegating?\n")
		}
		defaultStartOption := "Start in " + ux.FormatDuration(defaultStakingStartLeadTime)
		startTimeOptions := []string{defaultStartOption, custom}
		startTimeOption, err := app.Prompt.CaptureList("Start time", startTimeOptions)
		if err != nil {
			return time.Time{}, 0, err
		}
		switch startTimeOption {
		case defaultStartOption:
			useDefaultStartTime = true
		default:
			start, err := promptStart()
			if err != nil {
				return time.Time{}, 0, err
			}
			startTimeStr = start.Format(constants.TimeParseLayout)
		}
	}

	var (
		err   error
		start time.Time
	)
	if startTimeStr != "" {
		start, err = time.Parse(constants.TimeParseLayout, startTimeStr)
		if err != nil {
			return time.Time{}, 0, err
		}
		if start.Before(time.Now().Add(constants.StakingMinimumLeadTime)) {
			return time.Time{}, 0, fmt.Errorf("time should be at least %s in the future ", constants.StakingMinimumLeadTime)
		}
	} else {
		start = time.Now().Add(defaultStakingStartLeadTime)
	}

	// this sets either the global var duration or useDefaultDuration to enable repeated execution with
	// state keeping from node cmds
	if duration == 0 && !useDefaultDuration {
		msg := "How long should your validator validate for?"
		if !isValidator {
			msg = "How long do you want to delegate for?"
		}
		const defaultDurationOption = "Until primary network validator expires"
		durationOptions := []string{defaultDurationOption, custom}
		durationOption, err := app.Prompt.CaptureList(msg, durationOptions)
		if err != nil {
			return time.Time{}, 0, err
		}
		switch durationOption {
		case defaultDurationOption:
			useDefaultDuration = true
		default:
			duration, err = PromptDuration(start, network)
			if err != nil {
				return time.Time{}, 0, err
			}
		}
	}

	var selectedDuration time.Duration
	if useDefaultDuration {
		// avoid setting both globals useDefaultDuration and duration
		selectedDuration, err = getMaxValidationTime(network, nodeID, start)
		if err != nil {
			return time.Time{}, 0, err
		}
	} else {
		selectedDuration = duration
	}

	return start, selectedDuration, nil
}

func promptStart() (time.Time, error) {
	txt := "When should the validator start validating? Enter a UTC datetime in 'YYYY-MM-DD HH:MM:SS' format"
	return app.Prompt.CaptureDate(txt)
}

func PromptNodeID() (ids.NodeID, error) {
	ux.Logger.PrintToUser("Next, we need the NodeID of the validator you want to whitelist.")
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Check https://docs.avax.network/apis/avalanchego/apis/info#infogetnodeid for instructions about how to query the NodeID from your node")
	ux.Logger.PrintToUser("(Edit host IP address and port to match your deployment, if needed).")

	txt := "What is the NodeID of the validator you'd like to whitelist?"
	return app.Prompt.CaptureNodeID(txt)
}

func getWeight() (uint64, error) {
	// this sets either the global var weight or useDefaultWeight to enable repeated execution with
	// state keeping from node cmds
	if weight == 0 && !useDefaultWeight {
		defaultWeight := fmt.Sprintf("Default (%d)", constants.DefaultStakeWeight)
		txt := "What stake weight would you like to assign to the validator?"
		weightOptions := []string{defaultWeight, "Custom"}
		weightOption, err := app.Prompt.CaptureList(txt, weightOptions)
		if err != nil {
			return 0, err
		}
		switch weightOption {
		case defaultWeight:
			useDefaultWeight = true
		default:
			weight, err = app.Prompt.CaptureWeight(txt)
			if err != nil {
				return 0, err
			}
		}
	}
	if useDefaultWeight {
		return constants.DefaultStakeWeight, nil
	}
	return weight, nil
}
