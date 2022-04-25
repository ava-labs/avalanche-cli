/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/user"
	"path/filepath"

	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/precompile"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

var filename string
var fast bool

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create [subnetName]",
	Short: "Create a new subnet genesis",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Args: cobra.ExactArgs(1),
	Run:  createGenesis,
}

func init() {
	subnetCmd.AddCommand(createCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// createCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	createCmd.Flags().StringVarP(&filename, "filename", "f", "", "filepath of genesis to use")
	createCmd.Flags().BoolVarP(&fast, "fast", "z", false, "use default values to minimize configuration")
}

func getChainId() (*big.Int, error) {
	// TODO check against known chain ids and provide warning
	fmt.Println("Select your subnet's ChainId. It can be any positive integer.")

	chainId, err := capturePositiveBigInt("ChainId")
	if err != nil {
		return nil, err
	}

	return chainId, nil
}

func getAllocation() (core.GenesisAlloc, error) {
	first := true

	allocation := core.GenesisAlloc{}

	for {
		firstStr := "Would you like to airdrop tokens?"
		secondStr := "Would you like to airdrop more tokens?"

		var promptStr string
		if promptStr = secondStr; first == true {
			promptStr = firstStr
			first = false
		}

		continueAirdrop, err := captureYesNo(promptStr)
		if err != nil {
			return nil, err
		}

		if continueAirdrop {
			addressHex, err := captureAddress("Address")
			if err != nil {
				return nil, err
			}

			amount, err := capturePositiveBigInt("Amount (in wei)")
			if err != nil {
				return nil, err
			}

			account := core.GenesisAccount{
				Balance: amount,
			}

			allocation[addressHex] = account

		} else {
			return allocation, nil
		}
	}
}

func configureContractAllowList() (precompile.ContractDeployerAllowListConfig, error) {
	addAdmin := "Add admin"
	preview := "Preview"
	doneMsg := "Done"

	config := precompile.ContractDeployerAllowListConfig{}
	allowList := precompile.AllowListConfig{
		BlockTimestamp:  big.NewInt(0),
		AllowListAdmins: []common.Address{},
	}

	for {
		listDecision, err := captureList(
			"Configure contract deployment allow list:",
			[]string{addAdmin, preview, doneMsg},
		)
		if err != nil {
			return config, err
		}

		switch listDecision {
		case addAdmin:
			adminAddr, err := captureAddress("Admin Address")
			if err != nil {
				return config, err
			}
			allowList.AllowListAdmins = append(allowList.AllowListAdmins, adminAddr)
		case preview:
			fmt.Println("Admins:")
			for i, addr := range allowList.AllowListAdmins {
				fmt.Printf("%d. %s\n", i, addr.Hex())
			}
		case doneMsg:
			config.AllowListConfig = allowList
			return config, nil
		default:
			return config, errors.New("Unexpected option")
		}
	}
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
	const nativeMint = "Native Minting"
	const contractAllowList = "Contract deployment whitelist"
	const txAllowList = "Transaction allow list"
	const cancel = "Cancel"

	first := true

	remainingPrecompiles := []string{nativeMint, contractAllowList, txAllowList, cancel}

	for {
		firstStr := "Would you like to add a custom precompile?"
		secondStr := "Would you like to add additional precompiles?"

		var promptStr string
		if promptStr = secondStr; first == true {
			promptStr = firstStr
			first = false
		}

		addPrecompile, err := captureYesNo(promptStr)
		if err != nil {
			return config, err
		}

		if addPrecompile {
			precompileDecision, err := captureList(
				"Choose precompile:",
				remainingPrecompiles,
			)
			if err != nil {
				return config, err
			}

			switch precompileDecision {
			case nativeMint:
				fmt.Println("TODO")
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, nativeMint)
				if err != nil {
					return config, err
				}
			case contractAllowList:
				contractConfig, err := configureContractAllowList()
				if err != nil {
					return config, err
				}
				config.ContractDeployerAllowListConfig = contractConfig
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, contractAllowList)
				if err != nil {
					return config, err
				}
			case txAllowList:
				fmt.Println("TODO")
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, txAllowList)
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

func createGenesis(cmd *cobra.Command, args []string) {
	fmt.Println("creating subnet", args[0])

	if filename == "" {
		genesis := core.Genesis{}
		conf := params.SubnetEVMDefaultChainConfig

		chainId, err := getChainId()
		if err != nil {
			fmt.Println(err)
			return
		}
		conf.ChainID = chainId

		allocation, err := getAllocation()
		if err != nil {
			fmt.Println(err)
			return
		}

		*conf, err = getPrecompiles(*conf)
		if err != nil {
			fmt.Println(err)
			return
		}

		genesis.Alloc = allocation

		genesis.Config = conf

		jsonBytes, err := genesis.MarshalJSON()
		if err != nil {
			fmt.Println(err)
			return
		}

		var prettyJSON bytes.Buffer
		err = json.Indent(&prettyJSON, jsonBytes, "", "    ")
		if err != nil {
			log.Println("JSON parse error: ", err)
			return
		}

		fmt.Println(string(prettyJSON.Bytes()))
		usr, _ := user.Current()
		newpath := filepath.Join(usr.HomeDir, ".avalanche-cli")
		genesisPath := filepath.Join(newpath, args[0]+"_genesis.json")
		err = os.WriteFile(genesisPath, prettyJSON.Bytes(), 0644)
		if err != nil {
			fmt.Println(err)
			return
		}
	} else {
		fmt.Println("Using specified genesis")
	}
}
