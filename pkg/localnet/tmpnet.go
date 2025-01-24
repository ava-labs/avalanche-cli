package localnet

import (
	"encoding/json"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/genesis"
	avagonode "github.com/ava-labs/avalanchego/node"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/ava-labs/avalanchego/utils/logging"

	dircopy "github.com/otiai10/copy"
)

type BootstrappingStatus int64

const (
	UndefinedBootstrappingStatus BootstrappingStatus = iota
	NotBootstrapped
	PartiallyBootstrapped
	FullyBootstrapped
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
	err := network.Bootstrap(
		ctx,
		log,
	)
	return network, err
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

func TmpNetLoad(
	log logging.Logger,
	networkDir string,
) (*tmpnet.Network, error) {
	network, err := tmpnet.ReadNetwork(networkDir)
	if err != nil {
		return nil, err
	}
	ctx, cancel := GetDefaultTimeout()
	defer cancel()
	err = network.StartNodes(ctx, log, network.Nodes...)
	return network, err
}

func TmpNetStop(
	networkDir string,
) error {
	ctx, cancel := GetDefaultTimeout()
	defer cancel()
	return tmpnet.StopNetwork(ctx, networkDir)
}

func TmpNetBootstrappingStatus(networkDir string) (BootstrappingStatus, error) {
	status := UndefinedBootstrappingStatus
	network, err := tmpnet.ReadNetwork(networkDir)
	if err != nil {
		return status, err
	}
	bootstrappedCount := 0
	for _, node := range network.Nodes {
		processPath := filepath.Join(networkDir, node.NodeID.String(), config.DefaultProcessContextFilename)
		if utils.FileExists(processPath) {
			bs, err := os.ReadFile(processPath)
			if err != nil {
				return status, fmt.Errorf("failed to read node process context at %s: %w", processPath, err)
			}
			processContext := avagonode.ProcessContext{}
			if err := json.Unmarshal(bs, &processContext); err != nil {
				return status, fmt.Errorf("failed to unmarshal node process context at %s: %w", processPath, err)
			}
			if _, err := utils.GetProcess(processContext.PID); err == nil {
				status = PartiallyBootstrapped
				bootstrappedCount++
			}
		}
	}
	switch bootstrappedCount {
	case 0:
		return NotBootstrapped, nil
	case len(network.Nodes):
		return FullyBootstrapped, nil
	default:
		return status, nil
	}
}
