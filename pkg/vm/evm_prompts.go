// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"fmt"
	"math/big"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/plugin/evm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/olekukonko/tablewriter"

	"github.com/ava-labs/avalanchego/utils/logging"
)

type DefaultsKind uint

const (
	NoDefaults DefaultsKind = iota
	TestDefaults
	ProductionDefaults
)

const (
	latest                       = "latest"
	preRelease                   = "pre-release"
	explainOption                = "Explain the difference"
	enableExternalGasTokenPrompt = false

	// Options for native token allocation in genesis configuration
	allocateToNewKeyOption = "Allocate 1m tokens to a new account"
	allocateToEwoqOption   = "Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)"
	customAllocationOption = "Define a custom allocation (Recommended for production)"

	// Options for native minter precompile configuration
	fixedSupplyOption   = "No, I want the supply of the native tokens be hard-capped"
	dynamicSupplyOption = "Yes, I want to be able to mint additional the native tokens (Native Minter Precompile ON)"

	// Options for modifying the initial token allocation
	addAddressAllocationOption     = "Add an address to the initial token allocation"
	changeAddressAllocationOption  = "Edit the amount of an address in the initial token allocation"
	removeAddressAllocationOption  = "Remove an address from the initial token allocation"
	previewAddressAllocationOption = "Preview the initial token allocation"
	confirmAddressAllocationOption = "Confirm and finalize the initial token allocation"
)

type FeeConfig struct {
	lowThroughput    bool
	mediumThroughput bool
	highThroughput   bool
	useDynamicFees   bool
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
	UseTeleporter                       bool
	UseExternalGasToken                 bool
	initialTokenAllocation              core.GenesisAlloc
	feeConfig                           FeeConfig
	enableNativeMinterPrecompile        bool
	nativeMinterPrecompileAllowList     AllowList
	enableFeeManagerPrecompile          bool
	feeManagerPrecompileAllowList       AllowList
	enableRewardManagerPrecompile       bool
	rewardManagerPrecompileAllowList    AllowList
	enableTransactionPrecompile         bool
	transactionPrecompileAllowList      AllowList
	enableContractDeployerPrecompile    bool
	contractDeployerPrecompileAllowList AllowList
	enableWarpPrecompile                bool
	UsePoAValidatorManager              bool
	UsePoSValidatorManager              bool
}

func PromptTokenSymbol(
	app *application.Avalanche,
	tokenSymbol string,
) (string, error) {
	if tokenSymbol != "" {
		return tokenSymbol, nil
	}
	return app.Prompt.CaptureString("Token Symbol")
}

func PromptVMType(
	app *application.Avalanche,
	useSubnetEvm bool,
	useCustom bool,
) (models.VMType, error) {
	if useSubnetEvm {
		return models.SubnetEvm, nil
	}
	if useCustom {
		return models.CustomVM, nil
	}
	subnetEvmOption := "Subnet-EVM"
	customVMOption := "Custom VM"
	options := []string{subnetEvmOption, customVMOption, explainOption}
	var subnetTypeStr string
	for {
		option, err := app.Prompt.CaptureList(
			"Which Virtual Machine would you like to use?",
			options,
		)
		if err != nil {
			return "", err
		}
		switch option {
		case subnetEvmOption:
			subnetTypeStr = models.SubnetEvm
		case customVMOption:
			subnetTypeStr = models.CustomVM
		case explainOption:
			ux.Logger.PrintToUser("Virtual machines are the blueprint the defines the application-level logic of a blockchain. It determines the language and rules for writing and executing smart contracts, as well as other blockchain logic.")
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser("Subnet-EVM is an EVM-compatible virtual machine that supports smart contract development in Solidity. This VM is an out-of-the-box solution for Blockchain deployers who want a dApp development experience that is nearly identical to Ethereum, without having to manage or create a custom virtual machine. For more information, please visit: https://github.com/ava-labs/subnet-evm")
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser("Custom VMs are virtual machines created using SDKs such as Precompile-EVM, HyperSDK, Rust-SDK. For more information please visit: https://docs.avax.network/learn/avalanche/virtual-machines.")
			continue
		}
		break
	}
	return models.VMTypeFromString(subnetTypeStr), nil
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
func PromptSubnetEVMGenesisParams(
	app *application.Avalanche,
	sc *models.Sidecar,
	version string,
	chainID uint64,
	tokenSymbol string,
	blockchainName string,
	useTeleporter *bool,
	defaultsKind DefaultsKind,
	useWarp bool,
	useExternalGasToken bool,
) (SubnetEVMGenesisParams, string, error) {
	var (
		err    error
		params SubnetEVMGenesisParams
	)
	params.initialTokenAllocation = core.GenesisAlloc{}

	if sc.PoA() {
		params.UsePoAValidatorManager = true
		params.initialTokenAllocation[common.HexToAddress(sc.PoAValidatorManagerOwner)] = core.GenesisAccount{
			Balance: defaultPoAOwnerBalance,
		}
	}

	if sc.PoS() {
		params.UsePoSValidatorManager = true

		params.enableNativeMinterPrecompile = true
		params.nativeMinterPrecompileAllowList.AdminAddresses = append(
			params.nativeMinterPrecompileAllowList.AdminAddresses,
			common.HexToAddress(validatorManagerSDK.ProxyContractAddress),
		)
		params.enableRewardManagerPrecompile = true
	}

	// Chain ID
	params.chainID = chainID
	if params.chainID == 0 {
		params.chainID, err = app.Prompt.CaptureUint64("Chain ID")
		if err != nil {
			return SubnetEVMGenesisParams{}, "", err
		}
	}

	// Gas Kind
	params, err = promptGasTokenKind(app, defaultsKind, useExternalGasToken, params)
	if err != nil {
		return SubnetEVMGenesisParams{}, "", err
	}

	// Native Gas Details
	if !params.UseExternalGasToken {
		params, tokenSymbol, err = promptNativeGasToken(app, version, tokenSymbol, blockchainName, defaultsKind, params)
		if err != nil {
			return SubnetEVMGenesisParams{}, "", err
		}
	}

	// Transaction / Gas Fees
	params, err = promptFeeConfig(app, version, defaultsKind, params)
	if err != nil {
		return SubnetEVMGenesisParams{}, "", err
	}

	// Interoperability
	params.UseTeleporter, err = PromptInterop(app, useTeleporter, defaultsKind, params.UseExternalGasToken)
	if err != nil {
		return SubnetEVMGenesisParams{}, "", err
	}

	// Warp
	params.enableWarpPrecompile = useWarp
	if (params.UseTeleporter || params.UseExternalGasToken) && !params.enableWarpPrecompile {
		return SubnetEVMGenesisParams{}, "", fmt.Errorf("warp should be enabled for teleporter to work")
	}

	// Permissioning
	params, err = promptPermissioning(app, version, defaultsKind, params)
	if err != nil {
		return SubnetEVMGenesisParams{}, "", err
	}

	if sc.PoS() || sc.PoA() { // Teleporter bytecode makes genesis too big given the current max size (we include the bytecode for ValidatorManager, a proxy, and proxy admin)
		params.UseTeleporter = false
	}

	return params, tokenSymbol, nil
}

// prompts for wether to use a remote or native gas token
func promptGasTokenKind(
	app *application.Avalanche,
	defaultsKind DefaultsKind,
	useExternalGasToken bool,
	params SubnetEVMGenesisParams,
) (SubnetEVMGenesisParams, error) {
	if useExternalGasToken {
		params.UseExternalGasToken = true
	} else if enableExternalGasTokenPrompt && defaultsKind == NoDefaults {
		var err error
		nativeTokenOption := "The blockchain's native token"
		externalTokenOption := "A token from another blockchain"
		options := []string{nativeTokenOption, externalTokenOption, explainOption}
		for {
			var option string
			if enableExternalGasTokenPrompt {
				option, err = app.Prompt.CaptureList(
					"Which token will be used for transaction fee payments?",
					options,
				)
				if err != nil {
					return SubnetEVMGenesisParams{}, err
				}
			} else {
				option = nativeTokenOption
			}
			switch option {
			case externalTokenOption:
				params.UseExternalGasToken = true
			case nativeTokenOption:
			case explainOption:
				ux.Logger.PrintToUser("Every blockchain uses a token to manage access to its limited resources. For example, ETH is the native token of Ethereum, and AVAX is the native token of the Avalanche C-Chain. Users pay transaction fees with these tokens. If demand exceeds capacity, transaction fees increase, requiring users to pay more tokens for their transactions.")
				ux.Logger.PrintToUser("")
				ux.Logger.PrintToUser(logging.Bold.Wrap("The blockchain's native token"))
				ux.Logger.PrintToUser("Each blockchain on Avalanche has its own transaction fee token. To issue transactions users don't need to acquire ETH or AVAX and therefore the transaction fees are completely isolated.")
				ux.Logger.PrintToUser("")
				ux.Logger.PrintToUser(logging.Bold.Wrap("A token from another blockchain"))
				ux.Logger.PrintToUser("Use an ERC-20 token (USDC, WETH, etc.) or the native token (e.g. AVAX) of another blockchain within the Avalanche network as the transaction fee token.")
				ux.Logger.PrintToUser("")
				ux.Logger.PrintToUser("If a token from another blockchain is used, the interoperability protocol Teleporter will be activated automatically. For more info on Teleporter, visit: https://github.com/ava-labs/teleporter")
				continue
			}
			break
		}
	}
	return params, nil
}

// prompts for wether to use defaults to build the config
func PromptDefaults(
	app *application.Avalanche,
	defaultsKind DefaultsKind,
) (DefaultsKind, error) {
	if defaultsKind == NoDefaults {
		useTestDefaultsOption := "I want to use defaults for a test environment"
		useProductionDefaultsOption := "I want to use defaults for a production environment"
		specifyMyValuesOption := "I don't want to use default values"
		options := []string{useTestDefaultsOption, useProductionDefaultsOption, specifyMyValuesOption, explainOption}
		for {
			option, err := app.Prompt.CaptureList(
				"Do you want to use default values for the Blockchain configuration?",
				options,
			)
			if err != nil {
				return NoDefaults, err
			}
			switch option {
			case useTestDefaultsOption:
				defaultsKind = TestDefaults
			case useProductionDefaultsOption:
				defaultsKind = ProductionDefaults
			case specifyMyValuesOption:
				defaultsKind = NoDefaults
			case explainOption:
				ux.Logger.PrintToUser("Blockchain configuration default values:\n- Use latest Subnet-EVM release\n- Allocate 1 million tokens to:\n   - a newly created key (production)\n   - ewoq - %s (test)\n- Supply of the native token will be hard-capped\n- Set gas fee config as low throughput (12 mil gas per block)\n- Use constant gas prices\n- Disable further adjustments in transaction fee configuration\n- Transaction fees are burned\n- Enable interoperation with other blockchains\n- Allow any user to deploy smart contracts, send transactions, and interact with your blockchain.\n", PrefundedEwoqAddress.Hex())
				continue
			}
			break
		}
	}
	return defaultsKind, nil
}

func displayAllocations(alloc core.GenesisAlloc) {
	header := []string{"Address", "Balance"}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	table.SetAutoMergeCells(true)
	table.SetRowLine(true)
	for address, account := range alloc {
		table.Append([]string{address.Hex(), utils.FormatAmount(account.Balance, 18)})
	}
	table.Render()
}

func addNewKeyAllocation(allocations core.GenesisAlloc, app *application.Avalanche, subnetName string) error {
	keyName := utils.GetDefaultBlockchainAirdropKeyName(subnetName)
	k, err := app.GetKey(keyName, models.NewLocalNetwork(), true)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("prefunding address %s with balance %s", k.C(), defaultEVMAirdropAmount)
	allocations[common.HexToAddress(k.C())] = core.GenesisAccount{
		Balance: defaultEVMAirdropAmount,
	}
	return nil
}

func addEwoqAllocation(allocations core.GenesisAlloc) {
	allocations[PrefundedEwoqAddress] = core.GenesisAccount{
		Balance: defaultEVMAirdropAmount,
	}
}

func getNativeGasTokenAllocationConfig(
	allocations core.GenesisAlloc,
	app *application.Avalanche,
	subnetName string,
	tokenSymbol string,
) error {
	// Get the type of initial token allocation from the user prompt.
	allocOption, err := app.Prompt.CaptureList(
		"How should the initial token allocation be structured?",
		[]string{allocateToNewKeyOption, allocateToEwoqOption, customAllocationOption},
	)
	if err != nil {
		return err
	}

	// If the user chooses to allocate to a new key, generate a new key and allocate the default amount to it.
	if allocOption == allocateToNewKeyOption {
		return addNewKeyAllocation(allocations, app, subnetName)
	}

	if allocOption == allocateToEwoqOption {
		ux.Logger.PrintToUser("prefunding address %s with balance %s", PrefundedEwoqAddress, defaultEVMAirdropAmount)
		addEwoqAllocation(allocations)
		return nil
	}

	if allocOption == customAllocationOption {
		if len(allocations) != 0 {
			fmt.Println()
			fmt.Println(logging.Bold.Wrap("Addresses automatically allocated"))
			displayAllocations(allocations)
		}
		for {
			// Prompt for the action the user wants to take on the allocation list.
			action, err := app.Prompt.CaptureList(
				"How would you like to modify the initial token allocation?",
				[]string{
					addAddressAllocationOption,
					changeAddressAllocationOption,
					removeAddressAllocationOption,
					previewAddressAllocationOption,
					confirmAddressAllocationOption,
				},
			)
			if err != nil {
				return err
			}

			switch action {
			case addAddressAllocationOption:
				address, err := app.Prompt.CaptureAddress("Address to allocate to")
				if err != nil {
					return err
				}

				// Check if the address already has an allocation entry.
				if _, ok := allocations[address]; ok {
					ux.Logger.PrintToUser("Address already has an allocation entry. Use edit or remove to modify.")
					continue
				}

				balance, err := app.Prompt.CaptureUint64(fmt.Sprintf("Amount to allocate (in %s units)", tokenSymbol))
				if err != nil {
					return err
				}

				allocations[address] = core.GenesisAccount{
					Balance: new(big.Int).Mul(new(big.Int).SetUint64(balance), OneAvax),
				}
			case changeAddressAllocationOption:
				address, err := app.Prompt.CaptureAddress("Address to update the allocation of")
				if err != nil {
					return err
				}

				// Check the address has an existing allocation entry.
				if _, ok := allocations[address]; !ok {
					ux.Logger.PrintToUser("Address not found in the allocation list")
					continue
				}

				balance, err := app.Prompt.CaptureUint64(fmt.Sprintf("Updated amount to allocate (in %s units)", tokenSymbol))
				if err != nil {
					return err
				}
				allocations[address] = core.GenesisAccount{
					Balance: new(big.Int).Mul(new(big.Int).SetUint64(balance), OneAvax),
				}
			case removeAddressAllocationOption:
				address, err := app.Prompt.CaptureAddress("Address to remove from the allocation list")
				if err != nil {
					return err
				}

				// Check the address has an existing allocation entry.
				if _, ok := allocations[address]; !ok {
					ux.Logger.PrintToUser("Address not found in the allocation list")
					continue
				}

				delete(allocations, address)
			case previewAddressAllocationOption:
				displayAllocations(allocations)
			case confirmAddressAllocationOption:
				displayAllocations(allocations)
				confirm, err := app.Prompt.CaptureYesNo("Are you sure you want to finalize this allocation list?")
				if err != nil {
					return err
				}
				if confirm {
					return nil
				}
			default:
				return fmt.Errorf("invalid allocation modification option")
			}
		}
	}
	return fmt.Errorf("invalid allocation option")
}

func getNativeMinterPrecompileConfig(
	app *application.Avalanche,
	alreadyEnabled bool,
	allowList AllowList,
	version string,
) (AllowList, bool, error) {
	if !alreadyEnabled {
		option, err := app.Prompt.CaptureList(
			"Allow minting of new native tokens?",
			[]string{fixedSupplyOption, dynamicSupplyOption},
		)
		if err != nil {
			return AllowList{}, false, err
		}
		if option == fixedSupplyOption {
			return AllowList{}, false, nil
		}
	} else {
		confirm, err := app.Prompt.CaptureYesNo("Minting of native tokens automatically enabled. Do you want to configure allow list?")
		if err != nil {
			return AllowList{}, false, err
		}
		if !confirm {
			return AllowList{}, false, nil
		}
	}

	for {
		allowList, cancel, err := GenerateAllowList(app, allowList, "mint native tokens", version)
		if err != nil {
			return AllowList{}, false, err
		}
		if cancel {
			continue
		}
		return allowList, true, nil
	}
}

// prompts for token symbol, initial token allocation, and native minter precompile
// configuration
//
// if tokenSymbol is not defined, will prompt for it
// is useDefaults is true, will:
// - use native gas token, allocating 1m to a newly created key
// - disable native minter precompile
func promptNativeGasToken(
	app *application.Avalanche,
	version string,
	tokenSymbol string,
	blockchainName string,
	defaultsKind DefaultsKind,
	params SubnetEVMGenesisParams,
) (SubnetEVMGenesisParams, string, error) {
	var err error
	tokenSymbol, err = PromptTokenSymbol(app, tokenSymbol)
	if err != nil {
		return SubnetEVMGenesisParams{}, "", err
	}

	if defaultsKind == TestDefaults {
		ux.Logger.PrintToUser("prefunding address %s with balance %s", PrefundedEwoqAddress, defaultEVMAirdropAmount)
		addEwoqAllocation(params.initialTokenAllocation)
		return params, tokenSymbol, nil
	}

	if defaultsKind == ProductionDefaults {
		err = addNewKeyAllocation(params.initialTokenAllocation, app, blockchainName)
		return params, tokenSymbol, err
	}

	// No defaults case. Prompt for initial token allocation and native minter precompile options.
	if err := getNativeGasTokenAllocationConfig(params.initialTokenAllocation, app, blockchainName, tokenSymbol); err != nil {
		return SubnetEVMGenesisParams{}, "", err
	}

	allowList, nativeMinterEnabled, err := getNativeMinterPrecompileConfig(
		app,
		params.enableNativeMinterPrecompile,
		params.nativeMinterPrecompileAllowList,
		version,
	)
	if err != nil {
		return SubnetEVMGenesisParams{}, "", err
	}

	params.enableNativeMinterPrecompile = nativeMinterEnabled
	params.nativeMinterPrecompileAllowList = allowList
	return params, tokenSymbol, nil
}

// prompts for transaction fees, fee manager precompile
// and reward manager precompile configuration
//
// is useDefaults is true, will:
// - customize fee config for low throughput
// - disable fee manager precompile
// - disable reward manager precompile
func promptFeeConfig(
	app *application.Avalanche,
	version string,
	defaultsKind DefaultsKind,
	params SubnetEVMGenesisParams,
) (SubnetEVMGenesisParams, error) {
	if defaultsKind != NoDefaults {
		params.feeConfig.lowThroughput = true
		params.feeConfig.useDynamicFees = false
		return params, nil
	}
	var cancel bool
	customizeOption := "Customize fee config"
	lowOption := "Low block size    / Low Throughput    12 mil gas per block"
	mediumOption := "Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)"
	highOption := "High block size   / High Throughput   20 mil gas per block"
	options := []string{lowOption, mediumOption, highOption, customizeOption, explainOption}
	for {
		option, err := app.Prompt.CaptureList(
			"How should the transaction fees be configured on your Blockchain?",
			options,
		)
		if err != nil {
			return SubnetEVMGenesisParams{}, err
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
				return SubnetEVMGenesisParams{}, err
			}
			params.feeConfig.blockRate, err = app.Prompt.CapturePositiveBigInt(setBlockRate)
			if err != nil {
				return SubnetEVMGenesisParams{}, err
			}
			params.feeConfig.minBaseFee, err = app.Prompt.CapturePositiveBigInt(setMinBaseFee)
			if err != nil {
				return SubnetEVMGenesisParams{}, err
			}
			params.feeConfig.targetGas, err = app.Prompt.CapturePositiveBigInt(setTargetGas)
			if err != nil {
				return SubnetEVMGenesisParams{}, err
			}
			params.feeConfig.baseDenominator, err = app.Prompt.CapturePositiveBigInt(setBaseFeeChangeDenominator)
			if err != nil {
				return SubnetEVMGenesisParams{}, err
			}
			params.feeConfig.minBlockGas, err = app.Prompt.CapturePositiveBigInt(setMinBlockGas)
			if err != nil {
				return SubnetEVMGenesisParams{}, err
			}
			params.feeConfig.maxBlockGas, err = app.Prompt.CapturePositiveBigInt(setMaxBlockGas)
			if err != nil {
				return SubnetEVMGenesisParams{}, err
			}
			params.feeConfig.gasStep, err = app.Prompt.CapturePositiveBigInt(setGasStep)
			if err != nil {
				return SubnetEVMGenesisParams{}, err
			}
		case explainOption:
			ux.Logger.PrintToUser("Gas limit is the maximum amount of gas that fits in a block and gas target is the expected amount of gas consumed in a rolling ten-second period")
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser("Higher gas limit and higher gas target both increase your max throughput. If the targeted amount of gas is not consumed, the dynamic fee algorithm will decrease the base fee until it reaches the minimum.")
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser("By allowing more transactions to occur on your network, the network state will increase at a faster rate, which will lead to higher infrastructure costs.")
			continue
		}
		break
	}
	dontUseDynamicFeesOption := "No, I prefer to have constant gas prices"
	useDynamicFeesOption := "Yes, I would like my blockchain to have dynamic fees"
	options = []string{dontUseDynamicFeesOption, useDynamicFeesOption, explainOption}
	for {
		option, err := app.Prompt.CaptureList(
			"Do you want dynamic fees on your blockchain?",
			options,
		)
		if err != nil {
			return SubnetEVMGenesisParams{}, err
		}
		switch option {
		case dontUseDynamicFeesOption:
			params.feeConfig.useDynamicFees = false
		case useDynamicFeesOption:
			params.feeConfig.useDynamicFees = true
		case explainOption:
			ux.Logger.PrintToUser("By disabling dynamic fees you effectively make your gas fees constant. In that case, you may\nwant to have your own congestion control, by fully controlling activity on the chain.\nIf setting dynamic fees, gas fees will be automatically adjusted giving automatic congestion control.")
			continue
		}
		break
	}
	dontChangeFeeSettingsOption := "No, use the transaction fee configuration set in the genesis block"
	changeFeeSettingsOption := "Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production (Fee Manager Precompile ON)"
	options = []string{dontChangeFeeSettingsOption, changeFeeSettingsOption, explainOption}
	for {
		option, err := app.Prompt.CaptureList(
			"Should transaction fees be adjustable without a network upgrade?",
			options,
		)
		if err != nil {
			return SubnetEVMGenesisParams{}, err
		}
		switch option {
		case dontChangeFeeSettingsOption:
		case changeFeeSettingsOption:
			params.feeManagerPrecompileAllowList, cancel, err = GenerateAllowList(app, AllowList{}, "adjust the gas fees", version)
			if err != nil {
				return SubnetEVMGenesisParams{}, err
			}
			if cancel {
				continue
			}
			params.enableFeeManagerPrecompile = true
		case explainOption:
			ux.Logger.PrintToUser("The Fee Manager Precompile enables specified accounts to change the fee parameters without a network upgrade.")
			continue
		}
		break
	}
	burnFees := "Yes, I want the transaction fees to be burned"
	distributeFees := "No, I want to customize accumulated transaction fees distribution (Reward Manager Precompile ON)"
	options = []string{burnFees, distributeFees, explainOption}
	for {
		option, err := app.Prompt.CaptureList(
			"Do you want the transaction fees to be burned (sent to a blackhole address)? All transaction fees on Avalanche are burned by default",
			options,
		)
		if err != nil {
			return SubnetEVMGenesisParams{}, err
		}
		switch option {
		case burnFees:
		case distributeFees:
			params.rewardManagerPrecompileAllowList, cancel, err = GenerateAllowList(app, AllowList{}, "customize gas fees distribution", version)
			if err != nil {
				return SubnetEVMGenesisParams{}, err
			}
			if cancel {
				continue
			}
			params.enableRewardManagerPrecompile = true
		case explainOption:
			ux.Logger.PrintToUser("Fee reward mechanism is configured with stateful precompile contract RewardManager. The configuration can include burning fees, sending fees to a predefined address, or enabling fees to be collected by block producers. For more info, please visit: https://docs.avax.network/build/subnet/upgrade/customize-a-subnet#changing-fee-reward-mechanisms")
			continue
		}
		break
	}
	return params, nil
}

// if useTeleporter is defined, will enable/disable teleporter based on it
// is useDefaults is true, will enable teleporter
// if using external gas token, will assume teleporter to be enabled
// if other cases, prompts the user for wether to enable teleporter
func PromptInterop(
	app *application.Avalanche,
	useTeleporterFlag *bool,
	defaultsKind DefaultsKind,
	useExternalGasToken bool,
) (bool, error) {
	switch {
	case useTeleporterFlag != nil:
		return *useTeleporterFlag, nil
	case defaultsKind != NoDefaults:
		return true, nil
	case useExternalGasToken:
		return true, nil
	default:
		interoperatingBlockchainOption := "Yes, I want to enable my blockchain to interoperate with other blockchains and the C-Chain"
		isolatedBlockchainOption := "No, I want to run my blockchain isolated"
		options := []string{interoperatingBlockchainOption, isolatedBlockchainOption, explainOption}
		for {
			option, err := app.Prompt.CaptureList(
				"Do you want to connect your blockchain with other blockchains or the C-Chain?",
				options,
			)
			if err != nil {
				return false, err
			}
			switch option {
			case isolatedBlockchainOption:
				return false, nil
			case interoperatingBlockchainOption:
				return true, nil
			case explainOption:
				ux.Logger.PrintToUser("Avalanche enables native interoperability between blockchains through Avalanche Warp Messaging protocol (AWM). For more information about interoperability in Avalanche, please visit: https://docs.avax.network/build/cross-chain/awm/overview")
				continue
			}
		}
	}
}

func promptPermissioning(
	app *application.Avalanche,
	version string,
	defaultsKind DefaultsKind,
	params SubnetEVMGenesisParams,
) (SubnetEVMGenesisParams, error) {
	if defaultsKind != NoDefaults {
		return params, nil
	}
	var cancel bool
	noOption := "No"
	yesOption := "Yes"
	options := []string{yesOption, noOption, explainOption}
	for {
		option, err := app.Prompt.CaptureList(
			"Do you want to enable anyone to issue transactions and deploy smart contracts to your blockchain?",
			options,
		)
		if err != nil {
			return SubnetEVMGenesisParams{}, err
		}
		switch option {
		case noOption:
			anyoneCanSubmitTransactionsOption := "Yes, I want anyone to be able to issue transactions on my blockchain"
			approvedCanSubmitTransactionsOption := "No, I want only approved addresses to issue transactions on my blockchain (Transaction Allow List ON)"
			options := []string{anyoneCanSubmitTransactionsOption, approvedCanSubmitTransactionsOption, explainOption}
			for {
				option, err := app.Prompt.CaptureList(
					"Do you want to enable anyone to issue transactions to your blockchain?",
					options,
				)
				if err != nil {
					return SubnetEVMGenesisParams{}, err
				}
				switch option {
				case approvedCanSubmitTransactionsOption:
					params.transactionPrecompileAllowList, cancel, err = GenerateAllowList(app, AllowList{}, "issue transactions", version)
					if err != nil {
						return SubnetEVMGenesisParams{}, err
					}
					if cancel {
						continue
					}
					params.enableTransactionPrecompile = true
				case explainOption:
					ux.Logger.PrintToUser("The Transaction Allow List is a precompile contract that allows you to specify a list of addresses that are allowed to submit transactions to your blockchain. This list can be dynamically changed by calling the precompile.")
					ux.Logger.PrintToUser("")
					ux.Logger.PrintToUser("This feature is useful for permissioning your blockchain and lets you easiliy implement KYC measures. Only authorized users can send transactions or deploy smart contracts on your blockchain. For more information, please visit: https://docs.avax.network/build/subnet/upgrade/customize-a-subnet#restricting-who-can-submit-transactions.")
					continue
				}
				break
			}
			anyoneCanDeployContractsOption := "Yes, I want anyone to be able to deploy smart contracts on my blockchain"
			approvedCanDeployContractsOption := "No, I want only approved addresses to deploy smart contracts on my blockchain (Smart Contract Deployer Allow List ON)"
			options = []string{anyoneCanDeployContractsOption, approvedCanDeployContractsOption, explainOption}
			for {
				option, err := app.Prompt.CaptureList(
					"Do you want to enable anyone to deploy smart contracts on your blockchain?",
					options,
				)
				if err != nil {
					return SubnetEVMGenesisParams{}, err
				}
				switch option {
				case approvedCanDeployContractsOption:
					params.contractDeployerPrecompileAllowList, cancel, err = GenerateAllowList(app, AllowList{}, "deploy smart contracts", version)
					if err != nil {
						return SubnetEVMGenesisParams{}, err
					}
					if cancel {
						continue
					}
					params.enableContractDeployerPrecompile = true
				case explainOption:
					ux.Logger.PrintToUser("While you may wish to allow anyone to interact with the contract on your blockchain to your blockchain, you may want to restrict who can deploy smart contracts and create dApps on your chain.")
					ux.Logger.PrintToUser("")
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
	return params, nil
}

func PromptVMVersion(
	app *application.Avalanche,
	repoName string,
	vmVersion string,
) (string, error) {
	switch vmVersion {
	case latest:
		return app.Downloader.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
			constants.AvaLabsOrg,
			repoName,
		))
	case preRelease:
		return app.Downloader.GetLatestPreReleaseVersion(
			constants.AvaLabsOrg,
			repoName,
		)
	case "":
		return promptUserForVMVersion(app, repoName)
	}
	return vmVersion, nil
}

func promptUserForVMVersion(
	app *application.Avalanche,
	repoName string,
) (string, error) {
	var (
		latestReleaseVersion    string
		latestPreReleaseVersion string
		err                     error
	)
	if os.Getenv(constants.OperateOfflineEnvVarName) == "" {
		latestReleaseVersion, err = app.Downloader.GetLatestReleaseVersion(
			binutils.GetGithubLatestReleaseURL(
				constants.AvaLabsOrg,
				repoName,
			),
		)
		if err != nil {
			return "", err
		}
		latestPreReleaseVersion, err = app.Downloader.GetLatestPreReleaseVersion(
			constants.AvaLabsOrg,
			repoName,
		)
		if err != nil {
			return "", err
		}
	} else {
		latestReleaseVersion = evm.Version
		latestPreReleaseVersion = evm.Version
	}

	useCustom := "Specify custom version"
	useLatestRelease := "Use latest release version"
	useLatestPreRelease := "Use latest pre-release version"

	defaultPrompt := "Version"

	versionOptions := []string{useLatestRelease, useCustom}
	if latestPreReleaseVersion != latestReleaseVersion {
		versionOptions = []string{useLatestPreRelease, useLatestRelease, useCustom}
	}

	versionOption, err := app.Prompt.CaptureList(
		defaultPrompt,
		versionOptions,
	)
	if err != nil {
		return "", err
	}

	if versionOption == useLatestPreRelease {
		return latestPreReleaseVersion, err
	}

	if versionOption == useLatestRelease {
		return latestReleaseVersion, err
	}

	// prompt for version
	versions, err := app.Downloader.GetAllReleasesForRepo(
		constants.AvaLabsOrg,
		constants.SubnetEVMRepoName,
	)
	if err != nil {
		return "", err
	}
	version, err := app.Prompt.CaptureList("Pick the version for this VM", versions)
	if err != nil {
		return "", err
	}

	return version, nil
}
