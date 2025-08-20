// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package messengercmd

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/duallogger"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/interchain"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-tooling-sdk-go/evm"
	contractSDK "github.com/ava-labs/avalanche-tooling-sdk-go/evm/contract"
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

var msgFlags MsgFlags

// avalanche interchain messenger sendMsg
func NewSendMsgCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sendMsg [sourceBlockchainName] [destinationBlockchainName] [messageContent]",
		Short: "Verifies exchange of ICM message between two blockchains",
		Long:  `Sends and wait reception for a ICM msg between two blockchains.`,
		RunE:  sendMsg,
		Args:  cobrautils.ExactArgs(3),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &msgFlags.Network, true, networkoptions.DefaultSupportedNetworkOptions)
	msgFlags.PrivateKeyFlags.AddToCmd(cmd, "as message originator and to pay source blockchain fees")
	cmd.Flags().BoolVar(&msgFlags.HexEncodedMessage, "hex-encoded", false, "given message is hex encoded")
	cmd.Flags().StringVar(&msgFlags.DestinationAddress, "destination-address", "", "deliver the message to the given contract destination address")
	cmd.Flags().StringVar(&msgFlags.SourceRPCEndpoint, "source-rpc", "", "use the given source blockchain rpc endpoint")
	cmd.Flags().StringVar(&msgFlags.DestRPCEndpoint, "dest-rpc", "", "use the given destination blockchain rpc endpoint")
	return cmd
}

func sendMsg(_ *cobra.Command, args []string) error {
	sourceBlockchainName := args[0]
	destBlockchainName := args[1]
	message := args[2]

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		msgFlags.Network,
		true,
		false,
		networkoptions.DefaultSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	sourceChainSpec := contract.ChainSpec{}
	if isCChain(sourceBlockchainName) {
		sourceChainSpec.CChain = true
	} else {
		sourceChainSpec.BlockchainName = sourceBlockchainName
	}
	sourceRPCEndpoint := msgFlags.SourceRPCEndpoint
	if sourceRPCEndpoint == "" {
		sourceRPCEndpoint, _, err = contract.GetBlockchainEndpoints(app, network, sourceChainSpec, true, false)
		if err != nil {
			return err
		}
	}

	destChainSpec := contract.ChainSpec{}
	if isCChain(destBlockchainName) {
		destChainSpec.CChain = true
	} else {
		destChainSpec.BlockchainName = destBlockchainName
	}
	destRPCEndpoint := msgFlags.DestRPCEndpoint
	if destRPCEndpoint == "" {
		destRPCEndpoint, _, err = contract.GetBlockchainEndpoints(app, network, destChainSpec, true, false)
		if err != nil {
			return err
		}
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
	privateKey, err := msgFlags.PrivateKeyFlags.GetPrivateKey(app, genesisPrivateKey)
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

	sourceBlockchainID, err := contract.GetBlockchainID(app, network, sourceChainSpec)
	if err != nil {
		return err
	}
	_, sourceMessengerAddress, err := contract.GetICMInfo(app, network, sourceChainSpec, false, false, true)
	if err != nil {
		return err
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
		return fmt.Errorf("different ICM messenger addresses among blockchains: %s vs %s", sourceMessengerAddress, destMessengerAddress)
	}

	messageBytes := []byte(message)
	if msgFlags.HexEncodedMessage {
		toDecode := message
		if strings.HasPrefix(toDecode, "0x") {
			toDecode = strings.TrimPrefix(toDecode, "0x")
		} else if strings.HasPrefix(toDecode, "0X") {
			toDecode = strings.TrimPrefix(toDecode, "0X")
		}
		messageBytes, err = hex.DecodeString(toDecode)
		if err != nil {
			return fmt.Errorf("invalid hex format at %s", message)
		}
	}
	destAddr := common.Address{}
	if msgFlags.DestinationAddress != "" {
		if err := prompts.ValidateAddress(msgFlags.DestinationAddress); err != nil {
			return fmt.Errorf("failure validating address %s: %w", msgFlags.DestinationAddress, err)
		}
		destAddr = common.HexToAddress(msgFlags.DestinationAddress)
	}
	// send tx to the ICM contract at the source
	ux.Logger.PrintToUser("Delivering message %q from source blockchain %q (%s)", message, sourceBlockchainName, sourceBlockchainID)
	tx, receipt, err := interchain.SendCrossChainMessage(
		duallogger.NewDualLogger(true, app),
		sourceRPCEndpoint,
		common.HexToAddress(sourceMessengerAddress),
		privateKey,
		destBlockchainID,
		destAddr,
		messageBytes,
	)
	if err != nil {
		return err
	}
	if err == contractSDK.ErrFailedReceiptStatus {
		txHash := tx.Hash().String()
		ux.Logger.PrintToUser("error: source receipt status for tx %s is not ReceiptStatusSuccessful", txHash)
		trace, err := evm.GetTxTrace(sourceRPCEndpoint, txHash)
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

	event, err := evm.GetEventFromLogs(receipt.Logs, interchain.ParseSendCrossChainMessage)
	if err != nil {
		return err
	}

	if destBlockchainID != ids.ID(event.DestinationBlockchainID[:]) {
		return fmt.Errorf("invalid destination blockchain id at source event, expected %s, got %s", destBlockchainID, ids.ID(event.DestinationBlockchainID[:]))
	}

	receivedMessage := string(event.Message.Message)
	if msgFlags.HexEncodedMessage {
		receivedMessage = common.Bytes2Hex(event.Message.Message)
	}
	if string(messageBytes) != string(event.Message.Message) {
		return fmt.Errorf("invalid message content at source event, expected %s, got %s", message, receivedMessage)
	}

	// receive and process head from destination
	ux.Logger.PrintToUser("Waiting for message to be delivered to destination blockchain %q (%s)", destBlockchainName, destBlockchainID)

	arrivalCheckInterval := 100 * time.Millisecond
	arrivalCheckTimeout := 10 * time.Second
	t0 := time.Now()
	for {
		if b, err := interchain.MessageReceived(
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

	ux.Logger.PrintToUser("Message successfully delivered!")

	return nil
}

func isCChain(subnetName string) bool {
	return strings.ToLower(subnetName) == "c-chain" || strings.ToLower(subnetName) == "cchain"
}
