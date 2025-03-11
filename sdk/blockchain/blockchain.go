// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ava-labs/avalanchego/vms/platformvm/txs"

	"github.com/ava-labs/avalanche-cli/sdk/multisig"
	utilsSDK "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanche-cli/sdk/wallet"

	"github.com/ava-labs/avalanche-cli/sdk/vm"

	"github.com/ava-labs/avalanchego/ids"
	commonAvago "github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
	"github.com/ava-labs/subnet-evm/commontype"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/params"
)

var (
	errMissingSubnetID            = fmt.Errorf("missing Subnet ID")
	errMissingBlockchainID        = fmt.Errorf("missing Blockchain ID")
	errMissingRPC                 = fmt.Errorf("missing RPC URL")
	errMissingBootstrapValidators = fmt.Errorf("missing bootstrap validators")
	errMissingOwnerAddress        = fmt.Errorf("missing Owner Address")
)

type SubnetParams struct {
	// File path of Genesis to use
	// Do not set SubnetEVMParams or CustomVMParams
	// if GenesisFilePath value is set
	//
	// See https://docs.avax.network/build/subnet/upgrade/customize-a-subnet#genesis for
	// information on Genesis
	GenesisFilePath string

	// Subnet-EVM parameters to use
	// Do not set SubnetEVM value if you are using Custom VM
	SubnetEVM *SubnetEVMParams

	// Name is alias for the Subnet, it is used to derive VM ID, which is required
	// during for createBlockchainTx
	Name string
}

type SubnetEVMParams struct {
	// ChainID identifies the current chain and is used for replay protection
	ChainID *big.Int

	// FeeConfig sets the configuration for the dynamic fee algorithm
	FeeConfig commontype.FeeConfig

	// Allocation specifies the initial state that is part of the genesis block.
	Allocation core.GenesisAlloc

	// Ethereum uses Precompiles to efficiently implement cryptographic primitives within the EVM
	// instead of re-implementing the same primitives in Solidity.
	//
	// Precompiles are a shortcut to execute a function implemented by the EVM itself,
	// rather than an actual contract. A precompile is associated with a fixed address defined in
	// the EVM. There is no byte code associated with that address.
	//
	// For more information regarding Precompiles, head to https://docs.avax.network/build/vm/evm/intro.
	Precompiles params.Precompiles

	// Timestamp
	// TODO: add description what timestamp is
	Timestamp *uint64
}

type CustomVMParams struct {
	// File path of the Custom VM binary to use
	VMFilePath string

	// Git Repo URL to be used to build Custom VM
	// Only set CustomVMRepoURL value when VMFilePath value is not set
	CustomVMRepoURL string

	// Git branch or commit to be used to build Custom VM
	// Only set CustomVMBranch value when VMFilePath value is not set
	CustomVMBranch string

	// Filepath of the script to be used to build Custom VM
	// Only set CustomVMBuildScript value when VMFilePath value is not set
	CustomVMBuildScript string
}

type Subnet struct {
	// Name is alias for the Subnet
	Name string

	// Genesis is the initial state of a blockchain when it is first created. Each Virtual Machine
	// defines the format and semantics of its genesis data.
	//
	// For more information regarding Genesis, head to https://docs.avax.network/build/subnet/upgrade/customize-a-subnet#genesis
	Genesis []byte

	// SubnetID is the transaction ID from an issued CreateSubnetTX and is used to identify
	// the target Subnet for CreateChainTx and AddValidatorTx
	SubnetID ids.ID

	// BlockchainID is the transaction ID from an issued CreateChainTx
	BlockchainID ids.ID

	// VMID specifies the vm that the new chain will run when CreateChainTx is called
	VMID ids.ID

	// DeployInfo contains all the necessary information for createSubnetTx
	DeployInfo DeployParams

	// RPC URL that Subnet can be reached at
	RPC string

	// OwnerAddress is address of the owner of the Validator Manager Contract
	OwnerAddress *common.Address

	// BootstrapValidators are bootstrap validators that are included in the ConvertSubnetToL1Tx call
	// that made Subnet a sovereign L1
	BootstrapValidators []*txs.ConvertSubnetToL1Validator
}

func (c *Subnet) SetParams(controlKeys []ids.ShortID, subnetAuthKeys []ids.ShortID, threshold uint32) {
	c.DeployInfo = DeployParams{
		ControlKeys:    controlKeys,
		SubnetAuthKeys: subnetAuthKeys,
		Threshold:      threshold,
	}
}

// SetSubnetControlParams sets:
//   - control keys, which are keys that are allowed to make changes to a Subnet
//   - threshold, which is the number of keys that need to sign a transaction that changes
//     a Subnet
func (c *Subnet) SetSubnetControlParams(controlKeys []ids.ShortID, threshold uint32) {
	c.DeployInfo.ControlKeys = controlKeys
	c.DeployInfo.Threshold = threshold
}

// SetSubnetAuthKeys sets subnetAuthKeys, which are keys that are being used to sign a transaction
// that changes a Subnet
func (c *Subnet) SetSubnetAuthKeys(subnetAuthKeys []ids.ShortID) {
	c.DeployInfo.SubnetAuthKeys = subnetAuthKeys
}

type DeployParams struct {
	// ControlKeys is a list of P-Chain addresses that are authorized to create new chains and add
	// new validators to the Subnet
	ControlKeys []ids.ShortID

	// SubnetAuthKeys is a list of P-Chain addresses that will be used to sign transactions that
	// will modify the Subnet.
	//
	// SubnetAuthKeys has to be a subset of ControlKeys
	SubnetAuthKeys []ids.ShortID

	// Threshold is the minimum number of signatures needed before a transaction can be issued
	// Number of addresses in SubnetAuthKeys has to be more than or equal to Threshold number
	Threshold uint32
}

// New takes SubnetParams as input and creates Subnet as an output
//
// The created Subnet object can be used to :
//   - Create the Subnet on a specified network (Fuji / Mainnet)
//   - Create Blockchain(s) in the Subnet
//   - Add Validator(s) into the Subnet
func New(subnetParams *SubnetParams) (*Subnet, error) {
	if subnetParams.GenesisFilePath != "" && subnetParams.SubnetEVM != nil {
		return nil, fmt.Errorf("genesis file path cannot be non-empty if SubnetEVM params is not empty")
	}

	if subnetParams.GenesisFilePath == "" && subnetParams.SubnetEVM == nil {
		return nil, fmt.Errorf("genesis file path and SubnetEVM params params cannot all be empty")
	}

	if subnetParams.Name == "" {
		return nil, fmt.Errorf("SubnetEVM name cannot be empty")
	}

	var genesisBytes []byte
	var err error
	switch {
	case subnetParams.GenesisFilePath != "":
		genesisBytes, err = os.ReadFile(subnetParams.GenesisFilePath)
	case subnetParams.SubnetEVM != nil:
		genesisBytes, err = createEvmGenesis(subnetParams.SubnetEVM)
	default:
	}
	if err != nil {
		return nil, err
	}

	vmID, err := vmID(subnetParams.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM ID from %s: %w", subnetParams.Name, err)
	}
	subnet := Subnet{
		Name:    subnetParams.Name,
		VMID:    vmID,
		Genesis: genesisBytes,
	}
	return &subnet, nil
}

func (c *Subnet) SetSubnetID(subnetID ids.ID) {
	c.SubnetID = subnetID
}

func createEvmGenesis(
	subnetEVMParams *SubnetEVMParams,
) ([]byte, error) {
	genesis := core.Genesis{}
	genesis.Timestamp = *subnetEVMParams.Timestamp

	conf := params.SubnetEVMDefaultChainConfig
	conf.NetworkUpgrades = params.NetworkUpgrades{}

	var err error

	if subnetEVMParams.ChainID == nil {
		return nil, fmt.Errorf("genesis params chain ID cannot be empty")
	}

	if subnetEVMParams.FeeConfig == commontype.EmptyFeeConfig {
		return nil, fmt.Errorf("genesis params fee config cannot be empty")
	}

	if subnetEVMParams.Allocation == nil {
		return nil, fmt.Errorf("genesis params allocation cannot be empty")
	}
	allocation := subnetEVMParams.Allocation

	if subnetEVMParams.Precompiles == nil {
		return nil, fmt.Errorf("genesis params precompiles cannot be empty")
	}

	conf.FeeConfig = subnetEVMParams.FeeConfig
	conf.GenesisPrecompiles = subnetEVMParams.Precompiles

	conf.ChainID = subnetEVMParams.ChainID

	genesis.Alloc = allocation
	genesis.Config = conf
	genesis.Difficulty = vm.Difficulty
	genesis.GasLimit = conf.FeeConfig.GasLimit.Uint64()

	jsonBytes, err := genesis.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonBytes, "", "    ")
	if err != nil {
		return nil, err
	}

	return prettyJSON.Bytes(), nil
}

func vmID(vmName string) (ids.ID, error) {
	if len(vmName) > 32 {
		return ids.Empty, fmt.Errorf("VM name must be <= 32 bytes, found %d", len(vmName))
	}
	b := make([]byte, 32)
	copy(b, []byte(vmName))
	return ids.ToID(b)
}

func (c *Subnet) Commit(ms multisig.Multisig, wallet wallet.Wallet, waitForTxAcceptance bool) (ids.ID, error) {
	if ms.Undefined() {
		return ids.Empty, multisig.ErrUndefinedTx
	}
	isReady, err := ms.IsReadyToCommit()
	if err != nil {
		return ids.Empty, err
	}
	if !isReady {
		return ids.Empty, errors.New("tx is not fully signed so can't be committed")
	}
	tx, err := ms.GetWrappedPChainTx()
	if err != nil {
		return ids.Empty, err
	}
	const (
		repeats             = 3
		sleepBetweenRepeats = 2 * time.Second
	)
	var issueTxErr error
	if err != nil {
		return ids.Empty, err
	}
	for i := 0; i < repeats; i++ {
		ctx, cancel := utilsSDK.GetAPILargeContext()
		defer cancel()
		options := []commonAvago.Option{commonAvago.WithContext(ctx)}
		if !waitForTxAcceptance {
			options = append(options, commonAvago.WithAssumeDecided())
		}
		// TODO: split error checking and recovery between issuing and waiting for status
		issueTxErr = wallet.P().IssueTx(tx, options...)
		if issueTxErr == nil {
			break
		}
		if ctx.Err() != nil {
			issueTxErr = fmt.Errorf("timeout issuing/verifying tx with ID %s: %w", tx.ID(), issueTxErr)
		} else {
			issueTxErr = fmt.Errorf("error issuing tx with ID %s: %w", tx.ID(), issueTxErr)
		}
		time.Sleep(sleepBetweenRepeats)
	}
	if issueTxErr != nil {
		return ids.Empty, fmt.Errorf("issue tx error %w", issueTxErr)
	}
	if _, ok := ms.PChainTx.Unsigned.(*txs.CreateSubnetTx); ok {
		c.SubnetID = tx.ID()
	}
	return tx.ID(), issueTxErr
}

// InitializeProofOfAuthority setups PoA manager after a successful execution of
// ConvertSubnetToL1Tx on P-Chain
// needs the list of validators for that tx,
// [convertSubnetValidators], together with an evm [ownerAddress]
// to set as the owner of the PoA manager
func (c *Subnet) InitializeProofOfAuthority(
	ctx context.Context,
	network models.Network,
	privateKey string,
	aggregatorExtraPeerEndpoints []info.Peer,
	aggregatorAllowPrivatePeers bool,
	aggregatorLogger logging.Logger,
	validatorManagerAddressStr string,
	useACP99 bool,
	skipInitializeValidatorManager bool,
) error {
	if c.SubnetID == ids.Empty {
		return fmt.Errorf("unable to initialize Proof of Authority: %w", errMissingSubnetID)
	}

	if c.BlockchainID == ids.Empty {
		return fmt.Errorf("unable to initialize Proof of Authority: %w", errMissingBlockchainID)
	}

	if c.RPC == "" {
		return fmt.Errorf("unable to initialize Proof of Authority: %w", errMissingRPC)
	}

	if c.OwnerAddress == nil {
		return fmt.Errorf("unable to initialize Proof of Authority: %w", errMissingOwnerAddress)
	}

	if len(c.BootstrapValidators) == 0 {
		return fmt.Errorf("unable to initialize Proof of Authority: %w", errMissingBootstrapValidators)
	}

	if err := evm.SetupProposerVM(
		c.RPC,
		privateKey,
	); err != nil {
		ux.Logger.RedXToUser("failure setting proposer VM on L1: %s", err)
	}
	managerAddress := common.HexToAddress(validatorManagerAddressStr)

	if !skipInitializeValidatorManager {
		tx, _, err := validatormanager.PoAValidatorManagerInitialize(
			c.RPC,
			managerAddress,
			privateKey,
			c.SubnetID,
			*c.OwnerAddress,
			useACP99,
		)
		if err != nil {
			if !errors.Is(err, validatormanager.ErrAlreadyInitialized) {
				return evm.TransactionError(tx, err, "failure initializing poa validator manager")
			}
			ux.Logger.PrintToUser("Warning: the PoA contract is already initialized.")
		}
	}

	subnetConversionSignedMessage, err := validatormanager.GetPChainSubnetToL1ConversionMessage(
		ctx,
		network,
		aggregatorLogger,
		0,
		aggregatorAllowPrivatePeers,
		aggregatorExtraPeerEndpoints,
		c.SubnetID,
		c.BlockchainID,
		managerAddress,
		c.BootstrapValidators,
	)
	if err != nil {
		return fmt.Errorf("failure signing subnet conversion warp message: %w", err)
	}

	tx, _, err := validatormanager.InitializeValidatorsSet(
		c.RPC,
		managerAddress,
		privateKey,
		c.SubnetID,
		c.BlockchainID,
		c.BootstrapValidators,
		subnetConversionSignedMessage,
	)
	if err != nil {
		return evm.TransactionError(tx, err, "failure initializing validators set on poa manager")
	}

	return nil
}

func (c *Subnet) InitializeProofOfStake(
	ctx context.Context,
	network models.Network,
	privateKey string,
	aggregatorExtraPeerEndpoints []info.Peer,
	aggregatorAllowPrivatePeers bool,
	aggregatorLogger logging.Logger,
	posParams validatormanager.PoSParams,
	validatorManagerAddressStr string,
	skipInitializeValidatorManager bool,
) error {
	if err := evm.SetupProposerVM(
		c.RPC,
		privateKey,
	); err != nil {
		ux.Logger.RedXToUser("failure setting proposer VM on L1: %s", err)
	}
	managerAddress := common.HexToAddress(validatorManagerAddressStr)
	if !skipInitializeValidatorManager {
		tx, _, err := validatormanager.PoSValidatorManagerInitialize(
			c.RPC,
			managerAddress,
			privateKey,
			c.SubnetID,
			posParams,
		)
		if err != nil {
			if !errors.Is(err, validatormanager.ErrAlreadyInitialized) {
				return evm.TransactionError(tx, err, "failure initializing native PoS validator manager")
			}
			ux.Logger.PrintToUser("Warning: the PoS contract is already initialized.")
		}
	}

	subnetConversionSignedMessage, err := validatormanager.GetPChainSubnetToL1ConversionMessage(
		ctx,
		network,
		aggregatorLogger,
		0,
		aggregatorAllowPrivatePeers,
		aggregatorExtraPeerEndpoints,
		c.SubnetID,
		c.BlockchainID,
		managerAddress,
		c.BootstrapValidators,
	)
	if err != nil {
		return fmt.Errorf("failure signing subnet conversion warp message: %w", err)
	}

	tx, _, err := validatormanager.InitializeValidatorsSet(
		c.RPC,
		managerAddress,
		privateKey,
		c.SubnetID,
		c.BlockchainID,
		c.BootstrapValidators,
		subnetConversionSignedMessage,
	)
	if err != nil {
		return evm.TransactionError(tx, err, "failure initializing validators set on pos manager")
	}
	return nil
}
