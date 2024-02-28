// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"fmt"
	"strings"
	"encoding/hex"
	"math/big"

	teleportermessenger "github.com/ava-labs/teleporter/abi-bindings/go/Teleporter/TeleporterMessenger"
	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ethereum/go-ethereum/common"

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

	sourceSubnetName := strings.ToLower(args[0])
	destSubnetName := strings.ToLower(args[1])

	sourceChainID, sourceMessengerAddressStr, sourceKey, err := getSubnetParams(network, sourceSubnetName)
	if err != nil {
		return err
	}
	destChainID, destMessengerAddressStr, _, err := getSubnetParams(network, destSubnetName)
	if err != nil {
		return err
	}

	if sourceMessengerAddressStr != destMessengerAddressStr {
		fmt.Println("different teleporter messenger addresses among subnets: %s vs %s", sourceMessengerAddressStr, destMessengerAddressStr)
	}

	sourceClient, err := evm.GetClient(network.BlockchainEndpoint(sourceChainID.String()))
	if err != nil {
		return err
	}
	sourceSigner, err := evm.GetSigner(sourceClient, hex.EncodeToString(sourceKey.Raw()))
	if err != nil {
		return err
	}
	sourceMessengerAddress := common.HexToAddress(sourceMessengerAddressStr)
	sourceMessenger, err := teleportermessenger.NewTeleporterMessenger(sourceMessengerAddress, sourceClient)
	if err != nil {
		return err
	}
	sourceAddress := common.HexToAddress(sourceKey.C())

	// send tx to the source teleporter contract
        msgInput := teleportermessenger.TeleporterMessageInput{
                DestinationBlockchainID: destChainID,
                DestinationAddress:      sourceAddress,
                FeeInfo: teleportermessenger.TeleporterFeeInfo{
                        FeeTokenAddress: sourceAddress,
                        Amount:          big.NewInt(0),                         
                },
                RequiredGasLimit:        big.NewInt(1),
                AllowedRelayerAddresses: []common.Address{},
                Message:                 []byte{11, 1, 2, 3, 4, 11},
        }
	tx, err := sourceMessenger.SendCrossChainMessage(sourceSigner, msgInput)
	if err != nil {
		return err
	}
	receipt, b, err := evm.WaitForTransaction(sourceClient, tx)
	if err != nil {
		return err
	}
	if !b {
		return fmt.Errorf("receipt status is not ReceiptStatusSuccessful")
	}
	_ = receipt

	return nil
}

func getSubnetParams(network models.Network, subnetName string) (ids.ID, string, *key.SoftKey, error) {
	var (
		chainID                    ids.ID
		err                        error
		teleporterMessengerAddress string
		k                          *key.SoftKey
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
