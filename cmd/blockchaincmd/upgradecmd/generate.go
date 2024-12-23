// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	avalancheSDK "github.com/ava-labs/avalanche-cli/sdk/vm"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/coreth/ethclient"
	"github.com/ava-labs/subnet-evm/commontype"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/precompile/contracts/deployerallowlist"
	"github.com/ava-labs/subnet-evm/precompile/contracts/feemanager"
	"github.com/ava-labs/subnet-evm/precompile/contracts/nativeminter"
	"github.com/ava-labs/subnet-evm/precompile/contracts/rewardmanager"
	"github.com/ava-labs/subnet-evm/precompile/contracts/txallowlist"
	subnetevmutils "github.com/ava-labs/subnet-evm/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const (
	feeConfigKey   = "initialFeeConfig"
	initialMintKey = "initialMint"

	managerLabel = "manager"
	adminLabel   = "admin"

	NativeMint        = "Native Minting"
	ContractAllowList = "Contract Deployment Allow List"
	TxAllowList       = "Transaction Allow List"
	FeeManager        = "Adjust Fee Settings Post Deploy"
	RewardManager     = "Customize Fees Distribution"
)

var blockchainName string

// avalanche blockchain upgrade generate
func newUpgradeGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate [blockchainName]",
		Short: "Generate the configuration file to upgrade blockchain nodes",
		Long: `The blockchain upgrade generate command builds a new upgrade.json file to customize your Blockchain. It
guides the user through the process using an interactive wizard.`,
		RunE: upgradeGenerateCmd,
		Args: cobrautils.ExactArgs(1),
	}
	return cmd
}

func upgradeGenerateCmd(_ *cobra.Command, args []string) error {
	blockchainName = args[0]
	if !app.GenesisExists(blockchainName) {
		ux.Logger.PrintToUser("The provided subnet name %q does not exist", blockchainName)
		return nil
	}
	// print some warning/info message
	ux.Logger.PrintToUser(logging.Bold.Wrap(logging.Yellow.Wrap(
		"Performing a network upgrade requires coordinating the upgrade network-wide.")))
	ux.Logger.PrintToUser(logging.White.Wrap(logging.Reset.Wrap(
		"A network upgrade changes the rule set used to process and verify blocks, " +
			"such that any node that upgrades incorrectly or fails to upgrade by the time " +
			"that upgrade goes into effect may become out of sync with the rest of the network.\n")))
	ux.Logger.PrintToUser(logging.Bold.Wrap(logging.Red.Wrap(
		"Any mistakes in configuring network upgrades or coordinating them on validators " +
			"may cause the network to halt and recovering may be difficult.")))
	ux.Logger.PrintToUser(logging.Reset.Wrap(
		"Please consult " + logging.Cyan.Wrap(
			"https://docs.avax.network/avalanche-l1s/upgrade/customize-avalanche-l1#network-upgrades-enabledisable-precompiles ") +
			logging.Reset.Wrap("for more information")))

	txt := "Press [Enter] to continue, or abort by choosing 'no'"
	yes, err := app.Prompt.CaptureYesNo(txt)
	if err != nil {
		return err
	}
	if !yes {
		ux.Logger.PrintToUser("Aborted by user")
		return nil
	}

	allPreComps := []string{
		ContractAllowList,
		FeeManager,
		NativeMint,
		TxAllowList,
		RewardManager,
	}

	fmt.Println()
	ux.Logger.PrintToUser(logging.Yellow.Wrap(
		"Avalanchego and this tool support configuring multiple precompiles. " +
			"However, we suggest to only configure one per upgrade."))
	fmt.Println()

	// use the correct data types from subnet-evm right away
	precompiles := params.UpgradeConfig{
		PrecompileUpgrades: make([]params.PrecompileUpgrade, 0),
	}

	for {
		precomp, err := app.Prompt.CaptureList("Select the precompile to configure", allPreComps)
		if err != nil {
			return err
		}

		ux.Logger.PrintToUser(fmt.Sprintf("Set parameters for the %q precompile", precomp))
		if cancelled, err := promptParams(precomp, &precompiles.PrecompileUpgrades); err != nil {
			return err
		} else if cancelled {
			continue
		}

		if len(allPreComps) > 1 {
			yes, err := app.Prompt.CaptureNoYes("Should we configure another precompile?")
			if err != nil {
				return err
			}
			if !yes {
				break
			}

			for i := 0; i < len(allPreComps); i++ {
				if allPreComps[i] == precomp {
					allPreComps = append(allPreComps[:i], allPreComps[i+1:]...)
					break
				}
			}
		}
	}

	jsonBytes, err := json.Marshal(&precompiles)
	if err != nil {
		return err
	}

	return app.WriteUpgradeFile(blockchainName, jsonBytes)
}

func queryActivationTimestamp() (time.Time, error) {
	const (
		in5min   = "In 5 minutes"
		in1day   = "In 1 day"
		in1week  = "In 1 week"
		in2weeks = "In 2 weeks"
		custom   = "Custom"
	)
	options := []string{in5min, in1day, in1week, in2weeks, custom}
	choice, err := app.Prompt.CaptureList("When should the precompile be activated?", options)
	if err != nil {
		return time.Time{}, err
	}

	var date time.Time
	now := time.Now()

	switch choice {
	case in5min:
		date = now.Add(5 * time.Minute)
	case in1day:
		date = now.Add(24 * time.Hour)
	case in1week:
		date = now.Add(7 * 24 * time.Hour)
	case in2weeks:
		date = now.Add(14 * 24 * time.Hour)
	case custom:
		date, err = app.Prompt.CaptureFutureDate(
			"Enter the block activation UTC datetime in 'YYYY-MM-DD HH:MM:SS' format", time.Now().Add(time.Minute).UTC())
		if err != nil {
			return time.Time{}, err
		}
	}

	ux.Logger.PrintToUser("The chosen block activation time is %s", date.Format(constants.TimeParseLayout))
	return date, nil
}

func promptParams(precomp string, precompiles *[]params.PrecompileUpgrade) (bool, error) {
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return false, err
	}
	date, err := queryActivationTimestamp()
	if err != nil {
		return false, err
	}
	switch precomp {
	case ContractAllowList:
		return promptContractAllowListParams(&sc, precompiles, date)
	case TxAllowList:
		return promptTxAllowListParams(&sc, precompiles, date)
	case NativeMint:
		return promptNativeMintParams(&sc, precompiles, date)
	case FeeManager:
		return promptFeeManagerParams(&sc, precompiles, date)
	case RewardManager:
		return promptRewardManagerParams(&sc, precompiles, date)
	default:
		return false, fmt.Errorf("unexpected precompile identifier: %q", precomp)
	}
}

func promptNativeMintParams(
	sc *models.Sidecar,
	precompiles *[]params.PrecompileUpgrade,
	date time.Time,
) (bool, error) {
	initialMint := map[common.Address]*math.HexOrDecimal256{}
	adminAddrs, managerAddrs, enabledAddrs, cancelled, err := promptAdminManagerAndEnabledAddresses(sc, "mint native tokens")
	if cancelled || err != nil {
		return cancelled, err
	}
	yes, err := app.Prompt.CaptureYesNo(fmt.Sprintf("Airdrop more tokens? (`%s` section in file)", initialMintKey))
	if err != nil {
		return false, err
	}
	if yes {
		_, cancel, err := prompts.CaptureListDecision(
			app.Prompt,
			"How would you like to distribute your funds",
			func(_ string) (string, error) {
				addr, err := app.Prompt.CaptureAddress("Address to airdrop to")
				if err != nil {
					return "", err
				}
				amount, err := app.Prompt.CaptureUint64(fmt.Sprintf("Amount to airdrop (in %s units)", sc.TokenSymbol))
				if err != nil {
					return "", err
				}
				initialMint[addr] = math.NewHexOrDecimal256(int64(amount))
				return fmt.Sprintf("%s-%d", addr.Hex(), amount), nil
			},
			"Add an address to amount pair",
			"Address-Amount",
			"Hex-formatted address and it's initial amount value, "+
				"for example: 0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC (address) and 1000000000000000000 (value)",
		)
		if err != nil {
			return false, err
		}
		if cancel {
			return true, nil
		}
	}
	config := nativeminter.NewConfig(
		subnetevmutils.NewUint64(uint64(date.Unix())),
		adminAddrs,
		enabledAddrs,
		managerAddrs,
		initialMint,
	)
	upgrade := params.PrecompileUpgrade{
		Config: config,
	}
	*precompiles = append(*precompiles, upgrade)
	return false, nil
}

func promptRewardManagerParams(
	sc *models.Sidecar,
	precompiles *[]params.PrecompileUpgrade,
	date time.Time,
) (bool, error) {
	adminAddrs, managerAddrs, enabledAddrs, cancelled, err := promptAdminManagerAndEnabledAddresses(sc, "customize fee distribution")
	if cancelled || err != nil {
		return cancelled, err
	}
	initialConfig, err := ConfigureInitialRewardConfig()
	if err != nil {
		return false, err
	}
	config := rewardmanager.NewConfig(
		subnetevmutils.NewUint64(uint64(date.Unix())),
		adminAddrs,
		enabledAddrs,
		managerAddrs,
		initialConfig,
	)
	upgrade := params.PrecompileUpgrade{
		Config: config,
	}
	*precompiles = append(*precompiles, upgrade)
	return false, nil
}

func ConfigureInitialRewardConfig() (*rewardmanager.InitialRewardConfig, error) {
	config := &rewardmanager.InitialRewardConfig{}

	burnFees := "I am fine with transaction fees being burned (Reward Manager Precompile OFF)"
	distributeFees := "I want to customize the transaction fee distribution (Reward Manager Precompile ON)"
	options := []string{burnFees, distributeFees}
	option, err := app.Prompt.CaptureList(
		"By default, all transaction fees on Avalanche are burned (sent to a blackhole address). (Reward Manager Precompile)",
		options,
	)
	if err != nil {
		return config, err
	}
	if option == burnFees {
		return config, nil
	}

	feeRcpdPrompt := "Allow block producers to claim fees?"
	allowFeeRecipients, err := app.Prompt.CaptureYesNo(feeRcpdPrompt)
	if err != nil {
		return config, err
	}
	if allowFeeRecipients {
		config.AllowFeeRecipients = true
		return config, nil
	}

	rewardPrompt := "Provide the address to which fees will be sent to"
	rewardAddress, err := app.Prompt.CaptureAddress(rewardPrompt)
	if err != nil {
		return config, err
	}
	config.RewardAddress = rewardAddress
	return config, nil
}

func promptFeeManagerParams(
	sc *models.Sidecar,
	precompiles *[]params.PrecompileUpgrade,
	date time.Time,
) (bool, error) {
	adminAddrs, managerAddrs, enabledAddrs, cancelled, err := promptAdminManagerAndEnabledAddresses(sc, "adjust the gas fees")
	if cancelled || err != nil {
		return cancelled, err
	}
	yes, err := app.Prompt.CaptureYesNo(fmt.Sprintf(
		"Do you want to update the fee config upon precompile activation? ('%s' section in file)", feeConfigKey))
	if err != nil {
		return false, err
	}
	var feeConfig *commontype.FeeConfig
	if yes {
		chainConfig, err := GetFeeConfig(params.ChainConfig{}, false)
		if err != nil {
			return false, err
		}
		feeConfig = &chainConfig.FeeConfig
	}
	config := feemanager.NewConfig(
		subnetevmutils.NewUint64(uint64(date.Unix())),
		adminAddrs,
		enabledAddrs,
		managerAddrs,
		feeConfig,
	)
	upgrade := params.PrecompileUpgrade{
		Config: config,
	}
	*precompiles = append(*precompiles, upgrade)
	return false, nil
}

func GetFeeConfig(config params.ChainConfig, useDefault bool) (
	params.ChainConfig,
	error,
) {
	const (
		lowOption    = "Low block size    / Low Throughput    12 mil gas per block"
		mediumOption = "Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)"
		highOption   = "High block size   / High Throughput   20 mil gas per block"
		customFee    = "Customize fee config"

		setGasLimit                 = "Set gas limit"
		setBlockRate                = "Set target block rate"
		setMinBaseFee               = "Set min base fee"
		setTargetGas                = "Set target gas"
		setBaseFeeChangeDenominator = "Set base fee change denominator"
		setMinBlockGas              = "Set min block gas cost"
		setMaxBlockGas              = "Set max block gas cost"
		setGasStep                  = "Set block gas cost step"
	)

	config.FeeConfig = avalancheSDK.StarterFeeConfig

	if useDefault {
		config.FeeConfig.GasLimit = vm.LowGasLimit
		config.FeeConfig.TargetGas = config.FeeConfig.TargetGas.Mul(config.FeeConfig.GasLimit, vm.NoDynamicFeesGasLimitToTargetGasFactor)
		return config, nil
	}

	feeConfigOptions := []string{lowOption, mediumOption, highOption, customFee}

	feeDefault, err := app.Prompt.CaptureList(
		"How would you like to set fees",
		feeConfigOptions,
	)
	if err != nil {
		return config, err
	}

	useDynamicFees := false
	if feeDefault != customFee {
		useDynamicFees, err = app.Prompt.CaptureYesNo("Do you want to enable dynamic fees?")
		if err != nil {
			return config, err
		}
	}

	switch feeDefault {
	case lowOption:
		vm.SetStandardGas(&config.FeeConfig, vm.LowGasLimit, vm.LowTargetGas, useDynamicFees)
		return config, nil
	case mediumOption:
		vm.SetStandardGas(&config.FeeConfig, vm.MediumGasLimit, vm.MediumTargetGas, useDynamicFees)
		return config, err
	case highOption:
		vm.SetStandardGas(&config.FeeConfig, vm.HighGasLimit, vm.HighTargetGas, useDynamicFees)
		return config, err
	default:
		ux.Logger.PrintToUser("Customizing fee config")
	}

	gasLimit, err := app.Prompt.CapturePositiveBigInt(setGasLimit)
	if err != nil {
		return config, err
	}

	blockRate, err := app.Prompt.CapturePositiveBigInt(setBlockRate)
	if err != nil {
		return config, err
	}

	minBaseFee, err := app.Prompt.CapturePositiveBigInt(setMinBaseFee)
	if err != nil {
		return config, err
	}

	targetGas, err := app.Prompt.CapturePositiveBigInt(setTargetGas)
	if err != nil {
		return config, err
	}

	baseDenominator, err := app.Prompt.CapturePositiveBigInt(setBaseFeeChangeDenominator)
	if err != nil {
		return config, err
	}

	minBlockGas, err := app.Prompt.CapturePositiveBigInt(setMinBlockGas)
	if err != nil {
		return config, err
	}

	maxBlockGas, err := app.Prompt.CapturePositiveBigInt(setMaxBlockGas)
	if err != nil {
		return config, err
	}

	gasStep, err := app.Prompt.CapturePositiveBigInt(setGasStep)
	if err != nil {
		return config, err
	}

	feeConf := commontype.FeeConfig{
		GasLimit:                 gasLimit,
		TargetBlockRate:          blockRate.Uint64(),
		MinBaseFee:               minBaseFee,
		TargetGas:                targetGas,
		BaseFeeChangeDenominator: baseDenominator,
		MinBlockGasCost:          minBlockGas,
		MaxBlockGasCost:          maxBlockGas,
		BlockGasCostStep:         gasStep,
	}

	config.FeeConfig = feeConf

	return config, nil
}

func promptContractAllowListParams(
	sc *models.Sidecar,
	precompiles *[]params.PrecompileUpgrade,
	date time.Time,
) (bool, error) {
	adminAddrs, managerAddrs, enabledAddrs, cancelled, err := promptAdminManagerAndEnabledAddresses(sc, "deploy smart contracts")
	if cancelled || err != nil {
		return cancelled, err
	}
	config := deployerallowlist.NewConfig(
		subnetevmutils.NewUint64(uint64(date.Unix())),
		adminAddrs,
		enabledAddrs,
		managerAddrs,
	)
	upgrade := params.PrecompileUpgrade{
		Config: config,
	}
	*precompiles = append(*precompiles, upgrade)
	return false, nil
}

func promptTxAllowListParams(
	sc *models.Sidecar,
	precompiles *[]params.PrecompileUpgrade,
	date time.Time,
) (bool, error) {
	adminAddrs, managerAddrs, enabledAddrs, cancelled, err := promptAdminManagerAndEnabledAddresses(sc, "issue transactions")
	if cancelled || err != nil {
		return cancelled, err
	}
	config := txallowlist.NewConfig(
		subnetevmutils.NewUint64(uint64(date.Unix())),
		adminAddrs,
		enabledAddrs,
		managerAddrs,
	)
	upgrade := params.PrecompileUpgrade{
		Config: config,
	}
	*precompiles = append(*precompiles, upgrade)
	return false, nil
}

func getCClient(apiEndpoint string, blockchainID string) (ethclient.Client, error) {
	cClient, err := ethclient.Dial(fmt.Sprintf("%s/ext/bc/%s/rpc", apiEndpoint, blockchainID))
	if err != nil {
		return nil, err
	}
	return cClient, nil
}

func ensureHaveBalanceLocalNetwork(which string, addresses []common.Address, blockchainID string) error {
	cClient, err := getCClient(constants.LocalAPIEndpoint, blockchainID)
	if err != nil {
		return err
	}
	for _, address := range addresses {
		// we can break at the first address who has a non-zero balance
		accountBalance, err := getAccountBalance(cClient, address.String())
		if err != nil {
			return err
		}
		if accountBalance > float64(0) {
			return nil
		}
	}
	return fmt.Errorf("at least one of the %s addresses requires a positive token balance", which)
}

func ensureHaveBalance(
	sc *models.Sidecar,
	which string,
	addresses []common.Address,
) error {
	if len(addresses) < 1 {
		return nil
	}
	switch sc.VM {
	case models.SubnetEvm:
		// Currently only checking if admins have balance for subnets deployed in Local Network
		if networkData, ok := sc.Networks["Local Network"]; ok {
			blockchainID := networkData.BlockchainID.String()
			if err := ensureHaveBalanceLocalNetwork(which, addresses, blockchainID); err != nil {
				return err
			}
		}
	default:
		app.Log.Warn("Unsupported VM type", zap.Any("vm-type", sc.VM))
	}
	return nil
}

func getAccountBalance(cClient ethclient.Client, addrStr string) (float64, error) {
	addr := common.HexToAddress(addrStr)
	ctx, cancel := utils.GetAPIContext()
	balance, err := cClient.BalanceAt(ctx, addr, nil)
	defer cancel()
	if err != nil {
		return 0, err
	}
	// convert to nAvax
	balance = balance.Div(balance, big.NewInt(int64(units.Avax)))
	if balance.Cmp(big.NewInt(0)) == 0 {
		return 0, nil
	}
	return float64(balance.Uint64()) / float64(units.Avax), nil
}

func promptAdminManagerAndEnabledAddresses(
	sc *models.Sidecar,
	action string,
) ([]common.Address, []common.Address, []common.Address, bool, error) {
	allowList, cancelled, err := vm.GenerateAllowList(app, vm.AllowList{}, action, sc.VMVersion)
	if cancelled || err != nil {
		return nil, nil, nil, cancelled, err
	}
	if err := ensureHaveBalance(sc, adminLabel, allowList.AdminAddresses); err != nil {
		return nil, nil, nil, false, err
	}
	if err := ensureHaveBalance(sc, managerLabel, allowList.ManagerAddresses); err != nil {
		return nil, nil, nil, false, err
	}
	return allowList.AdminAddresses, allowList.ManagerAddresses, allowList.EnabledAddresses, false, nil
}
