// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"math/big"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/blockchain"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	warpMessage "github.com/ava-labs/avalanchego/vms/platformvm/warp/message"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/spf13/cobra"
)

var (
	localStartSupportedNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Devnet,
		networkoptions.Fuji,
		networkoptions.Mainnet,
	}
	avalanchegoBinaryPath string

	bootstrapIDs              []string
	bootstrapIPs              []string
	genesisPath               string
	upgradePath               string
	stakingTLSKeyPath         string
	stakingCertKeyPath        string
	stakingSignerKeyPath      string
	numNodes                  uint32
	nodeConfigPath            string
	partialSync               bool
	stakeAmount               uint64
	rpcURL                    string
	balance                   uint64
	clusterNameFlagValue      string
	remainingBalanceOwnerAddr string
	disableOwnerAddr          string
	aggregatorLogLevel        string
	aggregatorLogToStdout     bool
	delegationFee             uint16
	publicKey                 string
	pop                       string
)

// const snapshotName = "local_snapshot"
func newLocalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "local",
		Short: "(ALPHA Warning) Suite of commands for a local avalanche node",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node local command suite provides a collection of commands related to local nodes`,
		RunE: cobrautils.CommandSuiteUsage,
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
		Short: "(ALPHA Warning) Create a new validator on local machine",
		Long: `(ALPHA Warning) This command is currently in experimental mode. 

The node local start command sets up a validator on a local server. 
The validator will be validating the Avalanche Primary Network and Subnet 
of your choice. By default, the command runs an interactive wizard. It 
walks you through all the steps you need to set up a validator.
Once this command is completed, you will have to wait for the validator
to finish bootstrapping on the primary network before running further
commands on it, e.g. validating a Subnet. You can check the bootstrapping
status by running avalanche node status local 
`,
		Args:              cobra.ExactArgs(1),
		RunE:              localStartNode,
		PersistentPostRun: handlePostRun,
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, localStartSupportedNetworkOptions)
	cmd.Flags().BoolVar(&useLatestAvalanchegoReleaseVersion, "latest-avalanchego-version", false, "install latest avalanchego release version on node/s")
	cmd.Flags().BoolVar(&useLatestAvalanchegoPreReleaseVersion, "latest-avalanchego-pre-release-version", true, "install latest avalanchego pre-release version on node/s")
	cmd.Flags().StringVar(&useCustomAvalanchegoVersion, "custom-avalanchego-version", "", "install given avalanchego version on node/s")
	cmd.Flags().StringVar(&avalanchegoBinaryPath, "avalanchego-path", "", "use this avalanchego binary path")
	cmd.Flags().StringArrayVar(&bootstrapIDs, "bootstrap-id", []string{}, "nodeIDs of bootstrap nodes")
	cmd.Flags().StringArrayVar(&bootstrapIPs, "bootstrap-ip", []string{}, "IP:port pairs of bootstrap nodes")
	cmd.Flags().StringVar(&genesisPath, "genesis", "", "path to genesis file")
	cmd.Flags().StringVar(&upgradePath, "upgrade", "", "path to upgrade file")
	cmd.Flags().StringVar(&stakingTLSKeyPath, "staking-tls-key-path", "", "path to provided staking tls key for node")
	cmd.Flags().StringVar(&stakingCertKeyPath, "staking-cert-key-path", "", "path to provided staking cert key for node")
	cmd.Flags().StringVar(&stakingSignerKeyPath, "staking-signer-key-path", "", "path to provided staking signer key for node")
	cmd.Flags().Uint32Var(&numNodes, "num-nodes", 1, "number of nodes to start")
	cmd.Flags().StringVar(&nodeConfigPath, "node-config", "", "path to common avalanchego config settings for all nodes")
	cmd.Flags().BoolVar(&partialSync, "partial-sync", true, "primary network partial sync")
	return cmd
}

func newLocalStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "(ALPHA Warning) Stop local node",
		Long:  `Stop local node.`,
		Args:  cobra.ExactArgs(0),
		RunE:  localStopNode,
	}
}

func newLocalTrackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "track [clusterName] [blockchainName]",
		Short: "(ALPHA Warning) make the local node at the cluster to track given blockchain",
		Long:  "(ALPHA Warning) make the local node at the cluster to track given blockchain",
		Args:  cobra.ExactArgs(2),
		RunE:  localTrack,
	}
	cmd.Flags().StringVar(&avalanchegoBinaryPath, "avalanchego-path", "", "use this avalanchego binary path")
	cmd.Flags().BoolVar(&useLatestAvalanchegoReleaseVersion, "latest-avalanchego-version", false, "install latest avalanchego release version on node/s")
	cmd.Flags().BoolVar(&useLatestAvalanchegoPreReleaseVersion, "latest-avalanchego-pre-release-version", true, "install latest avalanchego pre-release version on node/s")
	cmd.Flags().StringVar(&useCustomAvalanchegoVersion, "custom-avalanchego-version", "", "install given avalanchego version on node/s")
	return cmd
}

func newLocalDestroyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "destroy [clusterName]",
		Short: "(ALPHA Warning) Cleanup local node",
		Long:  `Cleanup local node.`,
		Args:  cobra.ExactArgs(1),
		RunE:  localDestroyNode,
	}
}

func newLocalStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "(ALPHA Warning) Get status of local node",
		Long:  `Get status of local node.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  localStatus,
	}

	cmd.Flags().StringVar(&blockchainName, "subnet", "", "specify the blockchain the node is syncing with")
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
	if useCustomAvalanchegoVersion != "" {
		useLatestAvalanchegoReleaseVersion = false
		useLatestAvalanchegoPreReleaseVersion = false
	}
	avaGoVersionSetting := node.AvalancheGoVersionSettings{
		UseCustomAvalanchegoVersion:           useCustomAvalanchegoVersion,
		UseLatestAvalanchegoPreReleaseVersion: useLatestAvalanchegoPreReleaseVersion,
		UseLatestAvalanchegoReleaseVersion:    useLatestAvalanchegoReleaseVersion,
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
		models.Network{},
		globalNetworkFlags,
		localStartSupportedNetworkOptions,
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
			useLatestAvalanchegoReleaseVersion = false
			useLatestAvalanchegoPreReleaseVersion = false
		}
		avaGoVersionSetting := node.AvalancheGoVersionSettings{
			UseCustomAvalanchegoVersion:           useCustomAvalanchegoVersion,
			UseLatestAvalanchegoPreReleaseVersion: useLatestAvalanchegoPreReleaseVersion,
			UseLatestAvalanchegoReleaseVersion:    useLatestAvalanchegoReleaseVersion,
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
		Use:   "status",
		Short: "(ALPHA Warning) Get status of local node",
		Long:  `Get status of local node.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  localValidate,
	}

	cmd.Flags().StringVar(&blockchainName, "subnet", "", "specify the blockchain the node is syncing with")
	cmd.Flags().StringVar(&blockchainName, "blockchain", "", "specify the blockchain the node is syncing with")
	cmd.Flags().Uint64Var(&stakeAmount, "stake-amount", 0, "(PoS only) amount of tokens to stake")
	cmd.Flags().StringVar(&rpcURL, "rpc", "", "connect to validator manager at the given rpc endpoint")
	cmd.Flags().Uint64Var(&balance, "balance", 0, "set the AVAX balance of the validator that will be used for continuous fee on P-Chain")
	cmd.Flags().Uint16Var(&delegationFee, "delegation-fee", 100, "(PoS only) delegation fee (in bips)")
	// TODO: do we need these below?
	cmd.Flags().StringVar(&aggregatorLogLevel, "aggregator-log-level", constants.DefaultAggregatorLogLevel, "log level to use with signature aggregator")
	cmd.Flags().BoolVar(&aggregatorLogToStdout, "aggregator-log-to-stdout", false, "use stdout for signature aggregator logs")

	return cmd
}

func localValidate(_ *cobra.Command, args []string) error {
	clusterName := ""
	if len(args) > 0 {
		clusterName = args[0]
	}

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		true,
		false,
		[]networkoptions.NetworkOption{},
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

	// should take input prior to here for stake amount, delegation fee, and min stake duration
	if stakeAmount == 0 {
		stakeAmount, err = app.Prompt.CaptureUint64Compare(
			"Enter the amount of token to stake",
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
	duration = genesis.FujiParams.MinStakeDuration

	//ux.Logger.PrintToUser(logging.Yellow.Wrap("Validation manager owner %s pays for the initialization of the validator's registration (Blockchain gas token)"), sc.ValidatorManagerOwner)
	chainSpec := contract.ChainSpec{}
	if rpcURL == "" {
		rpcURL, _, err = contract.GetBlockchainEndpoints(
			app,
			models.NewFujiNetwork(),
			chainSpec,
			true,
			false,
		)
		if err != nil {
			return err
		}
	}

	ux.Logger.PrintToUser(logging.Yellow.Wrap("RPC Endpoint: %s"), rpcURL)

	if balance == 0 {
		availableBalance, err := utils.GetNetworkBalance(kc.Addresses().List(), network.Endpoint)
		if err != nil {
			return err
		}
		balance, err = blockchain.PromptValidatorBalance(app, availableBalance/units.Avax)
		if err != nil {
			return err
		}
	} else {
		// convert to nanoAVAX
		balance *= units.Avax
	}

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

	extraAggregatorPeers, err := blockchain.GetAggregatorExtraPeers(app, clusterNameFlagValue, []string{})
	if err != nil {
		return err
	}
	aggregatorLogger, err := utils.NewLogger(
		"signature-aggregator",
		aggregatorLogLevel,
		constants.DefaultAggregatorLogLevel,
		app.GetAggregatorLogDir(clusterNameFlagValue),
		aggregatorLogToStdout,
		ux.Logger.PrintToUser,
	)
	if err != nil {
		return err
	}
	var nodeIDStr string
	// get node data
	nodeInfo, err := node.GetNodeInfo(clusterName)
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
	blockchainTimestamp, err := blockchain.GetBlockchainTimestamp(network)
	if err != nil {
		return fmt.Errorf("failed to get blockchain timestamp: %w", err)
	}
	expiry := uint64(blockchainTimestamp.Add(constants.DefaultValidationIDExpiryDuration).Unix())

	blsInfo, err := blockchain.GetBLSInfo(publicKey, pop)
	if err != nil {
		return fmt.Errorf("failure parsing BLS info: %w", err)
	}
	payerPrivateKey := ""
	ux.Logger.PrintToUser("A private key is needed to pay for the contract deploy fees.")
	ux.Logger.PrintToUser("It will also be considered the owner address of the contract, beign able to call")
	ux.Logger.PrintToUser("the contract methods only available to owners.")
	payerPrivateKey, err = prompts.PromptPrivateKey(
		app.Prompt,
		"deploy the contract",
		app.GetKeyDir(),
		app.GetKey,
		"",
		"",
	)
	if err != nil {
		return err
	}
	signedMessage, validationID, err := validatormanager.InitValidatorRegistration(
		app,
		network,
		rpcURL,
		chainSpec,
		payerPrivateKey,
		nodeID,
		blsInfo.PublicKey[:],
		expiry,
		remainingBalanceOwners,
		disableOwners,
		weight,
		extraAggregatorPeers,
		true,
		aggregatorLogger,
		true,
		delegationFee,
		duration,
		big.NewInt(int64(stakeAmount)),
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

	return validatormanager.FinishValidatorRegistration(
		app,
		network,
		rpcURL,
		chainSpec,
		payerPrivateKey,
		validationID,
		extraAggregatorPeers,
		true,
		aggregatorLogger,
	)
}
