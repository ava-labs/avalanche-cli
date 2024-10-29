// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blockchain

import (
	"context"
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	"math/big"
	"testing"
	"time"

	"github.com/ava-labs/subnet-evm/utils"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanche-cli/sdk/keychain"
	"github.com/ava-labs/avalanche-cli/sdk/network"
	"github.com/ava-labs/avalanche-cli/sdk/vm"
	"github.com/ava-labs/avalanche-cli/sdk/wallet"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ethereum/go-ethereum/common"
)

func getDefaultSubnetEVMGenesis() SubnetParams {
	allocation := core.GenesisAlloc{}
	defaultAmount, _ := new(big.Int).SetString(vm.DefaultEvmAirdropAmount, 10)
	allocation[common.HexToAddress("INITIAL_ALLOCATION_ADDRESS")] = core.GenesisAccount{
		Balance: defaultAmount,
	}
	genesisBlock0Timestamp := utils.TimeToNewUint64(time.Now())
	return SubnetParams{
		SubnetEVM: &SubnetEVMParams{
			ChainID:     big.NewInt(123456),
			FeeConfig:   vm.StarterFeeConfig,
			Allocation:  allocation,
			Precompiles: params.Precompiles{},
			Timestamp:   genesisBlock0Timestamp,
		},
		Name: "TestSubnet",
	}
}

func TestSubnetDeploy(t *testing.T) {
	require := require.New(t)
	subnetParams := getDefaultSubnetEVMGenesis()
	newSubnet, err := New(&subnetParams)
	require.NoError(err)
	network := network.FujiNetwork()

	keychain, err := keychain.NewKeychain(network, "KEY_PATH", nil)
	require.NoError(err)

	controlKeys := keychain.Addresses().List()
	subnetAuthKeys := keychain.Addresses().List()
	threshold := 1
	newSubnet.SetSubnetControlParams(controlKeys, uint32(threshold))
	wallet, err := wallet.New(
		context.Background(),
		&primary.WalletConfig{
			URI:          network.Endpoint,
			AVAXKeychain: keychain.Keychain,
			EthKeychain:  secp256k1fx.NewKeychain(),
			SubnetIDs:    nil,
		},
	)
	require.NoError(err)
	deploySubnetTx, err := newSubnet.CreateSubnetTx(wallet)
	require.NoError(err)
	subnetID, err := newSubnet.Commit(*deploySubnetTx, wallet, true)
	require.NoError(err)
	fmt.Printf("subnetID %s \n", subnetID.String())
	time.Sleep(2 * time.Second)
	newSubnet.SetSubnetAuthKeys(subnetAuthKeys)
	deployChainTx, err := newSubnet.CreateBlockchainTx(wallet)
	require.NoError(err)
	blockchainID, err := newSubnet.Commit(*deployChainTx, wallet, true)
	require.NoError(err)
	fmt.Printf("blockchainID %s \n", blockchainID.String())
}

func TestSubnetDeployMultiSig(t *testing.T) {
	require := require.New(t)
	subnetParams := getDefaultSubnetEVMGenesis()
	newSubnet, _ := New(&subnetParams)
	network := network.FujiNetwork()

	keychainA, err := keychain.NewKeychain(network, "KEY_PATH_A", nil)
	require.NoError(err)
	keychainB, err := keychain.NewKeychain(network, "KEY_PATH_B", nil)
	require.NoError(err)
	keychainC, err := keychain.NewKeychain(network, "KEY_PATH_C", nil)
	require.NoError(err)

	controlKeys := []ids.ShortID{}
	controlKeys = append(controlKeys, keychainA.Addresses().List()[0])
	controlKeys = append(controlKeys, keychainB.Addresses().List()[0])
	controlKeys = append(controlKeys, keychainC.Addresses().List()[0])

	subnetAuthKeys := []ids.ShortID{}
	subnetAuthKeys = append(subnetAuthKeys, keychainA.Addresses().List()[0])
	subnetAuthKeys = append(subnetAuthKeys, keychainB.Addresses().List()[0])
	threshold := 2
	newSubnet.SetSubnetControlParams(controlKeys, uint32(threshold))

	walletA, err := wallet.New(
		context.Background(),
		&primary.WalletConfig{
			URI:          network.Endpoint,
			AVAXKeychain: keychainA.Keychain,
			EthKeychain:  secp256k1fx.NewKeychain(),
			SubnetIDs:    nil,
		},
	)
	require.NoError(err)

	deploySubnetTx, err := newSubnet.CreateSubnetTx(walletA)
	require.NoError(err)
	subnetID, err := newSubnet.Commit(*deploySubnetTx, walletA, true)
	require.NoError(err)
	fmt.Printf("subnetID %s \n", subnetID.String())

	// we need to wait to allow the transaction to reach other nodes in Fuji
	time.Sleep(2 * time.Second)

	newSubnet.SetSubnetAuthKeys(subnetAuthKeys)
	// first signature of CreateChainTx using keychain A
	deployChainTx, err := newSubnet.CreateBlockchainTx(walletA)
	require.NoError(err)

	// include subnetID in PChainTxsToFetch when creating second wallet
	walletB, err := wallet.New(
		context.Background(),
		&primary.WalletConfig{
			URI:          network.Endpoint,
			AVAXKeychain: keychainB.Keychain,
			EthKeychain:  secp256k1fx.NewKeychain(),
			SubnetIDs:    []ids.ID{subnetID},
		},
	)
	require.NoError(err)

	// second signature using keychain B
	err = walletB.P().Signer().Sign(context.Background(), deployChainTx.PChainTx)
	require.NoError(err)

	// since we are using the fee paying key as control key too, we can commit the transaction
	// on chain immediately since the number of signatures has been reached
	blockchainID, err := newSubnet.Commit(*deployChainTx, walletA, true)
	require.NoError(err)
	fmt.Printf("blockchainID %s \n", blockchainID.String())
}

func TestSubnetDeployLedger(t *testing.T) {
	require := require.New(t)
	subnetParams := getDefaultSubnetEVMGenesis()
	newSubnet, err := New(&subnetParams)
	require.NoError(err)
	network := network.FujiNetwork()

	ledgerInfo := keychain.LedgerParams{
		LedgerAddresses: []string{"P-fujixxxxxxxxx"},
	}
	keychainA, err := keychain.NewKeychain(network, "", &ledgerInfo)
	require.NoError(err)

	addressesIDs, err := address.ParseToIDs([]string{"P-fujiyyyyyyyy"})
	require.NoError(err)
	controlKeys := addressesIDs
	subnetAuthKeys := addressesIDs
	threshold := 1

	newSubnet.SetSubnetControlParams(controlKeys, uint32(threshold))

	walletA, err := wallet.New(
		context.Background(),
		&primary.WalletConfig{
			URI:          network.Endpoint,
			AVAXKeychain: keychainA.Keychain,
			EthKeychain:  secp256k1fx.NewKeychain(),
			SubnetIDs:    nil,
		},
	)

	require.NoError(err)
	deploySubnetTx, err := newSubnet.CreateSubnetTx(walletA)
	require.NoError(err)
	subnetID, err := newSubnet.Commit(*deploySubnetTx, walletA, true)
	require.NoError(err)
	fmt.Printf("subnetID %s \n", subnetID.String())

	time.Sleep(2 * time.Second)

	newSubnet.SetSubnetAuthKeys(subnetAuthKeys)
	deployChainTx, err := newSubnet.CreateBlockchainTx(walletA)
	require.NoError(err)

	ledgerInfoB := keychain.LedgerParams{
		LedgerAddresses: []string{"P-fujiyyyyyyyy"},
	}
	err = keychainA.Ledger.LedgerDevice.Disconnect()
	require.NoError(err)

	keychainB, err := keychain.NewKeychain(network, "", &ledgerInfoB)
	require.NoError(err)

	walletB, err := wallet.New(
		context.Background(),
		&primary.WalletConfig{
			URI:          network.Endpoint,
			AVAXKeychain: keychainB.Keychain,
			EthKeychain:  secp256k1fx.NewKeychain(),
			SubnetIDs:    []ids.ID{subnetID},
		},
	)
	require.NoError(err)

	// second signature
	err = walletB.P().Signer().Sign(context.Background(), deployChainTx.PChainTx)
	require.NoError(err)

	blockchainID, err := newSubnet.Commit(*deployChainTx, walletB, true)
	require.NoError(err)

	fmt.Printf("blockchainID %s \n", blockchainID.String())
}

func createSovereignSubnet() error {
	subnetParams := getDefaultSubnetEVMGenesis()
	newSubnet, err := New(&subnetParams)
	if err != nil {
		return err
	}
	network := network.FujiNetwork()

	keychain, err := keychain.NewKeychain(network, "KEY_PATH", nil)
	if err != nil {
		return err
	}
	controlKeys := keychain.Addresses().List()
	subnetAuthKeys := keychain.Addresses().List()
	threshold := 1
	newSubnet.SetSubnetControlParams(controlKeys, uint32(threshold))
	wallet, err := wallet.New(
		context.Background(),
		&primary.WalletConfig{
			URI:          network.Endpoint,
			AVAXKeychain: keychain.Keychain,
			EthKeychain:  secp256k1fx.NewKeychain(),
			SubnetIDs:    nil,
		},
	)
	if err != nil {
		return err
	}
	deploySubnetTx, err := newSubnet.CreateSubnetTx(wallet)
	if err != nil {
		return err
	}
	subnetID, err := newSubnet.Commit(*deploySubnetTx, wallet, true)
	if err != nil {
		return err
	}
	fmt.Printf("subnetID %s \n", subnetID.String())
	time.Sleep(2 * time.Second)
	newSubnet.SetSubnetAuthKeys(subnetAuthKeys)
	deployChainTx, err := newSubnet.CreateBlockchainTx(wallet)
	if err != nil {
		return err
	}
	blockchainID, err := newSubnet.Commit(*deployChainTx, wallet, true)
	if err != nil {
		return err
	}
	fmt.Printf("blockchainID %s \n", blockchainID.String())
}

func TestSubnetInitValidatorManager(t *testing.T) {
	require := require.New(t)

	if err := validatormanager.SetupPoA(
		subnetSDK,
		network,
		genesisPrivateKey,
		extraAggregatorPeers,
		aggregatorLogLevel,
	); err != nil {
		return err
	}
}
