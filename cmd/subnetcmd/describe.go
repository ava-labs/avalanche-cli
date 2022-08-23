// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"fmt"
	"math/big"
	"os"
	"strconv"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// avalanche subnet describe
func newDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe [subnetName]",
		Short: "Print a summary of the subnet’s configuration",
		Long: `The subnet describe command prints the details of a subnet configuration
to the console. By default, the command will print a summary of the
configuration. By providing the --genesis flag, the command will instead
print out the raw genesis file.`,
		RunE: readGenesis,
		Args: cobra.ExactArgs(1),
	}
	cmd.Flags().BoolVarP(
		&printGenesisOnly,
		"genesis",
		"g",
		false,
		"Print the genesis to the console directly instead of the summary",
	)
	return cmd
}

var printGenesisOnly bool

func printGenesis(subnetName string) error {
	genesisFile := app.GetGenesisPath(subnetName)
	gen, err := os.ReadFile(genesisFile)
	if err != nil {
		return err
	}
	fmt.Println(string(gen))
	return nil
}

func printDetails(genesis core.Genesis, sc models.Sidecar) {
	const art = `
 _____       _        _ _
|  __ \     | |      (_) |
| |  | | ___| |_ __ _ _| |___
| |  | |/ _ \ __/ _` + `  | | / __|
| |__| |  __/ || (_| | | \__ \
|_____/ \___|\__\__,_|_|_|___/
`
	fmt.Print(art)
	table := tablewriter.NewWriter(os.Stdout)
	header := []string{"Parameter", "Value"}
	table.SetHeader(header)
	table.SetRowLine(true)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	table.Append([]string{"Subnet Name", sc.Subnet})
	table.Append([]string{"ChainID", genesis.Config.ChainID.String()})
	table.Append([]string{"Token Name", app.GetTokenName(sc.Subnet)})
	for net, data := range sc.Networks {
		if data.SubnetID != ids.Empty {
			table.Append([]string{fmt.Sprintf("%s SubnetID", net), data.SubnetID.String()})
		}
		if data.BlockchainID != ids.Empty {
			table.Append([]string{fmt.Sprintf("%s BlockchainID", net), data.BlockchainID.String()})
		}
	}
	table.Render()
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
	table.Append([]string{"TargetGas (per 10s)", genesis.Config.FeeConfig.TargetGas.String()})
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

	// TX allow list
	for _, address := range genesis.Config.TxAllowListConfig.AllowListAdmins {
		table.Append([]string{"Tx Allow list", address.Hex()})
		precompileSet = true
	}

	// Fee config allow list
	for _, address := range genesis.Config.FeeManagerConfig.AllowListAdmins {
		table.Append([]string{"Fee Config Allow list", address.Hex()})
		precompileSet = true
	}

	if precompileSet {
		table.Render()
	} else {
		ux.Logger.PrintToUser("No precompiles set")
	}
}

func describeSubnetEvmGenesis(sc models.Sidecar) error {
	// Load genesis
	genesis, err := app.LoadEvmGenesis(sc.Subnet)
	if err != nil {
		return err
	}

	printDetails(genesis, sc)
	// Write gas table
	printGasTable(genesis)
	// fmt.Printf("\n\n")
	printAirdropTable(genesis)
	printPrecompileTable(genesis)
	return nil
}

func readGenesis(cmd *cobra.Command, args []string) error {
	subnetName := args[0]
	if !app.GenesisExists(subnetName) {
		ux.Logger.PrintToUser("The provided subnet name %q does not exist", subnetName)
		return nil
	}
	if printGenesisOnly {
		if err := printGenesis(subnetName); err != nil {
			return err
		}
	} else {
		// read in sidecar
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return err
		}

		switch sc.VM {
		case models.SubnetEvm:
			err = describeSubnetEvmGenesis(sc)
		default:
			app.Log.Warn("Unknown genesis format", zap.Any("vm-type", sc.VM))
			ux.Logger.PrintToUser("Printing genesis")
			err = printGenesis(subnetName)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
