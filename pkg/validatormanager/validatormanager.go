// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
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
	validators := []txs.ConvertSubnetValidator{
		{
			NodeID: nodeID,
			Weight: 15,
			Signer: proofOfPossesion,
		},
	}
	unsignedTx := &txs.ConvertSubnetTx{
		Subnet:     subnetID,
		ChainID:    blockchainID,
		Address:    managerAddress.Bytes(),
		Validators: validators,
	}
	tx := txs.Tx{Unsigned: unsignedTx}

	subnetConversionID, err := getSubnetConversionID(&tx)
	if err != nil {
		return err
	}
	addressedCallPayload, err := warpMessage.NewSubnetConversion(subnetConversionID)
	if err != nil {
		return err
	}
	subnetConversionAddressedCall, err := warpPayload.NewAddressedCall(
		common.Address{}.Bytes(),
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
	fmt.Printf("%#v\n", subnetConversionUnsignedMessage)

	return nil
}

func getSubnetConversionID(tx *txs.Tx) (ids.ID, error) {
	subnetConversionData := []byte{}
	txID := tx.ID()
	convertSubnetTx, b := tx.Unsigned.(*txs.ConvertSubnetTx)
	if !b {
		return ids.Empty, fmt.Errorf("expected txs.ConvertSubneTx, got %T", tx.Unsigned)
	}
	subnetConversionData = append(subnetConversionData, txID[:]...)
	subnetConversionData = append(subnetConversionData, convertSubnetTx.ChainID[:]...)
	subnetConversionData = binary.BigEndian.AppendUint32(subnetConversionData, uint32(len(convertSubnetTx.Address)))
	subnetConversionData = append(subnetConversionData, convertSubnetTx.Address...)
	subnetConversionData = binary.BigEndian.AppendUint32(subnetConversionData, uint32(len(convertSubnetTx.Validators)))
	for _, validator := range convertSubnetTx.Validators {
		subnetConversionData = append(subnetConversionData, validator.NodeID[:]...)
		subnetConversionData = binary.BigEndian.AppendUint64(subnetConversionData, validator.Weight)
		blsPublicKey := bls.PublicKeyToCompressedBytes(validator.Signer.Key())
		subnetConversionData = append(subnetConversionData, blsPublicKey...)
	}
	return sha256.Sum256(subnetConversionData), nil
}
