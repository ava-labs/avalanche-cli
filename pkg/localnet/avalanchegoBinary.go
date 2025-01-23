package localnet

import (
	"fmt"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
)

// * checks if avalanchego is installed in the local binary path
// * if not, it downloads and installs it (os - and archive dependent)
// * returns the location of the avalanchego path
func SetupAvalancheGoBinary(
	app *application.Avalanche,
	avalancheGoVersion string,
	avalancheGoBinaryPath string,
) (string, error) {
	if avalancheGoBinaryPath == "" {
		_, avalancheGoDir, err := binutils.SetupAvalanchego(app, avalancheGoVersion)
		if err != nil {
			return "", fmt.Errorf("failed setting up avalanchego binary: %w", err)
		}
		avalancheGoBinaryPath = filepath.Join(avalancheGoDir, "avalanchego")
	}
	if !utils.IsExecutable(avalancheGoBinaryPath) {
		return "", fmt.Errorf("avalancheGo binary %s does not exist", avalancheGoBinaryPath)
	}
	return avalancheGoBinaryPath, nil
}
