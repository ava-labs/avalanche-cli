// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/coreth/ethclient"
	"go.uber.org/zap"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/commontype"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/precompile"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/spf13/cobra"
)

const (
	blockTimestampKey   = "blockTimestamp"
	feeConfigKey        = "initialFeeConfig"
	initialMintKey      = "initialMint"
	adminAddressesKey   = "adminAddresses"
	enabledAddressesKey = "enabledAddresses"

	enabledLabel = "enabled"
	adminLabel   = "admin"
)

// avalanche subnet upgrade generate
func newUpgradeGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate [subnetName]",
		Short: "Generate the configuration file to upgrade subnet nodes",
		Long: `Upgrades to subnet nodes can be executed by providing a upgrade.json file to the nodes.
This command starts a wizard guiding the user generating the required file.`,
		RunE: upgradeGenerateCmd,
		Args: cobra.ExactArgs(1),
	}
	return cmd
}

func upgradeGenerateCmd(_ *cobra.Command, args []string) error {
	subnetName := args[0]
	if !app.GenesisExists(subnetName) {
		ux.Logger.PrintToUser("The provided subnet name %q does not exist", subnetName)
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
			"https://docs.avax.network/subnets/customize-a-subnet#network-upgrades-enabledisable-precompiles ") +
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
		vm.ContractAllowList,
		vm.FeeManager,
		vm.NativeMint,
		vm.TxAllowList,
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
		if err := promptParams(precomp, &precompiles.PrecompileUpgrades, subnetName); err != nil {
			return err
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

	return app.WriteUpgradeFile(subnetName, jsonBytes)
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

func promptParams(precomp string, precompiles *[]params.PrecompileUpgrade, subnetName string) error {
	date, err := queryActivationTimestamp()
	if err != nil {
		return err
	}
	switch precomp {
	case vm.ContractAllowList:
		return promptContractAllowListParams(precompiles, date)
	case vm.TxAllowList:
		return promptTxAllowListParams(precompiles, date, subnetName)
	case vm.NativeMint:
		return promptNativeMintParams(precompiles, date)
	case vm.FeeManager:
		return promptFeeManagerParams(precompiles, date)
	default:
		return fmt.Errorf("unexpected precompile identifier: %q", precomp)
	}
}

func promptNativeMintParams(precompiles *[]params.PrecompileUpgrade, date time.Time) error {
	initialMint := map[common.Address]*math.HexOrDecimal256{}

	adminAddrs, enabledAddrs, err := promptAdminAndEnabledAddresses()
	if err != nil {
		return err
	}

	yes, err := app.Prompt.CaptureYesNo(fmt.Sprintf("Airdrop more tokens? (`%s` section in file)", initialMintKey))
	if err != nil {
		return err
	}

	if yes {
		_, cancel, err := prompts.CaptureListDecision(
			app.Prompt,
			"How would you like to distribute your funds",
			func(s string) (string, error) {
				addr, err := app.Prompt.CaptureAddress("Address to airdrop to")
				if err != nil {
					return "", err
				}
				amount, err := app.Prompt.CaptureUint64("Amount to airdrop (in AVAX units)")
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
			return err
		}
		if cancel {
			return errors.New("aborted by user")
		}
	}

	config := precompile.NewContractNativeMinterConfig(
		big.NewInt(date.Unix()),
		adminAddrs,
		enabledAddrs,
		initialMint,
	)
	upgrade := params.PrecompileUpgrade{
		ContractNativeMinterConfig: config,
	}
	*precompiles = append(*precompiles, upgrade)
	return nil
}

func promptFeeManagerParams(precompiles *[]params.PrecompileUpgrade, date time.Time) error {
	adminAddrs, enabledAddrs, err := promptAdminAndEnabledAddresses()
	if err != nil {
		return err
	}

	yes, err := app.Prompt.CaptureYesNo(fmt.Sprintf(
		"Do you want to update the fee config upon precompile activation? ('%s' section in file)", feeConfigKey))
	if err != nil {
		return err
	}

	var feeConfig *commontype.FeeConfig

	if yes {
		chainConfig, _, err := vm.GetFeeConfig(params.ChainConfig{}, app)
		if err != nil {
			return err
		}
		feeConfig = &chainConfig.FeeConfig
	}

	config := precompile.NewFeeManagerConfig(
		big.NewInt(date.Unix()),
		adminAddrs,
		enabledAddrs,
		feeConfig,
	)
	upgrade := params.PrecompileUpgrade{
		FeeManagerConfig: config,
	}
	*precompiles = append(*precompiles, upgrade)
	return nil
}

func promptContractAllowListParams(precompiles *[]params.PrecompileUpgrade, date time.Time) error {
	adminAddrs, enabledAddrs, err := promptAdminAndEnabledAddresses()
	if err != nil {
		return err
	}

	config := precompile.NewContractDeployerAllowListConfig(
		big.NewInt(date.Unix()),
		adminAddrs,
		enabledAddrs,
	)
	upgrade := params.PrecompileUpgrade{
		ContractDeployerAllowListConfig: config,
	}
	*precompiles = append(*precompiles, upgrade)
	return nil
}

func promptTxAllowListParams(precompiles *[]params.PrecompileUpgrade, date time.Time, subnetName string) error {
	adminAddrs, enabledAddrs, err := promptAdminAndEnabledAddresses()
	if err != nil {
		return err
	}

	if err = ensureAdminsHaveBalance(adminAddrs, subnetName); err != nil {
		return err
	}
	config := precompile.NewTxAllowListConfig(
		big.NewInt(date.Unix()),
		adminAddrs,
		enabledAddrs,
	)
	upgrade := params.PrecompileUpgrade{
		TxAllowListConfig: config,
	}
	*precompiles = append(*precompiles, upgrade)
	return nil
}

func getCClient(apiEndpoint string, blockchainID string) (ethclient.Client, error) {
	cClient, err := ethclient.Dial(fmt.Sprintf("%s/ext/bc/%s/rpc", apiEndpoint, blockchainID))
	if err != nil {
		return nil, err
	}
	return cClient, nil
}

func ensureAdminsHaveBalanceLocalNetwork(admins []common.Address, blockchainID string) error {
	cClient, err := getCClient(constants.LocalAPIEndpoint, blockchainID)
	if err != nil {
		return err
	}

	for _, admin := range admins {
		// we can break at the first admin who has a non-zero balance
		accountBalance, err := getAccountBalance(context.Background(), cClient, admin.String())
		if err != nil {
			return err
		}
		if accountBalance > float64(0) {
			return nil
		}
	}

	return errors.New("none of the addresses in the transaction allow list precompile have any tokens allocated to them. Currently, no address can transact on the network. Airdrop some funds to one of the allow list addresses to continue")
}

func ensureAdminsHaveBalance(admins []common.Address, subnetName string) error {
	if len(admins) < 1 {
		return nil
	}

	if !app.GenesisExists(subnetName) {
		ux.Logger.PrintToUser("The provided subnet name %q does not exist", subnetName)
		return nil
	}

	// read in sidecar
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}
	switch sc.VM {
	case models.SubnetEvm:
		// Currently only checking if admins have balance for subnets deployed in Local Network
		if networkData, ok := sc.Networks["Local Network"]; ok {
			blockchainID := networkData.BlockchainID.String()
			if err = ensureAdminsHaveBalanceLocalNetwork(admins, blockchainID); err != nil {
				return err
			}
		}
	default:
		app.Log.Warn("Unknown genesis format", zap.Any("vm-type", sc.VM))
	}
	return nil
}

func getAccountBalance(ctx context.Context, cClient ethclient.Client, addrStr string) (float64, error) {
	addr := common.HexToAddress(addrStr)
	ctx, cancel := context.WithTimeout(ctx, constants.RequestTimeout)
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

func promptAdminAndEnabledAddresses() ([]common.Address, []common.Address, error) {
	var admin, enabled []common.Address

	for {
		if err := captureAddress(adminLabel, &admin); err != nil {
			return nil, nil, err
		}
		if err := captureAddress(enabledLabel, &enabled); err != nil {
			return nil, nil, err
		}

		if len(enabled) == 0 && len(admin) == 0 {
			ux.Logger.PrintToUser(fmt.Sprintf(
				"We need at least one address for either '%s' or '%s'. Otherwise abort.", enabledAddressesKey, adminAddressesKey))
			continue
		}
		return admin, enabled, nil
	}
}

func captureAddress(which string, addrsField *[]common.Address) error {
	yes, err := app.Prompt.CaptureYesNo(fmt.Sprintf("Add '%sAddresses'?", which))
	if err != nil {
		return err
	}
	if yes {
		var (
			cancel bool
			err    error
		)
		*addrsField, cancel, err = prompts.CaptureListDecision(
			app.Prompt,
			fmt.Sprintf("Provide '%sAddresses'", which),
			app.Prompt.CaptureAddress,
			"Add an address",
			"Address",
			fmt.Sprintf("Hex-formatted %s addresses", which),
		)
		if err != nil {
			return err
		}
		if cancel {
			return errors.New("aborted by user")
		}
	}
	return nil
}
