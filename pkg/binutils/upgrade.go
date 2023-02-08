package binutils

import (
	"fmt"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"go.uber.org/zap"
)

func UpgradeVM(app *application.Avalanche, vmID string, vmBinPath string) error {
	// shut down network
	// save a temporary snapshot
	snapName := constants.TmpSnapshotName + time.Now().Format(constants.TimestampFormat)
	app.Log.Debug("saving temporary snapshot for upgrade bytes", zap.String("snapshot-name", snapName))
	cli, err := NewGRPCClient()
	if err != nil {
		return err
	}
	ctx := GetAsyncContext()
	_, err = cli.SaveSnapshot(ctx, snapName)

	if err != nil {
		return err
	}
	app.Log.Debug("network stopped and named temporary snapshot created. Applying update")

	installer := NewPluginBinaryDownloader(app)
	if err = installer.UpgradeVM(vmID, vmBinPath); err != nil {
		return fmt.Errorf("failed to upgrade vm: %w", err)
	}

	app.Log.Debug("restarting network")

	// restart network
	_, err = cli.LoadSnapshot(ctx, snapName)
	if err != nil {
		return err
	}

	_, err = cli.WaitForHealthy(ctx)
	if err != nil {
		return fmt.Errorf("failed waiting for network to become healthy: %w", err)
	}

	return nil
}
