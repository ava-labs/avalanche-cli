// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var readCmd = &cobra.Command{
	Use:   "describe",
	Short: "Print a summary of the subnet’s configuration",
	RunE:  readGenesis,
	Args:  cobra.ExactArgs(1),
}

var printGenesisOnly bool

func printGenesis(subnetName string) error {
	genesisFile := filepath.Join(baseDir, subnetName+genesis_suffix)
	gen, err := os.ReadFile(genesisFile)
	if err != nil {
		return err
	}
	fmt.Println(string(gen))
	return nil
}

func printGasTable(genesis core.Genesis) {
	// Generated here with BIG font
	// https://patorjk.com/software/taag/#p=display&f=Big&t=Precompiles
	const art = `
  _____              _____             __ _
 / ____|            / ____|           / _(_)
| |  __  __ _ ___  | |     ___  _ __ | |_ _  __ _
| | |_ |/ _` + `  / __| | |    / _ \| '_ \|  _| |/ _` + `  |
| |__| | (_| \__ \ | |___| (_) | | | | | | | (_| |
 \_____|\__,_|___/  \_____\___/|_| |_|_| |_|\__, |
                                             __/ |
                                            |___/
`

	fmt.Print(art)
	table := tablewriter.NewWriter(os.Stdout)
	header := []string{"Gas Parameter", "Value"}
	table.SetHeader(header)
	table.SetRowLine(true)

	table.Append([]string{"GasLimit", genesis.Config.FeeConfig.GasLimit.String()})
	table.Append([]string{"MinBaseFee", genesis.Config.FeeConfig.MinBaseFee.String()})
	table.Append([]string{"TargetGas", genesis.Config.FeeConfig.TargetGas.String()})
	table.Append([]string{"BaseFeeChangeDenominator", genesis.Config.FeeConfig.BaseFeeChangeDenominator.String()})
	table.Append([]string{"MinBlockGasCost", genesis.Config.FeeConfig.MinBlockGasCost.String()})
	table.Append([]string{"MaxBlockGasCost", genesis.Config.FeeConfig.MaxBlockGasCost.String()})
	table.Append([]string{"TargetBlockRate", strconv.FormatUint(genesis.Config.FeeConfig.TargetBlockRate, 10)})
	table.Append([]string{"BlockGasCostStep", genesis.Config.FeeConfig.BlockGasCostStep.String()})

	table.Render()
}

func printAirdropTable(genesis core.Genesis) {
	const art = `
          _         _
    /\   (_)       | |
   /  \   _ _ __ __| |_ __ ___  _ __
  / /\ \ | | '__/ _` + `  | '__/ _ \| '_ \
 / ____ \| | | | (_| | | | (_) | |_) |
/_/    \_\_|_|  \__,_|_|  \___/| .__/
                               | |
                               |_|
`
	fmt.Print(art)
	if len(genesis.Alloc) > 0 {
		table := tablewriter.NewWriter(os.Stdout)
		header := []string{"Address", "Airdrop Amount (10^18)", "Airdrop Amount (wei)"}
		table.SetHeader(header)
		table.SetRowLine(true)

		for address := range genesis.Alloc {
			amount := genesis.Alloc[address].Balance
			formattedAmount := new(big.Int).Div(amount, big.NewInt(params.Ether))
			table.Append([]string{address.Hex(), formattedAmount.String(), amount.String()})
		}

		table.Render()
	} else {
		fmt.Printf("No airdrops allocated")
	}
}

func printPrecompileTable(genesis core.Genesis) {
	const art = `

  _____                                    _ _
 |  __ \                                  (_) |
 | |__) | __ ___  ___ ___  _ __ ___  _ __  _| | ___  ___
 |  ___/ '__/ _ \/ __/ _ \| '_ ` + `  _ \| '_ \| | |/ _ \/ __|
 | |   | | |  __/ (_| (_) | | | | | | |_) | | |  __/\__ \
 |_|   |_|  \___|\___\___/|_| |_| |_| .__/|_|_|\___||___/
                                    | |
                                    |_|

`
	fmt.Print(art)

	table := tablewriter.NewWriter(os.Stdout)
	header := []string{"Precompile", "Admin"}
	table.SetHeader(header)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	table.SetRowLine(true)

	precompileSet := false

	// Native Minting
	// timestamp := genesis.Config.ContractNativeMinterConfig.BlockTimestamp
	for _, address := range genesis.Config.ContractNativeMinterConfig.AllowListAdmins {
		table.Append([]string{"Native Minter", address.Hex()})
		precompileSet = true
	}

	// Contract allow list
	for _, address := range genesis.Config.ContractDeployerAllowListConfig.AllowListAdmins {
		table.Append([]string{"Contract Allow list", address.Hex()})
		precompileSet = true
	}

	if precompileSet {
		table.Render()
	} else {
		ux.PrintToUser("No precompiles set", log)
	}
}

func describeSubnetEvmGenesis(subnetName string, sc models.Sidecar) error {
	// Load genesis
	genesis, err := loadEvmGenesis(subnetName)
	if err != nil {
		return err
	}

	// Write gas table
	printGasTable(genesis)
	// fmt.Printf("\n\n")
	printAirdropTable(genesis)
	printPrecompileTable(genesis)
	return nil
}

func readGenesis(cmd *cobra.Command, args []string) error {
	subnetName := args[0]
	if printGenesisOnly {
		if err := printGenesis(subnetName); err != nil {
			return err
		}
	} else {
		// read in sidecar
		sc, err := loadSidecar(subnetName)
		if err != nil {
			return err
		}

		switch sc.Vm {
		case models.SubnetEvm:
			err = describeSubnetEvmGenesis(subnetName, sc)
		default:
			log.Warn("Unknown genesis format for", sc.Vm)
			ux.PrintToUser("Printing genesis", log)
			err = printGenesis(subnetName)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
