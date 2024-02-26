// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"fmt"
	"strings"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/spf13/cobra"
)

var (
	useLocal   bool
	useDevnet  bool
	useFuji    bool
	useMainnet bool
	endpoint   string
)

// avalanche teleporter msg
func newMsgCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "msg [subnet1Name] [subnet2Name]",
		Short:        "Sends and wait reception for a teleporter msg between two subnets",
		Long:         `Sends and wait reception for a teleporter msg between two subnets.`,
		SilenceUsage: true,
		RunE:         msg,
		Args:         cobra.ExactArgs(2),
	}
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "use the given endpoint for network operations")
	cmd.Flags().BoolVarP(&useLocal, "local", "l", false, "operate on a local network")
	cmd.Flags().BoolVar(&useDevnet, "devnet", false, "operate on a devnet network")
	cmd.Flags().BoolVarP(&useFuji, "testnet", "t", false, "operate on testnet (alias to `fuji`)")
	cmd.Flags().BoolVarP(&useFuji, "fuji", "f", false, "operate on fuji (alias to `testnet`")
	cmd.Flags().BoolVarP(&useMainnet, "mainnet", "m", false, "operate on mainnet")
	return cmd
}

func msg(cmd *cobra.Command, args []string) error {
	network, err := subnetcmd.GetNetworkFromCmdLineFlags(
		useLocal,
		useDevnet,
		useFuji,
		useMainnet,
		"",
		false,
		[]models.NetworkKind{models.Local},
	)
	if err != nil {
		return err
	}

	subnetName1 := strings.ToLower(args[0])
	subnetName2 := strings.ToLower(args[1])

	chainID1, messengerAddress1, key1, err := getSubnetParams(network, subnetName1)
	if err != nil {
		return err
	}
	chainID2, messengerAddress2, key2, err := getSubnetParams(network, subnetName2)
	if err != nil {
		return err
	}

	fmt.Println(chainID1)
	fmt.Println(chainID2)
    _ = key1
    _ = key2

    if messengerAddress1 != messengerAddress2 {
        fmt.Println("different teleporter messenger addresses among subnets: %s vs %s", messengerAddress1, messengerAddress2)
    }

	return nil
}

func getSubnetParams(network models.Network, subnetName string) (ids.ID, string, *key.SoftKey, error) {
	var (
		chainID                    ids.ID
		err                        error
		teleporterMessengerAddress string
        k *key.SoftKey
	)
	if subnetName == "c-chain" || subnetName == "cchain" {
		chainID, err = getChainID(network.Endpoint, "C")
		if network.Kind == models.Local {
			extraLocalNetworkData, err := subnet.GetExtraLocalNetworkData(app)
			if err != nil {
				return ids.Empty, "", nil, err
			}
			teleporterMessengerAddress = extraLocalNetworkData.CChainTeleporterMessengerAddress
            k, err = key.LoadEwoq(network.ID)
            if err != nil {
                return ids.Empty, "", nil, err
            }
		}
	} else {
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return ids.Empty, "", nil, err
		}
		chainID = sc.Networks[network.Name()].BlockchainID
		teleporterMessengerAddress = sc.Networks[network.Name()].TeleporterMessengerAddress
		keyPath := app.GetKeyPath(sc.TeleporterKey)
		k, err = key.LoadSoft(network.ID, keyPath)
		if err != nil {
			return ids.Empty, "", nil, err
		}
	}
	if chainID == ids.Empty {
		return ids.Empty, "", nil, fmt.Errorf("chainID for subnet %s not found on network %s", subnetName, network.Name())
	}
	if teleporterMessengerAddress == "" {
		return ids.Empty, "", nil, fmt.Errorf("teleporter messenger address for subnet %s not found on network %s", subnetName, network.Name())
	}
	return chainID, teleporterMessengerAddress, k, err
}

func getChainID(endpoint string, chainName string) (ids.ID, error) {
	client := info.NewClient(endpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	return client.GetBlockchainID(ctx, chainName)
}
