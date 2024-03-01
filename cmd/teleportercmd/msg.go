// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/core/types"
	teleportermessenger "github.com/ava-labs/teleporter/abi-bindings/go/Teleporter/TeleporterMessenger"
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
		Use:          "msg [sourceSubnetName] [destinationSubnetName] [messageContent]",
		Short:        "Verifies exchange of teleporter message between two subnets",
		Long:         `Sends and wait reception for a teleporter msg between two subnets (Currently only for local network).`,
		SilenceUsage: true,
		RunE:         msg,
		Args:         cobra.ExactArgs(3),
	}
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "use the given endpoint for network operations")
	cmd.Flags().BoolVarP(&useLocal, "local", "l", false, "operate on a local network")
	cmd.Flags().BoolVar(&useDevnet, "devnet", false, "operate on a devnet network")
	cmd.Flags().BoolVarP(&useFuji, "testnet", "t", false, "operate on testnet (alias to `fuji`)")
	cmd.Flags().BoolVarP(&useFuji, "fuji", "f", false, "operate on fuji (alias to `testnet`")
	cmd.Flags().BoolVarP(&useMainnet, "mainnet", "m", false, "operate on mainnet")
	return cmd
}

func msg(_ *cobra.Command, args []string) error {
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

	sourceSubnetName := args[0]
	destSubnetName := args[1]
	message := args[2]

	sourceChainID, sourceMessengerAddress, sourceKey, err := getSubnetParams(network, sourceSubnetName)
	if err != nil {
		return err
	}
	destChainID, destMessengerAddress, _, err := getSubnetParams(network, destSubnetName)
	if err != nil {
		return err
	}

	if sourceMessengerAddress != destMessengerAddress {
		return fmt.Errorf("different teleporter messenger addresses among subnets: %s vs %s", sourceMessengerAddress, destMessengerAddress)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	// get clients + messengers
	sourceClient, err := evm.GetClient(network.BlockchainEndpoint(sourceChainID.String()))
	if err != nil {
		return err
	}
	sourceMessenger, err := teleportermessenger.NewTeleporterMessenger(common.HexToAddress(sourceMessengerAddress), sourceClient)
	if err != nil {
		return err
	}
	destWebSocketClient, err := evm.GetClient(network.BlockchainWSEndpoint(destChainID.String()))
	if err != nil {
		return err
	}
	destMessenger, err := teleportermessenger.NewTeleporterMessenger(common.HexToAddress(destMessengerAddress), destWebSocketClient)
	if err != nil {
		return err
	}

	// subscribe to get new heads from destination
	destHeadsCh := make(chan *types.Header, 10)
	destHeadsSubscription, err := destWebSocketClient.SubscribeNewHead(ctx, destHeadsCh)
	if err != nil {
		return err
	}
	defer destHeadsSubscription.Unsubscribe()

	// send tx to the teleporter contract at the source
	sourceSigner, err := evm.GetSigner(sourceClient, hex.EncodeToString(sourceKey.Raw()))
	if err != nil {
		return err
	}
	sourceAddress := common.HexToAddress(sourceKey.C())
	msgInput := teleportermessenger.TeleporterMessageInput{
		DestinationBlockchainID: destChainID,
		DestinationAddress:      sourceAddress,
		FeeInfo: teleportermessenger.TeleporterFeeInfo{
			FeeTokenAddress: sourceAddress,
			Amount:          big.NewInt(0),
		},
		RequiredGasLimit:        big.NewInt(1),
		AllowedRelayerAddresses: []common.Address{},
		Message:                 []byte(message),
	}
	ux.Logger.PrintToUser("Delivering message %q to source subnet %q", message, sourceSubnetName)
	tx, err := sourceMessenger.SendCrossChainMessage(sourceSigner, msgInput)
	if err != nil {
		return err
	}
	sourceReceipt, b, err := evm.WaitForTransaction(sourceClient, tx)
	if err != nil {
		return err
	}
	if !b {
		return fmt.Errorf("source receipt status is not ReceiptStatusSuccessful")
	}
	sourceEvent, err := evm.GetEventFromLogs(sourceReceipt.Logs, sourceMessenger.ParseSendCrossChainMessage)
	if err != nil {
		return err
	}

	if destChainID != ids.ID(sourceEvent.DestinationBlockchainID[:]) {
		return fmt.Errorf("invalid destination blockchain id at source event, expected %s, got %s", destChainID, ids.ID(sourceEvent.DestinationBlockchainID[:]))
	}
	if message != string(sourceEvent.Message.Message) {
		return fmt.Errorf("invalid message content at source event, expected %s, got %s", message, string(sourceEvent.Message.Message))
	}

	// receive and process head from destination
	ux.Logger.PrintToUser("Waiting for message to be received at destination subnet subnet %q", destSubnetName)
	var head *types.Header
	select {
	case head = <-destHeadsCh:
	case <-ctx.Done():
		return ctx.Err()
	}
	blockNumber := head.Number
	block, err := destWebSocketClient.BlockByNumber(ctx, blockNumber)
	if err != nil {
		return err
	}
	if len(block.Transactions()) != 1 {
		return fmt.Errorf("expected to have only one transaction on new block at destination")
	}
	destReceipt, err := destWebSocketClient.TransactionReceipt(ctx, block.Transactions()[0].Hash())
	if err != nil {
		return err
	}
	if destReceipt.Status != types.ReceiptStatusSuccessful {
		return fmt.Errorf("dest receipt status is not ReceiptStatusSuccessful")
	}
	destEvent, err := evm.GetEventFromLogs(destReceipt.Logs, destMessenger.ParseReceiveCrossChainMessage)
	if err != nil {
		return err
	}

	if sourceChainID != ids.ID(destEvent.SourceBlockchainID[:]) {
		return fmt.Errorf("invalid source blockchain id at dest event, expected %s, got %s", sourceChainID, ids.ID(destEvent.SourceBlockchainID[:]))
	}
	if message != string(destEvent.Message.Message) {
		return fmt.Errorf("invalid message content at source event, expected %s, got %s", message, string(destEvent.Message.Message))
	}
	if sourceEvent.MessageID != destEvent.MessageID {
		return fmt.Errorf("unexpected difference between message ID at source and dest events: %s vs %s", hex.EncodeToString(sourceEvent.MessageID[:]), hex.EncodeToString(destEvent.MessageID[:]))
	}

	ux.Logger.PrintToUser("Message successfully Teleported!")

	return nil
}

func getSubnetParams(network models.Network, subnetName string) (ids.ID, string, *key.SoftKey, error) {
	var (
		chainID                    ids.ID
		err                        error
		teleporterMessengerAddress string
		k                          *key.SoftKey
	)
	if strings.ToLower(subnetName) == "c-chain" || strings.ToLower(subnetName) == "cchain" {
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
