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
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/subnet-evm/plugin/evm"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ava-labs/avalanchego/utils/logging"
)

const (
	latest        = "latest"
	preRelease    = "pre-release"
	explainOption = "Explain the difference"
)

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
	UseTeleporter                       bool
	UseExternalGasToken                 bool
	initialTokenAllocation              InitialTokenAllocation
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
			"VM",
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
			ux.Logger.PrintToUser(" ")
			ux.Logger.PrintToUser("Subnet-EVM is a EVM-compatible virtual machine that supports smart contract development in Solidity. This VM is an out-of-box solution for Subnet deployers who want a dApp development experience that is nearly identical to Ethereum, without having to manage or create a custom virtual machine. Subnet-EVM can be configured with this CLI to meet the developers requirements without writing code. For more information, please visit: https://github.com/ava-labs/subnet-evm")
			ux.Logger.PrintToUser(" ")
			ux.Logger.PrintToUser("Custom VMs created with SDKs such as the Precompile-EVM, HyperSDK, Rust-SDK and that are written in golang or rust can be deployed on Avalanche using the second option. You can provide the path to the binary directly or provide the code as well as the build script. In addition to the VM you need to provide the genesis file. More information can be found in the docs at https://docs.avax.network/learn/avalanche/virtual-machines.")
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
	version string,
	chainID uint64,
	tokenSymbol string,
	useTeleporter *bool,
	useDefaults bool,
	useWarp bool,
) (SubnetEVMGenesisParams, string, error) {
	var (
		err    error
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
	// Gas Token
	params, tokenSymbol, err = promptGasToken(app, version, tokenSymbol, useDefaults, params)
	if err != nil {
		return SubnetEVMGenesisParams{}, "", err
	}
	// Transaction / Gas Fees
	params, err = promptFeeConfig(app, version, useDefaults, params)
	if err != nil {
		return SubnetEVMGenesisParams{}, "", err
	}
	// Interoperability
	params, err = promptInteropt(app, useTeleporter, useDefaults, params)
	if err != nil {
		return SubnetEVMGenesisParams{}, "", err
	}
	// Warp
	params.enableWarpPrecompile = useWarp
	if (params.UseTeleporter || params.UseExternalGasToken) && !params.enableWarpPrecompile {
		return SubnetEVMGenesisParams{}, "", fmt.Errorf("warp should be enabled for teleporter to work")
	}
	// Permissioning
	params, err = promptPermissioning(app, version, useDefaults, params)
	if err != nil {
		return SubnetEVMGenesisParams{}, "", err
	}
	return params, tokenSymbol, nil
}

// prompts for wether to use a remote or native gas token,
// and in the case of native, also prompts for token symbol,
// initial token allocation, and native minter precompile
// configuration
//
// if tokenSymbol is not defined, will prompt for it
// is useDefaults is true, will:
// - use native gas token, allocating 1m to a newly created key
// - disable native minter precompile
func promptGasToken(
	app *application.Avalanche,
	version string,
	tokenSymbol string,
	useDefaults bool,
	params SubnetEVMGenesisParams,
) (SubnetEVMGenesisParams, string, error) {
	var (
		err    error
		cancel bool
	)
	if useDefaults {
		if tokenSymbol == "" {
			tokenSymbol, err = app.Prompt.CaptureString("Token Symbol")
			if err != nil {
				return SubnetEVMGenesisParams{}, "", err
			}
		}
		params.initialTokenAllocation.allocToNewKey = true
		return params, tokenSymbol, nil
	}
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
			params.UseExternalGasToken = true
		case nativeTokenOption:
			if tokenSymbol == "" {
				tokenSymbol, err = app.Prompt.CaptureString("Token Symbol")
				if err != nil {
					return SubnetEVMGenesisParams{}, "", err
				}
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
			for {
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
					params.nativeMinterPrecompileAllowList, cancel, err = GenerateAllowList(app, "mint native tokens", version)
					if err != nil {
						return SubnetEVMGenesisParams{}, "", err
					}
					if cancel {
						continue
					}
					params.enableNativeMinterPrecompile = true
				}
				break
			}
		case explainOption:
			ux.Logger.PrintToUser("Every blockchain uses a token to manage access to its limited resources. For example, ETH is the native token of Ethereum, and AVAX is the native token of the Avalanche C-Chain. Users pay transaction fees with these tokens. If demand exceeds capacity, transaction fees increase, requiring users to pay more tokens for their transactions.")
			ux.Logger.PrintToUser(" ")
			ux.Logger.PrintToUser(logging.Bold.Wrap("The blockchain's native token"))
			ux.Logger.PrintToUser("Each blockchain on Avalanche has its own transaction fee token. To issue transactions users don't need to acquire ETH or AVAX and therefore the transaction fees are completely isolated.")
			ux.Logger.PrintToUser(" ")
			ux.Logger.PrintToUser(logging.Bold.Wrap("A token from another blockchain"))
			ux.Logger.PrintToUser("You can use an ERC-20 token (e.g., USDC, WETH) or the native token (e.g., AVAX) of another blockchain within the Avalanche network as the transaction fee token. This is achieved through a bridge contract and the Native Minter Precompile. When a user bridges a token from another blockchain, it is locked on the home chain, a message is relayed to the Subnet, and the token is minted to the sender’s account.")
			ux.Logger.PrintToUser(" ")
			ux.Logger.PrintToUser("If a token from another blockchain is used, the interoperability protocol Teleporter is required and activated automatically.")
			continue
		}
		break
	}
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
	useDefaults bool,
	params SubnetEVMGenesisParams,
) (SubnetEVMGenesisParams, error) {
	if useDefaults {
		params.feeConfig.lowThroughput = true
		return params, nil
	}
	var cancel bool
	customizeOption := "Customize fee config"
	lowOption := "Low disk use    / Low Throughput    1.5 mil gas/s (C-Chain's setting)"
	mediumOption := "Medium disk use / Medium Throughput 2 mil   gas/s"
	highOption := "High disk use   / High Throughput   5 mil   gas/s"
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
			return SubnetEVMGenesisParams{}, err
		}
		switch option {
		case dontChangeFeeSettingsOption:
		case changeFeeSettingsOption:
			params.feeManagerPrecompileAllowList, cancel, err = GenerateAllowList(app, "adjust the gas fees", version)
			if err != nil {
				return SubnetEVMGenesisParams{}, err
			}
			if cancel {
				continue
			}
			params.enableFeeManagerPrecompile = true
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
			return SubnetEVMGenesisParams{}, err
		}
		switch option {
		case burnFees:
		case distributeFees:
			params.rewardManagerPrecompileAllowList, cancel, err = GenerateAllowList(app, "customize gas fees distribution", version)
			if err != nil {
				return SubnetEVMGenesisParams{}, err
			}
			if cancel {
				continue
			}
			params.enableRewardManagerPrecompile = true
		case explainOption:
			ux.Logger.PrintToUser("The fee reward mechanism can be configured with a stateful precompile contract called the RewardManager. The configuration can include burning fees, sending fees to a predefined address, or enabling fees to be collected by block producers. For more info, please visit: https://docs.avax.network/build/subnet/upgrade/customize-a-subnet#changing-fee-reward-mechanisms")
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
func promptInteropt(
	app *application.Avalanche,
	useTeleporter *bool,
	useDefaults bool,
	params SubnetEVMGenesisParams,
) (SubnetEVMGenesisParams, error) {
	switch {
	case useTeleporter != nil:
		params.UseTeleporter = *useTeleporter
	case useDefaults:
		params.UseTeleporter = true
	case params.UseExternalGasToken:
	default:
		interoperatingBlockchainOption := "Yes, I want my blockchain to be able to interoperate with other blockchains and the C-Chain"
		isolatedBlockchainOption := "No, I want to run my blockchain isolated"
		options := []string{interoperatingBlockchainOption, isolatedBlockchainOption, explainOption}
		for {
			option, err := app.Prompt.CaptureList(
				"Do you want to connect your blockchain with other blockchains or the C-Chain? (Deploy Teleporter along with its Registry)",
				options,
			)
			if err != nil {
				return SubnetEVMGenesisParams{}, err
			}
			switch option {
			case isolatedBlockchainOption:
			case interoperatingBlockchainOption:
				params.UseTeleporter = true
			case explainOption:
				ux.Logger.PrintToUser("Avalanche enables native interoperability between blockchains with the VM-agnostic Avalanche Warp Messaging protocol (AWM). Teleporter is a messaging protocol built on top of AWM that provides a developer-friendly interface for sending and receiving cross-chain messages to and from EVM-compatible blockchains. This communication protocol can be used for bridges and other protocols.")
				continue
			}
			break
		}
	}
	return params, nil
}

func promptPermissioning(
	app *application.Avalanche,
	version string,
	useDefaults bool,
	params SubnetEVMGenesisParams,
) (SubnetEVMGenesisParams, error) {
	if useDefaults {
		return params, nil
	}
	var cancel bool
	noOption := "No"
	yesOption := "Yes"
	options := []string{noOption, yesOption, explainOption}
	for {
		option, err := app.Prompt.CaptureList(
			"Do you want to add permissioning to your blockchain?",
			options,
		)
		if err != nil {
			return SubnetEVMGenesisParams{}, err
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
					return SubnetEVMGenesisParams{}, err
				}
				switch option {
				case approvedCanSubmitTransactionsOption:
					params.transactionPrecompileAllowList, cancel, err = GenerateAllowList(app, "issue transactions", version)
					if err != nil {
						return SubnetEVMGenesisParams{}, err
					}
					if cancel {
						continue
					}
					params.enableTransactionPrecompile = true
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
					return SubnetEVMGenesisParams{}, err
				}
				switch option {
				case approvedCanDeployContractsOption:
					params.contractDeployerPrecompileAllowList, cancel, err = GenerateAllowList(app, "deploy smart contracts", version)
					if err != nil {
						return SubnetEVMGenesisParams{}, err
					}
					if cancel {
						continue
					}
					params.enableContractDeployerPrecompile = true
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
