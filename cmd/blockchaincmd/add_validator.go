// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/blockchain"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/signatureaggregator"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	"github.com/ava-labs/avalanche-cli/sdk/evm"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanche-cli/sdk/validator"
	"github.com/ava-labs/avalanchego/ids"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	warpMessage "github.com/ava-labs/avalanchego/vms/platformvm/warp/message"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

var (
	nodeIDStr                           string
	nodeEndpoint                        string
	balanceAVAX                         float64
	weight                              uint64
	startTimeStr                        string
	duration                            time.Duration
	defaultValidatorParams              bool
	useDefaultStartTime                 bool
	useDefaultDuration                  bool
	useDefaultWeight                    bool
	waitForTxAcceptance                 bool
	publicKey                           string
	pop                                 string
	remainingBalanceOwnerAddr           string
	disableOwnerAddr                    string
	delegationFee                       uint16
	errNoSubnetID                       = errors.New("failed to find the subnet ID for this subnet, has it been deployed/created on this network?")
	errMutuallyExclusiveDurationOptions = errors.New("--use-default-duration/--use-default-validator-params and --staking-period are mutually exclusive")
	errMutuallyExclusiveStartOptions    = errors.New("--use-default-start-time/--use-default-validator-params and --start-time are mutually exclusive")
	errMutuallyExclusiveWeightOptions   = errors.New("--use-default-validator-params and --weight are mutually exclusive")
	ErrNotPermissionedSubnet            = errors.New("subnet is not permissioned")
	clusterNameFlagValue                string
	createLocalValidator                bool
	externalValidatorManagerOwner       bool
	validatorManagerOwner               string
	httpPort                            uint32
	stakingPort                         uint32
	addValidatorFlags                   BlockchainAddValidatorFlags
)

type BlockchainAddValidatorFlags struct {
	RPC         string
	SigAggFlags flags.SignatureAggregatorFlags
}

const (
	validatorWeightFlag = "weight"
)

// avalanche blockchain addValidator
func newAddValidatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addValidator [blockchainName]",
		Short: "Add a validator to an L1",
		Long: `The blockchain addValidator command adds a node as a validator to
an L1 of the user provided deployed network. If the network is proof of 
authority, the owner of the validator manager contract must sign the 
transaction. If the network is proof of stake, the node must stake the L1's
staking token. Both processes will issue a RegisterL1ValidatorTx on the P-Chain.

This command currently only works on Blockchains deployed to either the Fuji
Testnet or Mainnet.`,
		RunE: addValidator,
		Args: cobrautils.MaximumNArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, networkoptions.DefaultSupportedNetworkOptions)
	flags.AddRPCFlagToCmd(cmd, app, &addValidatorFlags.RPC)
	flags.AddSignatureAggregatorFlagsToCmd(cmd, &addValidatorFlags.SigAggFlags)
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet only]")
	cmd.Flags().Float64Var(
		&balanceAVAX,
		"balance",
		0,
		"set the AVAX balance of the validator that will be used for continuous fee on P-Chain",
	)
	cmd.Flags().BoolVarP(&useEwoq, "ewoq", "e", false, "use ewoq key [fuji/devnet only]")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	cmd.Flags().StringVar(&nodeIDStr, "node-id", "", "node-id of the validator to add")
	cmd.Flags().StringVar(&publicKey, "bls-public-key", "", "set the BLS public key of the validator to add")
	cmd.Flags().StringVar(&pop, "bls-proof-of-possession", "", "set the BLS proof of possession of the validator to add")
	cmd.Flags().StringVar(&remainingBalanceOwnerAddr, "remaining-balance-owner", "", "P-Chain address that will receive any leftover AVAX from the validator when it is removed from Subnet")
	cmd.Flags().StringVar(&disableOwnerAddr, "disable-owner", "", "P-Chain address that will able to disable the validator with a P-Chain transaction")
	cmd.Flags().BoolVar(&createLocalValidator, "create-local-validator", false, "create additional local validator and add it to existing running local node")
	cmd.Flags().BoolVar(&partialSync, "partial-sync", true, "set primary network partial sync for new validators")
	cmd.Flags().StringVar(&nodeEndpoint, "node-endpoint", "", "gather node id/bls from publicly available avalanchego apis on the given endpoint")
	cmd.Flags().DurationVar(&duration, "staking-period", 0, "how long this validator will be staking")
	cmd.Flags().BoolVar(&useDefaultStartTime, "default-start-time", false, "(for Subnets, not L1s) use default start time for subnet validator (5 minutes later for fuji & mainnet, 30 seconds later for devnet)")
	cmd.Flags().StringVar(&startTimeStr, "start-time", "", "(for Subnets, not L1s) UTC start time when this validator starts validating, in 'YYYY-MM-DD HH:MM:SS' format")
	cmd.Flags().BoolVar(&useDefaultDuration, "default-duration", false, "(for Subnets, not L1s) set duration so as to validate until primary validator ends its period")
	cmd.Flags().BoolVar(&defaultValidatorParams, "default-validator-params", false, "(for Subnets, not L1s) use default weight/start/duration params for subnet validator")
	cmd.Flags().StringSliceVar(&subnetAuthKeys, "subnet-auth-keys", nil, "(for Subnets, not L1s) control keys that will be used to authenticate add validator tx")
	cmd.Flags().StringVar(&outputTxPath, "output-tx-path", "", "(for Subnets, not L1s) file path of the add validator tx")
	cmd.Flags().BoolVar(&waitForTxAcceptance, "wait-for-tx-acceptance", true, "(for Subnets, not L1s) just issue the add validator tx, without waiting for its acceptance")
	cmd.Flags().Uint16Var(&delegationFee, "delegation-fee", 100, "(PoS only) delegation fee (in bips)")
	cmd.Flags().StringVar(&subnetIDstr, "subnet-id", "", "subnet ID (only if blockchain name is not provided)")
	cmd.Flags().Uint64Var(&weight, validatorWeightFlag, uint64(constants.DefaultStakeWeight), "set the weight of the validator")
	cmd.Flags().StringVar(&validatorManagerOwner, "validator-manager-owner", "", "force using this address to issue transactions to the validator manager")
	cmd.Flags().BoolVar(&externalValidatorManagerOwner, "external-evm-signature", false, "set this value to true when signing validator manager tx outside of cli (for multisig or ledger)")
	cmd.Flags().StringVar(&initiateTxHash, "initiate-tx-hash", "", "initiate tx is already issued, with the given hash")
	cmd.Flags().Uint32Var(&httpPort, "http-port", 0, "http port for node")
	cmd.Flags().Uint32Var(&stakingPort, "staking-port", 0, "staking port for node")

	return cmd
}

func preAddChecks(args []string) error {
	if nodeEndpoint != "" && createLocalValidator {
		return fmt.Errorf("cannot set both --node-endpoint and --create-local-validator")
	}
	if createLocalValidator && (nodeIDStr != "" || publicKey != "" || pop != "") {
		return fmt.Errorf("cannot set --node-id, --bls-public-key or --bls-proof-of-possession if --create-local-validator used")
	}
	if len(args) == 0 && createLocalValidator {
		return fmt.Errorf("use avalanche addValidator <subnetName> command to use local machine as new validator")
	}

	return nil
}

func addValidator(cmd *cobra.Command, args []string) error {
	var sc models.Sidecar
	blockchainName := ""
	networkOption := networkoptions.DefaultSupportedNetworkOptions
	if len(args) == 1 {
		blockchainName = args[0]
		_, err := ValidateSubnetNameAndGetChains([]string{blockchainName})
		if err != nil {
			return err
		}
		sc, err = app.LoadSidecar(blockchainName)
		if err != nil {
			return fmt.Errorf("failed to load sidecar: %w", err)
		}
		networkOption = networkoptions.GetNetworkFromSidecar(sc, networkoptions.DefaultSupportedNetworkOptions)
	}

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		true,
		false,
		networkOption,
		"",
	)
	if err != nil {
		return err
	}

	if network.ClusterName != "" {
		clusterNameFlagValue = network.ClusterName
		network = models.ConvertClusterToNetwork(network)
	}

	if len(args) == 0 {
		sc, err = importL1(blockchainIDStr, addValidatorFlags.RPC, network)
		if err != nil {
			return err
		}
	}

	if err := preAddChecks(args); err != nil {
		return err
	}

	if sc.Networks[network.Name()].ClusterName != "" {
		clusterNameFlagValue = sc.Networks[network.Name()].ClusterName
	}

	// TODO: will estimate fee in subsecuent PR
	fee := uint64(0)
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

	sovereign := sc.Sovereign

	if nodeEndpoint != "" {
		nodeIDStr, publicKey, pop, err = utils.GetNodeID(nodeEndpoint)
		if err != nil {
			return err
		}
	}

	if sovereign {
		if !cmd.Flags().Changed(validatorWeightFlag) {
			weight, err = app.Prompt.CaptureWeight(
				"What weight would you like to assign to the validator?",
				func(uint64) error { return nil },
			)
			if err != nil {
				return err
			}
		}
	}

	// if we don't have a nodeID or ProofOfPossession by this point, prompt user if we want to add additional local node
	if (!sovereign && nodeIDStr == "") || (sovereign && !createLocalValidator && nodeIDStr == "" && publicKey == "" && pop == "") {
		if len(args) == 0 {
			createLocalValidator = false
		} else {
			for {
				local := "Use my local machine to spin up an additional validator"
				existing := "I have an existing Avalanche node (we will require its NodeID and BLS info)"
				if option, err := app.Prompt.CaptureList(
					"How would you like to set up the new validator",
					[]string{local, existing},
				); err != nil {
					return err
				} else {
					createLocalValidator = option == local
					break
				}
			}
		}
	}

	subnetID := sc.Networks[network.Name()].SubnetID

	// if user chose to upsize a local node to add another local validator
	var localValidatorClusterName string
	if createLocalValidator {
		// TODO: make this to work even if there is no local cluster for the blockchain and network
		targetClusters, err := localnet.GetFilteredLocalClusters(app, true, network, blockchainName)
		if err != nil {
			return err
		}
		if len(targetClusters) == 0 {
			return fmt.Errorf("no local cluster is running for network %s and blockchain %s", network.Name(), blockchainName)
		}
		if len(targetClusters) != 1 {
			return fmt.Errorf("too many local clusters running for network %s and blockchain %s", network.Name(), blockchainName)
		}
		localValidatorClusterName = targetClusters[0]
		node, err := localnet.AddNodeToLocalCluster(app, ux.Logger.PrintToUser, localValidatorClusterName, httpPort, stakingPort)
		if err != nil {
			return err
		}
		nodeIDStr, publicKey, pop, err = utils.GetNodeID(node.URI)
		if err != nil {
			return err
		}
		sc, err = app.AddDefaultBlockchainRPCsToSidecar(blockchainName, network, []string{node.URI})
		if err != nil {
			return err
		}
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
	if err := CallAddValidator(
		deployer,
		network,
		kc,
		blockchainName,
		subnetID,
		nodeIDStr,
		publicKey,
		pop,
		weight,
		balanceAVAX,
		remainingBalanceOwnerAddr,
		disableOwnerAddr,
		sc,
		addValidatorFlags.RPC,
	); err != nil {
		return err
	}
	if createLocalValidator && network.Kind == models.Local {
		// For all blockchains validated by the cluster, set up an alias from blockchain name
		// into blockchain id, to be mainly used in the blockchain RPC
		return localnet.RefreshLocalClusterAliases(app, localValidatorClusterName)
	}
	return nil
}

func promptValidatorBalanceAVAX(availableBalance float64) (float64, error) {
	ux.Logger.PrintToUser("Validator's balance is used to pay for continuous fee to the P-Chain")
	ux.Logger.PrintToUser("When this Balance reaches 0, the validator will be considered inactive and will no longer participate in validating the L1")
	txt := "What balance would you like to assign to the validator (in AVAX)?"
	return app.Prompt.CaptureValidatorBalance(txt, availableBalance, constants.BootstrapValidatorBalanceAVAX)
}

func CallAddValidator(
	deployer *subnet.PublicDeployer,
	network models.Network,
	kc *keychain.Keychain,
	blockchainName string,
	subnetID ids.ID,
	nodeIDStr string,
	publicKey string,
	pop string,
	weight uint64,
	balanceAVAX float64,
	remainingBalanceOwnerAddr string,
	disableOwnerAddr string,
	sc models.Sidecar,
	rpcURL string,
) error {
	nodeID, err := ids.NodeIDFromString(nodeIDStr)
	if err != nil {
		return err
	}
	blsInfo, err := blockchain.ConvertToBLSProofOfPossession(publicKey, pop)
	if err != nil {
		return fmt.Errorf("failure parsing BLS info: %w", err)
	}

	blockchainTimestamp, err := blockchain.GetBlockchainTimestamp(network)
	if err != nil {
		return fmt.Errorf("failed to get blockchain timestamp: %w", err)
	}
	expiry := uint64(blockchainTimestamp.Add(constants.DefaultValidationIDExpiryDuration).Unix())
	chainSpec := contract.ChainSpec{
		BlockchainName: blockchainName,
	}
	if sc.Networks[network.Name()].BlockchainID.String() != "" {
		chainSpec.BlockchainID = sc.Networks[network.Name()].BlockchainID.String()
	}
	if sc.Networks[network.Name()].ValidatorManagerAddress == "" {
		return fmt.Errorf("unable to find Validator Manager address")
	}
	validatorManagerAddress = sc.Networks[network.Name()].ValidatorManagerAddress

	if validatorManagerOwner == "" {
		validatorManagerOwner = sc.ValidatorManagerOwner
	}

	var ownerPrivateKey string
	if !externalValidatorManagerOwner {
		var ownerPrivateKeyFound bool
		ownerPrivateKeyFound, _, _, ownerPrivateKey, err = contract.SearchForManagedKey(
			app,
			network,
			common.HexToAddress(validatorManagerOwner),
			true,
		)
		if err != nil {
			return err
		}
		if !ownerPrivateKeyFound {
			return fmt.Errorf("private key for Validator manager owner %s is not found", validatorManagerOwner)
		}
	}

	pos := sc.PoS()

	if pos {
		// should take input prior to here for delegation fee, and min stake duration
		if duration == 0 {
			duration, err = PromptDuration(time.Now(), network, true) // it's pos
			if err != nil {
				return nil
			}
		}
	}

	if sc.UseACP99 {
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Validator Manager Protocol: ACP99"))
	} else {
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Validator Manager Protocol: v1.0.0"))
	}

	ux.Logger.PrintToUser(logging.Yellow.Wrap("Validation manager owner %s pays for the initialization of the validator's registration (Blockchain gas token)"), validatorManagerOwner)

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

	ux.Logger.PrintToUser(logging.Yellow.Wrap("RPC Endpoint: %s"), rpcURL)

	totalWeight, err := validator.GetTotalWeight(network.SDKNetwork(), subnetID)
	if err != nil {
		return err
	}
	allowedChange := float64(totalWeight) * constants.MaxL1TotalWeightChange
	if float64(weight) > allowedChange {
		return fmt.Errorf("can't make change: desired validator weight %d exceeds max allowed weight change of %d", newWeight, uint64(allowedChange))
	}

	if balanceAVAX == 0 {
		availableBalance, err := utils.GetNetworkBalance(kc.Addresses().List(), network.Endpoint)
		if err != nil {
			return err
		}
		if availableBalance == 0 {
			return fmt.Errorf("chosen key has zero balance")
		}
		balanceAVAX, err = promptValidatorBalanceAVAX(float64(availableBalance) / float64(units.Avax))
		if err != nil {
			return err
		}
	}
	// convert to nanoAVAX
	balance := uint64(balanceAVAX * float64(units.Avax))

	if remainingBalanceOwnerAddr == "" {
		remainingBalanceOwnerAddr, err = blockchain.GetKeyForChangeOwner(app, network)
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
	extraAggregatorPeers, err := blockchain.GetAggregatorExtraPeers(app, clusterNameFlagValue)
	if err != nil {
		return err
	}
	aggregatorLogger, err := signatureaggregator.NewSignatureAggregatorLogger(
		addValidatorFlags.SigAggFlags.AggregatorLogLevel,
		addValidatorFlags.SigAggFlags.AggregatorLogToStdout,
		app.GetAggregatorLogDir(clusterNameFlagValue),
	)
	if err != nil {
		return err
	}
	aggregatorCtx, aggregatorCancel := sdkutils.GetTimedContext(constants.SignatureAggregatorTimeout)
	defer aggregatorCancel()
	signedMessage, validationID, rawTx, err := validatormanager.InitValidatorRegistration(
		aggregatorCtx,
		app,
		network,
		rpcURL,
		chainSpec,
		externalValidatorManagerOwner,
		validatorManagerOwner,
		ownerPrivateKey,
		nodeID,
		blsInfo.PublicKey[:],
		expiry,
		remainingBalanceOwners,
		disableOwners,
		weight,
		extraAggregatorPeers,
		aggregatorLogger,
		pos,
		delegationFee,
		duration,
		validatorManagerAddress,
		sc.UseACP99,
		initiateTxHash,
	)
	if err != nil {
		return err
	}
	if rawTx != nil {
		dump, err := evm.TxDump("Initializing Validator Registration", rawTx)
		if err == nil {
			ux.Logger.PrintToUser(dump)
		}
		return err
	}
	ux.Logger.PrintToUser("ValidationID: %s", validationID)

	txID, _, err := deployer.RegisterL1Validator(balance, blsInfo, signedMessage)
	if err != nil {
		if !strings.Contains(err.Error(), "warp message already issued for validationID") {
			return err
		}
		ux.Logger.PrintToUser(logging.LightBlue.Wrap("The Validation ID was already registered on the P-Chain. Proceeding to the next step"))
	} else {
		ux.Logger.PrintToUser("RegisterL1ValidatorTx ID: %s", txID)
		if err := blockchain.UpdatePChainHeight(
			"Waiting for P-Chain to update validator information ...",
		); err != nil {
			return err
		}
	}

	aggregatorCtx, aggregatorCancel = sdkutils.GetTimedContext(constants.SignatureAggregatorTimeout)
	defer aggregatorCancel()
	rawTx, err = validatormanager.FinishValidatorRegistration(
		aggregatorCtx,
		app,
		network,
		rpcURL,
		chainSpec,
		externalValidatorManagerOwner,
		validatorManagerOwner,
		ownerPrivateKey,
		validationID,
		extraAggregatorPeers,
		aggregatorLogger,
		validatorManagerAddress,
	)
	if err != nil {
		return err
	}
	if rawTx != nil {
		dump, err := evm.TxDump("Finish Validator Registration", rawTx)
		if err == nil {
			ux.Logger.PrintToUser(dump)
		}
		return err
	}

	ux.Logger.PrintToUser("  NodeID: %s", nodeID)
	ux.Logger.PrintToUser("  Network: %s", network.Name())
	// weight is inaccurate for PoS as it's fetched during registration
	if !pos {
		ux.Logger.PrintToUser("  Weight: %d", weight)
	}
	ux.Logger.PrintToUser("  Balance: %.2f", balanceAVAX)

	ux.Logger.GreenCheckmarkToUser("Validator successfully added to the L1")

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
	ux.Logger.PrintToUser("Your auth keys for add validator tx creation: %s", subnetAuthKeys)

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

func PromptDuration(start time.Time, network models.Network, isPos bool) (time.Duration, error) {
	for {
		txt := "How long should this validator be validating? Enter a duration, e.g. 8760h. Valid time units are \"ns\", \"us\" (or \"µs\"), \"ms\", \"s\", \"m\", \"h\""
		var d time.Duration
		var err error
		switch {
		case network.Kind == models.Fuji:
			d, err = app.Prompt.CaptureFujiDuration(txt)
		case network.Kind == models.Mainnet && isPos:
			d, err = app.Prompt.CaptureMainnetL1StakingDuration(txt)
		case network.Kind == models.Mainnet && !isPos:
			d, err = app.Prompt.CaptureMainnetDuration(txt)
		default:
			d, err = app.Prompt.CaptureDuration(txt)
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
			duration, err = PromptDuration(start, network, false) // notSoV
			if err != nil {
				return time.Time{}, 0, err
			}
		}
	}

	var selectedDuration time.Duration
	if useDefaultDuration {
		// avoid setting both globals useDefaultDuration and duration
		selectedDuration, err = utils.GetRemainingValidationTime(network.Endpoint, nodeID, avagoconstants.PrimaryNetworkID, start)
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
			weight, err = app.Prompt.CaptureWeight(txt, func(uint64) error { return nil })
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
