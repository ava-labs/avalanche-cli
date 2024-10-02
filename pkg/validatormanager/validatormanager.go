// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"math/big"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/sdk/interchain"
	"github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"
	warp "github.com/ava-labs/avalanchego/vms/platformvm/warp"
	warpMessage "github.com/ava-labs/avalanchego/vms/platformvm/warp/message"
	warpPayload "github.com/ava-labs/avalanchego/vms/platformvm/warp/payload"
	"github.com/ava-labs/subnet-evm/core"
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

func InitializePoAValidatorManager(
	rpcURL string,
	remoteAddress common.Address,
	privateKey string,
	subnetID ids.ID,
	ownerAddress common.Address,
) error {
	type Params struct {
		SubnetID               [32]byte
		ChurnPeriodSeconds     uint64
		MaximumChurnPercentage uint8
	}
	churnPeriodSeconds := uint64(0)
	maximumChurnPercentage := uint8(20)
	params := Params{
		SubnetID:               subnetID,
		ChurnPeriodSeconds:     churnPeriodSeconds,
		MaximumChurnPercentage: maximumChurnPercentage,
	}
	_, _, err := contract.TxToMethod(
		rpcURL,
		privateKey,
		remoteAddress,
		nil,
		"initialize((bytes32,uint64,uint8),address)",
		params,
		ownerAddress,
	)
	return err
}

func SetupPoA(
	app *application.Avalanche,
	network models.Network,
	blockchainName string,
) error {
	chainSpec := contract.ChainSpec{
		BlockchainName: blockchainName,
	}
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
	sc, err := app.LoadSidecar(chainSpec.BlockchainName)
	if err != nil {
		return err
	}
	_, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(app, network, chainSpec)
	if err != nil {
		return err
	}
	managerAddress := common.HexToAddress(ValidatorContractAddress)
	ownerAddress := common.HexToAddress(sc.PoAValidatorManagerOwner)
	_ = InitializePoAValidatorManager(
		rpcURL,
		managerAddress,
		genesisPrivateKey,
		subnetID,
		ownerAddress,
	)
	infoClient := info.NewClient(constants.LocalAPIEndpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	nodeID, proofOfPossesion, err := infoClient.GetNodeID(ctx)
	if err != nil {
		return err
	}
	blsPublicKey := bls.PublicKeyToCompressedBytes(proofOfPossesion.Key())
	subnetConversionValidatorData := []warpMessage.SubnetConversionValidatorData{
		{
			NodeID:       nodeID[:],
			BLSPublicKey: [48]byte(blsPublicKey),
			Weight:       15,
		},
	}
	subnetConversionData := warpMessage.SubnetConversionData{
		SubnetID:       subnetID,
		ManagerChainID: blockchainID,
		ManagerAddress: managerAddress.Bytes(),
		Validators:     subnetConversionValidatorData,
	}
	subnetConversionID, err := warpMessage.SubnetConversionID(subnetConversionData)
	if err != nil {
		return err
	}
	addressedCallPayload, err := warpMessage.NewSubnetConversion(subnetConversionID)
	if err != nil {
		return err
	}
	subnetConversionAddressedCall, err := warpPayload.NewAddressedCall(
		nil,
		addressedCallPayload.Bytes(),
	)
	if err != nil {
		return err
	}
	subnetConversionUnsignedMessage, err := warp.NewUnsignedMessage(
		network.ID,
		avagoconstants.PlatformChainID,
		subnetConversionAddressedCall.Bytes(),
	)
	if err != nil {
		return err
	}

	signatureAggregator, err := interchain.NewSignatureAggregator(
		network,
		app.Log,
		logging.Debug,
		ids.Empty,
		0,
	)
	if err != nil {
		return err
	}

	subnetConversionSignedMessage, err := signatureAggregator.Sign(subnetConversionUnsignedMessage, nil)
	if err != nil {
		return err
	}

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
	subnetConversionDataAux := SubnetConversionData{
		SubnetID:                     subnetID,
		ValidatorManagerBlockchainID: blockchainID,
		ValidatorManagerAddress:      managerAddress,
		InitialValidators: []InitialValidator{
			{
				NodeID:       nodeID[:],
				BlsPublicKey: blsPublicKey,
				Weight:       15,
			},
		},
	}

	_, _, err = contract.TxToMethodWithWarpMessage(
		rpcURL,
		genesisPrivateKey,
		managerAddress,
		subnetConversionSignedMessage,
		"initializeValidatorSet((bytes32,bytes32,address,[(bytes,bytes,uint64)]),uint32)",
		subnetConversionDataAux,
		uint32(0),
	)
	if err != nil {
		return err
	}

	return nil
}
