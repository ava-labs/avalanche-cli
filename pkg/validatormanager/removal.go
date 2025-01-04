// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"errors"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/sdk/interchain"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	warp "github.com/ava-labs/avalanchego/vms/platformvm/warp"
	warpMessage "github.com/ava-labs/avalanchego/vms/platformvm/warp/message"
	warpPayload "github.com/ava-labs/avalanchego/vms/platformvm/warp/payload"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/warp/messages"
	"github.com/ethereum/go-ethereum/common"
)

func InitializeValidatorRemoval(
	rpcURL string,
	managerAddress common.Address,
	privateKey string,
	validationID ids.ID,
	isPoS bool,
	uptimeProofSignedMessage *warp.Message,
	force bool,
) (*types.Transaction, *types.Receipt, error) {
	if isPoS {
		if force {
			return contract.TxToMethod(
				rpcURL,
				privateKey,
				managerAddress,
				big.NewInt(0),
				"force POS validator removal",
				validatorManagerSDK.ErrorSignatureToError,
				"forceInitializeEndValidation(bytes32,bool,uint32)",
				validationID,
				false, // no uptime proof if force
				uint32(0),
			)
		}
		// remove PoS validator with uptime proof
		return contract.TxToMethodWithWarpMessage(
			rpcURL,
			privateKey,
			managerAddress,
			uptimeProofSignedMessage,
			big.NewInt(0),
			"POS validator removal with uptime proof",
			validatorManagerSDK.ErrorSignatureToError,
			"initializeEndValidation(bytes32,bool,uint32)",
			validationID,
			true, // submit uptime proof
			uint32(0),
		)
	}
	// PoA case
	return contract.TxToMethod(
		rpcURL,
		privateKey,
		managerAddress,
		big.NewInt(0),
		"POA validator removal initialization",
		validatorManagerSDK.ErrorSignatureToError,
		"initializeEndValidation(bytes32)",
		validationID,
	)
}

func GetUptimeProofMessage(
	network models.Network,
	aggregatorLogger logging.Logger,
	aggregatorQuorumPercentage uint64,
	aggregatorExtraPeerEndpoints []info.Peer,
	subnetID ids.ID,
	blockchainID ids.ID,
	validationID ids.ID,
	uptime uint64,
) (*warp.Message, error) {
	uptimePayload, err := messages.NewValidatorUptime(validationID, uptime)
	if err != nil {
		return nil, err
	}
	addressedCall, err := warpPayload.NewAddressedCall(nil, uptimePayload.Bytes())
	if err != nil {
		return nil, err
	}
	uptimeProofUnsignedMessage, err := warp.NewUnsignedMessage(
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
		subnetID,
		aggregatorQuorumPercentage,
		true, // allow private peers
		aggregatorExtraPeerEndpoints,
	)
	if err != nil {
		return nil, err
	}
	return signatureAggregator.Sign(uptimeProofUnsignedMessage, nil)
}

func GetSubnetValidatorWeightMessage(
	network models.Network,
	aggregatorLogger logging.Logger,
	aggregatorQuorumPercentage uint64,
	aggregatorAllowPrivateIPs bool,
	aggregatorExtraPeerEndpoints []info.Peer,
	subnetID ids.ID,
	blockchainID ids.ID,
	managerAddress common.Address,
	validationID ids.ID,
	nonce uint64,
	weight uint64,
) (*warp.Message, error) {
	addressedCallPayload, err := warpMessage.NewL1ValidatorWeight(
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
		subnetID,
		aggregatorQuorumPercentage,
		aggregatorAllowPrivateIPs,
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
	aggregatorAllowPrivatePeers bool,
	aggregatorLogger logging.Logger,
	initWithPos bool,
	uptimeSec uint64,
	force bool,
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
	managerAddress := common.HexToAddress(validatorManagerSDK.ProxyContractAddress)
	validationID, err := GetRegisteredValidator(
		rpcURL,
		managerAddress,
		nodeID,
	)
	if err != nil {
		return nil, ids.Empty, err
	}
	if validationID == ids.Empty {
		return nil, ids.Empty, fmt.Errorf("node %s is not a L1 validator", nodeID)
	}

	signedUptimeProof := &warp.Message{}
	if initWithPos {
		if err != nil {
			return nil, ids.Empty, evm.TransactionError(nil, err, "failure getting uptime data")
		}
		if uptimeSec == 0 {
			uptimeSec, err = utils.GetL1ValidatorUptimeSeconds(rpcURL, nodeID)
			if err != nil {
				return nil, ids.Empty, evm.TransactionError(nil, err, "failure getting uptime data for nodeID: %s via %s ", nodeID, rpcURL)
			}
		}
		ux.Logger.PrintToUser("Using uptime: %ds", uptimeSec)
		signedUptimeProof, err = GetUptimeProofMessage(
			network,
			aggregatorLogger,
			0,
			aggregatorExtraPeerEndpoints,
			subnetID,
			blockchainID,
			validationID,
			uptimeSec,
		)
		if err != nil {
			return nil, ids.Empty, evm.TransactionError(nil, err, "failure getting uptime proof")
		}
	}
	tx, _, err := InitializeValidatorRemoval(
		rpcURL,
		managerAddress,
		ownerPrivateKey,
		validationID,
		initWithPos,
		signedUptimeProof, // is empty for non-PoS
		force,
	)
	if err != nil {
		if !errors.Is(err, validatorManagerSDK.ErrInvalidValidatorStatus) {
			return nil, ids.Empty, evm.TransactionError(tx, err, "failure initializing validator removal")
		}
		ux.Logger.PrintToUser(logging.LightBlue.Wrap("The validator removal process was already initialized. Proceeding to the next step"))
	}

	nonce := uint64(1)
	signedMsg, err := GetSubnetValidatorWeightMessage(
		network,
		aggregatorLogger,
		0,
		aggregatorAllowPrivatePeers,
		aggregatorExtraPeerEndpoints,
		subnetID,
		blockchainID,
		managerAddress,
		validationID,
		nonce,
		0,
	)
	return signedMsg, validationID, err
}

func CompleteValidatorRemoval(
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
		"complete poa validator removal",
		validatorManagerSDK.ErrorSignatureToError,
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
	aggregatorAllowPrivatePeers bool,
	aggregatorLogger logging.Logger,
) error {
	managerAddress := common.HexToAddress(validatorManagerSDK.ProxyContractAddress)
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return err
	}
	signedMessage, err := GetPChainSubnetValidatorRegistrationWarpMessage(
		network,
		rpcURL,
		aggregatorLogger,
		0,
		aggregatorAllowPrivatePeers,
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
	tx, _, err := CompleteValidatorRemoval(
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
