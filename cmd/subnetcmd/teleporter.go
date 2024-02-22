// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"fmt"
	"os"
	"context"

	teleportermessenger "github.com/ava-labs/teleporter/abi-bindings/go/Teleporter/TeleporterMessenger"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/precompile/contracts/warp"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"
)

// avalanche subnet teleporter
func newTeleporterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "teleporter",
		Short:             "Deploys teleporter to local network cchain",
		Long:              `Deploys teleporter to a local network cchain.`,
		SilenceUsage:      true,
		RunE:              deployTeleporter,
		PersistentPostRun: handlePostRun,
		Args:              cobra.ExactArgs(0),
	}
	return cmd
}

func deployTeleporter(cmd *cobra.Command, args []string) error {
	pp1sc, err := app.LoadSidecar("pp1")
	if err != nil {
		return err
	}
	pp2sc, err := app.LoadSidecar("pp2")
	if err != nil {
		return err
	}
	pp1BlockchainID := pp1sc.Networks[models.LocalNetwork.Name()].BlockchainID
	pp2BlockchainID := pp2sc.Networks[models.LocalNetwork.Name()].BlockchainID
	_, cchainBlockchainID, err := subnet.GetChainIDs(models.LocalNetwork.Endpoint, "C-Chain")
	if err != nil {
		return err
	}
	_ = cchainBlockchainID
	_ = pp1BlockchainID
	_ = pp2BlockchainID

	sourceBlockchainIDStr := pp2BlockchainID.String()
	destinationBlockchainIDStr := cchainBlockchainID


	extraLocalNetworkData := subnet.ExtraLocalNetworkData{}
	extraLocalNetworkDataPath := app.GetExtraLocalNetworkDataPath()
	if utils.FileExists(extraLocalNetworkDataPath) {
		bs, err := os.ReadFile(extraLocalNetworkDataPath)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(bs, &extraLocalNetworkData); err != nil {
			return err
		}
	}
	fmt.Println(extraLocalNetworkData)

	sourceUrl := models.LocalNetwork.BlockchainEndpoint(sourceBlockchainIDStr)
	destinationUrl := models.LocalNetwork.BlockchainEndpoint(destinationBlockchainIDStr)

	fmt.Println(destinationUrl)
	destinationClient, err := evm.GetClient(destinationUrl)
	if err != nil {
		return err
	}
	_ = destinationClient

	destinationWSUrl := models.LocalNetwork.BlockchainWSEndpoint(destinationBlockchainIDStr)
	fmt.Println(destinationWSUrl)
	destinationWSClient, err := evm.GetClient(destinationWSUrl)
	if err != nil {
		return err
	}
	_ = destinationWSClient
	newHeadsDest := make(chan *types.Header, 10)
	sub, err := destinationWSClient.SubscribeNewHead(context.Background(), newHeadsDest)
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()


	sourceClient, err := evm.GetClient(sourceUrl)
	if err != nil {
		return err
	}

	k, err := key.LoadEwoq(models.LocalNetwork.ID)
	if err != nil {
		return err
	}
	privKeyStr := hex.EncodeToString(k.Raw())
	signer, err := evm.GetSigner(sourceClient, privKeyStr)
	if err != nil {
		return err
	}
	teleporterMessengerContractAddressStr := "0xF7cBd95f1355f0d8d659864b92e2e9fbfaB786f7"
	teleporterMessengerContractAddress := common.HexToAddress(teleporterMessengerContractAddressStr)
	sourceMessenger, err := teleportermessenger.NewTeleporterMessenger(teleporterMessengerContractAddress, sourceClient)
	if err != nil {
		return err
	}
	destinationMessenger, err := teleportermessenger.NewTeleporterMessenger(teleporterMessengerContractAddress, destinationClient)
	if err != nil {
		return err
	}
	fundedAddress := common.HexToAddress(k.C())
	destinationBlockchainID, err := ids.FromString(destinationBlockchainIDStr)
	if err != nil {
		return err
	}
        input := teleportermessenger.TeleporterMessageInput{
                DestinationBlockchainID: destinationBlockchainID,
                DestinationAddress:      fundedAddress,
                FeeInfo: teleportermessenger.TeleporterFeeInfo{
                        FeeTokenAddress: fundedAddress,
                        Amount:          big.NewInt(0),                         
                },
                RequiredGasLimit:        big.NewInt(1),
                AllowedRelayerAddresses: []common.Address{},
                Message:                 []byte{11, 1, 2, 3, 4, 11},
        }
	// send tx to the source teleporter contract
	tx, err := sourceMessenger.SendCrossChainMessage(signer, input)
	if err != nil {
		return err
	}
	receipt, b, err := evm.WaitForTransaction(sourceClient, tx)
	if err != nil {
		return err
	}
	if !b {
		return fmt.Errorf("bad status")
	}
	event, err := GetEventFromLogs(receipt.Logs, sourceMessenger.ParseSendCrossChainMessage)
	if err != nil {
		return err
	}

	fmt.Println(hex.EncodeToString(event.MessageID[:]))
	fmt.Printf("%#v\n", event.Message)

	fmt.Println("Waiting for new block confirmation")
        newHead := <-newHeadsDest
        blockNumber := newHead.Number
	fmt.Println("Received new head", "height", blockNumber.Uint64(), "hash", newHead.Hash())

       block, err := destinationClient.BlockByNumber(context.Background(), blockNumber)
       if err != nil {
	       return err
       }
       fmt.Println(
                "Got block",
                "blockHash", block.Hash(),
                "blockNumber", block.NumberU64(),
                "transactions", block.Transactions(),
                "numTransactions", len(block.Transactions()),
                "block", block,
        )
        accessLists := block.Transactions()[0].AccessList()
	fmt.Println(accessLists)

	fmt.Println(accessLists[0].Address, warp.Module.Address)

        txHash := block.Transactions()[0].Hash()
        receipt, err = destinationClient.TransactionReceipt(context.Background(), txHash)
	if err != nil {
		return err
	}

        fmt.Println(receipt.Status == types.ReceiptStatusSuccessful)

        receiveEvent, err := GetEventFromLogs(receipt.Logs, destinationMessenger.ParseReceiveCrossChainMessage)
	if err != nil {
		return err
	}
	sourceBlockchainID, err := ids.FromString(sourceBlockchainIDStr)
	if err != nil {
		return err
	}
	fmt.Println(receiveEvent.SourceBlockchainID[:], sourceBlockchainID[:])
	fmt.Println(receiveEvent.MessageID[:], event.MessageID[:])


	return nil
}

// Returns the first log in 'logs' that is successfully parsed by 'parser'
func GetEventFromLogs[T any](logs []*types.Log, parser func(log types.Log) (T, error)) (T, error) {
        for _, log := range logs {
                event, err := parser(*log)
                if err == nil {
                        return event, nil
                }
        }
        return *new(T), fmt.Errorf("failed to find %T event in receipt logs", *new(T))
}
