// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ictt

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/sdk/evm/contract"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core/types"

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
		nil,
	)
	if err != nil {
		return common.Address{}, err
	}
	return contract.GetSmartContractCallResult[common.Address]("token", out)
}

func NativeTokenHomeGetTokenAddress(
	rpcURL string,
	address common.Address,
) (common.Address, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		address,
		"wrappedToken()->(address)",
		nil,
	)
	if err != nil {
		return common.Address{}, err
	}
	return contract.GetSmartContractCallResult[common.Address]("wrappedToken", out)
}

func TokenRemoteIsCollateralized(
	rpcURL string,
	address common.Address,
) (bool, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		address,
		"isCollateralized()->(bool)",
		nil,
	)
	if err != nil {
		return false, err
	}
	return contract.GetSmartContractCallResult[bool]("isCollateralized", out)
}

func TokenHomeGetDecimals(
	rpcURL string,
	address common.Address,
) (uint8, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		address,
		"tokenDecimals()->(uint8)",
		nil,
	)
	if err != nil {
		return 0, err
	}
	return contract.GetSmartContractCallResult[uint8]("tokenDecimals", out)
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
		nil,
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
		nil,
	)
	if err != nil {
		return common.Address{}, err
	}
	return contract.GetSmartContractCallResult[common.Address]("tokenHomeAddress", out)
}

func NativeTokenRemoteGetTotalNativeAssetSupply(
	rpcURL string,
	address common.Address,
) (*big.Int, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		address,
		"totalNativeAssetSupply()->(uint256)",
		nil,
	)
	if err != nil {
		return nil, err
	}
	return contract.GetSmartContractCallResult[*big.Int]("totalNativeAssetSupply", out)
}

func ERC20TokenHomeSend(
	logger logging.Logger,
	rpcURL string,
	homeAddress common.Address,
	privateKey string,
	destinationBlockchainID ids.ID,
	destinationICTTEndpoint common.Address,
	amountRecipient common.Address,
	amount *big.Int,
) (*types.Receipt, *types.Receipt, error) {
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
		return nil, nil, err
	}
	_, receipt, err := contract.TxToMethod(
		logger,
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
	)
	if err != nil {
		return nil, nil, err
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
	_, receipt2, err := contract.TxToMethod(
		logger,
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
	if err != nil {
		return nil, nil, err
	}

	return receipt, receipt2, nil
}

func NativeTokenHomeSend(
	logger logging.Logger,
	rpcURL string,
	homeAddress common.Address,
	privateKey string,
	destinationBlockchainID ids.ID,
	destinationICTTEndpoint common.Address,
	amountRecipient common.Address,
	amount *big.Int,
) (*types.Receipt, error) {
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
		return nil, err
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
	_, receipt, err := contract.TxToMethod(
		logger,
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
	return receipt, err
}

func ERC20TokenRemoteSend(
	logger logging.Logger,
	rpcURL string,
	remoteAddress common.Address,
	privateKey string,
	destinationBlockchainID ids.ID,
	destinationICTTEndpoint common.Address,
	amountRecipient common.Address,
	amount *big.Int,
) (*types.Receipt, *types.Receipt, error) {
	_, receipt, err := contract.TxToMethod(
		logger,
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
	)
	if err != nil {
		return nil, nil, err
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
	_, receipt2, err := contract.TxToMethod(
		logger,
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
	if err != nil {
		return nil, nil, err
	}
	return receipt, receipt2, err
}

func NativeTokenRemoteSend(
	logger logging.Logger,
	rpcURL string,
	remoteAddress common.Address,
	privateKey string,
	destinationBlockchainID ids.ID,
	destinationICTTEndpoint common.Address,
	amountRecipient common.Address,
	amount *big.Int,
) (*types.Receipt, error) {
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
	_, receipt, err := contract.TxToMethod(
		logger,
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
	return receipt, err
}

func NativeTokenHomeAddCollateral(
	logger logging.Logger,
	rpcURL string,
	homeAddress common.Address,
	privateKey string,
	remoteBlockchainID [32]byte,
	remoteAddress common.Address,
	amount *big.Int,
) error {
	_, _, err := contract.TxToMethod(
		logger,
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
	logger logging.Logger,
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
		logger,
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
	// TODO: use the same API node connection for this two operations
	time.Sleep(5 * time.Second)
	_, _, err = contract.TxToMethod(
		logger,
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
	logger logging.Logger,
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
			logger,
			rpcURL,
			homeAddress,
			privateKey,
			remoteBlockchainID,
			remoteAddress,
			amount,
		)
	case NativeTokenHome:
		return NativeTokenHomeAddCollateral(
			logger,
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
	logger logging.Logger,
	rpcURL string,
	address common.Address,
	privateKey string,
	destinationBlockchainID ids.ID,
	destinationAddress common.Address,
	amountRecipient common.Address,
	amount *big.Int,
) (*types.Receipt, *types.Receipt, error) {
	endpointKind, err := GetEndpointKind(
		rpcURL,
		address,
	)
	if err != nil {
		return nil, nil, err
	}
	switch endpointKind {
	case ERC20TokenRemote:
		return ERC20TokenRemoteSend(
			logger,
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
			logger,
			rpcURL,
			address,
			privateKey,
			destinationBlockchainID,
			destinationAddress,
			amountRecipient,
			amount,
		)
	case NativeTokenHome:
		receipt, err := NativeTokenHomeSend(
			logger,
			rpcURL,
			address,
			privateKey,
			destinationBlockchainID,
			destinationAddress,
			amountRecipient,
			amount,
		)
		return receipt, nil, err
	case NativeTokenRemote:
		receipt, err := NativeTokenRemoteSend(
			logger,
			rpcURL,
			address,
			privateKey,
			destinationBlockchainID,
			destinationAddress,
			amountRecipient,
			amount,
		)
		return receipt, nil, err
	}
	return nil, nil, fmt.Errorf("unknown ictt endpoint")
}
