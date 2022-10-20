// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/platformvm/validator"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
	"github.com/olekukonko/tablewriter"
)

type PublicDeployer struct {
	LocalDeployer
	usingLedger bool
	kc          keychain.Keychain
	network     models.Network
	app         *application.Avalanche
}

func NewPublicDeployer(app *application.Avalanche, usingLedger bool, kc keychain.Keychain, network models.Network) *PublicDeployer {
	return &PublicDeployer{
		LocalDeployer: *NewLocalDeployer(app, "", ""),
		app:           app,
		usingLedger:   usingLedger,
		kc:            kc,
		network:       network,
	}
}

func (d *PublicDeployer) AddValidator(
	subnetAuthKeys []string,
	subnet ids.ID,
	nodeID ids.NodeID,
	weight uint64,
	startTime time.Time,
	duration time.Duration,
) (*txs.Tx, error) {
	wallet, err := d.loadWallet(subnet)
	if err != nil {
		return nil, err
	}
	_, err = d.GetWalletSubnetAuthAddresses(subnetAuthKeys)
	if err != nil {
		return nil, err
	}
	validator := &validator.SubnetValidator{
		Validator: validator.Validator{
			NodeID: nodeID,
			Start:  uint64(startTime.Unix()),
			End:    uint64(startTime.Add(duration).Unix()),
			Wght:   weight,
		},
		Subnet: subnet,
	}
	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign add validator hash on the ledger device *** ")
	}

	var tx *txs.Tx
	if len(subnetAuthKeys) == 1 {
		id, err := wallet.P().IssueAddSubnetValidatorTx(validator)
		if err != nil {
			return nil, err
		}
		ux.Logger.PrintToUser("Transaction successful, transaction ID :%s", id)
	} else {
		tx, err = d.createAddSubnetValidatorTx(subnetAuthKeys, validator, wallet)
		if err != nil {
			return nil, err
		}
	}

	return tx, nil
}

func (d *PublicDeployer) Deploy(
	controlKeys []string,
	subnetAuthKeys []string,
	threshold uint32,
	chain string,
	genesis []byte,
) (ids.ID, bool, ids.ID, *txs.Tx, error) {
	wallet, err := d.loadWallet()
	if err != nil {
		return ids.Empty, false, ids.Empty, nil, err
	}
	vmID, err := utils.VMID(chain)
	if err != nil {
		return ids.Empty, false, ids.Empty, nil, fmt.Errorf("failed to create VM ID from %s: %w", chain, err)
	}

	_, err = d.GetWalletSubnetAuthAddresses(subnetAuthKeys)
	if err != nil {
		return ids.Empty, false, ids.Empty, nil, err
	}

	subnetID, err := d.createSubnetTx(controlKeys, threshold, wallet)
	if err != nil {
		return ids.Empty, false, ids.Empty, nil, err
	}
	ux.Logger.PrintToUser("Subnet has been created with ID: %s. Now creating blockchain...", subnetID.String())

	var (
		blockchainID       ids.ID
		blockchainTx       *txs.Tx
		blockchainDeployed bool
	)

	if len(subnetAuthKeys) == 1 {
		blockchainDeployed = true
		blockchainID, err = d.createAndIssueBlockchainTx(chain, vmID, subnetID, genesis, wallet)
		if err != nil {
			return ids.Empty, false, ids.Empty, nil, err
		}
	} else {
		blockchainTx, err = d.createBlockchainTx(subnetAuthKeys, chain, vmID, subnetID, genesis, wallet)
		if err != nil {
			return ids.Empty, false, ids.Empty, nil, err
		}
	}

	header := []string{"Deployment results", ""}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetRowLine(true)
	table.SetAutoMergeCells(true)
	table.Append([]string{"Chain Name", chain})
	table.Append([]string{"Subnet ID", subnetID.String()})
	table.Append([]string{"VM ID", vmID.String()})
	if blockchainDeployed {
		table.Append([]string{"Blockchain ID", blockchainID.String()})
		table.Append([]string{"RPC URL", fmt.Sprintf("%s/ext/bc/%s/rpc", constants.DefaultNodeRunURL, blockchainID.String())})
	}
	table.Render()

	return subnetID, blockchainDeployed, blockchainID, blockchainTx, nil
}

func (d *PublicDeployer) loadWallet(preloadTxs ...ids.ID) (primary.Wallet, error) {
	ctx := context.Background()

	var api string
	switch d.network {
	case models.Fuji:
		api = constants.FujiAPIEndpoint
	case models.Mainnet:
		api = constants.MainnetAPIEndpoint
	case models.Local:
		// used for E2E testing of public related paths
		api = constants.LocalAPIEndpoint
	default:
		return nil, fmt.Errorf("unsupported public network")
	}

	wallet, err := primary.NewWalletWithTxs(ctx, api, d.kc, preloadTxs...)
	if err != nil {
		return nil, err
	}
	return wallet, nil
}

func (d *PublicDeployer) createAndIssueBlockchainTx(
	chainName string,
	vmID,
	subnetID ids.ID,
	genesis []byte,
	wallet primary.Wallet,
) (ids.ID, error) {
	fxIDs := make([]ids.ID, 0)
	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign blockchain creation hash on the ledger device *** ")
	}
	return wallet.P().IssueCreateChainTx(subnetID, genesis, vmID, fxIDs, chainName)
}

func (d *PublicDeployer) getMultisigTxOptions(subnetAuthKeys []string) ([]common.Option, error) {
	options := []common.Option{}
	// addrs to use for signing
	customAddrsSet := ids.ShortSet{}
	for _, customAddrStr := range subnetAuthKeys {
		customAddr, err := address.ParseToID(customAddrStr)
		if err != nil {
			return options, err
		}
		customAddrsSet.Add(customAddr)
	}
	options = append(options, common.WithCustomAddresses(customAddrsSet))
	// set change to go to wallet addr (instead of any other subnet auth key)
	walletAddresses, err := d.GetWalletSubnetAuthAddresses(subnetAuthKeys)
	if err != nil {
		return options, err
	}
	walletAddrStr := walletAddresses[0]
	walletAddr, err := address.ParseToID(walletAddrStr)
	if err != nil {
		return options, err
	}
	changeOwner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     []ids.ShortID{walletAddr},
	}
	options = append(options, common.WithChangeOwner(changeOwner))
	return options, nil
}

func (d *PublicDeployer) createBlockchainTx(
	subnetAuthKeys []string,
	chainName string,
	vmID,
	subnetID ids.ID,
	genesis []byte,
	wallet primary.Wallet,
) (*txs.Tx, error) {
	fxIDs := make([]ids.ID, 0)
	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign blockchain creation hash on the ledger device *** ")
	}
	options, err := d.getMultisigTxOptions(subnetAuthKeys)
	if err != nil {
		return nil, err
	}
	// create tx
	unsignedTx, err := wallet.P().Builder().NewCreateChainTx(
		subnetID,
		genesis,
		vmID,
		fxIDs,
		chainName,
		options...,
	)
	if err != nil {
		return nil, err
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	// sign with current wallet
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

func (d *PublicDeployer) createAddSubnetValidatorTx(
	subnetAuthKeys []string,
	validator *validator.SubnetValidator,
	wallet primary.Wallet,
) (*txs.Tx, error) {
	options, err := d.getMultisigTxOptions(subnetAuthKeys)
	if err != nil {
		return nil, err
	}
	// create tx
	unsignedTx, err := wallet.P().Builder().NewAddSubnetValidatorTx(validator, options...)
	if err != nil {
		return nil, err
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	// sign with current wallet
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

func (d *PublicDeployer) createSubnetTx(controlKeys []string, threshold uint32, wallet primary.Wallet) (ids.ID, error) {
	addrs, err := address.ParseToIDs(controlKeys)
	if err != nil {
		return ids.Empty, err
	}
	owners := &secp256k1fx.OutputOwners{
		Addrs:     addrs,
		Threshold: threshold,
		Locktime:  0,
	}
	opts := []common.Option{}
	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign subnet creation hash on the ledger device *** ")
	}
	return wallet.P().IssueCreateSubnetTx(owners, opts...)
}

func (d *PublicDeployer) GetWalletSubnetAuthAddresses(subnetAuth []string) ([]string, error) {
	networkID, err := d.network.NetworkID()
	if err != nil {
		return nil, err
	}
	hrp := key.GetHRP(networkID)
	walletAddrs := d.kc.Addresses().List()
	if len(walletAddrs) == 0 {
		return nil, fmt.Errorf("no addrs in wallet")
	}
	subnetAuthAddrs := []string{}
	for _, walletAddr := range walletAddrs {
		walletAddrStr, err := address.Format("P", hrp, walletAddr[:])
		if err != nil {
			return nil, err
		}

		for _, addr := range subnetAuth {
			if addr == walletAddrStr {
				subnetAuthAddrs = append(subnetAuthAddrs, addr)
			}
		}
	}
	if len(subnetAuthAddrs) == 0 {
		return nil, fmt.Errorf("wallet addr not listed in subnet auth addresses")
	} else {
		return subnetAuthAddrs, nil
	}
}
