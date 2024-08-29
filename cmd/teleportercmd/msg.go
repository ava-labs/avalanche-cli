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
	SourceRPCEndpoint  string
	DestRPCEndpoint    string
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
		Use:   "msg [sourceBlockchainName] [destinationBlockchainName] [messageContent]",
		Short: "Verifies exchange of teleporter message between two subnets",
		Long:  `Sends and wait reception for a teleporter msg between two subnets (Currently only for local network).`,
		RunE:  msg,
		Args:  cobrautils.ExactArgs(3),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &msgFlags.Network, true, msgSupportedNetworkOptions)
	contract.AddPrivateKeyFlagsToCmd(cmd, &msgFlags.PrivateKeyFlags, "as message originator and to pay source blockchain fees")
	cmd.Flags().BoolVar(&msgFlags.HexEncodedMessage, "hex-encoded", false, "given message is hex encoded")
	cmd.Flags().StringVar(&msgFlags.DestinationAddress, "destination-address", "", "deliver the message to the given contract destination address")
	cmd.Flags().StringVar(&msgFlags.SourceRPCEndpoint, "source-rpc", "", "use the given source blockchain rpc endpoint")
	cmd.Flags().StringVar(&msgFlags.DestRPCEndpoint, "dest-rpc", "", "use the given destination blockchain rpc endpoint")
	return cmd
}

func msg(_ *cobra.Command, args []string) error {
	sourceBlockchainName := args[0]
	destBlockchainName := args[1]
	message := args[2]

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		msgFlags.Network,
		true,
		false,
		msgSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	genesisAddress, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
		app,
		network,
		contract.ChainSpec{
			BlockchainName: sourceBlockchainName,
			CChain:         isCChain(sourceBlockchainName),
		},
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
			"pay for fees at source blockchain",
			app.GetKeyDir(),
			app.GetKey,
			genesisAddress,
			genesisPrivateKey,
		)
		if err != nil {
			return err
		}
	}

	sourceChainSpec := contract.ChainSpec{
		BlockchainName: sourceBlockchainName,
		CChain:         isCChain(sourceBlockchainName),
	}
	sourceBlockchainID, err := contract.GetBlockchainID(app, network, sourceChainSpec)
	if err != nil {
		return err
	}
	_, sourceMessengerAddress, err := contract.GetICMInfo(app, network, sourceChainSpec, false, false, true)
	if err != nil {
		return err
	}
	destChainSpec := contract.ChainSpec{
		BlockchainName: destBlockchainName,
		CChain:         isCChain(destBlockchainName),
	}
	destBlockchainID, err := contract.GetBlockchainID(app, network, destChainSpec)
	if err != nil {
		return err
	}
	_, destMessengerAddress, err := contract.GetICMInfo(app, network, destChainSpec, false, false, true)
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
	ux.Logger.PrintToUser("Delivering message %q from source subnet %q (%s)", message, sourceBlockchainName, sourceBlockchainID)
	sourceRPCEndpoint := msgFlags.SourceRPCEndpoint
	if sourceRPCEndpoint == "" {
		sourceRPCEndpoint = network.BlockchainEndpoint(sourceBlockchainID.String())
	}
	tx, receipt, err := teleporter.SendCrossChainMessage(
		sourceRPCEndpoint,
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
		trace, err := evm.GetTrace(sourceRPCEndpoint, txHash)
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
	ux.Logger.PrintToUser("Waiting for message to be delivered to destination subnet %q (%s)", destBlockchainName, destBlockchainID)
	destRPCEndpoint := msgFlags.DestRPCEndpoint
	if destRPCEndpoint == "" {
		destRPCEndpoint = network.BlockchainEndpoint(destBlockchainID.String())
	}

	arrivalCheckInterval := 100 * time.Millisecond
	arrivalCheckTimeout := 10 * time.Second
	t0 := time.Now()
	for {
		if b, err := teleporter.MessageReceived(
			destRPCEndpoint,
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
