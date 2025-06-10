// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package primarycmd

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/sdk/evm"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const art = `
   _____       _____ _           _         _____
  / ____|     / ____| |         (_)       |  __ \
 | |   ______| |    | |__   __ _ _ _ __   | |__) |_ _ _ __ __ _ _ __ ___  ___ 
 | |  |______| |    | '_ \ / _  | | '_ \  |  ___/ _  | '__/ _  | '_   _ \/ __|
 | |____     | |____| | | | (_| | | | | | | |  | (_| | | | (_| | | | | | \__ \
  \_____|     \_____|_| |_|\__,_|_|_| |_| |_|   \__,_|_|  \__,_|_| |_| |_|___/
`

// avalanche primary describe
func newDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Print details of the primary network configuration",
		Long:  `The subnet describe command prints details of the primary network configuration to the console.`,
		RunE:  describe,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, networkoptions.LocalClusterSupportedNetworkOptions)
	return cmd
}

func describe(_ *cobra.Command, _ []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		false,
		false,
		networkoptions.LocalClusterSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	var (
		icmMessengerAddress string
		icmRegistryAddress  string
	)
	blockchainID, err := utils.GetChainID(network.Endpoint, "C")
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			networkUpMsg := ""
			if network.Kind != models.Fuji && network.Kind != models.Mainnet {
				networkUpMsg = fmt.Sprintf(" Is the %s up?", network.Name())
			}
			ux.Logger.RedXToUser("Could not connect to Primary Network at %s.%s", network.Endpoint, networkUpMsg)
			return nil
		}
		return err
	}
	if network.Kind == models.Local {
		if b, extraLocalNetworkData, err := localnet.GetExtraLocalNetworkData(app, ""); err != nil {
			return err
		} else if b {
			icmMessengerAddress = extraLocalNetworkData.CChainTeleporterMessengerAddress
			icmRegistryAddress = extraLocalNetworkData.CChainTeleporterRegistryAddress
		}
	} else if network.ClusterName != "" {
		if clusterConfig, err := app.GetClusterConfig(network.ClusterName); err != nil {
			return err
		} else {
			icmMessengerAddress = clusterConfig.ExtraNetworkData.CChainTeleporterMessengerAddress
			icmRegistryAddress = clusterConfig.ExtraNetworkData.CChainTeleporterRegistryAddress
		}
	}
	fmt.Print(logging.LightBlue.Wrap(art))
	blockchainIDHexEncoding := "0x" + hex.EncodeToString(blockchainID[:])
	rpcURL := network.CChainEndpoint()
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return err
	}
	evmChainID, err := client.GetChainID()
	if err != nil {
		return err
	}
	k, err := key.LoadEwoq(network.ID)
	if err != nil {
		return err
	}
	address := k.C()
	privKey := k.PrivKeyHex()
	balance, err := client.GetAddressBalance(address)
	if err != nil {
		return err
	}
	balance = balance.Div(balance, big.NewInt(int64(units.Avax)))
	balanceStr := fmt.Sprintf("%.9f", float64(balance.Uint64())/float64(units.Avax))
	var tableBuf bytes.Buffer
	table := tablewriter.NewWriter(&tableBuf)
	header := []string{"Parameter", "Value"}
	table.SetHeader(header)
	table.SetRowLine(true)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	table.Append([]string{"RPC URL", rpcURL})
	codespaceURL, err := utils.GetCodespaceURL(rpcURL)
	if err != nil {
		return err
	}
	if codespaceURL != "" {
		table.Append([]string{"Codespace RPC URL", codespaceURL})
	}
	table.Append([]string{"EVM Chain ID", fmt.Sprint(evmChainID)})
	table.Append([]string{"TOKEN SYMBOL", "AVAX"})
	table.Append([]string{"Address", address})
	table.Append([]string{"Balance", balanceStr})
	table.Append([]string{"Private Key", privKey})
	table.Append([]string{"BlockchainID (CB58)", blockchainID.String()})
	table.Append([]string{"BlockchainID (HEX)", blockchainIDHexEncoding})
	if icmMessengerAddress != "" {
		table.Append([]string{"ICM Messenger Address", icmMessengerAddress})
	}
	if icmRegistryAddress != "" {
		table.Append([]string{"ICM Registry Address", icmRegistryAddress})
	}
	table.Render()
	ux.Logger.Print(tableBuf.String())
	return nil
}
