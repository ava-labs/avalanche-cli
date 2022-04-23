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
	"strconv"

	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/precompile"
	"github.com/ethereum/go-ethereum/common"
	"github.com/manifoldco/promptui"
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

func validateInt(input string) error {
	_, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		return errors.New("Invalid number")
	}
	return nil
}

func validateBigInt(input string) error {
	n := new(big.Int)
	n, ok := n.SetString(input, 10)
	if !ok {
		return errors.New("Invalid number")
	}
	return nil
}

func validateAddress(input string) error {
	if !common.IsHexAddress(input) {
		return errors.New("Invalid address")
	}
	return nil
}

func captureAddress(promptStr string) (common.Address, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateAddress,
	}

	addressStr, err := prompt.Run()
	if err != nil {
		return common.Address{}, err
	}

	addressHex := common.HexToAddress(addressStr)
	return addressHex, nil
}

func captureYesNo(promptStr string) (bool, error) {
	const yes = "Yes"
	const no = "No"
	prompt := promptui.Select{
		Label: promptStr,
		Items: []string{yes, no},
	}

	_, decision, err := prompt.Run()
	if err != nil {
		return false, err
	}
	return decision == yes, nil
}

func getChainId() (*big.Int, error) {
	// TODO check positivity
	// TODO check against known chain ids and provide warning
	fmt.Println("Select your subnet's ChainId. It can be any positive integer.")

	// Set chainId
	prompt := promptui.Prompt{
		Label:    "ChainId",
		Validate: validateInt,
	}

	chainIdStr, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	chainIdInt, err := strconv.ParseInt(chainIdStr, 10, 64)
	if err != nil {
		// should never reach here
		return nil, err
	}
	return big.NewInt(chainIdInt), nil
}

func getAllocation() (core.GenesisAlloc, error) {
	done := false
	first := true

	allocation := core.GenesisAlloc{}

	for !done {
		firstStr := "Would you like to airdrop tokens?"
		secondStr := "Would you like to airdrop more tokens?"

		var promptStr string
		if promptStr = secondStr; first == true {
			promptStr = firstStr
			first = false
		}

		prompt1 := promptui.Select{
			Label: promptStr,
			Items: []string{"Yes", "No"},
		}

		_, decision, err := prompt1.Run()
		if err != nil {
			return nil, err
		}

		if decision == "Yes" {
			addressHex, err := captureAddress("Address")
			if err != nil {
				return nil, err
			}

			prompt3 := promptui.Prompt{
				Label:    "Amount (in wei)",
				Validate: validateBigInt,
			}

			amountStr, err := prompt3.Run()
			if err != nil {
				return nil, err
			}

			amountInt := new(big.Int)
			amountInt, ok := amountInt.SetString(amountStr, 10)
			if !ok {
				return nil, errors.New("SetString: error")
			}

			account := core.GenesisAccount{
				Balance: amountInt,
			}

			allocation[addressHex] = account

		} else {
			done = true
		}
	}
	return allocation, nil
}

func configureContractAllowList() (precompile.ContractDeployerAllowListConfig, error) {
	addAdmin := "Add admin"
	// addWhitelist := "Add whitelisted"
	preview := "Preview"
	doneMsg := "Done"

	config := precompile.ContractDeployerAllowListConfig{}
	allowList := precompile.AllowListConfig{
		BlockTimestamp:  big.NewInt(0),
		AllowListAdmins: []common.Address{},
	}

	for {
		prompt2 := promptui.Select{
			Label: "Configure contract deployment allow list:",
			Items: []string{addAdmin, preview, doneMsg},
		}

		_, listDecision, err := prompt2.Run()
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

func getPrecompiles(config params.ChainConfig) (params.ChainConfig, error) {
	const nativeMint = "Native Minting"
	const contractAllowList = "Contract deployment whitelist"
	const txAllowList = "Transaction allow list"
	const cancel = "Cancel"

	first := true

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
			prompt2 := promptui.Select{
				Label: "Choose precompile:",
				Items: []string{nativeMint, contractAllowList, txAllowList, cancel},
			}

			_, precompileDecision, err := prompt2.Run()
			if err != nil {
				return config, err
			}

			switch precompileDecision {
			case nativeMint:
				fmt.Println("TODO")
			case contractAllowList:
				contractConfig, err := configureContractAllowList()
				if err != nil {
					return config, err
				}
				config.ContractDeployerAllowListConfig = contractConfig
			case txAllowList:
				fmt.Println("TODO")
			case cancel:
				return config, nil
			}

		} else {
			return config, nil
		}
	}
}

func createGenesis(cmd *cobra.Command, args []string) {
	fmt.Println("creating subnet", args[0])
	// fmt.Println(*cmd)

	if filename == "" {
		fmt.Println("Let's create a genesis")
		genesis := core.Genesis{}
		conf := params.SubnetEVMDefaultChainConfig
		// conf.SetDefaults()

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
