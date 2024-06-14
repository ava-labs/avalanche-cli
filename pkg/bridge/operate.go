// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package bridge

import (
	_ "embed"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ethereum/go-ethereum/common"
)

type EndpointKind int64

const (
	Undefined EndpointKind = iota
	ERC20TokenHub
	NativeTokenHub
	ERC20TokenSpoke
)

func GetEndpointKind(
	rpcURL string,
	address common.Address,
) (EndpointKind, error) {
	if _, err := ERC20TokenHubGetTokenAddress(rpcURL, address); err == nil {
		return ERC20TokenHub, nil
	}
	if _, err := NativeTokenHubGetTokenAddress(rpcURL, address); err == nil {
		return NativeTokenHub, nil
	}
	if _, err := ERC20TokenSpokeGetTokenHubAddress(rpcURL, address); err == nil {
		return ERC20TokenSpoke, nil
	} else {
		return Undefined, err
	}
}

func ERC20TokenHubGetTokenAddress(
	rpcURL string,
	address common.Address,
) (common.Address, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		address,
		"token()->(address)",
	)
	if err != nil {
		return common.Address{}, err
	}
	tokenAddress, b := out[0].(common.Address)
	if !b {
		return common.Address{}, fmt.Errorf("error at token call, expected common.Address, got %T", out[0])
	}
	return tokenAddress, nil
}

func NativeTokenHubGetTokenAddress(
	rpcURL string,
	address common.Address,
) (common.Address, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		address,
		"wrappedToken()->(address)",
	)
	if err != nil {
		return common.Address{}, err
	}
	tokenAddress, b := out[0].(common.Address)
	if !b {
		return common.Address{}, fmt.Errorf("error at wrappedToken call, expected common.Address, got %T", out[0])
	}
	return tokenAddress, nil
}

func ERC20TokenSpokeGetTokenHubAddress(
	rpcURL string,
	address common.Address,
) (common.Address, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		address,
		"tokenHubAddress()->(address)",
	)
	if err != nil {
		return common.Address{}, err
	}
	tokenHubAddress, b := out[0].(common.Address)
	if !b {
		return common.Address{}, fmt.Errorf("error at tokenHubAddress call, expected common.Address, got %T", out[0])
	}
	return tokenHubAddress, nil
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

func ERC20TokenSpokeSend(
	rpcURL string,
	spokeAddress common.Address,
	privateKey string,
	destinationBlockchainID ids.ID,
	destinationBridgeEnd common.Address,
	amountRecipient common.Address,
	amount *big.Int,
) error {
	if err := contract.TxToMethod(
		rpcURL,
		privateKey,
		spokeAddress,
		nil,
		"approve(address, uint256)->(bool)",
		spokeAddress,
		amount,
	); err != nil {
		return err
	}
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
	params := Params{
		DestinationBlockchainID: destinationBlockchainID,
		DestinationBridgeEnd:    destinationBridgeEnd,
		AmountRecipient:         amountRecipient,
		PrimaryFeeTokenAddress:  common.Address{},
		PrimaryFee:              big.NewInt(0),
		SecondaryFee:            big.NewInt(0),
		RequiredGasLimit:        big.NewInt(250000),
		MultiHopFallback:        common.Address{},
	}
	return contract.TxToMethod(
		rpcURL,
		privateKey,
		spokeAddress,
		nil,
		"send((bytes32, address, address, address, uint256, uint256, uint256, address), uint256)",
		params,
		amount,
	)
}
