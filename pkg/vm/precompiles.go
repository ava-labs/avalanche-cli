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

const stageAfterPrecompile = doneStage
const stageBeforePrecompile = airdropStage

func contains(list []common.Address, element common.Address) bool {
	for _, val := range list {
		if val == element {
			return true
		}
	}
	return false
}

func getAdminList(initialPrompt string, info string) ([]common.Address, bool, error) {
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
			return []common.Address{}, false, err
		}

		switch listDecision {
		case addAdmin:
			adminAddr, err := prompts.CaptureAddress("Admin Address")
			if err != nil {
				return []common.Address{}, false, err
			}
			if contains(admins, adminAddr) {
				fmt.Println("Address already an admin")
				continue
			}
			admins = append(admins, adminAddr)
		case removeAdmin:
			index, err := prompts.CaptureIndex("Choose address to remove:", admins)
			if err != nil {
				return []common.Address{}, false, err
			}
			admins = append(admins[:index], admins[index+1:]...)
		case preview:
			fmt.Println("Admins:")
			for i, addr := range admins {
				fmt.Printf("%d. %s\n", i, addr.Hex())
			}
		case doneMsg:
			cancelled := len(admins) == 0
			return admins, cancelled, nil
		case moreInfo:
			fmt.Print(info)
		case cancelMsg:
			return []common.Address{}, true, nil
		default:
			return []common.Address{}, false, errors.New("Unexpected option")
		}
	}
}

func configureContractAllowList() (precompile.ContractDeployerAllowListConfig, bool, error) {
	config := precompile.ContractDeployerAllowListConfig{}
	prompt := "Configure contract deployment allow list"
	info := "\nThis precompile restricts who has the ability to deploy contracts " +
		"on your subnet.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet/#restricting-smart-contract-deployers\n\n"

	admins, cancelled, err := getAdminList(prompt, info)
	if err != nil {
		return config, false, err
	}

	config.AllowListConfig = precompile.AllowListConfig{
		BlockTimestamp:  big.NewInt(0),
		AllowListAdmins: admins,
	}

	return config, cancelled, nil
}

func configureTransactionAllowList() (precompile.TxAllowListConfig, bool, error) {
	config := precompile.TxAllowListConfig{}
	prompt := "Configure transaction allow list"
	info := "\nThis precompile restricts who has the ability to issue transactions " +
		"on your subnet.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet/#restricting-who-can-submit-transactions\n\n"

	admins, cancelled, err := getAdminList(prompt, info)
	if err != nil {
		return config, false, err
	}

	config.AllowListConfig = precompile.AllowListConfig{
		BlockTimestamp:  big.NewInt(0),
		AllowListAdmins: admins,
	}

	return config, cancelled, nil
}

func configureMinterList() (precompile.ContractNativeMinterConfig, bool, error) {
	config := precompile.ContractNativeMinterConfig{}
	prompt := "Configure native minting allow list"
	info := "\nThis precompile allows admins to permit designated contracts to mint the native token " +
		"on your subnet.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet#minting-native-coins\n\n"

	admins, cancelled, err := getAdminList(prompt, info)
	if err != nil {
		return config, false, err
	}

	config.AllowListConfig = precompile.AllowListConfig{
		BlockTimestamp:  big.NewInt(0),
		AllowListAdmins: admins,
	}

	return config, cancelled, nil
}

func removePrecompile(arr []string, s string) ([]string, error) {
	for i, val := range arr {
		if val == s {
			return append(arr[:i], arr[i+1:]...), nil
		}
	}
	return arr, errors.New("String not in array")
}

func getPrecompiles(config params.ChainConfig) (params.ChainConfig, creationStage, error) {
	const (
		nativeMint        = "Native Minting"
		contractAllowList = "Contract deployment whitelist"
		txAllowList       = "Transaction allow list"
		cancel            = "Cancel"
	)

	first := true

	remainingPrecompiles := []string{nativeMint, contractAllowList, txAllowList, cancel}

	for {
		firstStr := "Advanced: Would you like to add a custom precompile to modify the EVM?"
		secondStr := "Would you like to add additional precompiles?"

		var promptStr string
		if promptStr = secondStr; first {
			promptStr = firstStr
			first = false
		}

		addPrecompile, err := prompts.CaptureList(promptStr, []string{prompts.No, prompts.Yes, goBackMsg})
		if err != nil {
			return config, errored, err
		}

		switch addPrecompile {
		case prompts.No:
			return config, stageAfterPrecompile, nil
		case goBackMsg:
			return config, stageBeforePrecompile, nil
		}

		precompileDecision, err := prompts.CaptureList(
			"Choose precompile",
			remainingPrecompiles,
		)
		if err != nil {
			return config, errored, err
		}

		switch precompileDecision {
		case nativeMint:
			mintConfig, cancelled, err := configureMinterList()
			if err != nil {
				return config, errored, err
			}
			if !cancelled {
				config.ContractNativeMinterConfig = mintConfig
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, nativeMint)
				if err != nil {
					return config, errored, err
				}
			}
		case contractAllowList:
			contractConfig, cancelled, err := configureContractAllowList()
			if err != nil {
				return config, errored, err
			}
			if !cancelled {
				config.ContractDeployerAllowListConfig = contractConfig
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, contractAllowList)
				if err != nil {
					return config, errored, err
				}
			}
		case txAllowList:
			txConfig, cancelled, err := configureTransactionAllowList()
			if err != nil {
				return config, errored, err
			}
			if !cancelled {
				config.TxAllowListConfig = txConfig
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, txAllowList)
				if err != nil {
					return config, errored, err
				}
			}
		case cancel:
			return config, stageAfterPrecompile, nil
		}

		if len(remainingPrecompiles) == 1 {
			return config, stageAfterPrecompile, nil
		}

	}
}
