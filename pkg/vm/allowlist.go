// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"fmt"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ethereum/go-ethereum/common"
	"github.com/olekukonko/tablewriter"
	"golang.org/x/mod/semver"
)

func preview(
	adminAddresses []common.Address,
	managerAddresses []common.Address,
	enabledAddresses []common.Address,
) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	addRoleToPreviewTable(table, "Admins", adminAddresses)
	addRoleToPreviewTable(table, "Manager", managerAddresses)
	addRoleToPreviewTable(table, "Enabled", enabledAddresses)
	table.Render()
	fmt.Println()
	if len(adminAddresses) == 0 && len(managerAddresses) == 0 && len(enabledAddresses) == 0 {
		fmt.Println(logging.Red.Wrap("Caution: Allow lists are empty. You will not be able to easily change the precompile settings in the future."))
		fmt.Println()
	}
}

func addRoleToPreviewTable(table *tablewriter.Table, name string, addresses []common.Address) {
	if len(addresses) == 0 {
		table.Append([]string{name, strings.Repeat(" ", 11)})
	} else {
		addressesStr := strings.Join(utils.Map(addresses, func(a common.Address) string { return a.Hex() }), "\n")
		table.Append([]string{name, addressesStr})
	}
}

func getNewAddresses(
	app *application.Avalanche,
	adminAddresses []common.Address,
	managerAddresses []common.Address,
	enabledAddresses []common.Address,
) ([]common.Address, error) {
	newAddresses := []common.Address{}
	addresses, err := app.Prompt.CaptureAddresses("Enter the address of the account (or multiple comma separated):")
	if err != nil {
		return nil, err
	}
	for _, address := range addresses {
		switch {
		case utils.Belongs(adminAddresses, address):
			ux.Logger.RedXToUser(address.Hex() + " is already allowed as admin role")
		case utils.Belongs(managerAddresses, address):
			ux.Logger.RedXToUser(address.Hex() + " is already allowed as manager role")
		case utils.Belongs(enabledAddresses, address):
			ux.Logger.RedXToUser(address.Hex() + " is already allowed as enabled role")
		default:
			newAddresses = append(newAddresses, address)
		}
	}
	return newAddresses, nil
}

func removeAddress(
	app *application.Avalanche,
	addresses []common.Address,
	kind string,
) ([]common.Address, bool, error) {
	if len(addresses) == 0 {
		fmt.Printf("There are no %s addresses to remove from\n", kind)
		fmt.Println()
		return addresses, true, nil
	}
	cancelOption := "Cancel"
	prompt := "Select the address you want to remove"
	options := utils.Map(addresses, func(a common.Address) string { return a.Hex() })
	options = append(options, cancelOption)
	opt, err := app.Prompt.CaptureList(prompt, options)
	if err != nil {
		return addresses, false, err
	}
	if opt != cancelOption {
		addresses = utils.RemoveFromSlice(addresses, common.HexToAddress(opt))
		return addresses, false, nil
	}
	return addresses, true, nil
}

func GenerateAllowList(
	app *application.Avalanche,
	action string,
	evmVersion string,
) ([]common.Address, []common.Address, []common.Address, bool, error) {
	if !semver.IsValid(evmVersion) {
		return nil, nil, nil, false, fmt.Errorf("invalid semantic version %q", evmVersion)
	}
	managerRoleEnabled := semver.Compare(evmVersion, "v0.6.4") >= 0

	adminAddresses := []common.Address{}
	managerAddresses := []common.Address{}
	enabledAddresses := []common.Address{}

	promptTemplate := "Configure the addresses that are allowed to %s"
	prompt := fmt.Sprintf(promptTemplate, action)

	addOption := "Add an address for a role to the allow list"
	removeOption := "Remove address from the allow list"
	previewOption := "Preview Allow List"
	confirmOption := "Confirm Allow List"
	cancelOption := "Cancel"

	adminOption := "Admin"
	managerOption := "Manager"
	enabledOption := "Enabled"
	explainOption := "Explain the difference"

	for {
		options := []string{addOption, removeOption, previewOption, confirmOption, cancelOption}
		if len(adminAddresses) == 0 && len(managerAddresses) == 0 && len(enabledAddresses) == 0 {
			options = utils.RemoveFromSlice(options, removeOption)
		}
		option, err := app.Prompt.CaptureList(prompt, options)
		if err != nil {
			return nil, nil, nil, false, err
		}
		switch option {
		case addOption:
			addPrompt := "What role should the address have?"
			for {
				options := []string{adminOption, managerOption, enabledOption, explainOption, cancelOption}
				if !managerRoleEnabled {
					options = []string{adminOption, enabledOption, explainOption, cancelOption}
				}
				roleOption, err := app.Prompt.CaptureList(addPrompt, options)
				if err != nil {
					return nil, nil, nil, false, err
				}
				switch roleOption {
				case adminOption:
					addresses, err := getNewAddresses(app, adminAddresses, managerAddresses, enabledAddresses)
					if err != nil {
						return nil, nil, nil, false, err
					}
					adminAddresses = append(adminAddresses, addresses...)
				case managerOption:
					addresses, err := getNewAddresses(app, adminAddresses, managerAddresses, enabledAddresses)
					if err != nil {
						return nil, nil, nil, false, err
					}
					managerAddresses = append(managerAddresses, addresses...)
				case enabledOption:
					addresses, err := getNewAddresses(app, adminAddresses, managerAddresses, enabledAddresses)
					if err != nil {
						return nil, nil, nil, false, err
					}
					enabledAddresses = append(enabledAddresses, addresses...)
				case explainOption:
					fmt.Println("Enabled addresses can perform the permissioned behavior (issuing transactions, deploying contracts,\netc.), but cannot modify other roles.\nManager addresses can perform the permissioned behavior and can change enabled/disable addresses.\nAdmin addresses can perform the permissioned behavior, but can also add/remove other Admins, Managers\nand Enabled addresses.")
					fmt.Println()
					continue
				case cancelOption:
				}
				break
			}
		case removeOption:
			keepAsking := true
			for keepAsking {
				removePrompt := "What role does the address that should be removed have?"
				options := []string{}
				if len(adminAddresses) != 0 {
					options = append(options, adminOption)
				}
				if len(managerAddresses) != 0 && managerRoleEnabled {
					options = append(options, managerOption)
				}
				if len(enabledAddresses) != 0 {
					options = append(options, enabledOption)
				}
				options = append(options, cancelOption)
				roleOption, err := app.Prompt.CaptureList(removePrompt, options)
				if err != nil {
					return nil, nil, nil, false, err
				}
				switch roleOption {
				case adminOption:
					adminAddresses, keepAsking, err = removeAddress(app, adminAddresses, "admin")
					if err != nil {
						return nil, nil, nil, false, err
					}
				case managerOption:
					managerAddresses, keepAsking, err = removeAddress(app, managerAddresses, "manager")
					if err != nil {
						return nil, nil, nil, false, err
					}
				case enabledOption:
					enabledAddresses, keepAsking, err = removeAddress(app, enabledAddresses, "enabled")
					if err != nil {
						return nil, nil, nil, false, err
					}
				case cancelOption:
					keepAsking = false
				}
			}
		case previewOption:
			preview(adminAddresses, managerAddresses, enabledAddresses)
		case confirmOption:
			preview(adminAddresses, managerAddresses, enabledAddresses)
			confirmPrompt := "Confirm?"
			yesOption := "Yes"
			noOption := "No, keep editing"
			confirmOption, err := app.Prompt.CaptureList(
				confirmPrompt, []string{yesOption, noOption},
			)
			if err != nil {
				return nil, nil, nil, false, err
			}
			if confirmOption == yesOption {
				return adminAddresses, managerAddresses, enabledAddresses, false, nil
			}
		case cancelOption:
			return nil, nil, nil, true, err
		}
	}
}
