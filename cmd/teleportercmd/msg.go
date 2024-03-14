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

	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/core/types"
	teleportermessenger "github.com/ava-labs/teleporter/abi-bindings/go/Teleporter/TeleporterMessenger"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"
)

var (
	msgSupportedNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local, networkoptions.Cluster, networkoptions.Fuji, networkoptions.Mainnet, networkoptions.Devnet}
	globalNetworkFlags         networkoptions.NetworkFlags
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
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, msgSupportedNetworkOptions)
	return cmd
}

func msg(_ *cobra.Command, args []string) error {
	sourceSubnetName := args[0]
	destSubnetName := args[1]
	message := args[2]

	subnetNameToGetNetworkFrom := ""
	if !isCChain(sourceSubnetName) {
		subnetNameToGetNetworkFrom = sourceSubnetName
	}
	if !isCChain(destSubnetName) {
		subnetNameToGetNetworkFrom = destSubnetName
	}
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		globalNetworkFlags,
		true,
		msgSupportedNetworkOptions,
		subnetNameToGetNetworkFrom,
	)
	if err != nil {
		return err
	}

	_, sourceChainID, sourceMessengerAddress, _, sourceKey, err := getSubnetParams(network, sourceSubnetName)
	if err != nil {
		return err
	}
	_, destChainID, destMessengerAddress, _, _, err := getSubnetParams(network, destSubnetName)
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
	ux.Logger.PrintToUser("Delivering message %q from source subnet %q", message, sourceSubnetName)
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
	ux.Logger.PrintToUser("Waiting for message to be received on destination subnet %q", destSubnetName)
	var head *types.Header
	select {
	case head = <-destHeadsCh:
	case <-ctx.Done():
		return ctx.Err()
	}
	if sourceChainID == destChainID {
		// we have another block
		select {
		case head = <-destHeadsCh:
		case <-ctx.Done():
			return ctx.Err()
		}
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

func getSubnetParams(network models.Network, subnetName string) (ids.ID, ids.ID, string, string, *key.SoftKey, error) {
	var (
		subnetID                   ids.ID
		chainID                    ids.ID
		err                        error
		teleporterMessengerAddress string
		teleporterRegistryAddress  string
		k                          *key.SoftKey
	)
	if isCChain(subnetName) {
		subnetID = ids.Empty
		chainID, err = subnet.GetChainID(network, "C")
		if err != nil {
			return ids.Empty, ids.Empty, "", "", nil, err
		}
		if network.Kind == models.Local {
			extraLocalNetworkData, err := subnet.GetExtraLocalNetworkData(app)
			if err != nil {
				return ids.Empty, ids.Empty, "", "", nil, err
			}
			teleporterMessengerAddress = extraLocalNetworkData.CChainTeleporterMessengerAddress
			teleporterRegistryAddress = extraLocalNetworkData.CChainTeleporterRegistryAddress
			k, err = key.LoadEwoq(network.ID)
			if err != nil {
				return ids.Empty, ids.Empty, "", "", nil, err
			}
		}
	} else {
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return ids.Empty, ids.Empty, "", "", nil, err
		}
		subnetID = sc.Networks[network.Name()].SubnetID
		chainID = sc.Networks[network.Name()].BlockchainID
		teleporterMessengerAddress = sc.Networks[network.Name()].TeleporterMessengerAddress
		teleporterRegistryAddress = sc.Networks[network.Name()].TeleporterRegistryAddress
		keyPath := app.GetKeyPath(sc.TeleporterKey)
		k, err = key.LoadSoft(network.ID, keyPath)
		if err != nil {
			return ids.Empty, ids.Empty, "", "", nil, err
		}
	}
	if chainID == ids.Empty {
		return ids.Empty, ids.Empty, "", "", nil, fmt.Errorf("chainID for subnet %s not found on network %s", subnetName, network.Name())
	}
	if teleporterMessengerAddress == "" {
		return ids.Empty, ids.Empty, "", "", nil, fmt.Errorf("teleporter messenger address for subnet %s not found on network %s", subnetName, network.Name())
	}
	return subnetID, chainID, teleporterMessengerAddress, teleporterRegistryAddress, k, nil
}

func isCChain(subnetName string) bool {
	return strings.ToLower(subnetName) == "c-chain" || strings.ToLower(subnetName) == "cchain"
}
