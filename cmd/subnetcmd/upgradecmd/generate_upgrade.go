// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/storage"
	"github.com/ava-labs/subnet-evm/commontype"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

type PrecompilePrompt interface {
	PromptParams() error
	ToMap() map[string]interface{}
}

type contractAllowList struct {
	enabledAddresses []common.Address
	adminAddresses   []common.Address
}

type feeManager struct {
	adminAddresses   []common.Address
	enabledAddresses []common.Address
	initialFeeConfig commontype.FeeConfig
}

type nativeMint struct {
	adminAddresses   []common.Address
	enabledAddresses []common.Address
	initialMint      map[string]string
}

type txAllowList struct {
	enabledAddresses []common.Address
	adminAddresses   []common.Address
}

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
		Long:  `Upgrades to subnet nodes can be executed by providing a upgrade.json file to the nodes. This command starts a wizard guiding the user generating the required file.`,
		RunE:  upgradeGenerateCmd,
		Args:  cobra.ExactArgs(1),
	}
	return cmd
}

type Precompiles struct {
	PrecompileUpgrades map[string]interface{} `json:"precompileUpgrades"`
}

func upgradeGenerateCmd(cmd *cobra.Command, args []string) error {
	subnetName := args[0]
	if !app.GenesisExists(subnetName) {
		ux.Logger.PrintToUser("The provided subnet name %q does not exist", subnetName)
		return nil
	}
	// print some warning/info message
	ux.Logger.PrintToUser(logging.Yellow.Wrap("Performing a network upgrade requires coordinating the upgrade network-wide. A network upgrade changes the rule set used to process and verify blocks, such that any node that upgrades incorrectly or fails to upgrade by the time that upgrade goes into effect may become out of sync with the rest of the network.\n\nAny mistakes in configuring network upgrades or coordinating them on validators may cause the network to halt and recovering may be difficult."))
	ux.Logger.PrintToUser(logging.Cyan.Wrap("Please consult https://docs.avax.network/subnets/customize-a-subnet#network-upgrades-enabledisable-precompiles for more information"))

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
	ux.Logger.PrintToUser(logging.Yellow.Wrap("Avalanchego and this tool support configuring multiple precompiles. However, we suggest to only configure one"))
	fmt.Println()

	precompiles := Precompiles{
		PrecompileUpgrades: map[string]interface{}{},
	}

	for {
		precomp, err := app.Prompt.CaptureList("Select the precompile to configure", allPreComps)
		if err != nil {
			return err
		}
		var pp PrecompilePrompt
		switch precomp {
		case vm.ContractAllowList:
			pp = &contractAllowList{}
		case vm.TxAllowList:
			pp = &txAllowList{}
		case vm.NativeMint:
			pp = &nativeMint{}
		case vm.FeeManager:
			pp = &feeManager{}
		default:
			return fmt.Errorf("unexpected precompile identifier: %q", precomp)
		}

		ux.Logger.PrintToUser(fmt.Sprintf("Set parameters for the %q precompile", precomp))
		if err := pp.PromptParams(); err != nil {
			return err
		}

		mapForJSON := pp.ToMap()
		// TODO: This is requiring a timestamp 1 minute in the future
		// What is a sensible default?
		// An update requires planning and coordination, so it's not easy to think of a sensible default.
		// It's probably best to not try to be too smart and just assume the user to set something useful
		date, err := app.Prompt.CaptureFutureDate(
			"Enter the block activation UTC datetime in 'YYYY-MM-DD HH:MM:SS' format", time.Now().Add(time.Minute).UTC())
		if err != nil {
			return err
		}
		mapForJSON[blockTimestampKey] = date.Unix()

		precompiles.PrecompileUpgrades[vm.PrecompileToUpgradeString(vm.Precompile(precomp))] = mapForJSON

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

	return writeUpgradeFile(jsonBytes, subnetName)
}

func writeUpgradeFile(jsonBytes []byte, subnetName string) error {
	var (
		exists bool
		err    error
	)

	subnetPath := filepath.Join(app.GetUpgradeFilesDir(), subnetName)
	updateBytesFileName := filepath.Join(subnetPath, constants.UpdateBytesFileName)

	ux.Logger.PrintToUser(fmt.Sprintf("Writing %q file to %q...", constants.UpdateBytesFileName, subnetPath))

	exists, err = storage.FolderExists(app.GetUpgradeFilesDir())
	if err != nil {
		return err
	}
	if !exists {
		if err := os.Mkdir(app.GetUpgradeFilesDir(), constants.DefaultPerms755); err != nil {
			return err
		}
	}

	exists, err = storage.FolderExists(subnetPath)
	if err != nil {
		return err
	}
	if !exists {
		if err := os.Mkdir(subnetPath, constants.DefaultPerms755); err != nil {
			return err
		}
	}

	if err = os.WriteFile(updateBytesFileName, jsonBytes, constants.DefaultPerms755); err != nil {
		return err
	}
	ux.Logger.PrintToUser("File written successfully")
	return nil
}

func (p *nativeMint) PromptParams() error {
	if err := captureAddress(adminLabel, &p.adminAddresses); err != nil {
		return err
	}

	if err := captureAddress(enabledLabel, &p.enabledAddresses); err != nil {
		return err
	}

	yes, err := app.Prompt.CaptureYesNo(fmt.Sprintf("Add an `%s` section?", initialMintKey))
	if err != nil {
		return err
	}

	if yes {
		for {
			_, cancel, err := prompts.CaptureListDecision(
				app.Prompt,
				"Provide a pair of Ethereum address to initial mint amount",
				func(s string) (string, error) {
					addr, err := app.Prompt.CaptureAddress("What's the ethereum address")
					if err != nil {
						return "", err
					}
					amount, err := app.Prompt.CaptureUint64("What's its initial amount")
					if err != nil {
						return "", err
					}
					p.initialMint[addr.Hex()] = strconv.FormatUint(amount, 10)
					return "", nil
				},
				"Add an address to amount pair",
				"Address-Amount",
				"Ethereum address in Hex format and it's initial amount value, for example: 0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC (address) and 1000000000000000000 (value)",
			)
			if err != nil {
				return err
			}
			if cancel {
				return errors.New("aborted by user")
			}
		}
	}
	return nil
}

func (p *nativeMint) ToMap() map[string]interface{} {
	finalMap := toMap(&p.enabledAddresses, &p.adminAddresses)
	finalMap[initialMintKey] = p.initialMint
	return finalMap
}

func (p *feeManager) PromptParams() error {
	if err := captureAddress(adminLabel, &p.adminAddresses); err != nil {
		return err
	}

	if err := captureAddress(enabledLabel, &p.enabledAddresses); err != nil {
		return err
	}

	yes, err := app.Prompt.CaptureYesNo(fmt.Sprintf("Add an '%s' section?", feeConfigKey))
	if err != nil {
		return err
	}

	if yes {
		chainConfig, _, err := vm.GetFeeConfig(params.ChainConfig{}, app)
		if err != nil {
			return err
		}
		p.initialFeeConfig = chainConfig.FeeConfig
	}
	return nil
}

func (p *feeManager) ToMap() map[string]interface{} {
	finalMap := toMap(&p.enabledAddresses, &p.adminAddresses)
	finalMap[feeConfigKey] = p.initialFeeConfig
	return finalMap
}

func (p *contractAllowList) PromptParams() error {
	return enabledAdminPromptParams(&p.enabledAddresses, &p.adminAddresses)
}

func (p *contractAllowList) ToMap() map[string]interface{} {
	return toMap(&p.enabledAddresses, &p.adminAddresses)
}

func (p *txAllowList) PromptParams() error {
	return enabledAdminPromptParams(&p.enabledAddresses, &p.adminAddresses)
}

func (p *txAllowList) ToMap() map[string]interface{} {
	return toMap(&p.enabledAddresses, &p.adminAddresses)
}

func enabledAdminPromptParams(enabled *[]common.Address, admin *[]common.Address) error {
	for {
		if err := captureAddress(enabledLabel, enabled); err != nil {
			return err
		}
		if err := captureAddress(adminLabel, admin); err != nil {
			return err
		}

		if len(*enabled) == 0 && len(*admin) == 0 {
			ux.Logger.PrintToUser(fmt.Sprintf("We need at least one Ethereum address for either '%s' or '%s'. Otherwise abort.", enabledAddressesKey, adminAddressesKey))
			continue
		}
		return nil
	}
}

func toMap(enabledAddresses *[]common.Address, adminAddresses *[]common.Address) map[string]interface{} {
	finalMap := map[string]interface{}{}
	if len(*enabledAddresses) > 0 {
		enabled := make([]string, len(*enabledAddresses))
		for i := 0; i < len(*enabledAddresses); i++ {
			enabled[i] = (*enabledAddresses)[i].Hex()
		}
		finalMap[enabledAddressesKey] = enabled
	}

	if len(*adminAddresses) > 0 {
		admin := make([]string, len(*adminAddresses))
		for i := 0; i < len(*adminAddresses); i++ {
			admin[i] = (*adminAddresses)[i].Hex()
		}
		finalMap[adminAddressesKey] = admin
	}

	return finalMap
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
			fmt.Sprintf("Ethereum address in Hex format for %s addresses", which),
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
