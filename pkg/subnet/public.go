package subnet

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
)

type PublicDeployer struct {
	LocalSubnetDeployer
	privKeyPath string
	network     models.Network
	app         *application.Avalanche
}

func NewPublicDeployer(app *application.Avalanche, privKeyPath string, network models.Network) *PublicDeployer {
	return &PublicDeployer{
		LocalSubnetDeployer: *NewLocalSubnetDeployer(app),
		app:                 app,
		privKeyPath:         privKeyPath,
		network:             network,
	}
}

func (d *PublicDeployer) Deploy(controlKeys []string, threshold uint32, chain, genesis string) (ids.ID, ids.ID, error) {
	wallet, api, err := d.loadWallet()
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
	ux.Logger.PrintToUser(subnetID.String())

	blockchainID, err := d.createBlockchainTx(chain, vmID, subnetID, []byte(genesis), wallet)
	if err != nil {
		return ids.Empty, ids.Empty, err
	}
	ux.Logger.PrintToUser("Endpoint for blockchain %q with VM ID %q: %s/ext/bc/%s/rpc", blockchainID.String(), vmID.String(), api, blockchainID.String())
	return subnetID, blockchainID, nil
}

func (d *PublicDeployer) loadWallet() (primary.Wallet, string, error) {
	ctx := context.Background()

	var (
		api       string
		networkID uint32
	)

	switch d.network {
	case models.Fuji:
		api = constants.FujiAPIEndpoint
		networkID = avago_constants.FujiID
	case models.Mainnet:
		api = constants.MainnetAPIEndpoint
		networkID = avago_constants.MainnetID
	default:
		return nil, "", fmt.Errorf("unsupported public network")
	}

	sf, err := key.LoadSoft(networkID, d.privKeyPath)
	if err != nil {
		return nil, "", err
	}

	kc := sf.KeyChain()

	wallet, err := primary.NewWalletFromURI(ctx, api, kc)
	if err != nil {
		return nil, "", err
	}
	return wallet, api, nil
}

func (d *PublicDeployer) createBlockchainTx(chainName string, vmID, subnetID ids.ID, genesis []byte, wallet primary.Wallet) (ids.ID, error) {
	// TODO do we need any of these to be set?
	options := []common.Option{}
	fxIDs := make([]ids.ID, 0)
	return wallet.P().IssueCreateChainTx(subnetID, genesis, vmID, fxIDs, chainName, options...)
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
	return wallet.P().IssueCreateSubnetTx(owners, opts...)
}
