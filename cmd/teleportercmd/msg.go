// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/ava-labs/avalanche-cli/cmd/teleportercmd/bridgecmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/core/types"
	teleportermessenger "github.com/ava-labs/teleporter/abi-bindings/go/Teleporter/TeleporterMessenger"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"
)

var (
	msgSupportedNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Cluster,
		networkoptions.Fuji,
		networkoptions.Mainnet,
		networkoptions.Devnet,
	}
	globalNetworkFlags networkoptions.NetworkFlags
)

// avalanche teleporter msg
func newMsgCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "msg [sourceSubnetName] [destinationSubnetName] [messageContent]",
		Short: "Verifies exchange of teleporter message between two subnets",
		Long:  `Sends and wait reception for a teleporter msg between two subnets (Currently only for local network).`,
		RunE:  msg,
		Args:  cobrautils.ExactArgs(3),
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
		"",
		globalNetworkFlags,
		true,
		false,
		msgSupportedNetworkOptions,
		subnetNameToGetNetworkFrom,
	)
	if err != nil {
		return err
	}

	_, _, sourceChainID, sourceMessengerAddress, _, sourceKey, err := bridgecmd.GetSubnetParams(
		network,
		sourceSubnetName,
		isCChain(sourceSubnetName),
	)
	if err != nil {
		return err
	}
	_, _, destChainID, destMessengerAddress, _, _, err := bridgecmd.GetSubnetParams(
		network,
		destSubnetName,
		isCChain(destSubnetName),
	)
	if err != nil {
		return err
	}

	if sourceMessengerAddress != destMessengerAddress {
		return fmt.Errorf("different teleporter messenger addresses among subnets: %s vs %s", sourceMessengerAddress, destMessengerAddress)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
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
	ux.Logger.PrintToUser("Delivering message %q from source subnet %q (%s)", message, sourceSubnetName, sourceChainID)
	txOpts, err := evm.GetTxOptsWithSigner(sourceClient, sourceKey.PrivKeyHex())
	if err != nil {
		return err
	}
	txOpts.Context = ctx
	tx, err := sourceMessenger.SendCrossChainMessage(txOpts, msgInput)
	if err != nil {
		return err
	}
	sourceReceipt, b, err := evm.WaitForTransaction(sourceClient, tx)
	if err != nil {
		return err
	}
	if !b {
		txHash := tx.Hash().String()
		ux.Logger.PrintToUser("error: source receipt status for tx %s is not ReceiptStatusSuccessful", txHash)
		trace, err := evm.GetTrace(network.BlockchainEndpoint(sourceChainID.String()), txHash)
		if err != nil {
			ux.Logger.PrintToUser("error obtaining tx trace: %s", err)
			ux.Logger.PrintToUser("")
		} else {
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser("trace: %#v", trace)
			ux.Logger.PrintToUser("")
		}
		return fmt.Errorf("source receipt status for tx %s is not ReceiptStatusSuccessful", txHash)
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
	ux.Logger.PrintToUser("Waiting for message to be received on destination subnet %q (%s)", destSubnetName, destChainID)
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
		txHash := block.Transactions()[0].Hash().String()
		ux.Logger.PrintToUser("error: dest receipt status for tx %s is not ReceiptStatusSuccessful", txHash)
		trace, err := evm.GetTrace(network.BlockchainEndpoint(destChainID.String()), txHash)
		if err != nil {
			ux.Logger.PrintToUser("error obtaining tx trace: %s", err)
			ux.Logger.PrintToUser("")
		} else {
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser("trace: %#v", trace)
			ux.Logger.PrintToUser("")
		}
		return fmt.Errorf("dest receipt status for tx %s is not ReceiptStatusSuccessful", txHash)
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
