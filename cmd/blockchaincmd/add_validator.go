// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	"github.com/ava-labs/avalanchego/ids"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	warpMessage "github.com/ava-labs/avalanchego/vms/platformvm/warp/message"
	"github.com/spf13/cobra"
)

var (
	addValidatorSupportedNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Devnet,
		networkoptions.Fuji,
		networkoptions.Mainnet,
	}

	nodeIDStr                 string
	nodeEndpoint              string
	balance                   uint64
	weight                    uint64
	startTimeStr              string
	duration                  time.Duration
	defaultValidatorParams    bool
	useDefaultStartTime       bool
	useDefaultDuration        bool
	useDefaultWeight          bool
	waitForTxAcceptance       bool
	publicKey                 string
	pop                       string
	remainingBalanceOwnerAddr string
	disableOwnerAddr          string
	rpcURL                    string
	aggregatorLogLevel        string

	errNoSubnetID                       = errors.New("failed to find the subnet ID for this subnet, has it been deployed/created on this network?")
	errMutuallyExclusiveDurationOptions = errors.New("--use-default-duration/--use-default-validator-params and --staking-period are mutually exclusive")
	errMutuallyExclusiveStartOptions    = errors.New("--use-default-start-time/--use-default-validator-params and --start-time are mutually exclusive")
	errMutuallyExclusiveWeightOptions   = errors.New("--use-default-validator-params and --weight are mutually exclusive")
	ErrNotPermissionedSubnet            = errors.New("subnet is not permissioned")
	aggregatorExtraEndpoints            []string
)

// avalanche blockchain addValidator
func newAddValidatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addValidator [blockchainName]",
		Short: "Allow a validator to validate your blockchain's subnet",
		Long: `The blockchain addValidator command whitelists a primary network validator to
validate the subnet of the provided deployed Blockchain.

To add the validator to the Subnet's allow list, you first need to provide
the blockchainName and the validator's unique NodeID. The command then prompts
for the validation start time, duration, and stake weight. You can bypass
these prompts by providing the values with flags.

This command currently only works on Blockchains deployed to either the Fuji
Testnet or Mainnet.`,
		RunE: addValidator,
		Args: cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, addValidatorSupportedNetworkOptions)

	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet only]")
	cmd.Flags().Uint64Var(&weight, "weight", constants.NonBootstrapValidatorWeight, "set the staking weight of the validator to add")
	cmd.Flags().Uint64Var(&balance, "balance", 0, "set the AVAX balance of the validator that will be used for continuous fee to P-Chain")
	cmd.Flags().BoolVarP(&useEwoq, "ewoq", "e", false, "use ewoq key [fuji/devnet only]")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	cmd.Flags().StringVar(&nodeIDStr, "node-id", "", "node-id of the validator to add")
	cmd.Flags().StringVar(&publicKey, "bls-public-key", "", "set the BLS public key of the validator to add")
	cmd.Flags().StringVar(&pop, "bls-proof-of-possession", "", "set the BLS proof of possession of the validator to add")
	cmd.Flags().StringVar(&remainingBalanceOwnerAddr, "remaining-balance-owner", "", "P-Chain address that will receive any leftover AVAX from the validator when it is removed from Subnet")
	cmd.Flags().StringVar(&disableOwnerAddr, "disable-owner", "", "P-Chain address that will able to disable the validator with a P-Chain transaction")
	cmd.Flags().StringVar(&nodeEndpoint, "node-endpoint", "", "gather node id/bls from publicly available avalanchego apis on the given endpoint")
	cmd.Flags().StringSliceVar(&aggregatorExtraEndpoints, "aggregator-extra-endpoints", nil, "endpoints for extra nodes that are needed in signature aggregation")
	privateKeyFlags.AddToCmd(cmd, "to pay fees for completing the validator's registration (blockchain gas token)")
	cmd.Flags().StringVar(&rpcURL, "rpc", "", "connect to validator manager at the given rpc endpoint")
	cmd.Flags().StringVar(&aggregatorLogLevel, "aggregator-log-level", "Off", "log level to use with signature aggregator")
	cmd.Flags().DurationVar(&duration, "staking-period", 0, "(for non sovereign blockchain) how long this validator will be staking")
	cmd.Flags().BoolVar(&useDefaultStartTime, "default-start-time", false, "(for non sovereign blockchain) use default start time for subnet validator (5 minutes later for fuji & mainnet, 30 seconds later for devnet)")
	cmd.Flags().StringVar(&startTimeStr, "start-time", "", "(for non sovereign blockchain) UTC start time when this validator starts validating, in 'YYYY-MM-DD HH:MM:SS' format")
	cmd.Flags().BoolVar(&useDefaultDuration, "default-duration", false, "(for non sovereign blockchain) set duration so as to validate until primary validator ends its period")
	cmd.Flags().BoolVar(&defaultValidatorParams, "default-validator-params", false, "(for non sovereign blockchain) use default weight/start/duration params for subnet validator")
	cmd.Flags().StringSliceVar(&subnetAuthKeys, "subnet-auth-keys", nil, "(for non sovereign blockchain) control keys that will be used to authenticate add validator tx")
	cmd.Flags().StringVar(&outputTxPath, "output-tx-path", "", "(for non sovereign blockchain) file path of the add validator tx")
	cmd.Flags().BoolVar(&waitForTxAcceptance, "wait-for-tx-acceptance", true, "(for non sovereign blockchain) just issue the add validator tx, without waiting for its acceptance")
	return cmd
}

func addValidator(_ *cobra.Command, args []string) error {
	blockchainName := args[0]
	_, err := ValidateSubnetNameAndGetChains([]string{blockchainName})
	if err != nil {
		return err
	}

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		true,
		false,
		addValidatorSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	fee := network.GenesisParams().TxFeeConfig.StaticFeeConfig.AddSubnetValidatorFee
	kc, err := keychain.GetKeychainFromCmdLineFlags(
		app,
		"to pay for transaction fees on P-Chain",
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

	if nodeEndpoint != "" {
		infoClient := info.NewClient(nodeEndpoint)
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		nodeID, proofOfPossession, err := infoClient.GetNodeID(ctx)
		if err != nil {
			return err
		}
		nodeIDStr = nodeID.String()
		publicKey = "0x" + hex.EncodeToString(proofOfPossession.PublicKey[:])
		pop = "0x" + hex.EncodeToString(proofOfPossession.ProofOfPossession[:])
	}
	if nodeIDStr == "" {
		nodeID, err := PromptNodeID("add as a blockchain validator")
		if err != nil {
			return err
		}
		nodeIDStr = nodeID.String()
	}
	if err := prompts.ValidateNodeID(nodeIDStr); err != nil {
		return err
	}

	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return fmt.Errorf("failed to load sidecar: %w", err)
	}
	sovereign := sc.Sovereign

	if sovereign && publicKey == "" && pop == "" {
		publicKey, pop, err = promptProofOfPossession(true, true)
		if err != nil {
			return err
		}
	}

	network.HandlePublicNetworkSimulation()

	if !sovereign {
		if err := UpdateKeychainWithSubnetControlKeys(kc, network, blockchainName); err != nil {
			return err
		}
	}
	deployer := subnet.NewPublicDeployer(app, kc, network)
	if !sovereign {
		return CallAddValidatorNonSOV(deployer, network, kc, useLedger, blockchainName, nodeIDStr, defaultValidatorParams, waitForTxAcceptance)
	}
	return CallAddValidator(deployer, network, kc, blockchainName, nodeIDStr, publicKey, pop)
}

func promptValidatorBalance(availableBalance uint64) (uint64, error) {
	ux.Logger.PrintToUser("Validator's balance is used to pay for continuous fee to the P-Chain")
	ux.Logger.PrintToUser("When this Balance reaches 0, the validator will be considered inactive and will no longer participate in validating the L1")
	txt := "What balance would you like to assign to the bootstrap validator (in AVAX)?"
	return app.Prompt.CaptureValidatorBalance(txt, availableBalance)
}

func CallAddValidator(
	deployer *subnet.PublicDeployer,
	network models.Network,
	kc *keychain.Keychain,
	blockchainName string,
	nodeIDStr string,
	publicKey string,
	pop string,
) error {
	nodeID, err := ids.NodeIDFromString(nodeIDStr)
	if err != nil {
		return err
	}
	blsInfo, err := getBLSInfo(publicKey, pop)
	if err != nil {
		return fmt.Errorf("failure parsing BLS info: %w", err)
	}

	expiry := uint64(time.Now().Add(constants.DefaultValidationIDExpiryDuration).Unix())

	chainSpec := contract.ChainSpec{
		BlockchainName: blockchainName,
	}

	sc, err := app.LoadSidecar(chainSpec.BlockchainName)
	if err != nil {
		return fmt.Errorf("failed to load sidecar: %w", err)
	}
	ownerPrivateKeyFound, _, _, ownerPrivateKey, err := contract.SearchForManagedKey(
		app,
		network,
		common.HexToAddress(sc.PoAValidatorManagerOwner),
		true,
	)
	if err != nil {
		return err
	}
	if !ownerPrivateKeyFound {
		return fmt.Errorf("private key for PoA manager owner %s is not found", sc.PoAValidatorManagerOwner)
	}
	ux.Logger.PrintToUser(logging.Yellow.Wrap("PoA manager owner %s pays for the initialization of the validator's registration (Blockchain gas token)"), sc.PoAValidatorManagerOwner)

	if rpcURL == "" {
		rpcURL, _, err = contract.GetBlockchainEndpoints(
			app,
			network,
			chainSpec,
			true,
			false,
		)
		if err != nil {
			return err
		}
	}

	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	pClient := platformvm.NewClient(network.Endpoint)
	bal, err := pClient.GetBalance(ctx, kc.Addresses().List())
	if err != nil {
		return err
	}
	availableBalance := uint64(bal.Balance) / units.Avax

	if balance == 0 {
		balanceAVAX, err := promptValidatorBalance(availableBalance)
		if err != nil {
			return err
		}
		// convert to nanoAVAX
		balance = balanceAVAX * units.Avax
	}

	if remainingBalanceOwnerAddr == "" {
		remainingBalanceOwnerAddr, err = getKeyForChangeOwner(network)
		if err != nil {
			return err
		}
	}
	remainingBalanceOwnerAddrID, err := address.ParseToIDs([]string{remainingBalanceOwnerAddr})
	if err != nil {
		return fmt.Errorf("failure parsing remaining balanche owner address %s: %w", remainingBalanceOwnerAddr, err)
	}
	remainingBalanceOwners := warpMessage.PChainOwner{
		Threshold: 1,
		Addresses: remainingBalanceOwnerAddrID,
	}

	if disableOwnerAddr == "" {
		disableOwnerAddr, err = prompts.PromptAddress(
			app.Prompt,
			"be able to disable the validator using P-Chain transactions",
			app.GetKeyDir(),
			app.GetKey,
			"",
			network,
			prompts.PChainFormat,
			"Enter P-Chain address (Example: P-...)",
		)
		if err != nil {
			return err
		}
	}
	disableOwnerAddrID, err := address.ParseToIDs([]string{disableOwnerAddr})
	if err != nil {
		return fmt.Errorf("failure parsing disable owner address %s: %w", disableOwnerAddr, err)
	}
	disableOwners := warpMessage.PChainOwner{
		Threshold: 1,
		Addresses: disableOwnerAddrID,
	}

	extraAggregatorPeers, err := GetAggregatorExtraPeers(network, aggregatorExtraEndpoints)
	if err != nil {
		return err
	}

	signedMessage, validationID, err := validatormanager.InitValidatorRegistration(
		app,
		network,
		rpcURL,
		chainSpec,
		ownerPrivateKey,
		nodeID,
		blsInfo.PublicKey[:],
		expiry,
		remainingBalanceOwners,
		disableOwners,
		weight,
		extraAggregatorPeers,
		aggregatorLogLevel,
	)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("ValidationID: %s", validationID)

	txID, _, err := deployer.RegisterL1Validator(balance, blsInfo, signedMessage)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("RegisterSubnetValidatorTx ID: %s", txID)

	if err := UpdatePChainHeight(
		deployer,
		kc.Addresses().List()[0],
		"Waiting for P-Chain to update validator information ...",
	); err != nil {
		return err
	}

	if err := validatormanager.FinishValidatorRegistration(
		app,
		network,
		rpcURL,
		chainSpec,
		ownerPrivateKey,
		validationID,
		extraAggregatorPeers,
		aggregatorLogLevel,
	); err != nil {
		return err
	}

	ux.Logger.PrintToUser("  NodeID: %s", nodeID)
	ux.Logger.PrintToUser("  Network: %s", network.Name())
	ux.Logger.PrintToUser("  Weight: %d", weight)
	ux.Logger.PrintToUser("  Balance: %d", balance/units.Avax)
	ux.Logger.GreenCheckmarkToUser("Validator successfully added to the Subnet")

	return nil
}

func CallAddValidatorNonSOV(
	deployer *subnet.PublicDeployer,
	network models.Network,
	kc *keychain.Keychain,
	useLedgerSetting bool,
	blockchainName string,
	nodeIDStr string,
	defaultValidatorParamsSetting bool,
	waitForTxAcceptanceSetting bool,
) error {
	var start time.Time
	nodeID, err := ids.NodeIDFromString(nodeIDStr)
	if err != nil {
		return err
	}
	useLedger = useLedgerSetting
	defaultValidatorParams = defaultValidatorParamsSetting
	waitForTxAcceptance = waitForTxAcceptanceSetting

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

	_, err = ValidateSubnetNameAndGetChains([]string{blockchainName})
	if err != nil {
		return err
	}

	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}

	subnetID := sc.Networks[network.Name()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	isPermissioned, controlKeys, threshold, err := txutils.GetOwners(network, subnetID)
	if err != nil {
		return err
	}
	if !isPermissioned {
		return ErrNotPermissionedSubnet
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

	selectedWeight, err := getWeight()
	if err != nil {
		return err
	}
	if selectedWeight < constants.MinStakeWeight {
		return fmt.Errorf("invalid weight, must be greater than or equal to %d: %d", constants.MinStakeWeight, selectedWeight)
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

	isFullySigned, tx, remainingSubnetAuthKeys, err := deployer.AddValidatorNonSOV(
		waitForTxAcceptance,
		controlKeys,
		subnetAuthKeys,
		subnetID,
		nodeID,
		selectedWeight,
		start,
		selectedDuration,
	)
	if err != nil {
		return err
	}
	if !isFullySigned {
		if err := SaveNotFullySignedTx(
			"Add Validator",
			tx,
			blockchainName,
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

func PromptNodeID(goal string) (ids.NodeID, error) {
	txt := fmt.Sprintf("What is the NodeID of the node you want to %s?", goal)
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
