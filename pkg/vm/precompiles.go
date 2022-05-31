package vm

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/precompile"
	"github.com/ethereum/go-ethereum/common"
)

func contains(list []common.Address, element common.Address) bool {
	for _, val := range list {
		if val == element {
			return true
		}
	}
	return false
}

func getAdminList(initialPrompt string, info string) ([]common.Address, error) {
	const (
		addAdmin    = "Add admin"
		removeAdmin = "Remove admin"
		preview     = "Preview"
		moreInfo    = "More info"
		doneMsg     = "Done"
		cancelMsg   = "Cancel"
	)

	admins := []common.Address{}

	for {
		listDecision, err := prompts.CaptureList(
			initialPrompt,
			[]string{addAdmin, removeAdmin, preview, moreInfo, doneMsg, cancelMsg},
		)
		if err != nil {
			return []common.Address{}, err
		}

		switch listDecision {
		case addAdmin:
			adminAddr, err := prompts.CaptureAddress("Admin Address")
			if err != nil {
				return []common.Address{}, err
			}
			if contains(admins, adminAddr) {
				fmt.Println("Address already an admin")
				continue
			}
			admins = append(admins, adminAddr)
		case removeAdmin:
			index, err := prompts.CaptureIndex("Choose address to remove:", admins)
			if err != nil {
				return []common.Address{}, err
			}
			admins = append(admins[:index], admins[index+1:]...)
		case preview:
			fmt.Println("Admins:")
			for i, addr := range admins {
				fmt.Printf("%d. %s\n", i, addr.Hex())
			}
		case doneMsg:
			return admins, nil
		case moreInfo:
			fmt.Print(info)
		case cancelMsg:
			return []common.Address{}, nil
		default:
			return []common.Address{}, errors.New("Unexpected option")
		}
	}
}

func configureContractAllowList() (precompile.ContractDeployerAllowListConfig, error) {
	config := precompile.ContractDeployerAllowListConfig{}
	prompt := "Configure contract deployment allow list"
	info := "\nThis precompile restricts who has the ability to deploy contracts " +
		"on your subnet.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet/#restricting-smart-contract-deployers\n\n"

	admins, err := getAdminList(prompt, info)
	if err != nil {
		return config, err
	}

	allowList := precompile.AllowListConfig{
		BlockTimestamp:  big.NewInt(0),
		AllowListAdmins: admins,
	}

	config.AllowListConfig = allowList
	return config, nil
}

func configureTransactionAllowList() (precompile.TxAllowListConfig, error) {
	config := precompile.TxAllowListConfig{}
	prompt := "Configure transaction allow list"
	info := "\nThis precompile restricts who has the ability to issue transactions " +
		"on your subnet.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet/#restricting-who-can-submit-transactions\n\n"

	admins, err := getAdminList(prompt, info)
	if err != nil {
		return config, err
	}

	allowList := precompile.AllowListConfig{
		BlockTimestamp:  big.NewInt(0),
		AllowListAdmins: admins,
	}

	config.AllowListConfig = allowList
	return config, nil
}

func configureMinterList() (precompile.ContractNativeMinterConfig, error) {
	config := precompile.ContractNativeMinterConfig{}
	prompt := "Configure native minting allow list"
	info := "\nThis precompile allows admins to permit designated contracts to mint the native token " +
		"on your subnet.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet#minting-native-coins\n\n"

	admins, err := getAdminList(prompt, info)
	if err != nil {
		return config, err
	}

	allowList := precompile.AllowListConfig{
		BlockTimestamp:  big.NewInt(0),
		AllowListAdmins: admins,
	}

	config.AllowListConfig = allowList
	return config, nil
}

func removePrecompile(arr []string, s string) ([]string, error) {
	for i, val := range arr {
		if val == s {
			return append(arr[:i], arr[i+1:]...), nil
		}
	}
	return arr, errors.New("String not in array")
}

func getPrecompiles(config params.ChainConfig) (params.ChainConfig, error) {
	const (
		nativeMint        = "Native Minting"
		contractAllowList = "Contract deployment whitelist"
		txAllowList       = "Transaction allow list"
		cancel            = "Cancel"
	)

	first := true

	remainingPrecompiles := []string{nativeMint, contractAllowList, txAllowList, cancel}

	for {
		firstStr := "Would you like to add a custom precompile?"
		secondStr := "Would you like to add additional precompiles?"

		var promptStr string
		if promptStr = secondStr; first {
			promptStr = firstStr
			first = false
		}

		addPrecompile, err := prompts.CaptureYesNo(promptStr)
		if err != nil {
			return config, err
		}

		if addPrecompile {
			precompileDecision, err := prompts.CaptureList(
				"Choose precompile",
				remainingPrecompiles,
			)
			if err != nil {
				return config, err
			}

			switch precompileDecision {
			case nativeMint:
				mintConfig, err := configureMinterList()
				if err != nil {
					return config, err
				}
				if len(mintConfig.AllowListAdmins) > 0 {
					config.ContractNativeMinterConfig = mintConfig
					remainingPrecompiles, err = removePrecompile(remainingPrecompiles, nativeMint)
				}
				if err != nil {
					return config, err
				}
			case contractAllowList:
				contractConfig, err := configureContractAllowList()
				if err != nil {
					return config, err
				}
				if len(contractConfig.AllowListAdmins) > 0 {
					config.ContractDeployerAllowListConfig = contractConfig
					remainingPrecompiles, err = removePrecompile(remainingPrecompiles, contractAllowList)
				}
				if err != nil {
					return config, err
				}
			case txAllowList:
				txConfig, err := configureTransactionAllowList()
				if err != nil {
					return config, err
				}
				if len(txConfig.AllowListAdmins) > 0 {
					config.TxAllowListConfig = txConfig
					remainingPrecompiles, err = removePrecompile(remainingPrecompiles, txAllowList)
				}
				if err != nil {
					return config, err
				}
			case cancel:
				return config, nil
			}

			if len(remainingPrecompiles) == 1 {
				return config, nil
			}

		} else {
			return config, nil
		}
	}
}
