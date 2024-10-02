// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"fmt"
	"math/big"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/sdk/interchain"
	"github.com/ava-labs/avalanchego/ids"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	warp "github.com/ava-labs/avalanchego/vms/platformvm/warp"
	warpMessage "github.com/ava-labs/avalanchego/vms/platformvm/warp/message"
	warpPayload "github.com/ava-labs/avalanchego/vms/platformvm/warp/payload"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
)

const (
	ValidatorContractAddress = "0x5F584C2D56B4c356e7d82EC6129349393dc5df17"
)

//go:embed deployed_poa_validator_manager_bytecode.txt
var deployedPoAValidatorManagerBytecode []byte

func AddPoAValidatorManagerContractToAllocations(
	allocs core.GenesisAlloc,
) {
	deployedPoaValidatorManagerBytes := common.FromHex(strings.TrimSpace(string(deployedPoAValidatorManagerBytecode)))
	allocs[common.HexToAddress(ValidatorContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedPoaValidatorManagerBytes,
		Nonce:   1,
	}
}

// initializes contract [managerAddress] at [rpcURL], to
// manage validators on [subnetID], with
// owner given by [ownerAddress]
func PoAValidatorManagerInitialize(
	rpcURL string,
	managerAddress common.Address,
	privateKey string,
	subnetID ids.ID,
	ownerAddress common.Address,
) (*types.Transaction, *types.Receipt, error) {
	const (
		defaultChurnPeriodSeconds     = uint64(0)
		defaultMaximumChurnPercentage = uint8(20)
	)
	type Params struct {
		SubnetID               [32]byte
		ChurnPeriodSeconds     uint64
		MaximumChurnPercentage uint8
	}
	params := Params{
		SubnetID:               subnetID,
		ChurnPeriodSeconds:     defaultChurnPeriodSeconds,
		MaximumChurnPercentage: defaultMaximumChurnPercentage,
	}
	return contract.TxToMethod(
		rpcURL,
		privateKey,
		managerAddress,
		nil,
		"initialize((bytes32,uint64,uint8),address)",
		params,
		ownerAddress,
	)
}

// constructs p-chain-validated (signed) subnet conversion warp
// message, to be sent to the validators manager when
// initializing validators set
// the message specifies [subnetID] that is being converted
// together with the validator's manager [managerBlockchainID],
// [managerAddress], and the initial list of [validators]
func PoaValidatorManagerGetPChainSubnetConversionWarpMessage(
	network models.Network,
	aggregatorLogger logging.Logger,
	aggregatorLogLevel logging.Level,
	aggregatorQuorumPercentage uint64,
	subnetID ids.ID,
	managerBlockchainID ids.ID,
	managerAddress common.Address,
	validators []warpMessage.SubnetConversionValidatorData,
) (*warp.Message, error) {
	subnetConversionData := warpMessage.SubnetConversionData{
		SubnetID:       subnetID,
		ManagerChainID: managerBlockchainID,
		ManagerAddress: managerAddress.Bytes(),
		Validators:     validators,
	}
	subnetConversionID, err := warpMessage.SubnetConversionID(subnetConversionData)
	if err != nil {
		return nil, err
	}
	addressedCallPayload, err := warpMessage.NewSubnetConversion(subnetConversionID)
	if err != nil {
		return nil, err
	}
	subnetConversionAddressedCall, err := warpPayload.NewAddressedCall(
		nil,
		addressedCallPayload.Bytes(),
	)
	if err != nil {
		return nil, err
	}
	subnetConversionUnsignedMessage, err := warp.NewUnsignedMessage(
		network.ID,
		avagoconstants.PlatformChainID, // p-chain sign
		subnetConversionAddressedCall.Bytes(),
	)
	if err != nil {
		return nil, err
	}
	signatureAggregator, err := interchain.NewSignatureAggregator(
		network,
		aggregatorLogger,
		aggregatorLogLevel,
		ids.Empty, // primary network validators sign
		aggregatorQuorumPercentage,
	)
	if err != nil {
		return nil, err
	}
	return signatureAggregator.Sign(subnetConversionUnsignedMessage, nil)
}

func PoAValidatorManagerInitializeValidatorsSet(
	rpcURL string,
	managerAddress common.Address,
	privateKey string,
	subnetID ids.ID,
	managerBlockchainID ids.ID,
	// TODO replace with validators struct from ConvertSubnetTx
	validators []warpMessage.SubnetConversionValidatorData,
	subnetConversionSignedMessage *warp.Message,
) (*types.Transaction, *types.Receipt, error) {
	type InitialValidator struct {
		NodeID       []byte
		BlsPublicKey []byte
		Weight       uint64
	}
	type SubnetConversionData struct {
		SubnetID                     [32]byte
		ValidatorManagerBlockchainID [32]byte
		ValidatorManagerAddress      common.Address
		InitialValidators            []InitialValidator
	}
	initialValidators := []InitialValidator{}
	for _, validator := range validators {
		initialValidators = append(initialValidators, InitialValidator{
			NodeID:       validator.NodeID[:],
			BlsPublicKey: validator.BLSPublicKey[:],
			Weight:       validator.Weight,
		})
	}
	subnetConversionData := SubnetConversionData{
		SubnetID:                     subnetID,
		ValidatorManagerBlockchainID: managerBlockchainID,
		ValidatorManagerAddress:      managerAddress,
		InitialValidators:            initialValidators,
	}
	return contract.TxToMethodWithWarpMessage(
		rpcURL,
		privateKey,
		managerAddress,
		subnetConversionSignedMessage,
		"initializeValidatorSet((bytes32,bytes32,address,[(bytes,bytes,uint64)]),uint32)",
		subnetConversionData,
		uint32(0),
	)
}

func SetupPoA(
	app *application.Avalanche,
	network models.Network,
	chainSpec contract.ChainSpec,
	privateKey string,
	ownerAddress common.Address,
	validators []warpMessage.SubnetConversionValidatorData,
) error {
	rpcURL, _, err := contract.GetBlockchainEndpoints(
		app,
		network,
		chainSpec,
		true,
		false,
	)
	if err != nil {
		return err
	}
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return err
	}
	blockchainID, err := contract.GetBlockchainID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return err
	}
	managerAddress := common.HexToAddress(ValidatorContractAddress)
	tx, _, err := PoAValidatorManagerInitialize(
		rpcURL,
		managerAddress,
		privateKey,
		subnetID,
		ownerAddress,
	)
	if err != nil {
		txHashMsg := ""
		if tx != nil {
			txHashMsg = fmt.Sprintf(" (txHash=%s)", tx.Hash().String())
		}
		return fmt.Errorf("failure initializing poa validator manager: %w%s", err, txHashMsg)
	}
	subnetConversionSignedMessage, err := PoaValidatorManagerGetPChainSubnetConversionWarpMessage(
		network,
		app.Log,
		logging.Info,
		0,
		subnetID,
		blockchainID,
		managerAddress,
		validators,
	)
	if err != nil {
		return fmt.Errorf("failure signing subnet conversion warp message: %w", err)
	}
	tx, _, err = PoAValidatorManagerInitializeValidatorsSet(
		rpcURL,
		managerAddress,
		privateKey,
		subnetID,
		blockchainID,
		validators,
		subnetConversionSignedMessage,
	)
	if err != nil {
		txHashMsg := ""
		if tx != nil {
			txHashMsg = fmt.Sprintf(" (txHash=%s)", tx.Hash().String())
		}
		return fmt.Errorf("failure initializing validators set on poa manager: %w%s", err, txHashMsg)
	}
	return nil
}
