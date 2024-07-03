// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/metrics"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

const (
	forceFlag     = "force"
	latest        = "latest"
	preRelease    = "pre-release"
	explainOption = "Explain the difference"
)

type CreateFlags struct {
	chainID     uint64
	tokenSymbol string
	useDefaults bool
}

var (
	createFlags                    CreateFlags
	forceCreate                    bool
	useSubnetEvm                   bool
	genesisFile                    string
	vmFile                         string
	useCustom                      bool
	evmVersion                     string
	useLatestReleasedEvmVersion    bool
	useLatestPreReleasedEvmVersion bool
	useRepo                        bool
	useTeleporter                  bool
	useWarp                        bool

	errIllegalNameCharacter = errors.New(
		"illegal name character: only letters, no special characters allowed")
	errMutuallyExlusiveVersionOptions = errors.New("version flags --latest,--pre-release,vm-version are mutually exclusive")
	errMutuallyVMConfigOptions        = errors.New("specifying --genesis flag disables SubnetEVM config flags --evm-chain-id,--evm-token,--evm-defaults")
)

// avalanche subnet create
func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [subnetName]",
		Short: "Create a new subnet configuration",
		Long: `The subnet create command builds a new genesis file to configure your Subnet.
By default, the command runs an interactive wizard. It walks you through
all the steps you need to create your first Subnet.

The tool supports deploying Subnet-EVM, and custom VMs. You
can create a custom, user-generated genesis with a custom VM by providing
the path to your genesis and VM binaries with the --genesis and --vm flags.

By default, running the command with a subnetName that already exists
causes the command to fail. If you'd like to overwrite an existing
configuration, pass the -f flag.`,
		Args:              cobrautils.ExactArgs(1),
		RunE:              createSubnetConfig,
		PersistentPostRun: handlePostRun,
	}
	cmd.Flags().StringVar(&genesisFile, "genesis", "", "file path of genesis to use")
	cmd.Flags().BoolVar(&useSubnetEvm, "evm", false, "use the Subnet-EVM as the base template")
	cmd.Flags().StringVar(&evmVersion, "vm-version", "", "version of Subnet-EVM template to use")
	cmd.Flags().Uint64Var(&createFlags.chainID, "evm-chain-id", 0, "chain ID to use with Subnet-EVM")
	cmd.Flags().StringVar(&createFlags.tokenSymbol, "evm-token", "", "token symbol to use with Subnet-EVM")
	cmd.Flags().BoolVar(&createFlags.useDefaults, "evm-defaults", false, "use default settings for fees/airdrop/precompiles/teleporter with Subnet-EVM")
	cmd.Flags().BoolVar(&useCustom, "custom", false, "use a custom VM template")
	cmd.Flags().BoolVar(&useLatestPreReleasedEvmVersion, preRelease, false, "use latest Subnet-EVM pre-released version, takes precedence over --vm-version")
	cmd.Flags().BoolVar(&useLatestReleasedEvmVersion, latest, false, "use latest Subnet-EVM released version, takes precedence over --vm-version")
	cmd.Flags().BoolVarP(&forceCreate, forceFlag, "f", false, "overwrite the existing configuration if one exists")
	cmd.Flags().StringVar(&vmFile, "vm", "", "file path of custom vm to use. alias to custom-vm-path")
	cmd.Flags().StringVar(&vmFile, "custom-vm-path", "", "file path of custom vm to use")
	cmd.Flags().StringVar(&customVMRepoURL, "custom-vm-repo-url", "", "custom vm repository url")
	cmd.Flags().StringVar(&customVMBranch, "custom-vm-branch", "", "custom vm branch or commit")
	cmd.Flags().StringVar(&customVMBuildScript, "custom-vm-build-script", "", "custom vm build-script")
	cmd.Flags().BoolVar(&useRepo, "from-github-repo", false, "generate custom VM binary from github repository")
	cmd.Flags().BoolVar(&useWarp, "warp", true, "generate a vm with warp support (needed for teleporter)")
	cmd.Flags().BoolVar(&useTeleporter, "teleporter", false, "interoperate with other blockchains using teleporter")
	return cmd
}

func CallCreate(
	cmd *cobra.Command,
	subnetName string,
	forceCreateParam bool,
	genesisFileParam string,
	useSubnetEvmParam bool,
	useCustomParam bool,
	evmVersionParam string,
	evmChainIDParam uint64,
	tokenSymbolParam string,
	useDefaultsParam bool,
	useLatestReleasedEvmVersionParam bool,
	useLatestPreReleasedEvmVersionParam bool,
	customVMRepoURLParam string,
	customVMBranchParam string,
	customVMBuildScriptParam string,
) error {
	forceCreate = forceCreateParam
	genesisFile = genesisFileParam
	useSubnetEvm = useSubnetEvmParam
	evmVersion = evmVersionParam
	createFlags.chainID = evmChainIDParam
	createFlags.tokenSymbol = tokenSymbolParam
	createFlags.useDefaults = useDefaultsParam
	useLatestReleasedEvmVersion = useLatestReleasedEvmVersionParam
	useLatestPreReleasedEvmVersion = useLatestPreReleasedEvmVersionParam
	useCustom = useCustomParam
	customVMRepoURL = customVMRepoURLParam
	customVMBranch = customVMBranchParam
	customVMBuildScript = customVMBuildScriptParam
	return createSubnetConfig(cmd, []string{subnetName})
}

func detectVMTypeFromFlags() {
	// assumes custom
	if customVMRepoURL != "" || customVMBranch != "" || customVMBuildScript != "" {
		useCustom = true
	}
}

func moreThanOneVMSelected() bool {
	vmVars := []bool{useSubnetEvm, useCustom}
	firstSelect := false
	for _, val := range vmVars {
		if firstSelect && val {
			return true
		} else if val {
			firstSelect = true
		}
	}
	return false
}

func getVMFromFlag() models.VMType {
	if useSubnetEvm {
		return models.SubnetEvm
	}
	if useCustom {
		return models.CustomVM
	}
	return ""
}

// override postrun function from root.go, so that we don't double send metrics for the same command
func handlePostRun(_ *cobra.Command, _ []string) {}

func createSubnetConfig(cmd *cobra.Command, args []string) error {
	subnetName := args[0]
	if app.GenesisExists(subnetName) && !forceCreate {
		return errors.New("configuration already exists. Use --" + forceFlag + " parameter to overwrite")
	}

	if err := checkInvalidSubnetNames(subnetName); err != nil {
		return fmt.Errorf("subnet name %q is invalid: %w", subnetName, err)
	}

	detectVMTypeFromFlags()

	if moreThanOneVMSelected() {
		return errors.New("too many VMs selected. Provide at most one VM selection flag")
	}

	if !flags.EnsureMutuallyExclusive([]bool{useLatestReleasedEvmVersion, useLatestPreReleasedEvmVersion, evmVersion != ""}) {
		return errMutuallyExlusiveVersionOptions
	}

	if genesisFile != "" && (createFlags.chainID != 0 || createFlags.tokenSymbol != "" || createFlags.useDefaults) {
		return errMutuallyVMConfigOptions
	}

	subnetType := getVMFromFlag()

	if subnetType == "" {
		subnetEvmOption := "Subnet-EVM"
		customVMOption := "Custom VM"
		options := []string{subnetEvmOption, customVMOption, explainOption}
		var subnetTypeStr string
		for {
			option, err := app.Prompt.CaptureList(
				"VM",
				options,
			)
			if err != nil {
				return err
			}
			switch option {
			case subnetEvmOption:
				subnetTypeStr = models.SubnetEvm
			case customVMOption:
				subnetTypeStr = models.CustomVM
			case explainOption:
				ux.Logger.PrintToUser("Virtual machines are the blueprint the defines the application-level logic of a blockchain. It determines the language and rules for writing and executing smart contracts, as well as other blockchain logic.")
				ux.Logger.PrintToUser(" ")
				ux.Logger.PrintToUser("Subnet-EVM is a EVM-compatible virtual machine that supports smart contract development in Solidity. This VM is an out-of-box solution for Subnet deployers who want a dApp development experience that is nearly identical to Ethereum, without having to manage or create a custom virtual machine. Subnet-EVM can be configured with this CLI to meet the developers requirements without writing code. For more information, please visit: https://github.com/ava-labs/subnet-evm")
				ux.Logger.PrintToUser(" ")
				ux.Logger.PrintToUser("Custom VMs created with SDKs such as the Precompile-EVM, HyperSDK, Rust-SDK and that are written in golang or rust can be deployed on Avalanche using the second option. You can provide the path to the binary directly or provide the the code as well as the build script. In addition to the VM you need to provide the genesis file. More information can be found in the docs at https://docs.avax.network/learn/avalanche/virtual-machines.")
				continue
			}
			break
		}
		subnetType = models.VMTypeFromString(subnetTypeStr)
	}

	var (
		genesisBytes []byte
		sc           *models.Sidecar
		err          error
	)

	if useLatestReleasedEvmVersion {
		evmVersion = latest
	}

	if useLatestPreReleasedEvmVersion {
		evmVersion = preRelease
	}

	if evmVersion != latest && evmVersion != preRelease && evmVersion != "" && !semver.IsValid(evmVersion) {
		return fmt.Errorf("invalid version string, should be semantic version (ex: v1.1.1): %s", evmVersion)
	}

	genesisFileIsEVM := false
	if genesisFile != "" {
		genesisFileIsEVM, err = utils.PathIsSubnetEVMGenesis(genesisFile)
		if err != nil {
			return err
		}
	}

	if subnetType == models.SubnetEvm && genesisFile != "" && !genesisFileIsEVM {
		return fmt.Errorf("The provided genesis file has no proper Subnet-EVM format")
	}

	if subnetType == models.SubnetEvm {
		evmVersion, err = vm.GetVMVersion(app, constants.SubnetEVMRepoName, evmVersion)
		if err != nil {
			return err
		}
	}

	var teleporterInfo *teleporter.Info
	if useTeleporter {
		teleporterInfo, err = teleporter.GetInfo(app)
		if err != nil {
			return err
		}
	}

	switch subnetType {
	case models.SubnetEvm:
		genesisBytes, sc, err = vm.CreateEvmSubnetConfig(
			app,
			subnetName,
			genesisFile,
			evmVersion,
			true,
			createFlags.chainID,
			createFlags.tokenSymbol,
			createFlags.useDefaults,
			useWarp,
			teleporterInfo,
		)
		if err != nil {
			return err
		}
	case models.CustomVM:
		genesisBytes, sc, err = vm.CreateCustomSubnetConfig(
			app,
			subnetName,
			genesisFile,
			useRepo,
			customVMRepoURL,
			customVMBranch,
			customVMBuildScript,
			vmFile,
		)
		if err != nil {
			return err
		}
	default:
		return errors.New("not implemented")
	}

	if useTeleporter {
		sc.TeleporterReady = useTeleporter
		sc.TeleporterKey = constants.TeleporterKeyName
		sc.TeleporterVersion = teleporterInfo.Version
		if genesisFile != "" && genesisFileIsEVM {
			// evm genesis file was given. make appropriate checks and customizations for teleporter
			genesisBytes, err = addSubnetEVMGenesisPrefundedAddress(genesisBytes, teleporterInfo.FundedAddress, teleporterInfo.FundedBalance.String())
			if err != nil {
				return err
			}
		}
	}

	if err = app.WriteGenesisFile(subnetName, genesisBytes); err != nil {
		return err
	}

	sc.ImportedFromAPM = false
	if err = app.CreateSidecar(sc); err != nil {
		return err
	}
	if subnetType == models.SubnetEvm {
		err = sendMetrics(cmd, subnetType.RepoName(), subnetName)
		if err != nil {
			return err
		}
	}
	ux.Logger.GreenCheckmarkToUser("Successfully created subnet configuration")
	return nil
}

func addSubnetEVMGenesisPrefundedAddress(genesisBytes []byte, address string, balance string) ([]byte, error) {
	var genesisMap map[string]interface{}
	if err := json.Unmarshal(genesisBytes, &genesisMap); err != nil {
		return nil, err
	}
	allocI, ok := genesisMap["alloc"]
	if !ok {
		return nil, fmt.Errorf("alloc field not found on genesis")
	}
	alloc, ok := allocI.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected genesis alloc field to be map[string]interface, found %T", allocI)
	}
	trimmedAddress := strings.TrimPrefix(address, "0x")
	alloc[trimmedAddress] = map[string]interface{}{
		"balance": balance,
	}
	genesisMap["alloc"] = alloc
	return json.MarshalIndent(genesisMap, "", "  ")
}

func sendMetrics(cmd *cobra.Command, repoName, subnetName string) error {
	flags := make(map[string]string)
	flags[constants.SubnetType] = repoName
	genesis, err := app.LoadEvmGenesis(subnetName)
	if err != nil {
		return err
	}
	conf := genesis.Config.GenesisPrecompiles
	precompiles := make([]string, 6)
	for precompileName := range conf {
		precompileTag := "precompile-" + precompileName
		flags[precompileTag] = precompileName
		precompiles = append(precompiles, precompileName)
	}
	numAirdropAddresses := len(genesis.Alloc)
	for address := range genesis.Alloc {
		if address.String() != vm.PrefundedEwoqAddress.String() {
			precompileTag := "precompile-" + constants.CustomAirdrop
			flags[precompileTag] = constants.CustomAirdrop
			precompiles = append(precompiles, constants.CustomAirdrop)
			break
		}
	}
	sort.Strings(precompiles)
	precompilesJoined := strings.Join(precompiles, ",")
	flags[constants.PrecompileType] = precompilesJoined
	flags[constants.NumberOfAirdrops] = strconv.Itoa(numAirdropAddresses)
	metrics.HandleTracking(cmd, constants.MetricsSubnetCreateCommand, app, flags)
	return nil
}

func checkInvalidSubnetNames(name string) error {
	// this is currently exactly the same code as in avalanchego/vms/platformvm/create_chain_tx.go
	for _, r := range name {
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == ' ') {
			return errIllegalNameCharacter
		}
	}
	return nil
}

type InitialTokenAllocation struct {
	allocToNewKey bool
	allocToEwoq   bool
	customAddress common.Address
	customBalance uint64
}

type FeeConfig struct {
	lowThroughput    bool
	mediumThroughput bool
	highThroughput   bool
	gasLimit         *big.Int
	blockRate        *big.Int
	minBaseFee       *big.Int
	targetGas        *big.Int
	baseDenominator  *big.Int
	minBlockGas      *big.Int
	maxBlockGas      *big.Int
	gasStep          *big.Int
}

type SubnetEVMGenesisParams struct {
	chainID                             uint64
	useTeleporter                       bool
	useExternalGasToken                 bool
	initialTokenAllocation              InitialTokenAllocation
	feeConfig                           FeeConfig
	enableNativeMinterPrecompile        bool
	nativeMinterPrecompileAllowList     vm.AllowList
	enableFeeManagerPrecompile          bool
	feeManagerPrecompileAllowList       vm.AllowList
	enableRewardManagerPrecompile       bool
	rewardManagerPrecompileAllowList    vm.AllowList
	enableTransactionPrecompile         bool
	transactionPrecompileAllowList      vm.AllowList
	enableContractDeployerPrecompile    bool
	contractDeployerPrecompileAllowList vm.AllowList
	enableWarpPrecompile                bool
}

// ux to get the needed params to build a genesis for a SubnetEVM based VM
//
// if useDefaults is true, it will:
// - use native gas token, allocating 1m to a newly created key
// - customize fee config for low throughput
// - use teleporter
// - enable warp precompile
// - disable the other precompiles
// in the other case, will prompt for all these settings
//
// tokenSymbol is not needed to build a genesis but is needed in the ux flow
// as such, is returned separately from the genesis params
//
// prompts the user for chainID, tokenSymbol, and useTeleporter, unless
// provided in call args
func getSubnetEVMGenesisParams(
	cmd *cobra.Command,
	chainID uint64,
	tokenSymbol string,
	useTeleporter *bool,
	useDefaults bool,
) (SubnetEVMGenesisParams, string, error) {
	var (
		err    error
		cancel bool
		params SubnetEVMGenesisParams
	)
	// Chain ID
	params.chainID = chainID
	if params.chainID == 0 {
		params.chainID, err = app.Prompt.CaptureUint64("Chain ID")
		if err != nil {
			return SubnetEVMGenesisParams{}, "", err
		}
	}
	// Gas token
	nativeTokenOption := "The blockchain's native token"
	externalTokenOption := "A token from another blockchain"
	options := []string{nativeTokenOption, externalTokenOption, explainOption}
	for {
		option, err := app.Prompt.CaptureList(
			"Which token will be used for transaction fee payments?",
			options,
		)
		if err != nil {
			return SubnetEVMGenesisParams{}, "", err
		}
		switch option {
		case externalTokenOption:
			params.useExternalGasToken = true
		case nativeTokenOption:
			if tokenSymbol == "" {
				tokenSymbol, err = app.Prompt.CaptureString("Token Symbol")
				if err != nil {
					return SubnetEVMGenesisParams{}, "", err
				}
			}
			if useDefaults {
				params.initialTokenAllocation.allocToNewKey = true
				break
			}
			allocateToNewKeyOption := "Allocate 1m tokens to a newly created account"
			allocateToEwoqOption := "Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)"
			customAllocationOption := "Define a custom allocation (Recommended for production)"
			options := []string{allocateToNewKeyOption, allocateToEwoqOption, customAllocationOption}
			option, err := app.Prompt.CaptureList(
				"How should the initial token allocation be structured?",
				options,
			)
			if err != nil {
				return SubnetEVMGenesisParams{}, "", err
			}
			switch option {
			case allocateToNewKeyOption:
				params.initialTokenAllocation.allocToNewKey = true
			case allocateToEwoqOption:
				params.initialTokenAllocation.allocToEwoq = true
			case customAllocationOption:
				params.initialTokenAllocation.customAddress, err = app.Prompt.CaptureAddress("Address to allocate to")
				if err != nil {
					return SubnetEVMGenesisParams{}, "", err
				}
				params.initialTokenAllocation.customBalance, err = app.Prompt.CaptureUint64(fmt.Sprintf("Amount to allocate (in %s units)", tokenSymbol))
				if err != nil {
					return SubnetEVMGenesisParams{}, "", err
				}
			}
			fixedSupplyOption := "No, I want the supply of the native token be hard-capped. (Native Minter Precompile OFF)"
			dynamicSupplyOption := "Yes, I want to be able to mint additional the native tokens. (Native Minter Precompile ON)"
			options = []string{fixedSupplyOption, dynamicSupplyOption}
			option, err = app.Prompt.CaptureList(
				"Allow minting new native Tokens? (Native Minter Precompile)",
				options,
			)
			if err != nil {
				return SubnetEVMGenesisParams{}, "", err
			}
			switch option {
			case fixedSupplyOption:
			case dynamicSupplyOption:
				params.enableNativeMinterPrecompile = true
				params.nativeMinterPrecompileAllowList, cancel, err = vm.GenerateAllowList(app, "mint native tokens", evmVersion)
				if err != nil {
					return SubnetEVMGenesisParams{}, "", err
				}
				if cancel {
					return SubnetEVMGenesisParams{}, "", nil
				}
			}
		case explainOption:
			ux.Logger.PrintToUser("Every blockchain uses a token to manage access to its limited resources. For example, ETH is the native token of Ethereum, and AVAX is the native token of the Avalanche C-Chain. Users pay transaction fees with these tokens. If demand exceeds capacity, transaction fees increase, requiring users to pay more tokens for their transactions.")
			ux.Logger.PrintToUser(" ")
			ux.Logger.PrintToUser(logging.Bold.Wrap("The blockchain's native token"))
			ux.Logger.PrintToUser("Each blockchain on Avalanche has its own transaction fee token. To issue transactions users don't need to acquire ETH or AVAX and the therefore the transaction fees are completely isolated.")
			ux.Logger.PrintToUser(" ")
			ux.Logger.PrintToUser(logging.Bold.Wrap("A token from another blockchain"))
			ux.Logger.PrintToUser("You can use an ERC-20 token (e.g., USDC, WETH) or the native token (e.g., AVAX) of another blockchain within the Avalanche network as the transaction fee token. This is achieved through a bridge contract and the Native Minter Precompile. When a user bridges a token from another blockchain, it is locked on the home chain, a message is relayed to the Subnet, and the token is minted to the sender’s account.")
			ux.Logger.PrintToUser(" ")
			ux.Logger.PrintToUser("If a token from another blockchain is used, the interoperability protocol Teleporter is required and activated automatically.")
			continue
		}
		break
	}

	// Transaction / Gas Fees
	customizeOption := "Customize fee config"
	lowOption := "Low disk use    / Low Throughput    1.5 mil gas/s (C-Chain's setting)"
	mediumOption := "Medium disk use / Medium Throughput 2 mil   gas/s"
	highOption := "High disk use   / High Throughput   5 mil   gas/s"
	options = []string{lowOption, mediumOption, highOption, customizeOption, explainOption}
	for {
		option, err := app.Prompt.CaptureList(
			"How should the transaction fees be configured on your Blockchain?",
			options,
		)
		if err != nil {
			return SubnetEVMGenesisParams{}, "", err
		}
		const (
			setGasLimit                 = "Set gas limit"
			setBlockRate                = "Set target block rate"
			setMinBaseFee               = "Set min base fee"
			setTargetGas                = "Set target gas"
			setBaseFeeChangeDenominator = "Set base fee change denominator"
			setMinBlockGas              = "Set min block gas cost"
			setMaxBlockGas              = "Set max block gas cost"
			setGasStep                  = "Set block gas cost step"
		)
		switch option {
		case lowOption:
			params.feeConfig.lowThroughput = true
		case mediumOption:
			params.feeConfig.mediumThroughput = true
		case highOption:
			params.feeConfig.highThroughput = true
		case customizeOption:
			params.feeConfig.gasLimit, err = app.Prompt.CapturePositiveBigInt(setGasLimit)
			if err != nil {
				return SubnetEVMGenesisParams{}, "", err
			}
			params.feeConfig.blockRate, err = app.Prompt.CapturePositiveBigInt(setBlockRate)
			if err != nil {
				return SubnetEVMGenesisParams{}, "", err
			}
			params.feeConfig.minBaseFee, err = app.Prompt.CapturePositiveBigInt(setMinBaseFee)
			if err != nil {
				return SubnetEVMGenesisParams{}, "", err
			}
			params.feeConfig.targetGas, err = app.Prompt.CapturePositiveBigInt(setTargetGas)
			if err != nil {
				return SubnetEVMGenesisParams{}, "", err
			}
			params.feeConfig.baseDenominator, err = app.Prompt.CapturePositiveBigInt(setBaseFeeChangeDenominator)
			if err != nil {
				return SubnetEVMGenesisParams{}, "", err
			}
			params.feeConfig.minBlockGas, err = app.Prompt.CapturePositiveBigInt(setMinBlockGas)
			if err != nil {
				return SubnetEVMGenesisParams{}, "", err
			}
			params.feeConfig.maxBlockGas, err = app.Prompt.CapturePositiveBigInt(setMaxBlockGas)
			if err != nil {
				return SubnetEVMGenesisParams{}, "", err
			}
			params.feeConfig.gasStep, err = app.Prompt.CapturePositiveBigInt(setGasStep)
			if err != nil {
				return SubnetEVMGenesisParams{}, "", err
			}
		case explainOption:
			ux.Logger.PrintToUser("The two gas fee variables that have the largest impact on performance are the gas limit, the maximum amount of gas that fits in a block, and the gas target, the expected amount of gas consumed in a rolling ten-second period.")
			ux.Logger.PrintToUser(" ")
			ux.Logger.PrintToUser("By increasing the gas limit, you can fit more transactions into a single block which in turn increases your max throughput. Increasing the gas target has the same effect; if the targeted amount of gas is not consumed, the dynamic fee algorithm will decrease the base fee until it reaches the minimum.")
			ux.Logger.PrintToUser(" ")
			ux.Logger.PrintToUser("There is a long-term risk of increasing your gas parameters. By allowing more transactions to occur on your network, the network state will increase at a faster rate, meaning infrastructure costs and requirements will increase.")
			continue
		}
		break
	}
	dontChangeFeeSettingsOption := "No, use the transaction fee configuration set in the genesis block. (Fee Manager Precompile OFF)"
	changeFeeSettingsOption := "Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production. (Fee Manager Precompile ON)"
	options = []string{dontChangeFeeSettingsOption, changeFeeSettingsOption, explainOption}
	for {
		option, err := app.Prompt.CaptureList(
			"Should transaction fees be adjustable without a network upgrade? (Fee Manager Precompile)",
			options,
		)
		if err != nil {
			return SubnetEVMGenesisParams{}, "", err
		}
		switch option {
		case dontChangeFeeSettingsOption:
		case changeFeeSettingsOption:
			params.enableFeeManagerPrecompile = true
			params.feeManagerPrecompileAllowList, cancel, err = vm.GenerateAllowList(app, "adjust the gas fees", evmVersion)
			if err != nil {
				return SubnetEVMGenesisParams{}, "", err
			}
			if cancel {
				return SubnetEVMGenesisParams{}, "", nil
			}
		case explainOption:
			ux.Logger.PrintToUser("The Fee Manager Precompile enables you to give certain account the right to change the fee parameters set in the previous step on the fly without a network upgrade. This list can be dynamically changed by calling the precompile.")
			continue
		}
		break
	}
	burnFees := "Yes, I am fine with transaction fees being burned (Reward Manager Precompile OFF)"
	distributeFees := "No, I want to customize accumulated transaction fees distribution (Reward Manager Precompile ON)"
	options = []string{burnFees, distributeFees, explainOption}
	for {
		option, err := app.Prompt.CaptureList(
			"By default, all transaction fees on Avalanche are burned (sent to a blackhole address). Should the fees be burned??",
			options,
		)
		if err != nil {
			return SubnetEVMGenesisParams{}, "", err
		}
		switch option {
		case burnFees:
		case distributeFees:
			params.enableRewardManagerPrecompile = true
			params.rewardManagerPrecompileAllowList, cancel, err = vm.GenerateAllowList(app, "customize gas fees distribution", evmVersion)
			if err != nil {
				return SubnetEVMGenesisParams{}, "", err
			}
			if cancel {
				return SubnetEVMGenesisParams{}, "", nil
			}
		case explainOption:
			ux.Logger.PrintToUser("The fee reward mechanism can be configured with a stateful precompile contract called the RewardManager. The configuration can include burning fees, sending fees to a predefined address, or enabling fees to be collected by block producers. For more info, please visit: https://docs.avax.network/build/subnet/upgrade/customize-a-subnet#changing-fee-reward-mechanisms")
			continue
		}
		break
	}

	// Interoperability
	if params.useExternalGasToken {
		useTeleporter = true
	}
	if useDefaults {
		useTeleporter = true
	}
	flagName := "teleporter"
	if flag := cmd.Flags().Lookup(flagName); flag == nil {
		return SubnetEVMGenesisParams{}, "", fmt.Errorf("flag configuration %q not found for cmd %q", flagName, cmd.Use)
	} else if !flag.Changed && !params.useExternalGasToken {
		interoperatingBlockchainOption := "Yes, I want my blockchain to be able to interoperate with other blockchains and the C-Chain"
		isolatedBlockchainOption := "No, I want to run my blockchain isolated"
		options := []string{interoperatingBlockchainOption, isolatedBlockchainOption, explainOption}
		for {
			option, err := app.Prompt.CaptureList(
				"Do you want to connect your blockchain with other blockchains or the C-Chain? (Deploy Teleporter along with its Registry)",
				options,
			)
			if err != nil {
				return SubnetEVMGenesisParams{}, "", err
			}
			switch option {
			case interoperatingBlockchainOption:
				useTeleporter = true
			case isolatedBlockchainOption:
				useTeleporter = false
			case explainOption:
				ux.Logger.PrintToUser("Avalanche enables native interoperability between blockchains with the VM-agnostic Avalanche Warp Messaging protocol (AWM). Teleporter is a messaging protocol built on top of AWM that provides a developer-friendly interface for sending and receiving cross-chain messages to and from EVM-compatible blockchains. This communication protocol can be used for bridges and other protocols.")
				continue
			}
			break
		}
	}
	if useTeleporter && !useWarp {
		return SubnetEVMGenesisParams{}, "", fmt.Errorf("warp should be enabled for teleporter to work")
	}

	// Permissioning
	noOption := "No"
	yesOption := "Yes"
	options = []string{noOption, yesOption, explainOption}
	for {
		option, err := app.Prompt.CaptureList(
			"Do you want to add permissioning to your blockchain?",
			options,
		)
		if err != nil {
			return SubnetEVMGenesisParams{}, "", err
		}
		switch option {
		case yesOption:
			anyoneCanSubmitTransactionsOption := "No, I want anyone to be able to issue transactions on my blockchain. (Transaction Allow List OFF)"
			approvedCanSubmitTransactionsOption := "Yes, I want only approved addresses to issue transactions on my blockchain. (Transaction Allow List ON)"
			options := []string{anyoneCanSubmitTransactionsOption, approvedCanSubmitTransactionsOption, explainOption}
			for {
				option, err := app.Prompt.CaptureList(
					"Do you want to allow only certain addresses to issue transactions? (Transaction Allow List Precompile)",
					options,
				)
				if err != nil {
					return SubnetEVMGenesisParams{}, "", err
				}
				switch option {
				case approvedCanSubmitTransactionsOption:
					params.enableTransactionPrecompile = true
					params.transactionPrecompileAllowList, cancel, err = vm.GenerateAllowList(app, "issue transactions", evmVersion)
					if err != nil {
						return SubnetEVMGenesisParams{}, "", err
					}
					if cancel {
						return SubnetEVMGenesisParams{}, "", nil
					}
				case explainOption:
					ux.Logger.PrintToUser("The Transaction Allow List is a precompile contract that allows you to specify a list of addresses that are allowed to submit transactions to your blockchain. This list can be dynamically changed by calling the precompile.")
					ux.Logger.PrintToUser(" ")
					ux.Logger.PrintToUser("This feature is useful for permissioning your blockchain and lets you easiliy implement KYC measures. Only authorized users can send transactions or deploy smart contracts on your blockchain. For more information, please visit: https://docs.avax.network/build/subnet/upgrade/customize-a-subnet#restricting-who-can-submit-transactions.")
					continue
				}
				break
			}
			anyoneCanDeployContractsOption := "No, I want anyone to be able to deploy smart contracts on my blockchain. (Smart Contract Deployer Allow List OFF)"
			approvedCanDeployContractsOption := "Yes, I want only approved addresses to deploy smart contracts on my blockchain. (Smart Contract Deployer Allow List ON)"
			options = []string{anyoneCanDeployContractsOption, approvedCanDeployContractsOption, explainOption}
			for {
				option, err := app.Prompt.CaptureList(
					"Do you want to allow only certain addresses to deploy smart contracts on your blockchain? (Smart Contract Deployer Allow List Precompile)",
					options,
				)
				if err != nil {
					return SubnetEVMGenesisParams{}, "", err
				}
				switch option {
				case approvedCanDeployContractsOption:
					params.enableContractDeployerPrecompile = true
					params.contractDeployerPrecompileAllowList, cancel, err = vm.GenerateAllowList(app, "deploy smart contracts", evmVersion)
					if err != nil {
						return SubnetEVMGenesisParams{}, "", err
					}
					if cancel {
						return SubnetEVMGenesisParams{}, "", nil
					}
				case explainOption:
					ux.Logger.PrintToUser("While you may wish to allow anyone to interact with the contract on your blockchain to your blockchain, you may want to restrict who can deploy smart contracts and create dApps on your chain.")
					ux.Logger.PrintToUser(" ")
					ux.Logger.PrintToUser("The Smart Contract Deployer Allow List is a precompile contract that allows you to specify a list of addresses that are allowed to deploy smart contracts on your blockchain. For more information, please visit: https://docs.avax.network/build/subnet/upgrade/customize-a-subnet#restricting-smart-contract-deployers.")
					continue
				}
				break
			}
		case explainOption:
			ux.Logger.PrintToUser("You can permission your chain at different levels of interaction with EVM-Precompiles. These precompiles act as allowlists, preventing unapproved users from deploying smart contracts, sending transactions, or interacting with your blockchain. You may choose to apply as many or as little of these rules as you see fit.")
			continue
		}
		break
	}
	return SubnetEVMGenesisParams{}, "", nil
}
