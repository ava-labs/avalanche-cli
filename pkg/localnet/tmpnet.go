package localnet

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/ava-labs/avalanchego/utils/logging"

	dircopy "github.com/otiai10/copy"
)

func TmpNetCreate(
	log logging.Logger,
	rootDir string,
	avalancheGoBinPath string,
	pluginDir string,
	nodes []*tmpnet.Node,
	defaultFlags map[string]interface{},
	genesis *genesis.UnparsedConfig,
	upgradeBytes []byte,
) (*tmpnet.Network, error) {
	defaultFlags[config.UpgradeFileContentKey] = base64.StdEncoding.EncodeToString(upgradeBytes)
	network := &tmpnet.Network{
		Nodes:        nodes,
		Dir:          rootDir,
		DefaultFlags: defaultFlags,
		Genesis:      genesis,
	}
	if err := network.EnsureDefaultConfig(log, avalancheGoBinPath, pluginDir); err != nil {
		return nil, err
	}
	if err := network.Write(); err != nil {
		return nil, err
	}
	ctx, cancel := GetDefaultTimeout()
	defer cancel()
	return network, network.Bootstrap(
		ctx,
		log,
	)
}

func TmpNetMigrate(
	oldDir string,
	newDir string,
) error {
	if err := dircopy.Copy(oldDir, newDir); err != nil {
		return fmt.Errorf("failure storing network at %s onto %s: %w", oldDir, newDir, err)
	}
	entries, err := os.ReadDir(newDir)
	if err != nil {
		return fmt.Errorf("failed to read config dir %s: %w", newDir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		flagsFile := filepath.Join(newDir, entry.Name(), "flags.json")
		if utils.FileExists(flagsFile) {
			data, err := utils.ReadJSON(flagsFile)
			if err != nil {
				return err
			}
			data[config.ChainConfigDirKey] = filepath.Join(newDir, "chains")
			data[config.DataDirKey] = filepath.Join(newDir, entry.Name())
			data[config.GenesisFileKey] = filepath.Join(newDir, "genesis.json")
			if err := utils.WriteJSON(flagsFile, data); err != nil {
				return err
			}
		}
	}
	return nil
}
