// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"fmt"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/utils"

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
		option, err := app.Prompt.CaptureList(
			prompt, []string{addOption, removeOption, previewOption, confirmOption, cancelOption},
		)
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
					fmt.Println("The difference to be given by devrel people")
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
				options := []string{adminOption, managerOption, enabledOption, cancelOption}
				if !managerRoleEnabled {
					options = []string{adminOption, enabledOption, cancelOption}
				}
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
			preview(allowList)
		case confirmOption:
			if len(allowList.AdminAddresses) == 0 && len(allowList.ManagerAddresses) == 0 && len(allowList.EnabledAddresses) == 0 {
				fmt.Println("We need at least one address to have been added to the allow list. Otherwise cancel.")
				fmt.Println()
				continue
			}
			preview(allowList)
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
