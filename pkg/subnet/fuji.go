package subnet

import (
	"fmt"
	"os/exec"

	"github.com/ava-labs/avalanche-cli/pkg/app"
	"github.com/ava-labs/avalanchego/utils/logging"
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
	switch location {
	case Local:
		if err := d.startLocalNode(chain, chain_genesis); err != nil {
			return err
		}
	default:
		return fmt.Errorf("Only locally running avalanchego nodes supported")
	}
	return nil
}

func (d *FujiSubnetDeployer) startLocalNode(chain, chain_genesis string) error {
	avalancheGoBinPath, pluginDir, err := d.LocalSubnetDeployer.setupBinaries(chain, chain_genesis)
	if err != nil {
		return err
	}
	args := []string{"--network", "fuji", "--plugin-dir", pluginDir}
	exec.Command(avalancheGoBinPath, args...)

	return nil
}
