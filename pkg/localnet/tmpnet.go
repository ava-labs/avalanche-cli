package localnet

import (
	"encoding/json"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/ids"
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
	avalancheGoBinPath string,
) (*tmpnet.Network, error) {
	network, err := tmpnet.ReadNetwork(networkDir)
	if err != nil {
		return nil, err
	}
	ctx, cancel := GetDefaultTimeout()
	defer cancel()
	nodes := network.Nodes
	for i := range nodes {
		nodes[i].RuntimeConfig = &tmpnet.NodeRuntimeConfig{
			AvalancheGoPath: avalancheGoBinPath,
		}
	}
	err = network.StartNodes(ctx, log, nodes...)
	return network, err
}

func TmpNetStop(
	networkDir string,
) error {
	ctx, cancel := GetDefaultTimeout()
	defer cancel()
	return tmpnet.StopNetwork(ctx, networkDir)
}

func GetTmpNetBootstrappingStatus(networkDir string) (BootstrappingStatus, error) {
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

func GetTmpNetNetwork(networkDir string) (*tmpnet.Network, error) {
	return tmpnet.ReadNetwork(networkDir)
}

func GetTmpNetworkEndpoint(networkDir string) (string, error) {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return "", err
	}
	if len(network.Nodes) == 0 {
		return "", fmt.Errorf("no node found on local network at %s", networkDir)
	}
	return network.Nodes[0].URI, nil
}

type BlockchainInfo struct{
	Name string
	ID ids.ID
	SubnetID ids.ID
	VMID ids.ID
}

func GetTmpNetworkBlockchainInfo(networkDir string) ([]BlockchainInfo, error) {
	endpoint, err := GetTmpNetworkEndpoint(networkDir)
	if err != nil {
		return nil, err
	}
	pClient := platformvm.NewClient(endpoint)
	ctx, cancel := sdkutils.GetAPIContext()
	defer cancel()
	blockchains, err := pClient.GetBlockchains(ctx)
	if err != nil {
		return nil, err
	}
	blockchainsInfo := []BlockchainInfo{}
	for _, blockchain := range blockchains {
		if blockchain.Name == "C-Chain" || blockchain.Name == "X-Chain" {
			continue
		}
		blockchainInfo := BlockchainInfo{
			Name: blockchain.Name,
			ID:   blockchain.ID,
			SubnetID:  blockchain.SubnetID,
			VMID:      blockchain.VMID,
		}
		blockchainsInfo = append(blockchainsInfo, blockchainInfo)
	}
	return blockchainsInfo, nil
}
