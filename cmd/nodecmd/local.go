// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/blockchain"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/api"
	warpMessage "github.com/ava-labs/avalanchego/vms/platformvm/warp/message"

	"github.com/spf13/cobra"
)

var (
	avalanchegoBinaryPath string

	bootstrapIDs                 []string
	bootstrapIPs                 []string
	genesisPath                  string
	upgradePath                  string
	stakingTLSKeyPath            string
	stakingCertKeyPath           string
	stakingSignerKeyPath         string
	numNodes                     uint32
	nodeConfigPath               string
	partialSync                  bool
	stakeAmount                  uint64
	rpcURL                       string
	balanceAVAX                  float64
	remainingBalanceOwnerAddr    string
	disableOwnerAddr             string
	aggregatorLogLevel           string
	aggregatorLogToStdout        bool
	delegationFee                uint16
	publicKey                    string
	pop                          string
	minimumStakeDuration         uint64
	latestAvagoReleaseVersion    bool
	latestAvagoPreReleaseVersion bool
	validatorManagerAddress      string
	useACP99                     bool
)

// const snapshotName = "local_snapshot"
func newLocalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Suite of commands for a local avalanche node",
		Long:  `The node local command suite provides a collection of commands related to local nodes`,
		RunE:  cobrautils.CommandSuiteUsage,
	}
	// node local start
	cmd.AddCommand(newLocalStartCmd())
	// node local stop
	cmd.AddCommand(newLocalStopCmd())
	// node local destroy
	cmd.AddCommand(newLocalDestroyCmd())
	// node local track
	cmd.AddCommand(newLocalTrackCmd())
	// node local status
	cmd.AddCommand(newLocalStatusCmd())
	// node local validate
	cmd.AddCommand(newLocalValidateCmd())
	return cmd
}

func newLocalStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [clusterName]",
		Short: "Create new Avalanche nodes on local machine",
		Long: `The node local start command creates Avalanche nodes on the local machine.
Once this command is completed, you will have to wait for the Avalanche node
to finish bootstrapping on the primary network before running further
commands on it, e.g. validating a Subnet. 

You can check the bootstrapping status by running avalanche node status local.
`,
		Args:              cobra.ExactArgs(1),
		RunE:              localStartNode,
		PersistentPostRun: handlePostRun,
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, networkoptions.DefaultSupportedNetworkOptions)
	cmd.Flags().BoolVar(&latestAvagoReleaseVersion, "latest-avalanchego-version", true, "install latest avalanchego release version on node/s")
	cmd.Flags().BoolVar(&latestAvagoPreReleaseVersion, "latest-avalanchego-pre-release-version", false, "install latest avalanchego pre-release version on node/s")
	cmd.Flags().StringVar(&useCustomAvalanchegoVersion, "custom-avalanchego-version", "", "install given avalanchego version on node/s")
	cmd.Flags().StringVar(&avalanchegoBinaryPath, "avalanchego-path", "", "use this avalanchego binary path")
	cmd.Flags().StringArrayVar(&bootstrapIDs, "bootstrap-id", []string{}, "nodeIDs of bootstrap nodes")
	cmd.Flags().StringArrayVar(&bootstrapIPs, "bootstrap-ip", []string{}, "IP:port pairs of bootstrap nodes")
	cmd.Flags().StringVar(&genesisPath, "genesis", "", "path to genesis file")
	cmd.Flags().StringVar(&upgradePath, "upgrade", "", "path to upgrade file")
	cmd.Flags().StringVar(&stakingTLSKeyPath, "staking-tls-key-path", "", "path to provided staking tls key for node")
	cmd.Flags().StringVar(&stakingCertKeyPath, "staking-cert-key-path", "", "path to provided staking cert key for node")
	cmd.Flags().StringVar(&stakingSignerKeyPath, "staking-signer-key-path", "", "path to provided staking signer key for node")
	cmd.Flags().Uint32Var(&numNodes, "num-nodes", 1, "number of Avalanche nodes to create on local machine")
	cmd.Flags().StringVar(&nodeConfigPath, "node-config", "", "path to common avalanchego config settings for all nodes")
	cmd.Flags().BoolVar(&partialSync, "partial-sync", true, "primary network partial sync")
	return cmd
}

func newLocalStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop local node",
		Long:  `Stop local node.`,
		Args:  cobra.ExactArgs(0),
		RunE:  localStopNode,
	}
}

func newLocalTrackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "track [clusterName] [blockchainName]",
		Short: "Track specified blockchain with local node",
		Long:  "Track specified blockchain with local node",
		Args:  cobra.ExactArgs(2),
		RunE:  localTrack,
	}
	cmd.Flags().StringVar(&avalanchegoBinaryPath, "avalanchego-path", "", "use this avalanchego binary path")
	cmd.Flags().BoolVar(&latestAvagoReleaseVersion, "latest-avalanchego-version", true, "install latest avalanchego release version on node/s")
	cmd.Flags().BoolVar(&latestAvagoPreReleaseVersion, "latest-avalanchego-pre-release-version", false, "install latest avalanchego pre-release version on node/s")
	cmd.Flags().StringVar(&useCustomAvalanchegoVersion, "custom-avalanchego-version", "", "install given avalanchego version on node/s")
	return cmd
}

func newLocalDestroyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "destroy [clusterName]",
		Short: "Cleanup local node",
		Long:  `Cleanup local node.`,
		Args:  cobra.ExactArgs(1),
		RunE:  localDestroyNode,
	}
}

func newLocalStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Get status of local node",
		Long:  `Get status of local node.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  localStatus,
	}

	cmd.Flags().StringVar(&blockchainName, "l1", "", "specify the blockchain the node is syncing with")
	cmd.Flags().StringVar(&blockchainName, "blockchain", "", "specify the blockchain the node is syncing with")

	return cmd
}

func localStartNode(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	anrSettings := node.ANRSettings{
		GenesisPath:          genesisPath,
		UpgradePath:          upgradePath,
		BootstrapIDs:         bootstrapIDs,
		BootstrapIPs:         bootstrapIPs,
		StakingSignerKeyPath: stakingTLSKeyPath,
		StakingCertKeyPath:   stakingCertKeyPath,
		StakingTLSKeyPath:    stakingTLSKeyPath,
	}

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		false,
		true,
		networkoptions.DefaultSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	if network.Kind != models.Fuji {
		ux.Logger.PrintToUser("This version of Avalanche-CLI is only meant for Fuji deployments. To deploy in other networks, please use v1.8.8-rc4")
		ux.Logger.PrintToUser("curl -sSfL https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh | sh -s v1.8.8-rc4")
		return nil
	}
	// TODO: remove this check for releases above v1.8.7, once v1.13.0-fuji avalanchego is latest release
	if network.Kind == models.Fuji && useCustomAvalanchegoVersion == "" {
		latestAvagoVersion, err := app.Downloader.GetLatestReleaseVersion(
			constants.AvaLabsOrg,
			constants.AvalancheGoRepoName,
			"",
		)
		if err != nil {
			return err
		}
		versionComparison := semver.Compare(constants.FujiAvalancheGoV113, latestAvagoVersion)
		if versionComparison == 1 {
			useCustomAvalanchegoVersion = constants.FujiAvalancheGoV113
		}
	}
	if useCustomAvalanchegoVersion != "" {
		latestAvagoPreReleaseVersion = false
		latestAvagoReleaseVersion = false
	}
	avaGoVersionSetting := node.AvalancheGoVersionSettings{
		UseCustomAvalanchegoVersion:           useCustomAvalanchegoVersion,
		UseLatestAvalanchegoPreReleaseVersion: latestAvagoPreReleaseVersion,
		UseLatestAvalanchegoReleaseVersion:    latestAvagoReleaseVersion,
	}
	nodeConfig := make(map[string]interface{})
	if nodeConfigPath != "" {
		var err error
		nodeConfig, err = utils.ReadJSON(nodeConfigPath)
		if err != nil {
			return err
		}
	}
	if partialSync {
		nodeConfig[config.PartialSyncPrimaryNetworkKey] = true
	}
	return node.StartLocalNode(
		app,
		clusterName,
		avalanchegoBinaryPath,
		numNodes,
		nodeConfig,
		anrSettings,
		avaGoVersionSetting,
		network,
	)
}

func localStopNode(_ *cobra.Command, _ []string) error {
	return node.StopLocalNode(app)
}

func localDestroyNode(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	return node.DestroyLocalNode(app, clusterName)
}

func localTrack(_ *cobra.Command, args []string) error {
	if avalanchegoBinaryPath == "" {
		if useCustomAvalanchegoVersion != "" {
			latestAvagoReleaseVersion = false
			latestAvagoPreReleaseVersion = false
		}
		avaGoVersionSetting := node.AvalancheGoVersionSettings{
			UseCustomAvalanchegoVersion:           useCustomAvalanchegoVersion,
			UseLatestAvalanchegoPreReleaseVersion: latestAvagoPreReleaseVersion,
			UseLatestAvalanchegoReleaseVersion:    latestAvagoReleaseVersion,
		}
		avalancheGoVersion, err := node.GetAvalancheGoVersion(app, avaGoVersionSetting)
		if err != nil {
			return err
		}
		_, avagoDir, err := binutils.SetupAvalanchego(app, avalancheGoVersion)
		if err != nil {
			return fmt.Errorf("failed installing Avalanche Go version %s: %w", avalancheGoVersion, err)
		}
		avalanchegoBinaryPath = filepath.Join(avagoDir, "avalanchego")
	}
	return node.TrackSubnetWithLocalMachine(app, args[0], args[1], avalanchegoBinaryPath)
}

func localStatus(_ *cobra.Command, args []string) error {
	clusterName := ""
	if len(args) > 0 {
		clusterName = args[0]
	}
	if blockchainName != "" && clusterName == "" {
		return fmt.Errorf("--blockchain flag is only supported if clusterName is specified")
	}
	return node.LocalStatus(app, clusterName, blockchainName)
}

func notImplementedForLocal(what string) error {
	ux.Logger.PrintToUser("Unsupported cmd: %s is not supported by local clusters", logging.LightBlue.Wrap(what))
	return nil
}

func newLocalValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [clusterName]",
		Short: "Validate a specified L1 with an Avalanche Node set up on local machine (PoS only)",
		Long: `Use Avalanche Node set up on local machine to set up specified L1 by providing the
RPC URL of the L1. 

This command can only be used to validate Proof of Stake L1.`,
		Args: cobra.ExactArgs(1),
		RunE: localValidate,
	}

	cmd.Flags().StringVar(&blockchainName, "l1", "", "specify the blockchain the node is syncing with")
	cmd.Flags().StringVar(&blockchainName, "blockchain", "", "specify the blockchain the node is syncing with")
	cmd.Flags().Uint64Var(&stakeAmount, "stake-amount", 0, "amount of tokens to stake")
	cmd.Flags().StringVar(&rpcURL, "rpc", "", "connect to validator manager at the given rpc endpoint")
	cmd.Flags().Float64Var(&balanceAVAX, "balance", 0, "amount of AVAX to increase validator's balance by")
	cmd.Flags().Uint16Var(&delegationFee, "delegation-fee", 100, "delegation fee (in bips)")
	cmd.Flags().StringVar(&aggregatorLogLevel, "aggregator-log-level", constants.DefaultAggregatorLogLevel, "log level to use with signature aggregator")
	cmd.Flags().BoolVar(&aggregatorLogToStdout, "aggregator-log-to-stdout", false, "use stdout for signature aggregator logs")
	cmd.Flags().StringVar(&remainingBalanceOwnerAddr, "remaining-balance-owner", "", "P-Chain address that will receive any leftover AVAX from the validator when it is removed from Subnet")
	cmd.Flags().StringVar(&disableOwnerAddr, "disable-owner", "", "P-Chain address that will able to disable the validator with a P-Chain transaction")
	cmd.Flags().Uint64Var(&minimumStakeDuration, "minimum-stake-duration", constants.PoSL1MinimumStakeDurationSeconds, "minimum stake duration (in seconds)")
	cmd.Flags().StringVar(&validatorManagerAddress, "validator-manager-address", "", "validator manager address")
	cmd.Flags().BoolVar(&useACP99, "acp99", true, "use ACP99 contracts instead of v1.0.0 for validator managers")

	return cmd
}

func localValidate(_ *cobra.Command, args []string) error {
	clusterName := ""
	if len(args) > 0 {
		clusterName = args[0]
	}

	if clusterName == "" {
		return fmt.Errorf("local cluster name cannot be empty")
	}

	if ok, err := node.CheckClusterIsLocal(app, clusterName); err != nil || !ok {
		return fmt.Errorf("local cluster %q not found, please create it first using avalanche node local start %q", clusterName, clusterName)
	}

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		true,
		false,
		networkoptions.DefaultSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
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

	// should take input prior to here for stake amount, delegation fee, and min stake duration
	if stakeAmount == 0 {
		stakeAmount, err = app.Prompt.CaptureUint64Compare(
			"Enter the amount of token to stake for each validator",
			[]prompts.Comparator{
				{
					Label: "Positive",
					Type:  prompts.MoreThan,
					Value: 0,
				},
			},
		)
		if err != nil {
			return err
		}
	}

	if rpcURL == "" {
		rpcURL, err = app.Prompt.CaptureURL("What is the RPC endpoint?", false)
		if err != nil {
			return err
		}
	}
	_, blockchainID, err := utils.SplitAvalanchegoRPCURI(rpcURL)
	// if there is error that means RPC URL did not contain blockchain in it
	// RPC might be in the format of something like https://etna.avax-dev.network
	// We will prompt for blockchainID in that case
	if err != nil {
		blockchainID, err = app.Prompt.CaptureString("What is the Blockchain ID of the L1?")
		if err != nil {
			return err
		}
	}

	if validatorManagerAddress == "" {
		validatorManagerAddressAddrFmt, err := app.Prompt.CaptureAddress("What is the address of the Validator Manager?")
		if err != nil {
			return err
		}
		validatorManagerAddress = validatorManagerAddressAddrFmt.String()
	}

	chainSpec := contract.ChainSpec{
		BlockchainID: blockchainID,
	}
	if balanceAVAX == 0 {
		availableBalance, err := utils.GetNetworkBalance(kc.Addresses().List(), network.Endpoint)
		if err != nil {
			return err
		}
		prompt := "How many AVAX do you want to each validator to start with?"
		balanceAVAX, err = blockchain.PromptValidatorBalance(app, float64(availableBalance)/float64(units.Avax), prompt)
		if err != nil {
			return err
		}
	}
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

	ux.Logger.PrintToUser("A private key is needed to pay for initialization of the validator's registration (Blockchain gas token).")
	payerPrivateKey, err := prompts.PromptPrivateKey(
		app.Prompt,
		"pay the fee",
		app.GetKeyDir(),
		app.GetKey,
		"",
		"",
	)
	if err != nil {
		return err
	}

	extraAggregatorPeers, err := blockchain.GetAggregatorExtraPeers(app, clusterName, []string{})
	if err != nil {
		return err
	}
	aggregatorLogger, err := utils.NewLogger(
		constants.SignatureAggregatorLogName,
		aggregatorLogLevel,
		constants.DefaultAggregatorLogLevel,
		app.GetAggregatorLogDir(clusterName),
		aggregatorLogToStdout,
		ux.Logger.PrintToUser,
	)
	if err != nil {
		return err
	}

	ctx, cancel := network.BootstrappingContext()
	defer cancel()
	cli, err := binutils.NewGRPCClientWithEndpoint(
		binutils.LocalClusterGRPCServerEndpoint,
		binutils.WithAvoidRPCVersionCheck(true),
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if err != nil {
		return err
	}
	status, err := cli.Status(ctx)
	if err != nil {
		return err
	}

	if useACP99 {
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Validator Manager Protocol: ACP99"))
	} else {
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Validator Manager Protocol: v1.0.0"))
	}

	for _, node := range status.ClusterInfo.NodeInfos {
		if err = addAsValidator(network,
			node.Name,
			chainSpec,
			remainingBalanceOwners, disableOwners,
			extraAggregatorPeers,
			aggregatorLogger,
			kc,
			balance,
			payerPrivateKey,
			validatorManagerAddress,
			useACP99,
		); err != nil {
			return err
		}
	}

	ux.Logger.PrintToUser(" ")
	ux.Logger.GreenCheckmarkToUser("All validators are successfully added to the L1")
	return nil
}

func addAsValidator(network models.Network,
	nodeName string,
	chainSpec contract.ChainSpec,
	remainingBalanceOwners, disableOwners warpMessage.PChainOwner,
	extraAggregatorPeers []info.Peer,
	aggregatorLogger logging.Logger,
	kc *keychain.Keychain,
	balance uint64,
	payerPrivateKey string,
	validatorManagerAddressStr string,
	useACP99 bool,
) error {
	var nodeIDStr string
	// get node data
	nodeInfo, err := node.GetNodeInfo(nodeName)
	if err != nil {
		return err
	}
	nodeIDStr, publicKey, pop, err = node.GetNodeData(nodeInfo.Uri)
	if err != nil {
		return err
	}
	nodeID, err := ids.NodeIDFromString(nodeIDStr)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser(" ")
	ux.Logger.PrintToUser("Adding validator %s", nodeIDStr)
	ux.Logger.PrintToUser(" ")

	blockchainTimestamp, err := blockchain.GetBlockchainTimestamp(network)
	if err != nil {
		return fmt.Errorf("failed to get blockchain timestamp: %w", err)
	}
	expiry := uint64(blockchainTimestamp.Add(constants.DefaultValidationIDExpiryDuration).Unix())

	blsInfo, err := blockchain.ConvertToBLSProofOfPossession(publicKey, pop)
	if err != nil {
		return fmt.Errorf("failure parsing BLS info: %w", err)
	}

	aggregatorCtx, aggregatorCancel := sdkutils.GetTimedContext(constants.SignatureAggregatorTimeout)
	defer aggregatorCancel()
	signedMessage, validationID, _, err := validatormanager.InitValidatorRegistration(
		aggregatorCtx,
		app,
		network,
		rpcURL,
		chainSpec,
		false,
		"",
		payerPrivateKey,
		nodeID,
		blsInfo.PublicKey[:],
		expiry,
		remainingBalanceOwners,
		disableOwners,
		0,
		extraAggregatorPeers,
		true,
		aggregatorLogger,
		true,
		delegationFee,
		time.Duration(minimumStakeDuration)*time.Second,
		validatorManagerAddressStr,
		useACP99,
		"",
	)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("ValidationID: %s", validationID)

	deployer := subnet.NewPublicDeployer(app, kc, network)
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
	if _, err := validatormanager.FinishValidatorRegistration(
		aggregatorCtx,
		app,
		network,
		rpcURL,
		chainSpec,
		false,
		"",
		payerPrivateKey,
		validationID,
		extraAggregatorPeers,
		true,
		aggregatorLogger,
		validatorManagerAddress,
	); err != nil {
		return err
	}

	validatorWeight, err := getPoSValidatorWeight(network, chainSpec, nodeID)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("  NodeID: %s", nodeID)
	ux.Logger.PrintToUser("  Network: %s", network.Name())
	ux.Logger.PrintToUser("  Weight: %d", validatorWeight)
	ux.Logger.PrintToUser("  Balance: %.5f AVAX", float64(balance)/float64(units.Avax))
	ux.Logger.GreenCheckmarkToUser("Validator %s successfully added to the L1", nodeIDStr)
	return nil
}

func getPoSValidatorWeight(network models.Network, chainSpec contract.ChainSpec, nodeID ids.NodeID) (uint64, error) {
	pClient := platformvm.NewClient(network.Endpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return 0, err
	}
	validatorsList, err := pClient.GetValidatorsAt(ctx, subnetID, api.ProposedHeight)
	if err != nil {
		return 0, err
	}
	return validatorsList[nodeID].Weight, nil
}
