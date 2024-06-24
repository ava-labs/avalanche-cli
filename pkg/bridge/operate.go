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
	ERC20TokenHome
	NativeTokenHome
	ERC20TokenRemote
)

func GetEndpointKind(
	rpcURL string,
	address common.Address,
) (EndpointKind, error) {
	if _, err := ERC20TokenHomeGetTokenAddress(rpcURL, address); err == nil {
		return ERC20TokenHome, nil
	}
	if _, err := NativeTokenHomeGetTokenAddress(rpcURL, address); err == nil {
		return NativeTokenHome, nil
	}
	if _, err := ERC20TokenRemoteGetTokenHomeAddress(rpcURL, address); err == nil {
		return ERC20TokenRemote, nil
	} else {
		return Undefined, err
	}
}

func ERC20TokenHomeGetTokenAddress(
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

func NativeTokenHomeGetTokenAddress(
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

func ERC20TokenRemoteGetTokenHomeAddress(
	rpcURL string,
	address common.Address,
) (common.Address, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		address,
		"tokenHomeAddress()->(address)",
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

func ERC20TokenHomeSend(
	rpcURL string,
	homeAddress common.Address,
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
	tokenAddress, err := ERC20TokenHomeGetTokenAddress(rpcURL, homeAddress)
	if err != nil {
		return err
	}
	if _, _, err := contract.TxToMethod(
		rpcURL,
		privateKey,
		tokenAddress,
		nil,
		"approve(address, uint256)->(bool)",
		homeAddress,
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
	_, _, err = contract.TxToMethod(
		rpcURL,
		privateKey,
		homeAddress,
		nil,
		"send((bytes32, address, address, address, uint256, uint256, uint256, address), uint256)",
		params,
		amount,
	)
	return err
}

func NativeTokenHomeSend(
	rpcURL string,
	homeAddress common.Address,
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
	tokenAddress, err := NativeTokenHomeGetTokenAddress(rpcURL, homeAddress)
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
	_, _, err = contract.TxToMethod(
		rpcURL,
		privateKey,
		homeAddress,
		amount,
		"send((bytes32, address, address, address, uint256, uint256, uint256, address))",
		params,
	)
	return err
}

func ERC20TokenRemoteSend(
	rpcURL string,
	remoteAddress common.Address,
	privateKey string,
	destinationBlockchainID ids.ID,
	destinationBridgeEnd common.Address,
	amountRecipient common.Address,
	amount *big.Int,
) error {
	if _, _, err := contract.TxToMethod(
		rpcURL,
		privateKey,
		remoteAddress,
		nil,
		"approve(address, uint256)->(bool)",
		remoteAddress,
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
	_, _, err := contract.TxToMethod(
		rpcURL,
		privateKey,
		remoteAddress,
		nil,
		"send((bytes32, address, address, address, uint256, uint256, uint256, address), uint256)",
		params,
		amount,
	)
	return err
}
