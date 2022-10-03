package plugins

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/utils/logging"
)

func getVMID(sc models.Sidecar) (string, error) {
	// get vmid
	var vmid string
	if sc.ImportedFromAPM {
		vmid = sc.ImportedVMID
	} else {
		chainVMID, err := utils.VMID(sc.Name)
		if err != nil {
			return "", err
		}
		vmid = chainVMID.String()
	}
	return vmid, nil
}

func ManualUpgrade(app *application.Avalanche, sc models.Sidecar, targetVersion string) error {
	vmid, err := getVMID(sc)
	if err != nil {
		return err
	}
	pluginDir := app.GetTmpPluginDir()
	vmPath, err := CreatePluginFromVersion(app, sc.Name, sc.VM, targetVersion, vmid, pluginDir)
	if err != nil {
		return err
	}
	printUpgradeCmd(vmPath)
	return nil
}

func AutomatedUpgrade(app *application.Avalanche, sc models.Sidecar, targetVersion string, pluginDir string) error {
	// Attempt an automated update
	var err error
	if pluginDir == "" {
		pluginDir = FindPluginDir()
		if pluginDir != "" {
			ux.Logger.PrintToUser(logging.Bold.Wrap(logging.Green.Wrap("Found the VM plugin directory at %s")), pluginDir)
			yes, err := app.Prompt.CaptureYesNo("Is this where we should upgrade the VM?")
			if err != nil {
				return err
			}
			if yes {
				ux.Logger.PrintToUser("Will use plugin directory at %s to upgrade the VM", pluginDir)
			} else {
				pluginDir = ""
			}
		}
		if pluginDir == "" {
			pluginDir, err = app.Prompt.CaptureString("Path to your avalanchego plugin dir (likely avalanchego/build/plugins)")
			if err != nil {
				return err
			}
		}
	}

	pluginDir, err = SanitizePath(pluginDir)
	if err != nil {
		return err
	}

	vmid, err := getVMID(sc)
	if err != nil {
		return err
	}
	vmPath, err := CreatePluginFromVersion(app, sc.Name, sc.VM, targetVersion, vmid, pluginDir)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("VM binary written to %s", vmPath)

	return nil
}

func printUpgradeCmd(vmPath string) {
	msg := `
To upgrade your node, you must do two things:

1. Replace your VM binary in your node's plugin directory
2. Restart your node

To add the VM to your plugin directory, copy or scp from %s

If you installed avalanchego manually, your plugin directory is likely
avalanchego/build/plugins.
`

	ux.Logger.PrintToUser(msg, vmPath)
}
