// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ava-labs/avalanche-cli/cmd/teleportercmd/bridgecmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
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
	destinationAddress string
	hexEncodedMessage  bool
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
	cmd.Flags().BoolVar(&hexEncodedMessage, "hex-encoded", false, "given message is hex encoded")
	cmd.Flags().StringVar(&destinationAddress, "destination-address", "", "deliver the message to the given contract destination address")
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

	_, _, sourceBlockchainID, sourceMessengerAddress, _, sourceKey, err := bridgecmd.GetSubnetParams(
		network,
		sourceSubnetName,
		isCChain(sourceSubnetName),
	)
	if err != nil {
		return err
	}
	_, _, destBlockchainID, destMessengerAddress, _, _, err := bridgecmd.GetSubnetParams(
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

	// get clients + messengers
	sourceClient, err := evm.GetClient(network.BlockchainEndpoint(sourceBlockchainID.String()))
	if err != nil {
		return err
	}
	sourceMessenger, err := teleportermessenger.NewTeleporterMessenger(common.HexToAddress(sourceMessengerAddress), sourceClient)
	if err != nil {
		return err
	}

	encodedMessage := []byte(message)
	if hexEncodedMessage {
		encodedMessage = common.FromHex(message)
	}
	destAddr := common.Address{}
	if destinationAddress != "" {
		if err := prompts.ValidateAddress(destinationAddress); err != nil {
			return fmt.Errorf("failure validating address %s: %w", destinationAddress, err)
		}
		destAddr = common.HexToAddress(destinationAddress)
	}
	// send tx to the teleporter contract at the source
	msgInput := teleportermessenger.TeleporterMessageInput{
		DestinationBlockchainID: destBlockchainID,
		DestinationAddress:      destAddr,
		FeeInfo: teleportermessenger.TeleporterFeeInfo{
			FeeTokenAddress: common.Address{},
			Amount:          big.NewInt(0),
		},
		RequiredGasLimit:        big.NewInt(0),
		AllowedRelayerAddresses: []common.Address{},
		Message:                 encodedMessage,
	}
	ux.Logger.PrintToUser("Delivering message %q from source subnet %q (%s)", message, sourceSubnetName, sourceBlockchainID)
	txOpts, err := evm.GetTxOptsWithSigner(sourceClient, sourceKey.PrivKeyHex())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()
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
		trace, err := evm.GetTrace(network.BlockchainEndpoint(sourceBlockchainID.String()), txHash)
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

	if destBlockchainID != ids.ID(sourceEvent.DestinationBlockchainID[:]) {
		return fmt.Errorf("invalid destination blockchain id at source event, expected %s, got %s", destBlockchainID, ids.ID(sourceEvent.DestinationBlockchainID[:]))
	}
	if message != string(sourceEvent.Message.Message) {
		return fmt.Errorf("invalid message content at source event, expected %s, got %s", message, string(sourceEvent.Message.Message))
	}

	// receive and process head from destination
	ux.Logger.PrintToUser("Waiting for message to be delivered to destination subnet %q (%s)", destSubnetName, destBlockchainID)

	arrivalCheckInterval := time.Duration(100 * time.Millisecond)
	arrivalCheckTimeout := time.Duration(10 * time.Second)
	t0 := time.Now()
	for {
		if b, err := teleporter.MessageReceived(
			network.BlockchainEndpoint(destBlockchainID.String()),
			common.HexToAddress(destMessengerAddress),
			sourceEvent.MessageID,
		); err != nil {
			return err
		} else if b {
			break
		}
		elapsed := time.Since(t0)
		if elapsed > arrivalCheckTimeout {
			return fmt.Errorf("timeout waiting for message to be teleported")
		}
		time.Sleep(arrivalCheckInterval)
	}

	ux.Logger.PrintToUser("Message successfully Teleported!")

	return nil
}
