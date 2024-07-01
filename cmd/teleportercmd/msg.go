// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"
)

type MsgFlags struct {
	Network            networkoptions.NetworkFlags
	DestinationAddress string
	HexEncodedMessage  bool
	PrivateKeyFlags    contract.PrivateKeyFlags
}

var (
	msgSupportedNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Devnet,
		networkoptions.Fuji,
	}
	msgFlags MsgFlags
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
	networkoptions.AddNetworkFlagsToCmd(cmd, &msgFlags.Network, true, msgSupportedNetworkOptions)
	contract.AddPrivateKeyFlagsToCmd(cmd, &msgFlags.PrivateKeyFlags, "as message originator and to pay source blockchain fees")
	cmd.Flags().BoolVar(&msgFlags.HexEncodedMessage, "hex-encoded", false, "given message is hex encoded")
	cmd.Flags().StringVar(&msgFlags.DestinationAddress, "destination-address", "", "deliver the message to the given contract destination address")
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
		msgFlags.Network,
		true,
		false,
		msgSupportedNetworkOptions,
		subnetNameToGetNetworkFrom,
	)
	if err != nil {
		return err
	}

	genesisAddress, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
		app,
		network,
		sourceSubnetName,
		isCChain(sourceSubnetName),
		"",
	)
	if err != nil {
		return err
	}
	privateKey, err := contract.GetPrivateKeyFromFlags(
		app,
		msgFlags.PrivateKeyFlags,
		genesisPrivateKey,
	)
	if err != nil {
		return err
	}
	if privateKey == "" {
		privateKey, err = prompts.PromptPrivateKey(
			app.Prompt,
			"send the message",
			app.GetKeyDir(),
			app.GetKey,
			genesisAddress,
			genesisPrivateKey,
		)
		if err != nil {
			return err
		}
	}

	_, _, sourceBlockchainID, sourceMessengerAddress, _, _, err := teleporter.GetSubnetParams(
		app,
		network,
		sourceSubnetName,
		isCChain(sourceSubnetName),
	)
	if err != nil {
		return err
	}
	_, _, destBlockchainID, destMessengerAddress, _, _, err := teleporter.GetSubnetParams(
		app,
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

	encodedMessage := []byte(message)
	if msgFlags.HexEncodedMessage {
		encodedMessage = common.FromHex(message)
	}
	destAddr := common.Address{}
	if msgFlags.DestinationAddress != "" {
		if err := prompts.ValidateAddress(msgFlags.DestinationAddress); err != nil {
			return fmt.Errorf("failure validating address %s: %w", msgFlags.DestinationAddress, err)
		}
		destAddr = common.HexToAddress(msgFlags.DestinationAddress)
	}
	// send tx to the teleporter contract at the source
	ux.Logger.PrintToUser("Delivering message %q from source subnet %q (%s)", message, sourceSubnetName, sourceBlockchainID)
	tx, receipt, err := teleporter.SendCrossChainMessage(
		network.BlockchainEndpoint(sourceBlockchainID.String()),
		common.HexToAddress(sourceMessengerAddress),
		privateKey,
		destBlockchainID,
		destAddr,
		encodedMessage,
	)
	if err != nil {
		return err
	}
	if err == contract.ErrFailedReceiptStatus {
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

	event, err := evm.GetEventFromLogs(receipt.Logs, teleporter.ParseSendCrossChainMessage)
	if err != nil {
		return err
	}

	if destBlockchainID != ids.ID(event.DestinationBlockchainID[:]) {
		return fmt.Errorf("invalid destination blockchain id at source event, expected %s, got %s", destBlockchainID, ids.ID(event.DestinationBlockchainID[:]))
	}
	if message != string(event.Message.Message) {
		return fmt.Errorf("invalid message content at source event, expected %s, got %s", message, string(event.Message.Message))
	}

	// receive and process head from destination
	ux.Logger.PrintToUser("Waiting for message to be delivered to destination subnet %q (%s)", destSubnetName, destBlockchainID)

	arrivalCheckInterval := 100 * time.Millisecond
	arrivalCheckTimeout := 10 * time.Second
	t0 := time.Now()
	for {
		if b, err := teleporter.MessageReceived(
			network.BlockchainEndpoint(destBlockchainID.String()),
			common.HexToAddress(destMessengerAddress),
			event.MessageID,
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

func isCChain(subnetName string) bool {
	return strings.ToLower(subnetName) == "c-chain" || strings.ToLower(subnetName) == "cchain"
}
