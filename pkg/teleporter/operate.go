// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleporter

import (
	_ "embed"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ethereum/go-ethereum/common"
)

func MessageReceived(
	rpcURL string,
	messengerAddress common.Address,
	messageID ids.ShortID,
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
	received := out[0].(bool)
	return received, nil
}

func SendCrossChainMessage(
	rpcURL string,
	messengerAddress common.Address,
	privateKey string,
	destinationBlockchainID ids.ID,
	destinationAddress common.Address,
	message string,
) error {
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
		Message:                 []byte(message),
	}
	return contract.TxToMethod(
		rpcURL,
		privateKey,
		messengerAddress,
		nil,
		"sendCrossChainMessage((bytes32, address, (address, uint256), uin256, address[], bytes))->(bytes32)",
		params,
	)
}
