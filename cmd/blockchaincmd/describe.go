// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	icmgenesis "github.com/ava-labs/avalanche-cli/pkg/teleporter/genesis"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	anr_utils "github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/precompile/contracts/deployerallowlist"
	"github.com/ava-labs/subnet-evm/precompile/contracts/feemanager"
	"github.com/ava-labs/subnet-evm/precompile/contracts/nativeminter"
	"github.com/ava-labs/subnet-evm/precompile/contracts/rewardmanager"
	"github.com/ava-labs/subnet-evm/precompile/contracts/txallowlist"
	"github.com/ava-labs/subnet-evm/precompile/contracts/warp"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var printGenesisOnly bool

// avalanche blockchain describe
func newDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe [blockchainName]",
		Short: "Print a summary of the blockchain’s configuration",
		Long: `The blockchain describe command prints the details of a Blockchain configuration to the console.
By default, the command prints a summary of the configuration. By providing the --genesis
flag, the command instead prints out the raw genesis file.`,
		RunE: describe,
		Args: cobrautils.ExactArgs(1),
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

func printGenesis(blockchainName string) error {
	genesisFile := app.GetGenesisPath(blockchainName)
	gen, err := os.ReadFile(genesisFile)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(string(gen))
	return nil
}

func PrintSubnetInfo(blockchainName string, onlyLocalnetInfo bool) error {
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}

	genesisBytes, err := app.LoadRawGenesis(sc.Subnet)
	if err != nil {
		return err
	}

	// VM/Deploys
	t := table.NewWriter()
	t.Style().Title.Align = text.AlignCenter
	t.Style().Title.Format = text.FormatUpper
	t.Style().Options.SeparateRows = true
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
	})
	rowConfig := table.RowConfig{AutoMerge: true, AutoMergeAlign: text.AlignLeft}
	t.SetTitle(sc.Name)
	t.AppendRow(table.Row{"Name", sc.Name, sc.Name}, rowConfig)
	vmIDstr := sc.ImportedVMID
	if vmIDstr == "" {
		vmID, err := anr_utils.VMID(sc.Name)
		if err == nil {
			vmIDstr = vmID.String()
		} else {
			vmIDstr = constants.NotAvailableLabel
		}
	}
	t.AppendRow(table.Row{"VM ID", vmIDstr, vmIDstr}, rowConfig)
	t.AppendRow(table.Row{"VM Version", sc.VMVersion, sc.VMVersion}, rowConfig)

	locallyDeployed, err := localnet.Deployed(sc.Name)
	if err != nil {
		return err
	}

	localChainID := ""
	for net, data := range sc.Networks {
		network, err := networkoptions.GetNetworkFromSidecarNetworkName(app, net)
		if err != nil {
			return err
		}
		if network.Kind == models.Local && !locallyDeployed {
			continue
		}
		if network.Kind != models.Local && onlyLocalnetInfo {
			continue
		}
		genesisBytes, err := contract.GetBlockchainGenesis(
			app,
			network,
			contract.ChainSpec{
				BlockchainName: sc.Name,
			},
		)
		if err != nil {
			return err
		}
		if utils.ByteSliceIsSubnetEvmGenesis(genesisBytes) {
			genesis, err := utils.ByteSliceToSubnetEvmGenesis(genesisBytes)
			if err != nil {
				return err
			}
			t.AppendRow(table.Row{net, "ChainID", genesis.Config.ChainID.String()})
			if network.Kind == models.Local {
				localChainID = genesis.Config.ChainID.String()
			}
		}
		if data.SubnetID != ids.Empty {
			t.AppendRow(table.Row{net, "SubnetID", data.SubnetID.String()})
			isPermissioned, owners, threshold, err := txutils.GetOwners(network, data.SubnetID)
			if err != nil {
				return err
			}
			if isPermissioned {
				t.AppendRow(table.Row{net, fmt.Sprintf("Owners (Threhold=%d)", threshold), strings.Join(owners, "\n")})
			}
		}
		if data.BlockchainID != ids.Empty {
			hexEncoding := "0x" + hex.EncodeToString(data.BlockchainID[:])
			t.AppendRow(table.Row{net, "BlockchainID (CB58)", data.BlockchainID.String()})
			t.AppendRow(table.Row{net, "BlockchainID (HEX)", hexEncoding})
		}
	}
	ux.Logger.PrintToUser(t.Render())

	// Teleporter
	t = table.NewWriter()
	t.Style().Title.Align = text.AlignCenter
	t.Style().Title.Format = text.FormatUpper
	t.Style().Options.SeparateRows = true
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
	})
	t.SetTitle("Teleporter")
	hasTeleporterInfo := false
	for net, data := range sc.Networks {
		network, err := networkoptions.GetNetworkFromSidecarNetworkName(app, net)
		if err != nil {
			return err
		}
		if network.Kind == models.Local && !locallyDeployed {
			continue
		}
		if network.Kind != models.Local && onlyLocalnetInfo {
			continue
		}
		if data.TeleporterMessengerAddress != "" {
			t.AppendRow(table.Row{net, "Teleporter Messenger Address", data.TeleporterMessengerAddress})
			hasTeleporterInfo = true
		}
		if data.TeleporterRegistryAddress != "" {
			t.AppendRow(table.Row{net, "Teleporter Registry Address", data.TeleporterRegistryAddress})
			hasTeleporterInfo = true
		}
	}
	if hasTeleporterInfo {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser(t.Render())
	}

	// Token
	ux.Logger.PrintToUser("")
	t = table.NewWriter()
	t.Style().Title.Align = text.AlignCenter
	t.Style().Title.Format = text.FormatUpper
	t.Style().Options.SeparateRows = true
	t.SetTitle("Token")
	t.AppendRow(table.Row{"Token Name", sc.TokenName})
	t.AppendRow(table.Row{"Token Symbol", sc.TokenSymbol})
	ux.Logger.PrintToUser(t.Render())

	if utils.ByteSliceIsSubnetEvmGenesis(genesisBytes) {
		genesis, err := utils.ByteSliceToSubnetEvmGenesis(genesisBytes)
		if err != nil {
			return err
		}
		if err := printAllocations(sc, genesis); err != nil {
			return err
		}
		printSmartContracts(genesis)
		printPrecompiles(genesis)
	}

	if locallyDeployed {
		ux.Logger.PrintToUser("")
		if err := localnet.PrintEndpoints(ux.Logger.PrintToUser, sc.Name); err != nil {
			return err
		}

		localEndpoint := models.NewLocalNetwork().BlockchainEndpoint(sc.Name)
		codespaceEndpoint, err := utils.GetCodespaceURL(localEndpoint)
		if err != nil {
			return err
		}
		if codespaceEndpoint != "" {
			localEndpoint = codespaceEndpoint + "\n" + logging.Orange.Wrap("Please make sure to set visibility of port 9650 to public")
		}

		// Wallet
		t = table.NewWriter()
		t.Style().Title.Align = text.AlignCenter
		t.Style().Title.Format = text.FormatUpper
		t.Style().Options.SeparateRows = true
		t.SetTitle("Wallet Connection")
		t.AppendRow(table.Row{"Network RPC URL", localEndpoint})
		t.AppendRow(table.Row{"Network Name", sc.Name})
		t.AppendRow(table.Row{"Chain ID", localChainID})
		t.AppendRow(table.Row{"Token Symbol", sc.TokenSymbol})
		t.AppendRow(table.Row{"Token Name", sc.TokenName})
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser(t.Render())
	}

	return nil
}

func printAllocations(sc models.Sidecar, genesis core.Genesis) error {
	teleporterKeyAddress := ""
	teleporterPrivKey := ""
	if sc.TeleporterReady {
		k, err := key.LoadSoft(models.NewLocalNetwork().ID, app.GetKeyPath(sc.TeleporterKey))
		if err != nil {
			return err
		}
		teleporterKeyAddress = k.C()
		teleporterPrivKey = k.PrivKeyHex()
	}
	subnetAirdropKeyName, subnetAirdropAddress, subnetAirdropPrivKey, err := subnet.GetDefaultSubnetAirdropKeyInfo(app, sc.Name)
	if err != nil {
		return err
	}
	if len(genesis.Alloc) > 0 {
		ux.Logger.PrintToUser("")
		t := table.NewWriter()
		t.Style().Title.Align = text.AlignCenter
		t.Style().Title.Format = text.FormatUpper
		t.Style().Options.SeparateRows = true
		t.SetTitle("Initial Token Allocation")
		t.AppendHeader(table.Row{"Description", "Address and Private Key", "Amount (10^18)", "Amount (wei)"})
		for address, allocation := range genesis.Alloc {
			amount := allocation.Balance
			// we are only interested in supply distribution here
			if amount == nil || big.NewInt(0).Cmp(amount) == 0 {
				continue
			}
			formattedAmount := new(big.Int).Div(amount, big.NewInt(params.Ether))
			description := ""
			privKey := ""
			switch address.Hex() {
			case teleporterKeyAddress:
				description = fmt.Sprintf("%s\n%s", sc.TeleporterKey, logging.Orange.Wrap("Teleporter Deploys"))
				privKey = teleporterPrivKey
			case subnetAirdropAddress:
				description = fmt.Sprintf("%s\n%s", subnetAirdropKeyName, logging.Orange.Wrap("Main funded account"))
				privKey = subnetAirdropPrivKey
			case vm.PrefundedEwoqAddress.Hex():
				description = "Main funded account EWOQ"
				privKey = vm.PrefundedEwoqPrivate
			}
			t.AppendRow(table.Row{description, address.Hex() + "\n" + privKey, formattedAmount.String(), amount.String()})
		}
		ux.Logger.PrintToUser(t.Render())
	}
	return nil
}

func printSmartContracts(genesis core.Genesis) {
	if len(genesis.Alloc) == 0 {
		return
	}
	ux.Logger.PrintToUser("")
	t := table.NewWriter()
	t.Style().Title.Align = text.AlignCenter
	t.Style().Title.Format = text.FormatUpper
	t.Style().Options.SeparateRows = true
	t.SetTitle("Smart Contracts")
	t.AppendHeader(table.Row{"Description", "Address", "Deployer"})
	for address, allocation := range genesis.Alloc {
		if len(allocation.Code) == 0 {
			continue
		}
		var description, deployer string
		if address == common.HexToAddress(icmgenesis.MessengerContractAddress) {
			description = "ICM Messenger"
			deployer = icmgenesis.MessengerDeployerAddress
		}
		if address == common.HexToAddress(validatormanager.PoAValidarorMessengerContractAddress) {
			description = "PoA Validator Manager"
		}
		if address == common.HexToAddress(validatormanager.PoSValidarorMessengerContractAddress) {
			description = "PoS Validator Manager"
		}
		t.AppendRow(table.Row{description, address.Hex(), deployer})
	}
	ux.Logger.PrintToUser(t.Render())
}

func printPrecompiles(genesis core.Genesis) {
	ux.Logger.PrintToUser("")
	t := table.NewWriter()
	t.Style().Title.Align = text.AlignCenter
	t.Style().Title.Format = text.FormatUpper
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
	})
	t.SetTitle("Initial Precompile Configs")
	t.AppendHeader(table.Row{"Precompile", "Admin Addresses", "Manager Addresses", "Enabled Addresses"})

	warpSet := false
	allowListSet := false
	// Warp
	if genesis.Config.GenesisPrecompiles[warp.ConfigKey] != nil {
		t.AppendRow(table.Row{"Warp", "n/a", "n/a", "n/a"})
		warpSet = true
	}
	// Native Minting
	if genesis.Config.GenesisPrecompiles[nativeminter.ConfigKey] != nil {
		cfg := genesis.Config.GenesisPrecompiles[nativeminter.ConfigKey].(*nativeminter.Config)
		addPrecompileAllowListToTable(t, "Native Minter", cfg.AdminAddresses, cfg.ManagerAddresses, cfg.EnabledAddresses)
		allowListSet = true
	}
	// Contract allow list
	if genesis.Config.GenesisPrecompiles[deployerallowlist.ConfigKey] != nil {
		cfg := genesis.Config.GenesisPrecompiles[deployerallowlist.ConfigKey].(*deployerallowlist.Config)
		addPrecompileAllowListToTable(t, "Contract Allow List", cfg.AdminAddresses, cfg.ManagerAddresses, cfg.EnabledAddresses)
		allowListSet = true
	}
	// TX allow list
	if genesis.Config.GenesisPrecompiles[txallowlist.ConfigKey] != nil {
		cfg := genesis.Config.GenesisPrecompiles[txallowlist.Module.ConfigKey].(*txallowlist.Config)
		addPrecompileAllowListToTable(t, "Tx Allow List", cfg.AdminAddresses, cfg.ManagerAddresses, cfg.EnabledAddresses)
		allowListSet = true
	}
	// Fee config allow list
	if genesis.Config.GenesisPrecompiles[feemanager.ConfigKey] != nil {
		cfg := genesis.Config.GenesisPrecompiles[feemanager.ConfigKey].(*feemanager.Config)
		addPrecompileAllowListToTable(t, "Fee Config Allow List", cfg.AdminAddresses, cfg.ManagerAddresses, cfg.EnabledAddresses)
		allowListSet = true
	}
	// Reward config allow list
	if genesis.Config.GenesisPrecompiles[rewardmanager.ConfigKey] != nil {
		cfg := genesis.Config.GenesisPrecompiles[rewardmanager.ConfigKey].(*rewardmanager.Config)
		addPrecompileAllowListToTable(t, "Reward Manager Allow List", cfg.AdminAddresses, cfg.ManagerAddresses, cfg.EnabledAddresses)
		allowListSet = true
	}
	if warpSet || allowListSet {
		ux.Logger.PrintToUser(t.Render())
		if allowListSet {
			note := logging.Orange.Wrap("The allowlist is taken from the genesis and is not being updated if you make adjustments\nvia the precompile. Use readAllowList(address) instead.")
			ux.Logger.PrintToUser(note)
		}
	}
}

func addPrecompileAllowListToTable(
	t table.Writer,
	label string,
	adminAddresses []common.Address,
	managerAddresses []common.Address,
	enabledAddresses []common.Address,
) {
	t.AppendSeparator()
	admins := len(adminAddresses)
	managers := len(managerAddresses)
	enabled := len(enabledAddresses)
	max := max(admins, managers, enabled)
	for i := 0; i < max; i++ {
		var admin, manager, enable string
		if i < len(adminAddresses) && adminAddresses[i] != (common.Address{}) {
			admin = adminAddresses[i].Hex()
		}
		if i < len(managerAddresses) && managerAddresses[i] != (common.Address{}) {
			manager = managerAddresses[i].Hex()
		}
		if i < len(enabledAddresses) && enabledAddresses[i] != (common.Address{}) {
			enable = enabledAddresses[i].Hex()
		}
		t.AppendRow(table.Row{label, admin, manager, enable})
	}
}

func describe(_ *cobra.Command, args []string) error {
	blockchainName := args[0]
	if !app.GenesisExists(blockchainName) {
		ux.Logger.PrintToUser("The provided subnet name %q does not exist", blockchainName)
		return nil
	}
	if printGenesisOnly {
		return printGenesis(blockchainName)
	}
	if err := PrintSubnetInfo(blockchainName, false); err != nil {
		return err
	}
	if isEVM, _, err := app.HasSubnetEVMGenesis(blockchainName); err != nil {
		return err
	} else if !isEVM {
		sc, err := app.LoadSidecar(blockchainName)
		if err != nil {
			return err
		}
		app.Log.Warn("Unknown genesis format", zap.Any("vm-type", sc.VM))
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Printing genesis")
		return printGenesis(blockchainName)
	}
	return nil
}
