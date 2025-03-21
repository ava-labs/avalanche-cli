// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ictt

import (
	_ "embed"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ethereum/go-ethereum/common"
)

type EndpointKind int64

const (
	Undefined EndpointKind = iota
	ERC20TokenHome
	NativeTokenHome
	ERC20TokenRemote
	NativeTokenRemote
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
	if _, err := NativeTokenRemoteGetTotalNativeAssetSupply(rpcURL, address); err == nil {
		return NativeTokenRemote, nil
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
	return contract.GetMethodReturn[common.Address]("token", out)
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
	return contract.GetMethodReturn[common.Address]("wrappedToken", out)
}

func TokenRemoteIsCollateralized(
	rpcURL string,
	address common.Address,
) (bool, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		address,
		"isCollateralized()->(bool)",
	)
	if err != nil {
		return false, err
	}
	return contract.GetMethodReturn[bool]("isCollateralized", out)
}

func TokenHomeGetDecimals(
	rpcURL string,
	address common.Address,
) (uint8, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		address,
		"tokenDecimals()->(uint8)",
	)
	if err != nil {
		return 0, err
	}
	return contract.GetMethodReturn[uint8]("tokenDecimals", out)
}

type RegisteredRemote struct {
	Registered       bool
	CollateralNeeded *big.Int
	TokenMultiplier  *big.Int
	MultiplyOnRemote bool
}

func TokenHomeGetRegisteredRemote(
	rpcURL string,
	address common.Address,
	remoteBlockchainID [32]byte,
	remoteAddress common.Address,
) (RegisteredRemote, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		address,
		"registeredRemotes(bytes32, address)->(bool,uint256,uint256,bool)",
		remoteBlockchainID,
		remoteAddress,
	)
	if err != nil {
		return RegisteredRemote{}, err
	}
	var (
		registeredRemote RegisteredRemote
		b                bool
	)
	if len(out) != 4 {
		return RegisteredRemote{}, fmt.Errorf("error at registeredRemotes call, expected 4 return values, got %d", len(out))
	}
	registeredRemote.Registered, b = out[0].(bool)
	if !b {
		return RegisteredRemote{}, fmt.Errorf("error at registeredRemotes call, expected bool, got %T", out[0])
	}
	registeredRemote.CollateralNeeded, b = out[1].(*big.Int)
	if !b {
		return RegisteredRemote{}, fmt.Errorf("error at registeredRemotes call, expected *big.Int, got %T", out[1])
	}
	registeredRemote.TokenMultiplier, b = out[2].(*big.Int)
	if !b {
		return RegisteredRemote{}, fmt.Errorf("error at registeredRemotes call, expected *big.Int, got %T", out[2])
	}
	registeredRemote.MultiplyOnRemote, b = out[3].(bool)
	if !b {
		return RegisteredRemote{}, fmt.Errorf("error at registeredRemotes call, expected bool, got %T", out[3])
	}
	return registeredRemote, nil
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
	return contract.GetMethodReturn[common.Address]("tokenHomeAddress", out)
}

func NativeTokenRemoteGetTotalNativeAssetSupply(
	rpcURL string,
	address common.Address,
) (*big.Int, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		address,
		"totalNativeAssetSupply()->(uint256)",
	)
	if err != nil {
		return nil, err
	}
	return contract.GetMethodReturn[*big.Int]("totalNativeAssetSupply", out)
}

func ERC20TokenHomeSend(
	rpcURL string,
	homeAddress common.Address,
	privateKey string,
	destinationBlockchainID ids.ID,
	destinationICTTEndpoint common.Address,
	amountRecipient common.Address,
	amount *big.Int,
) error {
	type Params struct {
		DestinationBlockchainID [32]byte
		DestinationICTTEndpoint common.Address
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
		false,
		common.Address{},
		privateKey,
		tokenAddress,
		nil,
		"erc20 token approve",
		nil,
		"approve(address, uint256)->(bool)",
		homeAddress,
		amount,
	); err != nil {
		return err
	}
	params := Params{
		DestinationBlockchainID: destinationBlockchainID,
		DestinationICTTEndpoint: destinationICTTEndpoint,
		AmountRecipient:         amountRecipient,
		PrimaryFeeTokenAddress:  tokenAddress, // in theory this is optional
		PrimaryFee:              big.NewInt(0),
		SecondaryFee:            big.NewInt(0),
		RequiredGasLimit:        big.NewInt(250000),
		MultiHopFallback:        common.Address{},
	}
	_, _, err = contract.TxToMethod(
		rpcURL,
		false,
		common.Address{},
		privateKey,
		homeAddress,
		nil,
		"erc20 token home send",
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
	destinationICTTEndpoint common.Address,
	amountRecipient common.Address,
	amount *big.Int,
) error {
	type Params struct {
		DestinationBlockchainID [32]byte
		DestinationICTTEndpoint common.Address
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
		DestinationICTTEndpoint: destinationICTTEndpoint,
		AmountRecipient:         amountRecipient,
		PrimaryFeeTokenAddress:  tokenAddress, // in theory this is optional
		PrimaryFee:              big.NewInt(0),
		SecondaryFee:            big.NewInt(0),
		RequiredGasLimit:        big.NewInt(250000),
		MultiHopFallback:        common.Address{},
	}
	_, _, err = contract.TxToMethod(
		rpcURL,
		false,
		common.Address{},
		privateKey,
		homeAddress,
		amount,
		"native token home send",
		nil,
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
	destinationICTTEndpoint common.Address,
	amountRecipient common.Address,
	amount *big.Int,
) error {
	if _, _, err := contract.TxToMethod(
		rpcURL,
		false,
		common.Address{},
		privateKey,
		remoteAddress,
		nil,
		"erc20 token remote approve",
		nil,
		"approve(address, uint256)->(bool)",
		remoteAddress,
		amount,
	); err != nil {
		return err
	}
	type Params struct {
		DestinationBlockchainID [32]byte
		DestinationICTTEndpoint common.Address
		AmountRecipient         common.Address
		PrimaryFeeTokenAddress  common.Address
		PrimaryFee              *big.Int
		SecondaryFee            *big.Int
		RequiredGasLimit        *big.Int
		MultiHopFallback        common.Address
	}
	params := Params{
		DestinationBlockchainID: destinationBlockchainID,
		DestinationICTTEndpoint: destinationICTTEndpoint,
		AmountRecipient:         amountRecipient,
		PrimaryFeeTokenAddress:  common.Address{},
		PrimaryFee:              big.NewInt(0),
		SecondaryFee:            big.NewInt(0),
		RequiredGasLimit:        big.NewInt(250000),
		MultiHopFallback:        common.Address{},
	}
	_, _, err := contract.TxToMethod(
		rpcURL,
		false,
		common.Address{},
		privateKey,
		remoteAddress,
		nil,
		"erc20 token remote send",
		nil,
		"send((bytes32, address, address, address, uint256, uint256, uint256, address), uint256)",
		params,
		amount,
	)
	return err
}

func NativeTokenRemoteSend(
	rpcURL string,
	remoteAddress common.Address,
	privateKey string,
	destinationBlockchainID ids.ID,
	destinationICTTEndpoint common.Address,
	amountRecipient common.Address,
	amount *big.Int,
) error {
	type Params struct {
		DestinationBlockchainID [32]byte
		DestinationICTTEndpoint common.Address
		AmountRecipient         common.Address
		PrimaryFeeTokenAddress  common.Address
		PrimaryFee              *big.Int
		SecondaryFee            *big.Int
		RequiredGasLimit        *big.Int
		MultiHopFallback        common.Address
	}
	params := Params{
		DestinationBlockchainID: destinationBlockchainID,
		DestinationICTTEndpoint: destinationICTTEndpoint,
		AmountRecipient:         amountRecipient,
		PrimaryFeeTokenAddress:  remoteAddress, // in theory this is optional
		PrimaryFee:              big.NewInt(0),
		SecondaryFee:            big.NewInt(0),
		RequiredGasLimit:        big.NewInt(250000),
		MultiHopFallback:        common.Address{},
	}
	_, _, err := contract.TxToMethod(
		rpcURL,
		false,
		common.Address{},
		privateKey,
		remoteAddress,
		amount,
		"native token remote send",
		nil,
		"send((bytes32, address, address, address, uint256, uint256, uint256, address))",
		params,
	)
	return err
}

func NativeTokenHomeAddCollateral(
	rpcURL string,
	homeAddress common.Address,
	privateKey string,
	remoteBlockchainID [32]byte,
	remoteAddress common.Address,
	amount *big.Int,
) error {
	_, _, err := contract.TxToMethod(
		rpcURL,
		false,
		common.Address{},
		privateKey,
		homeAddress,
		amount,
		"native token home add collateral",
		nil,
		"addCollateral(bytes32, address)",
		remoteBlockchainID,
		remoteAddress,
	)
	return err
}

func ERC20TokenHomeAddCollateral(
	rpcURL string,
	homeAddress common.Address,
	privateKey string,
	remoteBlockchainID [32]byte,
	remoteAddress common.Address,
	amount *big.Int,
) error {
	tokenAddress, err := ERC20TokenHomeGetTokenAddress(rpcURL, homeAddress)
	if err != nil {
		return err
	}
	if _, _, err := contract.TxToMethod(
		rpcURL,
		false,
		common.Address{},
		privateKey,
		tokenAddress,
		nil,
		"erc20 token home approve",
		nil,
		"approve(address, uint256)->(bool)",
		homeAddress,
		amount,
	); err != nil {
		return err
	}
	_, _, err = contract.TxToMethod(
		rpcURL,
		false,
		common.Address{},
		privateKey,
		homeAddress,
		nil,
		"erc20 token home add collateral",
		nil,
		"addCollateral(bytes32, address, uint256)",
		remoteBlockchainID,
		remoteAddress,
		amount,
	)
	return err
}

func TokenHomeAddCollateral(
	rpcURL string,
	homeAddress common.Address,
	privateKey string,
	remoteBlockchainID [32]byte,
	remoteAddress common.Address,
	amount *big.Int,
) error {
	ux.Logger.PrintToUser("Collateralizing remote contract on the home chain")
	endpointKind, err := GetEndpointKind(
		rpcURL,
		homeAddress,
	)
	if err != nil {
		return err
	}
	switch endpointKind {
	case ERC20TokenHome:
		return ERC20TokenHomeAddCollateral(
			rpcURL,
			homeAddress,
			privateKey,
			remoteBlockchainID,
			remoteAddress,
			amount,
		)
	case NativeTokenHome:
		return NativeTokenHomeAddCollateral(
			rpcURL,
			homeAddress,
			privateKey,
			remoteBlockchainID,
			remoteAddress,
			amount,
		)
	case ERC20TokenRemote:
		return fmt.Errorf("trying to add collateral to an erc20 token remote endpoint")
	case NativeTokenRemote:
		return fmt.Errorf("trying to add collateral to a native token remote endpoint")
	}
	return fmt.Errorf("unknown ictt endpoint")
}

func Send(
	rpcURL string,
	address common.Address,
	privateKey string,
	destinationBlockchainID ids.ID,
	destinationAddress common.Address,
	amountRecipient common.Address,
	amount *big.Int,
) error {
	endpointKind, err := GetEndpointKind(
		rpcURL,
		address,
	)
	if err != nil {
		return err
	}
	switch endpointKind {
	case ERC20TokenRemote:
		return ERC20TokenRemoteSend(
			rpcURL,
			address,
			privateKey,
			destinationBlockchainID,
			destinationAddress,
			amountRecipient,
			amount,
		)
	case ERC20TokenHome:
		return ERC20TokenHomeSend(
			rpcURL,
			address,
			privateKey,
			destinationBlockchainID,
			destinationAddress,
			amountRecipient,
			amount,
		)
	case NativeTokenHome:
		return NativeTokenHomeSend(
			rpcURL,
			address,
			privateKey,
			destinationBlockchainID,
			destinationAddress,
			amountRecipient,
			amount,
		)
	case NativeTokenRemote:
		return NativeTokenRemoteSend(
			rpcURL,
			address,
			privateKey,
			destinationBlockchainID,
			destinationAddress,
			amountRecipient,
			amount,
		)
	}
	return fmt.Errorf("unknown ictt endpoint")
}
