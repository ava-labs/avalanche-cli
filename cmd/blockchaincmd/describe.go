// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
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
	icmgenesis "github.com/ava-labs/avalanche-cli/pkg/interchain/genesis"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	validatorManagerSDK "github.com/ava-labs/avalanche-tooling-sdk-go/validatormanager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/libevm/common"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/precompile/contracts/deployerallowlist"
	"github.com/ava-labs/subnet-evm/precompile/contracts/feemanager"
	"github.com/ava-labs/subnet-evm/precompile/contracts/nativeminter"
	"github.com/ava-labs/subnet-evm/precompile/contracts/rewardmanager"
	"github.com/ava-labs/subnet-evm/precompile/contracts/txallowlist"
	"github.com/ava-labs/subnet-evm/precompile/contracts/warp"

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
	ux.Logger.PrintToUser("%s", string(gen))
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
	t := ux.DefaultTable(sc.Name, nil)
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
	})
	rowConfig := table.RowConfig{AutoMerge: true, AutoMergeAlign: text.AlignLeft}
	t.AppendRow(table.Row{"Name", sc.Name, sc.Name}, rowConfig)
	vmIDstr := sc.ImportedVMID
	if vmIDstr == "" {
		vmID, err := utils.VMID(sc.Name)
		if err == nil {
			vmIDstr = vmID.String()
		} else {
			vmIDstr = constants.NotAvailableLabel
		}
	}
	t.AppendRow(table.Row{"VM ID", vmIDstr, vmIDstr}, rowConfig)
	t.AppendRow(table.Row{"VM Version", sc.VMVersion, sc.VMVersion}, rowConfig)
	t.AppendRow(table.Row{"Validation", sc.ValidatorManagement, sc.ValidatorManagement}, rowConfig)

	locallyDeployed := false
	localEndpoint := ""
	localChainID := ""
	for net, data := range sc.Networks {
		network, err := app.GetNetworkFromSidecarNetworkName(net)
		if err != nil {
			ux.Logger.RedXToUser("%s is supposed to be deployed to network %s: %s ", blockchainName, network.Name(), err)
			ux.Logger.PrintToUser("")
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
			if network.Kind != models.Local {
				return err
			}
			// ignore local network errors for cases
			// where local network is down but sidecar contains
			// local network metadata
			// (eg host restarts)
			continue
		} else if network.Kind == models.Local {
			locallyDeployed = true
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
			_, owners, threshold, err := txutils.GetOwners(network, data.SubnetID)
			if err != nil {
				return err
			}
			t.AppendRow(table.Row{net, fmt.Sprintf("Owners (Threhold=%d)", threshold), strings.Join(owners, "\n")})
		}
		if data.BlockchainID != ids.Empty {
			hexEncoding := "0x" + hex.EncodeToString(data.BlockchainID[:])
			t.AppendRow(table.Row{net, "BlockchainID (CB58)", data.BlockchainID.String()})
			t.AppendRow(table.Row{net, "BlockchainID (HEX)", hexEncoding})
		}
		endpoint, _, err := contract.GetBlockchainEndpoints(
			app,
			network,
			contract.ChainSpec{
				BlockchainName: sc.Name,
			},
			false,
			false,
		)
		if err != nil {
			return err
		}
		if network.Kind == models.Local {
			localEndpoint = endpoint
		}
		t.AppendRow(table.Row{net, "RPC Endpoint", endpoint})
		if data.ValidatorManagerBlockchainID != ids.Empty {
			t.AppendRow(table.Row{net, "Manager Blockchain ID", data.ValidatorManagerBlockchainID.String()})
		}
		if data.ValidatorManagerRPCEndpoint != "" {
			t.AppendRow(table.Row{net, "Manager RPC", data.ValidatorManagerRPCEndpoint})
		}
		if data.ValidatorManagerAddress != "" {
			t.AppendRow(table.Row{net, "Manager Address", data.ValidatorManagerAddress})
		}
		if data.SpecializedValidatorManagerAddress != "" {
			t.AppendRow(table.Row{net, "Specialized Manager Address", data.SpecializedValidatorManagerAddress})
		}
	}
	ux.Logger.PrintToUser("%s", t.Render())

	// ICM
	t = ux.DefaultTable("ICM", nil)
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
	})
	hasICMInfo := false
	for net, data := range sc.Networks {
		network, err := app.GetNetworkFromSidecarNetworkName(net)
		if err != nil {
			continue
		}
		if network.Kind == models.Local && !locallyDeployed {
			continue
		}
		if network.Kind != models.Local && onlyLocalnetInfo {
			continue
		}
		if data.TeleporterMessengerAddress != "" {
			t.AppendRow(table.Row{net, "ICM Messenger Address", data.TeleporterMessengerAddress})
			hasICMInfo = true
		}
		if data.TeleporterRegistryAddress != "" {
			t.AppendRow(table.Row{net, "ICM Registry Address", data.TeleporterRegistryAddress})
			hasICMInfo = true
		}
	}
	if hasICMInfo {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("%s", t.Render())
	}

	// Token
	ux.Logger.PrintToUser("")
	t = ux.DefaultTable("Token", nil)
	t.AppendRow(table.Row{"Token Name", sc.TokenName})
	t.AppendRow(table.Row{"Token Symbol", sc.TokenSymbol})
	ux.Logger.PrintToUser("%s", t.Render())

	if utils.ByteSliceIsSubnetEvmGenesis(genesisBytes) {
		genesis, err := utils.ByteSliceToSubnetEvmGenesis(genesisBytes)
		if err != nil {
			return err
		}
		if err := printAllocations(sc, genesis); err != nil {
			return err
		}
		printSmartContracts(sc, genesis)
		printPrecompiles(genesis)
	}

	if locallyDeployed {
		ux.Logger.PrintToUser("")
		if err := localnet.PrintEndpoints(app, ux.Logger.PrintToUser, sc.Name); err != nil {
			return err
		}

		codespaceEndpoint, err := utils.GetCodespaceURL(localEndpoint)
		if err != nil {
			return err
		}
		if codespaceEndpoint != "" {
			_, port, _, err := utils.GetURIHostPortAndPath(localEndpoint)
			if err != nil {
				return err
			}
			localEndpoint = codespaceEndpoint + "\n" + logging.Orange.Wrap(
				fmt.Sprintf("Please make sure to set visibility of port %d to public", port),
			)
		}

		// Wallet
		t = ux.DefaultTable("Wallet Connection", nil)
		t.AppendRow(table.Row{"Network RPC URL", localEndpoint})
		t.AppendRow(table.Row{"Network Name", sc.Name})
		t.AppendRow(table.Row{"Chain ID", localChainID})
		t.AppendRow(table.Row{"Token Symbol", sc.TokenSymbol})
		t.AppendRow(table.Row{"Token Name", sc.TokenName})
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("%s", t.Render())
	}

	return nil
}

func printAllocations(sc models.Sidecar, genesis core.Genesis) error {
	icmKeyAddress := ""
	if sc.TeleporterReady {
		k, err := key.LoadSoft(models.NewLocalNetwork().ID, app.GetKeyPath(sc.TeleporterKey))
		if err != nil {
			return err
		}
		icmKeyAddress = k.C()
	}
	_, subnetAirdropAddress, _, err := subnet.GetDefaultSubnetAirdropKeyInfo(app, sc.Name)
	if err != nil {
		return err
	}
	if len(genesis.Alloc) > 0 {
		ux.Logger.PrintToUser("")
		t := ux.DefaultTable(
			"Initial Token Allocation",
			table.Row{
				"Description",
				"Address and Private Key",
				fmt.Sprintf("Amount (%s)", sc.TokenSymbol),
				"Amount (wei)",
			},
		)
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
			case icmKeyAddress:
				description = logging.Orange.Wrap("Used by ICM")
			case subnetAirdropAddress:
				description = logging.Orange.Wrap("Main funded account")
			case vm.PrefundedEwoqAddress.Hex():
				description = logging.Orange.Wrap("Main funded account")
			case sc.ValidatorManagerOwner:
				description = logging.Orange.Wrap("Validator Manager Owner")
			case sc.ProxyContractOwner:
				description = logging.Orange.Wrap("Proxy Admin Owner")
			}
			var (
				found bool
				name  string
			)
			found, name, _, privKey, err = contract.SearchForManagedKey(app, models.NewLocalNetwork(), address, true)
			if err != nil {
				return err
			}
			if found {
				description = fmt.Sprintf("%s\n%s", description, name)
			}
			t.AppendRow(table.Row{description, address.Hex() + "\n" + privKey, formattedAmount.String(), amount.String()})
		}
		ux.Logger.PrintToUser("%s", t.Render())
	}
	return nil
}

func printSmartContracts(sc models.Sidecar, genesis core.Genesis) {
	if len(genesis.Alloc) == 0 {
		return
	}
	ux.Logger.PrintToUser("")
	t := ux.DefaultTable(
		"Smart Contracts",
		table.Row{"Description", "Address", "Deployer"},
	)
	for address, allocation := range genesis.Alloc {
		if len(allocation.Code) == 0 {
			continue
		}
		var description, deployer string
		switch {
		case address == common.HexToAddress(icmgenesis.MessengerContractAddress):
			description = "ICM Messenger"
			deployer = icmgenesis.MessengerDeployerAddress
		case address == common.HexToAddress(validatorManagerSDK.ValidatorMessagesContractAddress):
			description = "Validator Messages Lib"
		case address == common.HexToAddress(validatorManagerSDK.ValidatorContractAddress):
			if sc.PoA() {
				description = "PoA Validator Manager"
			} else {
				description = "Native Token Staking Manager"
			}
			if sc.UseACP99 {
				description = "ACP99 Compatible " + description
			} else {
				description = "v1.0.0 Compatible " + description
			}
		case address == common.HexToAddress(validatorManagerSDK.ValidatorProxyContractAddress):
			description = "Validator Transparent Proxy"
		case address == common.HexToAddress(validatorManagerSDK.ValidatorProxyAdminContractAddress):
			description = "Validator Proxy Admin"
			deployer = sc.ProxyContractOwner
		case address == common.HexToAddress(validatorManagerSDK.SpecializationProxyContractAddress):
			description = "Validator Specialization Transparent Proxy"
		case address == common.HexToAddress(validatorManagerSDK.SpecializationProxyAdminContractAddress):
			description = "Validator Specialization Proxy Admin"
		case address == common.HexToAddress(validatorManagerSDK.RewardCalculatorAddress):
			description = "Reward Calculator"
		}
		t.AppendRow(table.Row{description, address.Hex(), deployer})
	}
	ux.Logger.PrintToUser("%s", t.Render())
}

func printPrecompiles(genesis core.Genesis) {
	ux.Logger.PrintToUser("")
	t := ux.DefaultTable(
		"Initial Precompile Configs",
		table.Row{"Precompile", "Admin Addresses", "Manager Addresses", "Enabled Addresses"},
	)
	t.Style().Options.SeparateRows = false
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
	})

	warpSet := false
	allowListSet := false
	// Warp
	extra := params.GetExtra(genesis.Config)
	if extra.GenesisPrecompiles[warp.ConfigKey] != nil {
		t.AppendRow(table.Row{"Warp", "n/a", "n/a", "n/a"})
		warpSet = true
	}
	// Native Minting
	if extra.GenesisPrecompiles[nativeminter.ConfigKey] != nil {
		cfg := extra.GenesisPrecompiles[nativeminter.ConfigKey].(*nativeminter.Config)
		addPrecompileAllowListToTable(t, "Native Minter", cfg.AdminAddresses, cfg.ManagerAddresses, cfg.EnabledAddresses)
		allowListSet = true
	}
	// Contract allow list
	if extra.GenesisPrecompiles[deployerallowlist.ConfigKey] != nil {
		cfg := extra.GenesisPrecompiles[deployerallowlist.ConfigKey].(*deployerallowlist.Config)
		addPrecompileAllowListToTable(t, "Contract Allow List", cfg.AdminAddresses, cfg.ManagerAddresses, cfg.EnabledAddresses)
		allowListSet = true
	}
	// TX allow list
	if extra.GenesisPrecompiles[txallowlist.ConfigKey] != nil {
		cfg := extra.GenesisPrecompiles[txallowlist.Module.ConfigKey].(*txallowlist.Config)
		addPrecompileAllowListToTable(t, "Tx Allow List", cfg.AdminAddresses, cfg.ManagerAddresses, cfg.EnabledAddresses)
		allowListSet = true
	}
	// Fee config allow list
	if extra.GenesisPrecompiles[feemanager.ConfigKey] != nil {
		cfg := extra.GenesisPrecompiles[feemanager.ConfigKey].(*feemanager.Config)
		addPrecompileAllowListToTable(t, "Fee Config Allow List", cfg.AdminAddresses, cfg.ManagerAddresses, cfg.EnabledAddresses)
		allowListSet = true
	}
	// Reward config allow list
	if extra.GenesisPrecompiles[rewardmanager.ConfigKey] != nil {
		cfg := extra.GenesisPrecompiles[rewardmanager.ConfigKey].(*rewardmanager.Config)
		addPrecompileAllowListToTable(t, "Reward Manager Allow List", cfg.AdminAddresses, cfg.ManagerAddresses, cfg.EnabledAddresses)
		allowListSet = true
	}
	if warpSet || allowListSet {
		ux.Logger.PrintToUser("%s", t.Render())
		if allowListSet {
			note := logging.Orange.Wrap("The allowlist is taken from the genesis and is not being updated if you make adjustments\nvia the precompile. Use readAllowList(address) instead.")
			ux.Logger.PrintToUser("%s", note)
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
		ux.Logger.PrintToUser("The provided blockchain name %q does not exist", blockchainName)
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
