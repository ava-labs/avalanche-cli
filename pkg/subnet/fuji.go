package subnet

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/app"
	"github.com/ava-labs/avalanche-cli/pkg/wallet"
	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
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

type FujiSubnetDeployer struct {
	LocalSubnetDeployer
	baseDir string
	log     logging.Logger
}

func NewFujiSubnetDeployer(app *app.Avalanche) *FujiSubnetDeployer {
	return &FujiSubnetDeployer{
		LocalSubnetDeployer: *NewLocalSubnetDeployer(app),
		baseDir:             app.GetBaseDir(),
		log:                 app.Log,
	}
}

func (d *FujiSubnetDeployer) DeployToFuji(chain, chain_genesis string, location DeployLocation) error {
	txID, err := d.createSubnetTx()
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser(txID.String())
	return nil
}

func (d *FujiSubnetDeployer) createSubnetTx() (ids.ID, error) {
	uri := "https://api.avax-test.network"
	ctx := context.Background()
	keypath := "/tmp/fabkey"
	netID := constants.FujiID
	sf, err := wallet.LoadSoft(netID, keypath)
	if err != nil {
		return ids.Empty, err
	}
	kc := sf.KeyChain()
	walet, err := primary.NewWalletFromURI(ctx, uri, kc)
	if err != nil {
		return ids.Empty, err
	}
	// TODO empty owner => no controlkeys
	owner := &secp256k1fx.OutputOwners{}
	opts := []common.Option{}
	tx, err := walet.P().IssueCreateSubnetTx(owner, opts...)
	if err != nil {
		return ids.Empty, err
	}
	return tx, nil
}

func (d *FujiSubnetDeployer) StartValidator(chain, chain_genesis string, location DeployLocation) error {
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

func (d *FujiSubnetDeployer) startLocalNode(chain, chain_genesis string) error {
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
