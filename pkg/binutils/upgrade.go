package binutils

import (
	"fmt"
	"path"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/localnetworkinterface"
)

func UpgradeVM(app *application.Avalanche, vmID string, vmBinPath string) error {
	// Need to determine plugin directory from currently running avago
	nc := localnetworkinterface.NewStatusChecker()
	runningAvagoVersion, _, _, err := nc.GetCurrentNetworkVersion()
	if err != nil {
		return fmt.Errorf("failed to get running network info: %w", err)
	}

	pluginDir := path.Join(app.GetAvalanchegoBinDir(), "avalanchego-"+runningAvagoVersion, "plugins")

	// shut down network

	installer := NewPluginBinaryDownloader(app)
	if err = installer.UpgradeVM(vmID, vmBinPath, pluginDir); err != nil {
		return fmt.Errorf("failed to upgrade vm: %w", err)
	}

	// restart network

	return nil
}
