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
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/platformvm/validator"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/keychain"
	"github.com/olekukonko/tablewriter"
)

type PublicDeployer struct {
	LocalDeployer
	usingLedger bool
	kc          keychain.Accessor
	network     models.Network
	app         *application.Avalanche
}

func NewPublicDeployer(app *application.Avalanche, usingLedger bool, kc keychain.Accessor, network models.Network) *PublicDeployer {
	return &PublicDeployer{
		LocalDeployer: *NewLocalDeployer(app, "", ""),
		app:           app,
		usingLedger:   usingLedger,
		kc:            kc,
		network:       network,
	}
}

func (d *PublicDeployer) AddValidator(subnet ids.ID, nodeID ids.NodeID, weight uint64, startTime time.Time, duration time.Duration) error {
	wallet, err := d.loadWallet(subnet)
	if err != nil {
		return err
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
	id, err := wallet.P().IssueAddSubnetValidatorTx(validator)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("Transaction successful, transaction ID :%s", id)
	return nil
}

func (d *PublicDeployer) Deploy(controlKeys []string, threshold uint32, chain string, genesis []byte) (ids.ID, ids.ID, error) {
	wallet, err := d.loadWallet()
	if err != nil {
		return ids.Empty, ids.Empty, err
	}
	vmID, err := utils.VMID(chain)
	if err != nil {
		return ids.Empty, ids.Empty, fmt.Errorf("failed to create VM ID from %s: %w", chain, err)
	}

	subnetID, err := d.createSubnetTx(controlKeys, threshold, wallet)
	if err != nil {
		return ids.Empty, ids.Empty, err
	}
	ux.Logger.PrintToUser("Subnet has been created with ID: %s. Now creating blockchain...", subnetID.String())

	blockchainID, err := d.createBlockchainTx(chain, vmID, subnetID, genesis, wallet)
	if err != nil {
		return ids.Empty, ids.Empty, err
	}

	header := []string{"Deployment results", ""}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetRowLine(true)
	table.SetAutoMergeCells(true)
	table.Append([]string{"Chain Name", chain})
	table.Append([]string{"Subnet ID", subnetID.String()})
	table.Append([]string{"Blockchain ID", blockchainID.String()})
	table.Append([]string{"VM ID", vmID.String()})
	table.Append([]string{"RPC URL", fmt.Sprintf("%s/ext/bc/%s/rpc", constants.DefaultNodeRunURL, blockchainID.String())})

	table.Render()

	return subnetID, blockchainID, nil
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

func (d *PublicDeployer) createBlockchainTx(chainName string, vmID, subnetID ids.ID, genesis []byte, wallet primary.Wallet) (ids.ID, error) {
	// TODO do we need any of these to be set?
	options := []common.Option{}
	fxIDs := make([]ids.ID, 0)
	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign blockchain creation hash on the ledger device *** ")
	}
	return wallet.P().IssueCreateChainTx(subnetID, genesis, vmID, fxIDs, chainName, options...)
}

func (d *PublicDeployer) createSubnetTx(controlKeys []string, threshold uint32, wallet primary.Wallet) (ids.ID, error) {
	err := d.validateWalletIsSubnetOwner(controlKeys, threshold)
	if err != nil {
		return ids.Empty, err
	}
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

func (d *PublicDeployer) validateWalletIsSubnetOwner(controlKeys []string, threshold uint32) error {
	if threshold != 1 {
		return fmt.Errorf("multisig subnets are currently unsupported")
	}

	walletAddrs := d.kc.Addresses().List()
	if len(walletAddrs) == 0 {
		return fmt.Errorf("no addrs in wallet")
	}
	walletAddr := walletAddrs[0]

	networkID, err := d.network.NetworkID()
	if err != nil {
		return err
	}
	hrp := key.GetHRP(networkID)
	walletAddrStr, err := address.Format("P", hrp, walletAddr[:])
	if err != nil {
		return err
	}

	for _, addr := range controlKeys {
		if addr == walletAddrStr {
			return nil
		}
	}

	return fmt.Errorf("wallet addr not listed in subnet owners")
}
