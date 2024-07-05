// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"fmt"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/utils"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ethereum/go-ethereum/common"
	"github.com/olekukonko/tablewriter"
	"golang.org/x/mod/semver"
)

type AllowList struct {
	AdminAddresses   []common.Address
	ManagerAddresses []common.Address
	EnabledAddresses []common.Address
}

func preview(allowList AllowList) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	addRoleToPreviewTable(table, "Admins", allowList.AdminAddresses)
	addRoleToPreviewTable(table, "Manager", allowList.ManagerAddresses)
	addRoleToPreviewTable(table, "Enabled", allowList.EnabledAddresses)
	table.Render()
	fmt.Println()
	if len(allowList.AdminAddresses) == 0 && len(allowList.ManagerAddresses) == 0 && len(allowList.EnabledAddresses) == 0 {
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
	allowList AllowList,
) ([]common.Address, error) {
	newAddresses := []common.Address{}
	addresses, err := app.Prompt.CaptureAddresses("Enter the address of the account (or multiple comma separated):")
	if err != nil {
		return nil, err
	}
	for _, address := range addresses {
		switch {
		case utils.Belongs(allowList.AdminAddresses, address):
			fmt.Println(address.Hex() + " is already allowed as admin role")
		case utils.Belongs(allowList.ManagerAddresses, address):
			fmt.Println(address.Hex() + " is already allowed as manager role")
		case utils.Belongs(allowList.EnabledAddresses, address):
			fmt.Println(address.Hex() + " is already allowed as enabled role")
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
) (AllowList, bool, error) {
	if !semver.IsValid(evmVersion) {
		return AllowList{}, false, fmt.Errorf("invalid semantic version %q", evmVersion)
	}
	managerRoleEnabled := semver.Compare(evmVersion, "v0.6.4") >= 0

	allowList := AllowList{}

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
			return AllowList{}, false, err
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
					return AllowList{}, false, err
				}
				switch roleOption {
				case adminOption:
					addresses, err := getNewAddresses(app, allowList)
					if err != nil {
						return AllowList{}, false, err
					}
					allowList.AdminAddresses = append(allowList.AdminAddresses, addresses...)
				case managerOption:
					addresses, err := getNewAddresses(app, allowList)
					if err != nil {
						return AllowList{}, false, err
					}
					allowList.ManagerAddresses = append(allowList.ManagerAddresses, addresses...)
				case enabledOption:
					addresses, err := getNewAddresses(app, allowList)
					if err != nil {
						return AllowList{}, false, err
					}
					allowList.EnabledAddresses = append(allowList.EnabledAddresses, addresses...)
				case explainOption:
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
					return AllowList{}, false, err
				}
				switch roleOption {
				case adminOption:
					allowList.AdminAddresses, keepAsking, err = removeAddress(app, allowList.AdminAddresses, "admin")
					if err != nil {
						return AllowList{}, false, err
					}
				case managerOption:
					allowList.ManagerAddresses, keepAsking, err = removeAddress(app, allowList.ManagerAddresses, "manager")
					if err != nil {
						return AllowList{}, false, err
					}
				case enabledOption:
					allowList.EnabledAddresses, keepAsking, err = removeAddress(app, allowList.EnabledAddresses, "enabled")
					if err != nil {
						return AllowList{}, false, err
					}
				case cancelOption:
					keepAsking = false
				}
			}
		case previewOption:
			confirmPrompt := "Confirm?"
			yesOption := "Yes"
			noOption := "No, keep editing"
			confirmOption, err := app.Prompt.CaptureList(
				confirmPrompt, []string{yesOption, noOption},
			)
			if err != nil {
				return AllowList{}, false, err
			}
			if confirmOption == yesOption {
				return allowList, false, nil
			}
		case cancelOption:
			return AllowList{}, true, err
		}
	}
}
