// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"context"
	"fmt"
	"math/big"
	"time"

	cmdflags "github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/cmd/teleportercmd/bridgecmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	teleportermessenger "github.com/ava-labs/teleporter/abi-bindings/go/Teleporter/TeleporterMessenger"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"
)

type PrivateKeyFlags struct {
	PrivateKey string
	KeyName    string
	GenesisKey bool
}

type MsgFlags struct {
	Network            networkoptions.NetworkFlags
	DestinationAddress string
	HexEncodedMessage  bool
	PrivateKeyFlags    PrivateKeyFlags
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
	cmd.Flags().BoolVar(&msgFlags.HexEncodedMessage, "hex-encoded", false, "given message is hex encoded")
	cmd.Flags().StringVar(&msgFlags.DestinationAddress, "destination-address", "", "deliver the message to the given contract destination address")
	cmd.Flags().StringVar(&msgFlags.PrivateKeyFlags.PrivateKey, "private-key", "", "private key to use as message originator and to pay source blockchain fees")
	cmd.Flags().StringVar(&msgFlags.PrivateKeyFlags.KeyName, "key", "", "CLI stored key to use to use as message originator and to pay source blockchain fees")
	cmd.Flags().BoolVar(&msgFlags.PrivateKeyFlags.GenesisKey, "genesis-key", false, "use genesis aidrop key to use as message originator and to pay source blockchain fees")
	return cmd
}

func msg(_ *cobra.Command, args []string) error {
	sourceSubnetName := args[0]
	destSubnetName := args[1]
	message := args[2]

	if !cmdflags.EnsureMutuallyExclusive([]bool{
		msgFlags.PrivateKeyFlags.PrivateKey != "",
		msgFlags.PrivateKeyFlags.KeyName != "",
		msgFlags.PrivateKeyFlags.GenesisKey,
	}) {
		return fmt.Errorf("--private-key, --key and --genesis-key are mutually exclusive flags")
	}

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

	genesisAddress, genesisPrivateKey, err := getEVMSubnetPrefundedKey(
		network,
		sourceSubnetName,
		isCChain(sourceSubnetName),
	)
	if err != nil {
		return err
	}
	privateKey, err := getPrivateKeyFromFlags(
		msgFlags.PrivateKeyFlags,
		genesisPrivateKey,
	)
	if err != nil {
		return err
	}
	if privateKey == "" {
		privateKey, err = promptPrivateKey("send the message", genesisAddress, genesisPrivateKey)
		if err != nil {
			return err
		}
	}

	_, _, sourceBlockchainID, sourceMessengerAddress, _, _, err := bridgecmd.GetSubnetParams(
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
	txOpts, err := evm.GetTxOptsWithSigner(sourceClient, privateKey)
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

func getPrivateKeyFromFlags(
	flags PrivateKeyFlags,
	genesisPrivateKey string,
) (string, error) {
	privateKey := flags.PrivateKey
	if flags.KeyName != "" {
		k, err := app.GetKey(flags.KeyName, models.NewLocalNetwork(), false)
		if err != nil {
			return "", err
		}
		privateKey = k.PrivKeyHex()
	}
	if flags.GenesisKey {
		privateKey = genesisPrivateKey
	}
	return privateKey, nil
}

func getEVMSubnetPrefundedKey(
	network models.Network,
	subnetName string,
	isCChain bool,
) (string, string, error) {
	blockchainID := "C"
	if !isCChain {
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return "", "", fmt.Errorf("failed to load sidecar: %w", err)
		}
		if b, _, err := subnetcmd.HasSubnetEVMGenesis(subnetName); err != nil {
			return "", "", err
		} else if !b {
			return "", "", fmt.Errorf("getPrefundedKey only works on EVM based vms")
		}
		if sc.Networks[network.Name()].BlockchainID == ids.Empty {
			return "", "", fmt.Errorf("subnet has not been deployed to %s", network.Name())
		}
		blockchainID = sc.Networks[network.Name()].BlockchainID.String()
	}
	var (
		err     error
		chainID ids.ID
	)
	if isCChain || !network.StandardPublicEndpoint() {
		chainID, err = utils.GetChainID(network.Endpoint, blockchainID)
		if err != nil {
			return "", "", err
		}
	} else {
		chainID, err = ids.FromString(blockchainID)
		if err != nil {
			return "", "", err
		}
	}
	createChainTx, err := utils.GetBlockchainTx(network.Endpoint, chainID)
	if err != nil {
		return "", "", err
	}
	_, genesisAddress, genesisPrivateKey, err := subnet.GetSubnetAirdropKeyInfo(
		app,
		network,
		subnetName,
		createChainTx.GenesisData,
	)
	if err != nil {
		return "", "", err
	}
	return genesisAddress, genesisPrivateKey, nil
}

func promptPrivateKey(
	goal string,
	genesisAddress string,
	genesisPrivateKey string,
) (string, error) {
	privateKey := ""
	cliKeyOpt := "Get private key from an existing stored key (created from avalanche key create or avalanche key import)"
	customKeyOpt := "Custom"
	genesisKeyOpt := fmt.Sprintf("Use the private key of the Genesis Aidrop address %s", genesisAddress)
	keyOptions := []string{cliKeyOpt, customKeyOpt}
	if genesisPrivateKey != "" {
		keyOptions = []string{genesisKeyOpt, cliKeyOpt, customKeyOpt}
	}
	keyOption, err := app.Prompt.CaptureList(
		fmt.Sprintf("Which private key do you want to use to %s?", goal),
		keyOptions,
	)
	if err != nil {
		return "", err
	}
	switch keyOption {
	case cliKeyOpt:
		keyName, err := prompts.CaptureKeyName(app.Prompt, goal, app.GetKeyDir(), true)
		if err != nil {
			return "", err
		}
		k, err := app.GetKey(keyName, models.NewLocalNetwork(), false)
		if err != nil {
			return "", err
		}
		privateKey = k.PrivKeyHex()
	case customKeyOpt:
		privateKey, err = app.Prompt.CaptureString("Private Key")
		if err != nil {
			return "", err
		}
	case genesisKeyOpt:
		privateKey = genesisPrivateKey
	}
	return privateKey, nil
}
