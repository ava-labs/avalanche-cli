// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"fmt"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/subnet-evm/precompile/allowlist"
	"github.com/ava-labs/subnet-evm/precompile/contracts/txallowlist"
	"github.com/ava-labs/subnet-evm/precompile/precompileconfig"
	subnetevmutils "github.com/ava-labs/subnet-evm/utils"

	"github.com/ethereum/go-ethereum/common"
	"github.com/olekukonko/tablewriter"
)

func GetAddressList(
	initialPrompt string,
	info string,
	app *application.Avalanche,
) ([]common.Address, bool, error) {
	label := "Address"

	return prompts.CaptureListDecision(
		app.Prompt,
		initialPrompt,
		app.Prompt.CaptureAddress,
		"Enter Address ",
		label,
		info,
	)
}

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
}

func addRoleToPreviewTable(table *tablewriter.Table, name string, addresses []common.Address) {
	if len(addresses) == 0 {
		table.Append([]string{name, strings.Repeat(" ", 11)})
	} else {
		addressesStr := strings.Join(utils.Map(addresses, func(a common.Address) string { return a.Hex() }), "\n")
		table.Append([]string{name, addressesStr})
		/*
			for _, address := range addresses {
				table.Append([]string{name, address.Hex()})
			}
		*/
	}
}

func ConfigureTransactionAllowList(app *application.Avalanche) (txallowlist.Config, bool, error) {
	config := txallowlist.Config{}

	adminAddresses := []common.Address{}
	managerAddresses := []common.Address{}
	enabledAddresses := []common.Address{}

	action := "issue transactions"
	promptTemplate := "Configure the addresses that are allowed to %s"
	prompt := fmt.Sprintf(promptTemplate, action)

	addOption := "Add an address for a role to the allow list"
	removeOption := "Remove address from the allow list"
	previewOption := "Preview Allow List"
	confirmOption := "Confirm Allow List"
	cancelOption := "Cancel"
	continueEditing := true
	for continueEditing {
		option, err := app.Prompt.CaptureList(
			prompt, []string{addOption, removeOption, previewOption, confirmOption, cancelOption},
		)
		if err != nil {
			return config, false, err
		}
		switch option {
		case addOption:
			addPrompt := "What role should the address have?"
			adminOption := "Admin"
			managerOption := "Manager"
			enabledOption := "Enabled"
			explainOption := "Explain the difference"
			keepPrompting := true
			for keepPrompting {
				keepPrompting = false
				roleOption, err := app.Prompt.CaptureList(
					addPrompt, []string{adminOption, managerOption, enabledOption, explainOption, cancelOption},
				)
				switch roleOption {
				case adminOption:
					addresses, err := app.Prompt.CaptureAddresses("Enter the address of the account (or multiple comma separated):")
					if err != nil {
						return config, false, err
					}
					for _, address := range addresses {
						if utils.Belongs(adminAddresses, address) {
							fmt.Println(address.Hex() + " is already allowed")
						} else {
							adminAddresses = append(adminAddresses, address)
						}
					}
				case explainOption:
					fmt.Println("The difference is...")
					keepPrompting = true
				case cancelOption:
				}
				if err != nil {
					return config, false, err
				}
			}
		case previewOption:
			preview(adminAddresses, managerAddresses, enabledAddresses)
		case confirmOption:
			preview(adminAddresses, managerAddresses, enabledAddresses)
		case cancelOption:
			return config, true, nil
		}
	}
	return config, false, nil

	adminPrompt := "Configure transaction allow list admin addresses"
	managerPrompt := "Configure transaction allow list manager addresses"
	enabledPrompt := "Configure transaction allow list enabled addresses"
	info := "\nThis precompile restricts who has the ability to issue transactions " +
		"on your subnet.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet/#restricting-who-can-submit-transactions\n\n"

	admins, managers, enabled, cancelled, err := GetAdminManagerAndEnabledAddresses(
		adminPrompt,
		managerPrompt,
		enabledPrompt,
		info,
		app,
	)
	if err != nil {
		return config, false, err
	}

	config.AllowListConfig = allowlist.AllowListConfig{
		AdminAddresses:   admins,
		ManagerAddresses: managers,
		EnabledAddresses: enabled,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: subnetevmutils.NewUint64(0),
	}

	return config, cancelled, nil
}

func GetAdminManagerAndEnabledAddresses(
	adminPrompt string,
	managerPrompt string,
	enabledPrompt string,
	info string,
	app *application.Avalanche,
) ([]common.Address, []common.Address, []common.Address, bool, error) {
	admins, cancelled, err := GetAddressList(adminPrompt, info, app)
	if err != nil || cancelled {
		return nil, nil, nil, false, err
	}
	adminsMap := make(map[string]bool)
	for _, adminsAddress := range admins {
		adminsMap[adminsAddress.String()] = true
	}
	managers, cancelled, err := GetAddressList(managerPrompt, info, app)
	if err != nil || cancelled {
		return nil, nil, nil, false, err
	}
	managersMap := make(map[string]bool)
	for _, managerAddress := range managers {
		managersMap[managerAddress.String()] = true
	}
	enabled, cancelled, err := GetAddressList(enabledPrompt, info, app)
	if err != nil {
		return nil, nil, nil, false, err
	}
	for _, managerAddress := range managers {
		if _, ok := adminsMap[managerAddress.String()]; ok {
			return nil, nil, nil, false, fmt.Errorf(
				"can't have address %s in both admin and manager addresses",
				managerAddress.String(),
			)
		}
	}
	for _, enabledAddress := range enabled {
		if _, ok := adminsMap[enabledAddress.String()]; ok {
			return nil, nil, nil, false, fmt.Errorf(
				"can't have address %s in both admin and enabled addresses",
				enabledAddress.String(),
			)
		}
		if _, ok := managersMap[enabledAddress.String()]; ok {
			return nil, nil, nil, false, fmt.Errorf(
				"can't have address %s in both manager and enabled addresses",
				enabledAddress.String(),
			)
		}
	}
	return admins, managers, enabled, cancelled, nil
}
