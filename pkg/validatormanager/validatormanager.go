// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/sdk/interchain"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
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

var errAlreadyInitialized = errors.New("the contract is already initialized")

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
	tx, receipt, err := contract.TxToMethod(
		rpcURL,
		privateKey,
		managerAddress,
		nil,
		"initialize((bytes32,uint64,uint8),address)",
		params,
		ownerAddress,
	)
	if err != nil {
		trace, traceCallErr := contract.DebugTraceCall(
			rpcURL,
			privateKey,
			managerAddress,
			nil,
			"initialize((bytes32,uint64,uint8),address)",
			params,
			ownerAddress,
		)
		if traceCallErr != nil {
			ux.Logger.PrintToUser("Could not get debug trace for PoA initialization error on %s: %s", rpcURL, traceCallErr)
			ux.Logger.PrintToUser("Verify --debug flag value when calling 'blockchain create'")
			return tx, receipt, err
		}
		errorSignatureToError := map[string]error{
			"InvalidInitialization()": errAlreadyInitialized,
		}
		errorFromSignature, _ := evm.GetErrorFromTrace(trace, errorSignatureToError)
		if errorFromSignature != nil {
			return tx, receipt, errorFromSignature
		} else {
			ux.Logger.PrintToUser("error trace for PoA initialization error:")
			ux.Logger.PrintToUser("%#v", trace)
		}
	}
	return tx, receipt, err
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
	aggregatorExtraPeerEndpoints []info.Peer,
	subnetID ids.ID,
	managerBlockchainID ids.ID,
	managerAddress common.Address,
	convertSubnetValidators []*txs.ConvertSubnetValidator,
) (*warp.Message, error) {
	validators := []warpMessage.SubnetConversionValidatorData{}
	for _, convertSubnetValidator := range convertSubnetValidators {
		validators = append(validators, warpMessage.SubnetConversionValidatorData{
			NodeID:       convertSubnetValidator.NodeID[:],
			BLSPublicKey: convertSubnetValidator.Signer.PublicKey,
			Weight:       convertSubnetValidator.Weight,
		})
	}
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
		avagoconstants.PlatformChainID,
		subnetConversionAddressedCall.Bytes(),
	)
	if err != nil {
		return nil, err
	}
	signatureAggregator, err := interchain.NewSignatureAggregator(
		network,
		aggregatorLogger,
		aggregatorLogLevel,
		subnetID,
		aggregatorQuorumPercentage,
		aggregatorExtraPeerEndpoints,
	)
	if err != nil {
		return nil, err
	}
	return signatureAggregator.Sign(subnetConversionUnsignedMessage, subnetID[:])
}

// calls poa manager validators set init method,
// passing to it the p-chain signed [subnetConversionSignedMessage]
// so as to verify p-chain already proceesed the associated
// ConvertSubnetTx
func PoAValidatorManagerInitializeValidatorsSet(
	rpcURL string,
	managerAddress common.Address,
	privateKey string,
	subnetID ids.ID,
	managerBlockchainID ids.ID,
	convertSubnetValidators []*txs.ConvertSubnetValidator,
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
	validators := []InitialValidator{}
	for _, convertSubnetValidator := range convertSubnetValidators {
		validators = append(validators, InitialValidator{
			NodeID:       convertSubnetValidator.NodeID[:],
			BlsPublicKey: convertSubnetValidator.Signer.PublicKey[:],
			Weight:       convertSubnetValidator.Weight,
		})
	}
	subnetConversionData := SubnetConversionData{
		SubnetID:                     subnetID,
		ValidatorManagerBlockchainID: managerBlockchainID,
		ValidatorManagerAddress:      managerAddress,
		InitialValidators:            validators,
	}
	return contract.TxToMethodWithWarpMessage(
		rpcURL,
		privateKey,
		managerAddress,
		subnetConversionSignedMessage,
		big.NewInt(0),
		"initializeValidatorSet((bytes32,bytes32,address,[(bytes,bytes,uint64)]),uint32)",
		subnetConversionData,
		uint32(0),
	)
}

// setups PoA manager after a successful execution of
// ConvertSubnetTx on P-Chain
// needs the list of validators for that tx,
// [convertSubnetValidators], together with an evm [ownerAddress]
// to set as the owner of the PoA manager
func SetupPoA(
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	chainSpec contract.ChainSpec,
	privateKey string,
	ownerAddress common.Address,
	convertSubnetValidators []*txs.ConvertSubnetValidator,
	aggregatorExtraPeerEndpoints []info.Peer,
) error {
	if err := evm.SetupProposerVM(
		rpcURL,
		privateKey,
	); err != nil {
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
		if !errors.Is(err, errAlreadyInitialized) {
			return evm.TransactionError(tx, err, "failure initializing poa validator manager")
		}
		ux.Logger.PrintToUser("Warning: the PoA contract is already initialized.")
	}
	subnetConversionSignedMessage, err := PoaValidatorManagerGetPChainSubnetConversionWarpMessage(
		network,
		app.Log,
		logging.Trace,
		0,
		aggregatorExtraPeerEndpoints,
		subnetID,
		blockchainID,
		managerAddress,
		convertSubnetValidators,
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
		convertSubnetValidators,
		subnetConversionSignedMessage,
	)
	if err != nil {
		return evm.TransactionError(tx, err, "failure initializing validators set on poa manager")
	}
	return nil
}

// step 1 of flow for adding a new validator
func PoAValidatorManagerInitializeValidatorRegistration(
	rpcURL string,
	managerAddress common.Address,
	managerOwnerPrivateKey string,
	nodeID ids.NodeID,
	blsPublicKey []byte,
	expiry uint64,
	balanceOwners warpMessage.PChainOwner,
	disableOwners warpMessage.PChainOwner,
	weight uint64,
) (*types.Transaction, *types.Receipt, error) {
	type PChainOwner struct {
		Threshold uint32
		Addresses []common.Address
	}
	type ValidatorRegistrationInput struct {
		NodeID                []byte
		BlsPublicKey          []byte
		RegistrationExpiry    uint64
		RemainingBalanceOwner PChainOwner
		DisableOwner          PChainOwner
	}
	balanceOwnersAux := PChainOwner{
		Threshold: balanceOwners.Threshold,
		Addresses: utils.Map(balanceOwners.Addresses, func(addr ids.ShortID) common.Address {
			return common.BytesToAddress(addr[:])
		}),
	}
	disableOwnersAux := PChainOwner{
		Threshold: disableOwners.Threshold,
		Addresses: utils.Map(disableOwners.Addresses, func(addr ids.ShortID) common.Address {
			return common.BytesToAddress(addr[:])
		}),
	}
	validatorRegistrationInput := ValidatorRegistrationInput{
		NodeID:                nodeID[:],
		BlsPublicKey:          blsPublicKey,
		RegistrationExpiry:    expiry,
		RemainingBalanceOwner: balanceOwnersAux,
		DisableOwner:          disableOwnersAux,
	}
	return contract.TxToMethod(
		rpcURL,
		managerOwnerPrivateKey,
		managerAddress,
		big.NewInt(0),
		"initializeValidatorRegistration((bytes,bytes,uint64,(uint32,[address]),(uint32,[address])),uint64)",
		validatorRegistrationInput,
		weight,
	)
}

func PoaValidatorManagerGetSubnetValidatorRegistrationMessage(
	network models.Network,
	aggregatorLogger logging.Logger,
	aggregatorLogLevel logging.Level,
	aggregatorQuorumPercentage uint64,
	aggregatorExtraPeerEndpoints []info.Peer,
	subnetID ids.ID,
	blockchainID ids.ID,
	managerAddress common.Address,
	nodeID ids.NodeID,
	blsPublicKey [48]byte,
	expiry uint64,
	balanceOwners warpMessage.PChainOwner,
	disableOwners warpMessage.PChainOwner,
	weight uint64,
) (*warp.Message, ids.ID, error) {
	addressedCallPayload, err := warpMessage.NewRegisterSubnetValidator(
		subnetID,
		nodeID,
		blsPublicKey,
		expiry,
		balanceOwners,
		disableOwners,
		weight,
	)
	if err != nil {
		return nil, ids.Empty, err
	}
	validationID := addressedCallPayload.ValidationID()
	registerSubnetValidatorAddressedCall, err := warpPayload.NewAddressedCall(
		managerAddress.Bytes(),
		addressedCallPayload.Bytes(),
	)
	if err != nil {
		return nil, ids.Empty, err
	}
	registerSubnetValidatorUnsignedMessage, err := warp.NewUnsignedMessage(
		network.ID,
		blockchainID,
		registerSubnetValidatorAddressedCall.Bytes(),
	)
	if err != nil {
		return nil, ids.Empty, err
	}
	signatureAggregator, err := interchain.NewSignatureAggregator(
		network,
		aggregatorLogger,
		aggregatorLogLevel,
		subnetID,
		aggregatorQuorumPercentage,
		aggregatorExtraPeerEndpoints,
	)
	if err != nil {
		return nil, ids.Empty, err
	}
	signedMessage, err := signatureAggregator.Sign(registerSubnetValidatorUnsignedMessage, nil)
	return signedMessage, validationID, err
}

func GetRegisteredValidator(
	rpcURL string,
	managerAddress common.Address,
	nodeID ids.NodeID,
) (ids.ID, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		managerAddress,
		"registeredValidators(bytes)->(bytes32)",
		nodeID[:],
	)
	if err != nil {
		return ids.Empty, err
	}
	validatorID, b := out[0].([32]byte)
	if !b {
		return ids.Empty, fmt.Errorf("error at registeredValidators call, expected [32]byte, got %T", out[0])
	}
	return validatorID, nil
}

func PoaValidatorManagerGetPChainSubnetValidatorRegistrationnWarpMessage(
	network models.Network,
	aggregatorLogger logging.Logger,
	aggregatorLogLevel logging.Level,
	aggregatorQuorumPercentage uint64,
	aggregatorExtraPeerEndpoints []info.Peer,
	subnetID ids.ID,
	validationID ids.ID,
	registered bool,
) (*warp.Message, error) {
	addressedCallPayload, err := warpMessage.NewSubnetValidatorRegistration(validationID, registered)
	if err != nil {
		return nil, err
	}
	subnetValidatorRegistrationAddressedCall, err := warpPayload.NewAddressedCall(
		nil,
		addressedCallPayload.Bytes(),
	)
	if err != nil {
		return nil, err
	}
	subnetConversionUnsignedMessage, err := warp.NewUnsignedMessage(
		network.ID,
		avagoconstants.PlatformChainID,
		subnetValidatorRegistrationAddressedCall.Bytes(),
	)
	if err != nil {
		return nil, err
	}
	signatureAggregator, err := interchain.NewSignatureAggregator(
		network,
		aggregatorLogger,
		aggregatorLogLevel,
		subnetID,
		aggregatorQuorumPercentage,
		aggregatorExtraPeerEndpoints,
	)
	if err != nil {
		return nil, err
	}
	return signatureAggregator.Sign(subnetConversionUnsignedMessage, subnetID[:])
}

// last step of flow for adding a new validator
func PoAValidatorManagerCompleteValidatorRegistration(
	rpcURL string,
	managerAddress common.Address,
	privateKey string, // not need to be owner atm
	subnetValidatorRegistrationSignedMessage *warp.Message,
) (*types.Transaction, *types.Receipt, error) {
	return contract.TxToMethodWithWarpMessage(
		rpcURL,
		privateKey,
		managerAddress,
		subnetValidatorRegistrationSignedMessage,
		big.NewInt(0),
		"completeValidatorRegistration(uint32)",
		uint32(0),
	)
}

func InitValidatorRegistration(
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	chainSpec contract.ChainSpec,
	ownerPrivateKey string,
	nodeID ids.NodeID,
	blsPublicKey []byte,
	expiry uint64,
	balanceOwners warpMessage.PChainOwner,
	disableOwners warpMessage.PChainOwner,
	weight uint64,
	aggregatorExtraPeerEndpoints []info.Peer,
) (*warp.Message, ids.ID, error) {
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return nil, ids.Empty, err
	}
	blockchainID, err := contract.GetBlockchainID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return nil, ids.Empty, err
	}
	managerAddress := common.HexToAddress(ValidatorContractAddress)
	tx, _, err := PoAValidatorManagerInitializeValidatorRegistration(
		rpcURL,
		managerAddress,
		ownerPrivateKey,
		nodeID,
		blsPublicKey,
		expiry,
		balanceOwners,
		disableOwners,
		weight,
	)
	if err != nil {
		return nil, ids.Empty, evm.TransactionError(tx, err, "failure initializing validator registration")
	}
	return PoaValidatorManagerGetSubnetValidatorRegistrationMessage(
		network,
		app.Log,
		logging.Info,
		0,
		aggregatorExtraPeerEndpoints,
		subnetID,
		blockchainID,
		managerAddress,
		nodeID,
		[48]byte(blsPublicKey),
		expiry,
		balanceOwners,
		disableOwners,
		weight,
	)
}

func FinishValidatorRegistration(
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	chainSpec contract.ChainSpec,
	privateKey string,
	validationID ids.ID,
	aggregatorExtraPeerEndpoints []info.Peer,
) error {
	managerAddress := common.HexToAddress(ValidatorContractAddress)
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return err
	}
	signedMessage, err := PoaValidatorManagerGetPChainSubnetValidatorRegistrationnWarpMessage(
		network,
		app.Log,
		logging.Info,
		0,
		aggregatorExtraPeerEndpoints,
		subnetID,
		validationID,
		true,
	)
	if err != nil {
		return err
	}
	if err := evm.SetupProposerVM(
		rpcURL,
		privateKey,
	); err != nil {
		return err
	}
	tx, _, err := PoAValidatorManagerCompleteValidatorRegistration(
		rpcURL,
		managerAddress,
		privateKey,
		signedMessage,
	)
	if err != nil {
		return evm.TransactionError(tx, err, "failure completing validator registration")
	}
	return nil
}

func PoAValidatorManagerInitializeValidatorRemoval(
	rpcURL string,
	managerAddress common.Address,
	managerOwnerPrivateKey string,
	validationID [32]byte,
) (*types.Transaction, *types.Receipt, error) {
	return contract.TxToMethod(
		rpcURL,
		managerOwnerPrivateKey,
		managerAddress,
		big.NewInt(0),
		"initializeEndValidation(bytes32)",
		validationID,
	)
}

func PoaValidatorManagerGetSubnetValidatorWeightMessage(
	network models.Network,
	aggregatorLogger logging.Logger,
	aggregatorLogLevel logging.Level,
	aggregatorQuorumPercentage uint64,
	aggregatorExtraPeerEndpoints []info.Peer,
	subnetID ids.ID,
	blockchainID ids.ID,
	managerAddress common.Address,
	validationID ids.ID,
	nonce uint64,
	weight uint64,
) (*warp.Message, error) {
	addressedCallPayload, err := warpMessage.NewSubnetValidatorWeight(
		validationID,
		nonce,
		weight,
	)
	if err != nil {
		return nil, err
	}
	addressedCall, err := warpPayload.NewAddressedCall(
		managerAddress.Bytes(),
		addressedCallPayload.Bytes(),
	)
	if err != nil {
		return nil, err
	}
	unsignedMessage, err := warp.NewUnsignedMessage(
		network.ID,
		blockchainID,
		addressedCall.Bytes(),
	)
	if err != nil {
		return nil, err
	}
	signatureAggregator, err := interchain.NewSignatureAggregator(
		network,
		aggregatorLogger,
		aggregatorLogLevel,
		subnetID,
		aggregatorQuorumPercentage,
		aggregatorExtraPeerEndpoints,
	)
	if err != nil {
		return nil, err
	}
	return signatureAggregator.Sign(unsignedMessage, nil)
}

func InitValidatorRemoval(
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	chainSpec contract.ChainSpec,
	ownerPrivateKey string,
	nodeID ids.NodeID,
	aggregatorExtraPeerEndpoints []info.Peer,
) (*warp.Message, error) {
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return nil, err
	}
	blockchainID, err := contract.GetBlockchainID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return nil, err
	}
	managerAddress := common.HexToAddress(ValidatorContractAddress)
	validationID, err := GetRegisteredValidator(
		rpcURL,
		managerAddress,
		nodeID,
	)
	if err != nil {
		return nil, err
	}
	tx, _, err := PoAValidatorManagerInitializeValidatorRemoval(
		rpcURL,
		managerAddress,
		ownerPrivateKey,
		validationID,
	)
	if err != nil {
		return nil, evm.TransactionError(tx, err, "failure initializing validator removal")
	}
	nonce := uint64(1)
	return PoaValidatorManagerGetSubnetValidatorWeightMessage(
		network,
		app.Log,
		logging.Info,
		0,
		aggregatorExtraPeerEndpoints,
		subnetID,
		blockchainID,
		managerAddress,
		validationID,
		nonce,
		0,
	)
}

func PoAValidatorManagerCompleteValidatorRemoval(
	rpcURL string,
	managerAddress common.Address,
	privateKey string, // not need to be owner atm
	subnetValidatorRegistrationSignedMessage *warp.Message,
) (*types.Transaction, *types.Receipt, error) {
	return contract.TxToMethodWithWarpMessage(
		rpcURL,
		privateKey,
		managerAddress,
		subnetValidatorRegistrationSignedMessage,
		big.NewInt(0),
		"completeEndValidation(uint32)",
		uint32(0),
	)
}

func FinishValidatorRemoval(
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	chainSpec contract.ChainSpec,
	privateKey string,
	validationID ids.ID,
	aggregatorExtraPeerEndpoints []info.Peer,
) error {
	managerAddress := common.HexToAddress(ValidatorContractAddress)
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return err
	}
	signedMessage, err := PoaValidatorManagerGetPChainSubnetValidatorRegistrationnWarpMessage(
		network,
		app.Log,
		logging.Info,
		0,
		aggregatorExtraPeerEndpoints,
		subnetID,
		validationID,
		false,
	)
	if err != nil {
		return err
	}
	if err := evm.SetupProposerVM(
		rpcURL,
		privateKey,
	); err != nil {
		return err
	}
	tx, _, err := PoAValidatorManagerCompleteValidatorRemoval(
		rpcURL,
		managerAddress,
		privateKey,
		signedMessage,
	)
	if err != nil {
		return evm.TransactionError(tx, err, "failure completing validator removal")
	}
	return nil
}
