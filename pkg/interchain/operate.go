// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package interchain

import (
	_ "embed"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
)

func GetNextMessageID(
	rpcURL string,
	messengerAddress common.Address,
	destinationBlockchainID ids.ID,
) (ids.ID, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		messengerAddress,
		"getNextMessageID(bytes32)->(bytes32)",
		destinationBlockchainID,
	)
	if err != nil {
		return ids.Empty, err
	}
	return contract.GetSmartContractCallResult[[32]byte]("getNextMessageID", out)
}

func MessageReceived(
	rpcURL string,
	messengerAddress common.Address,
	messageID ids.ID,
) (bool, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		messengerAddress,
		"messageReceived(bytes32)->(bool)",
		messageID,
	)
	if err != nil {
		return false, err
	}
	return contract.GetSmartContractCallResult[bool]("messageReceived", out)
}

func SendCrossChainMessage(
	rpcURL string,
	messengerAddress common.Address,
	privateKey string,
	destinationBlockchainID ids.ID,
	destinationAddress common.Address,
	message []byte,
) (*types.Transaction, *types.Receipt, error) {
	type FeeInfo struct {
		FeeTokenAddress common.Address
		Amount          *big.Int
	}
	type Params struct {
		DestinationBlockchainID [32]byte
		DestinationAddress      common.Address
		FeeInfo                 FeeInfo
		RequiredGasLimit        *big.Int
		AllowedRelayerAddresses []common.Address
		Message                 []byte
	}
	params := Params{
		DestinationBlockchainID: destinationBlockchainID,
		DestinationAddress:      destinationAddress,
		FeeInfo: FeeInfo{
			FeeTokenAddress: common.Address{},
			Amount:          big.NewInt(0),
		},
		RequiredGasLimit:        big.NewInt(1),
		AllowedRelayerAddresses: []common.Address{},
		Message:                 message,
	}
	return contract.TxToMethod(
		rpcURL,
		false,
		common.Address{},
		privateKey,
		messengerAddress,
		nil,
		"send cross chain message",
		nil,
		"sendCrossChainMessage((bytes32, address, (address, uint256), uint256, [address], bytes))->(bytes32)",
		params,
	)
}

// events

type ICMMessageReceipt struct {
	ReceivedMessageNonce *big.Int
	RelayerRewardAddress common.Address
}
type ICMFeeInfo struct {
	FeeTokenAddress common.Address
	Amount          *big.Int
}
type ICMMessage struct {
	MessageNonce            *big.Int
	OriginSenderAddress     common.Address
	DestinationBlockchainID [32]byte
	DestinationAddress      common.Address
	RequiredGasLimit        *big.Int
	AllowedRelayerAddresses []common.Address
	Receipts                []ICMMessageReceipt
	Message                 []byte
}
type ICMMessengerSendCrossChainMessage struct {
	MessageID               [32]byte
	DestinationBlockchainID [32]byte
	Message                 ICMMessage
	FeeInfo                 ICMFeeInfo
}

func ParseSendCrossChainMessage(log types.Log) (*ICMMessengerSendCrossChainMessage, error) {
	event := new(ICMMessengerSendCrossChainMessage)
	if err := contract.UnpackLog(
		"SendCrossChainMessage(bytes32,bytes32,(uint256,address,bytes32,address,uint256,[address],[(uint256,address)],bytes),(address,uint256))",
		[]int{0, 1},
		log,
		event,
	); err != nil {
		return nil, err
	}
	return event, nil
}
