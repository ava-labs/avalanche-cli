// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package bridge

import (
	_ "embed"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ethereum/go-ethereum/common"
)

type HubKind int64

const (
	Undefined HubKind = iota
	ERC20TokenHub
	NativeTokenHub
)

func GetHubKind(
	rpcURL string,
	hubAddress common.Address,
) (HubKind, error) {
	if _, err := ERC20TokenHubGetTokenAddress(rpcURL, hubAddress); err == nil {
		return ERC20TokenHub, nil
	}
	if _, err := NativeTokenHubGetTokenAddress(rpcURL, hubAddress); err == nil {
		return NativeTokenHub, nil
	} else {
		return Undefined, err
	}
}

func ERC20TokenHubGetTokenAddress(
	rpcURL string,
	hubAddress common.Address,
) (common.Address, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		hubAddress,
		"token()->(address)",
	)
	if err != nil {
		return common.Address{}, err
	}
	tokenAddress := out[0].(common.Address)
	return tokenAddress, nil
}

func NativeTokenHubGetTokenAddress(
	rpcURL string,
	hubAddress common.Address,
) (common.Address, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		hubAddress,
		"wrappedToken()->(address)",
	)
	if err != nil {
		return common.Address{}, err
	}
	tokenAddress := out[0].(common.Address)
	return tokenAddress, nil
}

func ERC20TokenHubSend(
	rpcURL string,
	hubAddress common.Address,
	privateKey string,
	destinationBlockchainID ids.ID,
	destinationBridgeEnd common.Address,
	amountRecipient common.Address,
	amount *big.Int,
) error {
	type Params struct {
		DestinationBlockchainID [32]byte
		DestinationBridgeEnd    common.Address
		AmountRecipient         common.Address
		PrimaryFeeTokenAddress  common.Address
		PrimaryFee              *big.Int
		SecondaryFee            *big.Int
		RequiredGasLimit        *big.Int
		MultiHopFallback        common.Address
	}
	tokenAddress, err := ERC20TokenHubGetTokenAddress(rpcURL, hubAddress)
	if err != nil {
		return err
	}
	if err := contract.TxToMethod(
		rpcURL,
		privateKey,
		tokenAddress,
		nil,
		"approve(address, uint256)->(bool)",
		hubAddress,
		amount,
	); err != nil {
		return err
	}
	params := Params{
		DestinationBlockchainID: destinationBlockchainID,
		DestinationBridgeEnd:    destinationBridgeEnd,
		AmountRecipient:         amountRecipient,
		PrimaryFeeTokenAddress:  tokenAddress, // in theory this is optional
		PrimaryFee:              big.NewInt(0),
		SecondaryFee:            big.NewInt(0),
		RequiredGasLimit:        big.NewInt(250000),
		MultiHopFallback:        common.Address{},
	}
	return contract.TxToMethod(
		rpcURL,
		privateKey,
		hubAddress,
		nil,
		"send((bytes32, address, address, address, uint256, uint256, uint256, address), uint256)",
		params,
		amount,
	)
}

func NativeTokenHubSend(
	rpcURL string,
	hubAddress common.Address,
	privateKey string,
	destinationBlockchainID ids.ID,
	destinationBridgeEnd common.Address,
	amountRecipient common.Address,
	amount *big.Int,
) error {
	type Params struct {
		DestinationBlockchainID [32]byte
		DestinationBridgeEnd    common.Address
		AmountRecipient         common.Address
		PrimaryFeeTokenAddress  common.Address
		PrimaryFee              *big.Int
		SecondaryFee            *big.Int
		RequiredGasLimit        *big.Int
		MultiHopFallback        common.Address
	}
	tokenAddress, err := NativeTokenHubGetTokenAddress(rpcURL, hubAddress)
	if err != nil {
		return err
	}
	params := Params{
		DestinationBlockchainID: destinationBlockchainID,
		DestinationBridgeEnd:    destinationBridgeEnd,
		AmountRecipient:         amountRecipient,
		PrimaryFeeTokenAddress:  tokenAddress, // in theory this is optional
		PrimaryFee:              big.NewInt(0),
		SecondaryFee:            big.NewInt(0),
		RequiredGasLimit:        big.NewInt(250000),
		MultiHopFallback:        common.Address{},
	}
	return contract.TxToMethod(
		rpcURL,
		privateKey,
		hubAddress,
		amount,
		"send((bytes32, address, address, address, uint256, uint256, uint256, address))",
		params,
	)
}
