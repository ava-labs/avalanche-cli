// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/mod/semver"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanche-network-runner/server"
	anrutils "github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/storage"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"go.uber.org/zap"
)

type LocalDeployer struct {
	procChecker        binutils.ProcessChecker
	binChecker         binutils.BinaryChecker
	getClientFunc      getGRPCClientFunc
	binaryDownloader   binutils.PluginBinaryDownloader
	app                *application.Avalanche
	backendStartedHere bool
	setDefaultSnapshot setDefaultSnapshotFunc
	avagoVersion       string
	avagoBinaryPath    string
	vmBin              string
}

// uses either avagoVersion or avagoBinaryPath
func NewLocalDeployer(
	app *application.Avalanche,
	avagoVersion string,
	avagoBinaryPath string,
	vmBin string,
) *LocalDeployer {
	return &LocalDeployer{
		procChecker:        binutils.NewProcessChecker(),
		binChecker:         binutils.NewBinaryChecker(),
		getClientFunc:      binutils.NewGRPCClient,
		binaryDownloader:   binutils.NewPluginBinaryDownloader(app),
		app:                app,
		setDefaultSnapshot: SetDefaultSnapshot,
		avagoVersion:       avagoVersion,
		avagoBinaryPath:    avagoBinaryPath,
		vmBin:              vmBin,
	}
}

type getGRPCClientFunc func(...binutils.GRPCClientOpOption) (client.Client, error)

type setDefaultSnapshotFunc func(string, bool, string, bool) (bool, error)

type ICMSpec struct {
	SkipICMDeploy                bool
	SkipRelayerDeploy            bool
	Version                      string
	MessengerContractAddressPath string
	MessengerDeployerAddressPath string
	MessengerDeployerTxPath      string
	RegistryBydecodePath         string
}

type DeployInfo struct {
	SubnetID            ids.ID
	BlockchainID        ids.ID
	ICMMessengerAddress string
	ICMRegistryAddress  string
}

// DeployToLocalNetwork does the heavy lifting:
// * it checks the gRPC is running, if not, it starts it
// * kicks off the actual deployment
func (d *LocalDeployer) DeployToLocalNetwork(
	chain string,
	genesisPath string,
	icmSpec ICMSpec,
	subnetIDStr string,
) (*DeployInfo, error) {
	if err := d.StartServer(); err != nil {
		return nil, err
	}
	return d.doDeploy(chain, genesisPath, icmSpec, subnetIDStr)
}

func (d *LocalDeployer) StartServer() error {
	isRunning, err := d.procChecker.IsServerProcessRunning(d.app)
	if err != nil {
		return fmt.Errorf("failed querying if server process is running: %w", err)
	}
	if !isRunning {
		d.app.Log.Debug("gRPC server is not running")
		if err := binutils.StartServerProcess(d.app); err != nil {
			return fmt.Errorf("failed starting gRPC server process: %w", err)
		}
		d.backendStartedHere = true
	}
	return nil
}

func GetCurrentSupply(subnetID ids.ID) error {
	api := constants.LocalAPIEndpoint
	pClient := platformvm.NewClient(api)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	_, _, err := pClient.GetCurrentSupply(ctx, subnetID)
	return err
}

// BackendStartedHere returns true if the backend was started by this run,
// or false if it found it there already
func (d *LocalDeployer) BackendStartedHere() bool {
	return d.backendStartedHere
}

// doDeploy the actual deployment to the network runner
// steps:
//   - checks if the network has been started
//   - install all needed plugin binaries, for the the new VM, and the already deployed VMs
//   - either starts a network from the default snapshot if not started,
//     or restarts the already available network while preserving state
//   - waits completion of operation
//   - get from the network an available subnet ID to be used in blockchain creation
//   - deploy a new blockchain for the given VM ID, genesis, and available subnet ID
//   - waits completion of operation
//   - show status
func (d *LocalDeployer) doDeploy(chain string, genesisPath string, icmSpec ICMSpec, subnetIDStr string) (*DeployInfo, error) {
	needsRestart, avalancheGoBinPath, err := d.SetupLocalEnv()
	if err != nil {
		return nil, err
	}

	backendLogFile, err := binutils.GetBackendLogFile(d.app)
	var backendLogDir string
	if err == nil {
		// TODO should we do something if there _was_ an error?
		backendLogDir = filepath.Dir(backendLogFile)
	}

	cli, err := d.getClientFunc()
	if err != nil {
		return nil, fmt.Errorf("error creating gRPC Client: %w", err)
	}
	defer cli.Close()

	ctx, cancel := utils.GetANRContext()
	defer cancel()

	// loading sidecar before it's needed so we catch any error early
	sc, err := d.app.LoadSidecar(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to load sidecar: %w", err)
	}

	// check for network status
	networkBooted := true
	clusterInfo, err := WaitForHealthy(ctx, cli)
	logRootDir := clusterInfo.GetLogRootDir()
	if err != nil {
		if !server.IsServerError(err, server.ErrNotBootstrapped) {
			FindErrorLogs(logRootDir, backendLogDir)
			return nil, fmt.Errorf("failed to query network health: %w", err)
		} else {
			networkBooted = false
		}
	}

	chainVMID, err := anrutils.VMID(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM ID from %s: %w", chain, err)
	}
	d.app.Log.Debug("this VM will get ID", zap.String("vm-id", chainVMID.String()))

	if sc.RunRelayer && !icmSpec.SkipRelayerDeploy {
		// relayer stop/cleanup is neeeded in the case it is registered to blockchains
		// if not, network restart fails
		if err := teleporter.RelayerCleanup(
			d.app.GetLocalRelayerRunPath(models.Local),
			d.app.GetLocalRelayerStorageDir(models.Local),
		); err != nil {
			return nil, err
		}
	}

	if networkBooted && needsRestart {
		ux.Logger.PrintToUser("Restarting the network...")
		if _, err := cli.Stop(ctx); err != nil {
			return nil, fmt.Errorf("failed to stop network: %w", err)
		}
		if err := d.app.ResetPluginsDir(); err != nil {
			return nil, fmt.Errorf("failed to reset plugins dir: %w", err)
		}
		networkBooted = false
	}

	if !networkBooted {
		if err := d.startNetwork(ctx, cli, avalancheGoBinPath); err != nil {
			FindErrorLogs(logRootDir, backendLogDir)
			return nil, err
		}
	}

	// latest check for rpc compatibility
	statusChecker := localnet.NewStatusChecker()
	_, avagoRPCVersion, _, err := statusChecker.GetCurrentNetworkVersion()
	if err != nil {
		return nil, err
	}
	if avagoRPCVersion != sc.RPCVersion {
		if !networkBooted {
			_, _ = cli.Stop(ctx)
		}
		return nil, fmt.Errorf(
			"the avalanchego deployment uses rpc version %d but your subnet has version %d and is not compatible",
			avagoRPCVersion,
			sc.RPCVersion,
		)
	}

	// get VM info
	clusterInfo, err = WaitForHealthy(ctx, cli)
	if err != nil {
		FindErrorLogs(clusterInfo.GetLogRootDir(), backendLogDir)
		return nil, fmt.Errorf("failed to query network health: %w", err)
	}
	logRootDir = clusterInfo.GetLogRootDir()

	if alreadyDeployed(chainVMID, clusterInfo) {
		return nil, fmt.Errorf("subnet %s has already been deployed", chain)
	}

	numBlockchains := len(clusterInfo.CustomChains)

	subnetIDs := maps.Keys(clusterInfo.Subnets)

	// in order to make subnet deploy faster, a set of validated subnet IDs is preloaded
	// in the bootstrap snapshot
	// we select one to be used for creating the next blockchain, for that we use the
	// number of currently created blockchains as the index to select the next subnet ID,
	// so we get incremental selection
	sort.Strings(subnetIDs)
	if len(subnetIDs) == 0 {
		return nil, errors.New("the network has not preloaded subnet IDs")
	}

	// If not set via argument, deploy to the next available subnet
	if subnetIDStr == "" {
		subnetIDStr = subnetIDs[numBlockchains%len(subnetIDs)]
	}

	// if a chainConfig has been configured
	var (
		chainConfig            string
		chainConfigFile        = filepath.Join(d.app.GetSubnetDir(), chain, constants.ChainConfigFileName)
		perNodeChainConfig     string
		perNodeChainConfigFile = filepath.Join(d.app.GetSubnetDir(), chain, constants.PerNodeChainConfigFileName)
		subnetConfig           string
		subnetConfigFile       = filepath.Join(d.app.GetSubnetDir(), chain, constants.SubnetConfigFileName)
	)
	if _, err := os.Stat(chainConfigFile); err == nil {
		// currently the ANR only accepts the file as a path, not its content
		chainConfig = chainConfigFile
	}
	if _, err := os.Stat(perNodeChainConfigFile); err == nil {
		perNodeChainConfig = perNodeChainConfigFile
	}
	if _, err := os.Stat(subnetConfigFile); err == nil {
		subnetConfig = subnetConfigFile
	}

	// install the plugin binary for the new VM
	if err := d.installPlugin(chainVMID, d.vmBin); err != nil {
		return nil, err
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Deploying Blockchain. Wait until network acknowledges...")

	// create a new blockchain on the already started network, associated to
	// the given VM ID, genesis, and available subnet ID
	blockchainSpecs := []*rpcpb.BlockchainSpec{
		{
			VmName:   chain,
			Genesis:  genesisPath,
			SubnetId: &subnetIDStr,
			SubnetSpec: &rpcpb.SubnetSpec{
				SubnetConfig: subnetConfig,
			},
			ChainConfig:        chainConfig,
			BlockchainAlias:    chain,
			PerNodeChainConfig: perNodeChainConfig,
		},
	}
	deployBlockchainsInfo, err := cli.CreateBlockchains(
		ctx,
		blockchainSpecs,
	)
	if err != nil {
		FindErrorLogs(logRootDir, backendLogDir)
		pluginRemoveErr := d.removeInstalledPlugin(chainVMID)
		if pluginRemoveErr != nil {
			ux.Logger.PrintToUser("Failed to remove plugin binary: %s", pluginRemoveErr)
		}
		return nil, fmt.Errorf("failed to deploy blockchain: %w", err)
	}
	logRootDir = clusterInfo.GetLogRootDir()

	d.app.Log.Debug(deployBlockchainsInfo.String())

	clusterInfo, err = WaitForHealthy(ctx, cli)
	if err != nil {
		FindErrorLogs(logRootDir, backendLogDir)
		pluginRemoveErr := d.removeInstalledPlugin(chainVMID)
		if pluginRemoveErr != nil {
			ux.Logger.PrintToUser("Failed to remove plugin binary: %s", pluginRemoveErr)
		}
		return nil, fmt.Errorf("failed to query network health: %w", err)
	}

	var (
		icmMessengerAddress string
		icmRegistryAddress  string
	)
	if sc.TeleporterReady && !icmSpec.SkipICMDeploy {
		network := models.NewLocalNetwork()
		// get relayer address
		relayerAddress, relayerPrivateKey, err := teleporter.GetRelayerKeyInfo(d.app.GetKeyPath(constants.AWMRelayerKeyName))
		if err != nil {
			return nil, err
		}
		// relayer config file
		_, relayerConfigPath, err := GetLocalNetworkRelayerConfigPath(d.app)
		if err != nil {
			return nil, err
		}
		if err = teleporter.CreateBaseRelayerConfigIfMissing(
			relayerConfigPath,
			logging.Info.LowerString(),
			d.app.GetLocalRelayerStorageDir(models.Local),
			network,
		); err != nil {
			return nil, err
		}
		// deploy C-Chain
		ux.Logger.PrintToUser("")
		icmd := teleporter.Deployer{}
		if icmSpec.MessengerContractAddressPath != "" {
			if err := icmd.SetAssetsFromPaths(
				icmSpec.MessengerContractAddressPath,
				icmSpec.MessengerDeployerAddressPath,
				icmSpec.MessengerDeployerTxPath,
				icmSpec.RegistryBydecodePath,
			); err != nil {
				return nil, err
			}
		} else {
			icmVersion := ""
			switch {
			case icmSpec.Version != "" && icmSpec.Version != "latest":
				icmVersion = icmSpec.Version
			case sc.TeleporterVersion != "":
				icmVersion = sc.TeleporterVersion
			default:
				icmInfo, err := teleporter.GetInfo(d.app)
				if err != nil {
					return nil, err
				}
				icmVersion = icmInfo.Version
			}
			if err := icmd.DownloadAssets(
				d.app.GetTeleporterBinDir(),
				icmVersion,
			); err != nil {
				return nil, err
			}
		}
		cChainKey, err := key.LoadEwoq(network.ID)
		if err != nil {
			return nil, err
		}
		cchainAlreadyDeployed, cchainIcmMessengerAddress, cchainIcmRegistryAddress, err := icmd.Deploy(
			"c-chain",
			network.BlockchainEndpoint("C"),
			cChainKey.PrivKeyHex(),
			true,
			true,
		)
		if err != nil {
			return nil, err
		}
		if !cchainAlreadyDeployed {
			if err := localnet.WriteExtraLocalNetworkData(cchainIcmMessengerAddress, cchainIcmRegistryAddress); err != nil {
				return nil, err
			}
		}
		// deploy current blockchain
		ux.Logger.PrintToUser("")
		subnetID, blockchainID, err := utils.GetChainIDs(network.Endpoint, chain)
		if err != nil {
			return nil, err
		}
		teleporterKeyName := sc.TeleporterKey
		if teleporterKeyName == "" {
			genesisData, err := d.app.LoadRawGenesis(chain)
			if err != nil {
				return nil, err
			}
			teleporterKeyName, _, _, err = GetSubnetAirdropKeyInfo(d.app, network, chain, genesisData)
			if err != nil {
				return nil, err
			}
		}
		blockchainKey, err := key.LoadSoft(network.ID, d.app.GetKeyPath(teleporterKeyName))
		if err != nil {
			return nil, err
		}
		_, icmMessengerAddress, icmRegistryAddress, err = icmd.Deploy(
			chain,
			network.BlockchainEndpoint(blockchainID),
			blockchainKey.PrivKeyHex(),
			true,
			true,
		)
		if err != nil {
			return nil, err
		}
		if sc.RunRelayer && !icmSpec.SkipRelayerDeploy {
			if !cchainAlreadyDeployed {
				if err := teleporter.FundRelayer(
					network.BlockchainEndpoint("C"),
					cChainKey.PrivKeyHex(),
					relayerAddress,
				); err != nil {
					return nil, err
				}
				cchainSubnetID, cchainBlockchainID, err := utils.GetChainIDs(network.Endpoint, "C-Chain")
				if err != nil {
					return nil, err
				}
				if err = teleporter.AddSourceAndDestinationToRelayerConfig(
					relayerConfigPath,
					network.BlockchainEndpoint(cchainBlockchainID),
					network.BlockchainWSEndpoint(cchainBlockchainID),
					cchainSubnetID,
					cchainBlockchainID,
					cchainIcmRegistryAddress,
					cchainIcmMessengerAddress,
					relayerAddress,
					relayerPrivateKey,
				); err != nil {
					return nil, err
				}
			}
			if err := teleporter.FundRelayer(
				network.BlockchainEndpoint(blockchainID),
				blockchainKey.PrivKeyHex(),
				relayerAddress,
			); err != nil {
				return nil, err
			}
			if err = teleporter.AddSourceAndDestinationToRelayerConfig(
				relayerConfigPath,
				network.BlockchainEndpoint(blockchainID),
				network.BlockchainWSEndpoint(blockchainID),
				subnetID,
				blockchainID,
				icmRegistryAddress,
				icmMessengerAddress,
				relayerAddress,
				relayerPrivateKey,
			); err != nil {
				return nil, err
			}
			ux.Logger.PrintToUser("")
			// start relayer
			if err := teleporter.DeployRelayer(
				"latest",
				d.app.GetAWMRelayerBinDir(),
				relayerConfigPath,
				d.app.GetLocalRelayerLogPath(models.Local),
				d.app.GetLocalRelayerRunPath(models.Local),
				d.app.GetLocalRelayerStorageDir(models.Local),
			); err != nil {
				return nil, err
			}
		}
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Blockchain ready to use")
	ux.Logger.PrintToUser("")

	// we can safely ignore errors here as the subnets have already been generated
	subnetID, _ := ids.FromString(subnetIDStr)
	var blockchainID ids.ID
	for _, info := range clusterInfo.CustomChains {
		if info.VmId == chainVMID.String() {
			blockchainID, _ = ids.FromString(info.ChainId)
		}
	}
	return &DeployInfo{
		SubnetID:            subnetID,
		BlockchainID:        blockchainID,
		ICMMessengerAddress: icmMessengerAddress,
		ICMRegistryAddress:  icmRegistryAddress,
	}, nil
}

// SetupLocalEnv also does some heavy lifting:
// * sets up default snapshot if not installed
// * checks if avalanchego is installed in the local binary path
// * if not, it downloads it and installs it (os - and archive dependent)
// * returns the location of the avalanchego path
func (d *LocalDeployer) SetupLocalEnv() (bool, string, error) {
	avagoVersion := ""
	avalancheGoBinPath := ""
	if d.avagoBinaryPath != "" {
		avalancheGoBinPath = d.avagoBinaryPath
		// get avago version from binary
		out, err := exec.Command(avalancheGoBinPath, "--"+config.VersionKey).Output()
		if err != nil {
			return false, "", err
		}
		fullVersion := string(out)
		splittedFullVersion := strings.Split(fullVersion, " ")
		if len(splittedFullVersion) == 0 {
			return false, "", fmt.Errorf("invalid avalanchego version: %q", fullVersion)
		}
		version := splittedFullVersion[0]
		splittedVersion := strings.Split(version, "/")
		if len(splittedVersion) != 2 {
			return false, "", fmt.Errorf("invalid avalanchego version: %q", fullVersion)
		}
		avagoVersion = "v" + splittedVersion[1]
	} else {
		var (
			avagoDir string
			err      error
		)
		avagoVersion, avagoDir, err = d.setupLocalEnv()
		if err != nil {
			return false, "", fmt.Errorf("failed setting up local environment: %w", err)
		}
		avalancheGoBinPath = filepath.Join(avagoDir, "avalanchego")
	}

	configSingleNodeEnabled := d.app.Conf.GetConfigBoolValue(constants.ConfigSingleNodeEnabledKey)
	needsRestart, err := d.setDefaultSnapshot(d.app.GetSnapshotsDir(), false, avagoVersion, configSingleNodeEnabled)
	if err != nil {
		return false, "", fmt.Errorf("failed setting up snapshots: %w", err)
	}

	pluginDir := d.app.GetPluginsDir()

	if err := os.MkdirAll(pluginDir, constants.DefaultPerms755); err != nil {
		return false, "", fmt.Errorf("could not create pluginDir %s", pluginDir)
	}

	exists, err := storage.FolderExists(pluginDir)
	if !exists || err != nil {
		return false, "", fmt.Errorf("evaluated pluginDir to be %s but it does not exist", pluginDir)
	}

	// TODO: we need some better version management here
	// * compare latest to local version
	// * decide if force update or give user choice
	exists, err = storage.FileExists(avalancheGoBinPath)
	if !exists || err != nil {
		return false, "", fmt.Errorf(
			"evaluated avalancheGoBinPath to be %s but it does not exist", avalancheGoBinPath)
	}

	return needsRestart, avalancheGoBinPath, nil
}

func (d *LocalDeployer) setupLocalEnv() (string, string, error) {
	return binutils.SetupAvalanchego(d.app, d.avagoVersion)
}

// WaitForHealthy polls continuously until the network is ready to be used
func WaitForHealthy(
	ctx context.Context,
	cli client.Client,
) (*rpcpb.ClusterInfo, error) {
	cancel := make(chan struct{})
	defer close(cancel)
	go ux.PrintWait(cancel)
	resp, err := cli.WaitForHealthy(ctx)
	if err != nil {
		return nil, err
	}
	return resp.ClusterInfo, nil
}

// GetFirstEndpoint get a human readable endpoint for the given chain
func GetFirstEndpoint(clusterInfo *rpcpb.ClusterInfo, chain string) string {
	var endpoint string
	for _, nodeInfo := range clusterInfo.NodeInfos {
		for blockchainID, chainInfo := range clusterInfo.CustomChains {
			if chainInfo.ChainName == chain && nodeInfo.Name == clusterInfo.NodeNames[0] {
				endpoint = fmt.Sprintf("Endpoint at node %s for blockchain %q with VM ID %q: %s/ext/bc/%s/rpc", nodeInfo.Name, blockchainID, chainInfo.VmId, nodeInfo.GetUri(), blockchainID)
			}
		}
	}
	return endpoint
}

// HasEndpoints returns true if cluster info contains custom blockchains
func HasEndpoints(clusterInfo *rpcpb.ClusterInfo) bool {
	return len(clusterInfo.CustomChains) > 0
}

// return true if vm has already been deployed
func alreadyDeployed(chainVMID ids.ID, clusterInfo *rpcpb.ClusterInfo) bool {
	if clusterInfo != nil {
		for _, chainInfo := range clusterInfo.CustomChains {
			if chainInfo.VmId == chainVMID.String() {
				return true
			}
		}
	}
	return false
}

// get list of all needed plugins and install them
func (d *LocalDeployer) installPlugin(
	vmID ids.ID,
	vmBin string,
) error {
	return d.binaryDownloader.InstallVM(vmID.String(), vmBin)
}

// get list of all needed plugins and install them
func (d *LocalDeployer) removeInstalledPlugin(
	vmID ids.ID,
) error {
	return d.binaryDownloader.RemoveVM(vmID.String())
}

func getSnapshotLocs(isSingleNode bool, isPreCortina17 bool, isPreDurango11 bool) (string, string, string, string) {
	bootstrapSnapshotArchiveName := ""
	url := ""
	shaSumURL := ""
	pathInShaSum := ""
	if isSingleNode {
		switch {
		case isPreCortina17:
			bootstrapSnapshotArchiveName = constants.BootstrapSnapshotSingleNodePreCortina17ArchiveName
			url = constants.BootstrapSnapshotSingleNodePreCortina17URL
			shaSumURL = constants.BootstrapSnapshotSingleNodePreCortina17SHA256URL
			pathInShaSum = constants.BootstrapSnapshotSingleNodePreCortina17LocalPath
		case isPreDurango11:
			bootstrapSnapshotArchiveName = constants.BootstrapSnapshotSingleNodePreDurango11ArchiveName
			url = constants.BootstrapSnapshotSingleNodePreDurango11URL
			shaSumURL = constants.BootstrapSnapshotSingleNodePreDurango11SHA256URL
			pathInShaSum = constants.BootstrapSnapshotSingleNodePreDurango11LocalPath
		default:
			bootstrapSnapshotArchiveName = constants.BootstrapSnapshotSingleNodeArchiveName
			url = constants.BootstrapSnapshotSingleNodeURL
			shaSumURL = constants.BootstrapSnapshotSingleNodeSHA256URL
			pathInShaSum = constants.BootstrapSnapshotSingleNodeLocalPath
		}
	} else {
		switch {
		case isPreCortina17:
			bootstrapSnapshotArchiveName = constants.BootstrapSnapshotPreCortina17ArchiveName
			url = constants.BootstrapSnapshotPreCortina17URL
			shaSumURL = constants.BootstrapSnapshotPreCortina17SHA256URL
			pathInShaSum = constants.BootstrapSnapshotPreCortina17LocalPath
		case isPreDurango11:
			bootstrapSnapshotArchiveName = constants.BootstrapSnapshotPreDurango11ArchiveName
			url = constants.BootstrapSnapshotPreDurango11URL
			shaSumURL = constants.BootstrapSnapshotPreDurango11SHA256URL
			pathInShaSum = constants.BootstrapSnapshotPreDurango11LocalPath
		default:
			bootstrapSnapshotArchiveName = constants.BootstrapSnapshotArchiveName
			url = constants.BootstrapSnapshotURL
			shaSumURL = constants.BootstrapSnapshotSHA256URL
			pathInShaSum = constants.BootstrapSnapshotLocalPath
		}
	}
	return bootstrapSnapshotArchiveName, url, shaSumURL, pathInShaSum
}

func getExpectedDefaultSnapshotSHA256Sum(isSingleNode bool, isPreCortina17 bool, isPreDurango11 bool) (string, error) {
	_, _, url, path := getSnapshotLocs(isSingleNode, isPreCortina17, isPreDurango11)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed downloading sha256 sums: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed downloading sha256 sums: unexpected http status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	sha256FileBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed downloading sha256 sums: %w", err)
	}
	expectedSum, err := utils.SearchSHA256File(sha256FileBytes, path)
	if err != nil {
		return "", fmt.Errorf("failed obtaining snapshot sha256 sum: %w", err)
	}
	return expectedSum, nil
}

// Initialize default snapshot with bootstrap snapshot archive
// If force flag is set to true, overwrite the default snapshot if it exists
func SetDefaultSnapshot(snapshotsDir string, resetCurrentSnapshot bool, avagoVersion string, isSingleNode bool) (bool, error) {
	var (
		isPreCortina17 bool
		isPreDurango11 bool
	)
	if avagoVersion != "" {
		isPreCortina17 = semver.Compare(avagoVersion, constants.Cortina17Version) < 0
		isPreDurango11 = semver.Compare(avagoVersion, constants.Durango11Version) < 0
	}
	bootstrapSnapshotArchiveName, url, _, _ := getSnapshotLocs(isSingleNode, isPreCortina17, isPreDurango11)
	currentBootstrapNamePath := filepath.Join(snapshotsDir, constants.CurrentBootstrapNamePath)
	exists, err := storage.FileExists(currentBootstrapNamePath)
	if err != nil {
		return false, err
	}
	if exists {
		currentBootstrapNameBytes, err := os.ReadFile(currentBootstrapNamePath)
		if err != nil {
			return false, err
		}
		currentBootstrapName := string(currentBootstrapNameBytes)
		if currentBootstrapName != bootstrapSnapshotArchiveName {
			// there is a snapshot image change.
			resetCurrentSnapshot = true
		}
	} else {
		// we have no ref of currently used snapshot image
		resetCurrentSnapshot = true
	}
	bootstrapSnapshotArchivePath := filepath.Join(snapshotsDir, bootstrapSnapshotArchiveName)
	defaultSnapshotPath := filepath.Join(snapshotsDir, "anr-snapshot-"+constants.DefaultSnapshotName)
	defaultSnapshotInUse := false
	if _, err := os.Stat(defaultSnapshotPath); err == nil {
		defaultSnapshotInUse = true
	}
	// will download either if file not exists or if sha256 sum is not the same
	downloadSnapshot := false
	if _, err := os.Stat(bootstrapSnapshotArchivePath); os.IsNotExist(err) {
		downloadSnapshot = true
	} else {
		gotSum, err := utils.GetSHA256FromDisk(bootstrapSnapshotArchivePath)
		if err != nil {
			return false, err
		}
		expectedSum, err := getExpectedDefaultSnapshotSHA256Sum(isSingleNode, isPreCortina17, isPreDurango11)
		if err != nil {
			ux.Logger.PrintToUser("Warning: failure verifying that the local snapshot is the latest one: %s", err)
		} else if gotSum != expectedSum {
			downloadSnapshot = true
		}
	}
	if downloadSnapshot {
		resp, err := http.Get(url)
		if err != nil {
			return false, fmt.Errorf("failed downloading bootstrap snapshot: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("failed downloading bootstrap snapshot: unexpected http status code: %d", resp.StatusCode)
		}
		defer resp.Body.Close()
		bootstrapSnapshotBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return false, fmt.Errorf("failed downloading bootstrap snapshot: %w", err)
		}
		if err := os.WriteFile(bootstrapSnapshotArchivePath, bootstrapSnapshotBytes, constants.WriteReadReadPerms); err != nil {
			return false, fmt.Errorf("failed writing down bootstrap snapshot: %w", err)
		}
		if defaultSnapshotInUse {
			ux.Logger.PrintToUser(logging.Yellow.Wrap("A new network snapshot image is available. Replacing the current one."))
		}
		resetCurrentSnapshot = true
	}
	if resetCurrentSnapshot {
		if err := os.RemoveAll(defaultSnapshotPath); err != nil {
			return false, fmt.Errorf("failed removing default snapshot: %w", err)
		}
		bootstrapSnapshotBytes, err := os.ReadFile(bootstrapSnapshotArchivePath)
		if err != nil {
			return false, fmt.Errorf("failed reading bootstrap snapshot: %w", err)
		}
		if err := binutils.InstallArchive("tar.gz", bootstrapSnapshotBytes, snapshotsDir); err != nil {
			return false, fmt.Errorf("failed installing bootstrap snapshot: %w", err)
		}
		if err := os.WriteFile(currentBootstrapNamePath, []byte(bootstrapSnapshotArchiveName), constants.DefaultPerms755); err != nil {
			return false, err
		}
	}
	return resetCurrentSnapshot, nil
}

// start the network
func (d *LocalDeployer) startNetwork(
	ctx context.Context,
	cli client.Client,
	avalancheGoBinPath string,
) error {
	autoSave := d.app.Conf.GetConfigBoolValue(constants.ConfigSnapshotsAutoSaveKey)

	tmpDir, err := anrutils.MkDirWithTimestamp(filepath.Join(d.app.GetRunDir(), "network"))
	if err != nil {
		return err
	}

	rootDir := ""
	logDir := ""
	if !autoSave {
		rootDir = tmpDir
	} else {
		logDir = tmpDir
	}

	loadSnapshotOpts := []client.OpOption{
		client.WithExecPath(avalancheGoBinPath),
		client.WithRootDataDir(rootDir),
		client.WithLogRootDir(logDir),
		client.WithReassignPortsIfUsed(true),
		client.WithPluginDir(d.app.GetPluginsDir()),
	}

	// load global node configs if they exist
	configStr, err := d.app.Conf.LoadNodeConfig()
	if err != nil {
		return nil
	}
	if configStr != "" {
		loadSnapshotOpts = append(loadSnapshotOpts, client.WithGlobalNodeConfig(configStr))
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Booting Network. Wait until healthy...")
	resp, err := cli.LoadSnapshot(
		ctx,
		constants.DefaultSnapshotName,
		d.app.Conf.GetConfigBoolValue(constants.ConfigSnapshotsAutoSaveKey),
		loadSnapshotOpts...,
	)
	if err != nil {
		return fmt.Errorf("failed to start network :%w", err)
	}
	ux.Logger.PrintToUser("Node logs directory: %s/node<i>/logs", resp.ClusterInfo.LogRootDir)
	ux.Logger.PrintToUser("Network ready to use.")
	return nil
}

// Returns an error if the server cannot be contacted. You may want to ignore this error.
func GetLocallyDeployedSubnets() (map[string]struct{}, error) {
	deployedNames := map[string]struct{}{}
	// if the server can not be contacted, or there is a problem with the query,
	// DO NOT FAIL, just print No for deployed status
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	resp, err := cli.Status(ctx)
	if err != nil {
		return nil, err
	}

	for _, chain := range resp.GetClusterInfo().CustomChains {
		deployedNames[chain.ChainName] = struct{}{}
	}

	return deployedNames, nil
}

func IssueRemoveSubnetValidatorTx(kc keychain.Keychain, subnetID ids.ID, nodeID ids.NodeID) (ids.ID, error) {
	ctx := context.Background()
	api := constants.LocalAPIEndpoint
	wallet, err := primary.MakeWallet(
		ctx,
		&primary.WalletConfig{
			URI:          api,
			AVAXKeychain: kc,
			EthKeychain:  secp256k1fx.NewKeychain(),
			SubnetIDs:    []ids.ID{subnetID},
		},
	)
	if err != nil {
		return ids.Empty, err
	}

	tx, err := wallet.P().IssueRemoveSubnetValidatorTx(nodeID, subnetID)
	return tx.ID(), err
}

func GetSubnetValidators(subnetID ids.ID) ([]platformvm.ClientPermissionlessValidator, error) {
	api := constants.LocalAPIEndpoint
	pClient := platformvm.NewClient(api)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()

	return pClient.GetCurrentValidators(ctx, subnetID, nil)
}

func CheckNodeIsInSubnetValidators(subnetID ids.ID, nodeID string) (bool, error) {
	api := constants.LocalAPIEndpoint
	pClient := platformvm.NewClient(api)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()

	vals, err := pClient.GetCurrentValidators(ctx, subnetID, nil)
	if err != nil {
		return false, err
	}
	for _, v := range vals {
		if v.NodeID.String() == nodeID {
			return true, nil
		}
	}
	return false, nil
}

func GetLocalNetworkRelayerConfigPath(app *application.Avalanche) (bool, string, error) {
	clusterInfo, err := localnet.GetClusterInfo()
	if err != nil {
		return false, "", err
	}
	relayerConfigPath := app.GetLocalRelayerConfigPath(models.Local, clusterInfo.GetRootDataDir())
	return utils.FileExists(relayerConfigPath), relayerConfigPath, nil
}
