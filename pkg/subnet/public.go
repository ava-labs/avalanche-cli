package subnet

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/app"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/wallet"
	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/ava-labs/avalanchego/ids"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
)

type DeployLocation int

const (
	Local DeployLocation = iota
	Remote
)

type PublicSubnetDeployer struct {
	LocalSubnetDeployer
	baseDir     string
	privKeyPath string
	network     models.Network
	log         logging.Logger
}

func NewPublicSubnetDeployer(app *app.Avalanche, privKeyPath string, network models.Network) *PublicSubnetDeployer {
	return &PublicSubnetDeployer{
		LocalSubnetDeployer: *NewLocalSubnetDeployer(app),
		baseDir:             app.GetBaseDir(),
		privKeyPath:         privKeyPath,
		log:                 app.Log,
		network:             network,
	}
}

func (d *PublicSubnetDeployer) Deploy(controlKeys []string, threshold uint32) error {
	wallet, err := d.loadWallet()
	if err != nil {
		return err
	}

	subnetID, err := d.createSubnetTx(controlKeys, threshold, wallet)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser(subnetID.String())

	return nil
}

func (d *PublicSubnetDeployer) createBlockchainTx(chainName string, vmID, subnetID ids.ID, genesis []byte, wallet primary.Wallet) (ids.ID, error) {
	// TODO
	options := []common.Option{}
	fxIDs := make([]ids.ID, 0)
	return wallet.P().IssueCreateChainTx(subnetID, genesis, vmID, fxIDs, chainName, options...)
}

func (d *PublicSubnetDeployer) loadWallet() (primary.Wallet, error) {
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
		return nil, fmt.Errorf("Unsupported public network")
	}

	sf, err := wallet.LoadSoft(networkID, d.privKeyPath)
	if err != nil {
		return nil, err
	}

	kc := sf.KeyChain()

	walet, err := primary.NewWalletFromURI(ctx, api, kc)
	if err != nil {
		return nil, err
	}
	return walet, nil
}

func (d *PublicSubnetDeployer) createSubnetTx(controlKeys []string, threshold uint32, wallet primary.Wallet) (ids.ID, error) {
	var err error

	addrs := make([]ids.ShortID, len(controlKeys))
	for i, c := range controlKeys {
		addrs[i], err = address.ParseToID(c)
		if err != nil {
			return ids.Empty, err
		}
	}
	owners := &secp256k1fx.OutputOwners{
		Addrs:     addrs,
		Threshold: threshold,
		Locktime:  0,
	}
	opts := []common.Option{}
	return wallet.P().IssueCreateSubnetTx(owners, opts...)
}

func (d *PublicSubnetDeployer) StartValidator(chain, chain_genesis string, location DeployLocation) error {
	switch location {
	case Local:
		if err := d.startLocalNode(chain, chain_genesis); err != nil {
			return err
		}
	default:
		return fmt.Errorf("currently, only locally running avalanchego nodes supported")
	}
	return nil
}

func (d *PublicSubnetDeployer) startLocalNode(chain, chain_genesis string) error {
	avalancheGoBinPath, pluginDir, err := d.LocalSubnetDeployer.setupBinaries(chain, chain_genesis)
	if err != nil {
		return err
	}
	buildDir := filepath.Base(pluginDir)
	args := []string{"--network-id", "fuji", "--build-dir", buildDir}
	startCmd := exec.Command(avalancheGoBinPath, args...)
	fmt.Println("starting local fuji node...")
	outputFile, err := os.CreateTemp("", "startCmd*")
	if err != nil {
		return err
	}
	fmt.Println(outputFile.Name())
	// TODO: should this be redirected to this app's log file instead?
	startCmd.Stdout = outputFile
	startCmd.Stderr = outputFile
	if err := startCmd.Start(); err != nil {
		return err
	}

	return nil
}
