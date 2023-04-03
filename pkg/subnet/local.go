// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ava-labs/avalanchego/vms/platformvm"

	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"

	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanche-network-runner/server"
	anrutils "github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/storage"
	"github.com/ava-labs/coreth/params"
	spacesvmchain "github.com/ava-labs/spacesvm/chain"
	"github.com/ava-labs/subnet-evm/core"
	"go.uber.org/zap"
)

const (
	WriteReadReadPerms = 0o644
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
	vmBin              string
}

func NewLocalDeployer(app *application.Avalanche, avagoVersion string, vmBin string) *LocalDeployer {
	return &LocalDeployer{
		procChecker:        binutils.NewProcessChecker(),
		binChecker:         binutils.NewBinaryChecker(),
		getClientFunc:      binutils.NewGRPCClient,
		binaryDownloader:   binutils.NewPluginBinaryDownloader(app),
		app:                app,
		setDefaultSnapshot: SetDefaultSnapshot,
		avagoVersion:       avagoVersion,
		vmBin:              vmBin,
	}
}

type getGRPCClientFunc func(...binutils.GRPCClientOpOption) (client.Client, error)

type setDefaultSnapshotFunc func(string, bool) error

// DeployToLocalNetwork does the heavy lifting:
// * it checks the gRPC is running, if not, it starts it
// * kicks off the actual deployment
func (d *LocalDeployer) DeployToLocalNetwork(chain string, chainGenesis []byte, genesisPath string) (ids.ID, ids.ID, error) {
	if err := d.StartServer(); err != nil {
		return ids.Empty, ids.Empty, err
	}
	return d.doDeploy(chain, chainGenesis, genesisPath)
}

func getAssetID(wallet primary.Wallet, tokenName string, tokenSymbol string, maxSupply uint64) (ids.ID, error) {
	xWallet := wallet.X()
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs: []ids.ShortID{
			genesis.EWOQKey.PublicKey().Address(),
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultWalletCreationTimeout)
	subnetAssetID, err := xWallet.IssueCreateAssetTx(
		tokenName,
		tokenSymbol,
		9, //denomination for UI purposes only in explorer
		map[uint32][]verify.State{
			0: {
				&secp256k1fx.TransferOutput{
					Amt:          maxSupply,
					OutputOwners: *owner,
				},
			},
		},
		common.WithContext(ctx),
	)
	defer cancel()
	if err != nil {
		return ids.Empty, err
	}
	return subnetAssetID, nil
}

func exportToPChain(wallet primary.Wallet, owner *secp256k1fx.OutputOwners, subnetAssetID ids.ID, maxSupply uint64) error {
	xWallet := wallet.X()
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultWalletCreationTimeout)

	_, err := xWallet.IssueExportTx(
		ids.Empty,
		[]*avax.TransferableOutput{
			{
				Asset: avax.Asset{
					ID: subnetAssetID,
				},
				Out: &secp256k1fx.TransferOutput{
					Amt:          maxSupply,
					OutputOwners: *owner,
				},
			},
		},
		common.WithContext(ctx),
	)
	defer cancel()
	return err
}

func importFromXChain(wallet primary.Wallet, owner *secp256k1fx.OutputOwners) error {
	xWallet := wallet.X()
	pWallet := wallet.P()
	xChainID := xWallet.BlockchainID()
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultWalletCreationTimeout)
	_, err := pWallet.IssueImportTx(
		xChainID,
		owner,
		common.WithContext(ctx),
	)
	defer cancel()
	return err
}

func IssueTransformSubnetTx(
	elasticSubnetConfig models.ElasticSubnetConfig,
	kc keychain.Keychain,
	subnetID ids.ID,
	tokenName string,
	tokenSymbol string,
	maxSupply uint64,
) (ids.ID, ids.ID, error) {
	ctx := context.Background()
	api := constants.LocalAPIEndpoint
	wallet, err := primary.NewWalletWithTxs(ctx, api, kc, subnetID)
	if err != nil {
		return ids.Empty, ids.Empty, err
	}
	subnetAssetID, err := getAssetID(wallet, tokenName, tokenSymbol, maxSupply)
	if err != nil {
		return ids.Empty, ids.Empty, err
	}
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs: []ids.ShortID{
			genesis.EWOQKey.PublicKey().Address(),
		},
	}
	err = exportToPChain(wallet, owner, subnetAssetID, maxSupply)
	if err != nil {
		return ids.Empty, ids.Empty, err
	}
	err = importFromXChain(wallet, owner)
	if err != nil {
		return ids.Empty, ids.Empty, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultConfirmTxTimeout)
	transformSubnetTxID, err := wallet.P().IssueTransformSubnetTx(elasticSubnetConfig.SubnetID, subnetAssetID,
		elasticSubnetConfig.InitialSupply, elasticSubnetConfig.MaxSupply, elasticSubnetConfig.MinConsumptionRate,
		elasticSubnetConfig.MaxConsumptionRate, elasticSubnetConfig.MinValidatorStake, elasticSubnetConfig.MaxValidatorStake,
		elasticSubnetConfig.MinStakeDuration, elasticSubnetConfig.MaxStakeDuration, elasticSubnetConfig.MinDelegationFee,
		elasticSubnetConfig.MinDelegatorStake, elasticSubnetConfig.MaxValidatorWeightFactor, elasticSubnetConfig.UptimeRequirement,
		common.WithContext(ctx),
	)
	defer cancel()
	if err != nil {
		return ids.Empty, ids.Empty, err
	}
	return transformSubnetTxID, subnetAssetID, err
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
	ctx, cancel := context.WithTimeout(context.Background(), constants.E2ERequestTimeout)
	defer cancel()
	_, err := pClient.GetCurrentSupply(ctx, subnetID)
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
func (d *LocalDeployer) doDeploy(chain string, chainGenesis []byte, genesisPath string) (ids.ID, ids.ID, error) {
	avalancheGoBinPath, err := d.SetupLocalEnv()
	if err != nil {
		return ids.Empty, ids.Empty, err
	}

	backendLogFile, err := binutils.GetBackendLogFile(d.app)
	var backendLogDir string
	if err == nil {
		// TODO should we do something if there _was_ an error?
		backendLogDir = filepath.Dir(backendLogFile)
	}

	cli, err := d.getClientFunc()
	if err != nil {
		return ids.Empty, ids.Empty, fmt.Errorf("error creating gRPC Client: %w", err)
	}
	defer cli.Close()

	runDir := d.app.GetRunDir()

	ctx := binutils.GetAsyncContext()

	// loading sidecar before it's needed so we catch any error early
	sc, err := d.app.LoadSidecar(chain)
	if err != nil {
		return ids.Empty, ids.Empty, fmt.Errorf("failed to load sidecar: %w", err)
	}

	// check for network status
	networkBooted := true
	clusterInfo, err := WaitForHealthy(ctx, cli)
	rootDir := clusterInfo.GetRootDataDir()
	if err != nil {
		if !server.IsServerError(err, server.ErrNotBootstrapped) {
			utils.FindErrorLogs(rootDir, backendLogDir)
			return ids.Empty, ids.Empty, fmt.Errorf("failed to query network health: %w", err)
		} else {
			networkBooted = false
		}
	}

	chainVMID, err := anrutils.VMID(chain)
	if err != nil {
		return ids.Empty, ids.Empty, fmt.Errorf("failed to create VM ID from %s: %w", chain, err)
	}
	d.app.Log.Debug("this VM will get ID", zap.String("vm-id", chainVMID.String()))

	if !networkBooted {
		if err := d.startNetwork(ctx, cli, avalancheGoBinPath, runDir); err != nil {
			utils.FindErrorLogs(rootDir, backendLogDir)
			return ids.Empty, ids.Empty, err
		}
	}

	// get VM info
	clusterInfo, err = WaitForHealthy(ctx, cli)
	if err != nil {
		utils.FindErrorLogs(clusterInfo.GetRootDataDir(), backendLogDir)
		return ids.Empty, ids.Empty, fmt.Errorf("failed to query network health: %w", err)
	}
	rootDir = clusterInfo.GetRootDataDir()

	if alreadyDeployed(chainVMID, clusterInfo) {
		ux.Logger.PrintToUser("Subnet %s has already been deployed", chain)
		return ids.Empty, ids.Empty, nil
	}

	subnetIDs := clusterInfo.Subnets
	numBlockchains := len(clusterInfo.CustomChains)

	// in order to make subnet deploy faster, a set of validated subnet IDs is preloaded
	// in the bootstrap snapshot
	// we select one to be used for creating the next blockchain, for that we use the
	// number of currently created blockchains as the index to select the next subnet ID,
	// so we get incremental selection
	sort.Strings(subnetIDs)
	if len(subnetIDs) == 0 {
		return ids.Empty, ids.Empty, errors.New("the network has not preloaded subnet IDs")
	}
	subnetIDStr := subnetIDs[numBlockchains%len(subnetIDs)]

	// if a chainConfig has been configured
	var (
		chainConfig            string
		chainConfigFile        = filepath.Join(d.app.GetSubnetDir(), chain, constants.ChainConfigFileName)
		perNodeChainConfig     string
		perNodeChainConfigFile = filepath.Join(d.app.GetSubnetDir(), chain, constants.PerNodeChainConfigFileName)
	)
	if _, err := os.Stat(chainConfigFile); err == nil {
		// currently the ANR only accepts the file as a path, not its content
		chainConfig = chainConfigFile
	}
	if _, err := os.Stat(perNodeChainConfigFile); err == nil {
		perNodeChainConfig = perNodeChainConfigFile
	}

	// install the plugin binary for the new VM
	if err := d.installPlugin(chainVMID, d.vmBin); err != nil {
		return ids.Empty, ids.Empty, err
	}

	ux.Logger.PrintToUser("VMs ready.")

	// create a new blockchain on the already started network, associated to
	// the given VM ID, genesis, and available subnet ID
	blockchainSpecs := []*rpcpb.BlockchainSpec{
		{
			VmName:             chain,
			Genesis:            genesisPath,
			SubnetId:           &subnetIDStr,
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
		utils.FindErrorLogs(rootDir, backendLogDir)
		pluginRemoveErr := d.removeInstalledPlugin(chainVMID)
		if pluginRemoveErr != nil {
			ux.Logger.PrintToUser("Failed to remove plugin binary: %s", pluginRemoveErr)
		}
		return ids.Empty, ids.Empty, fmt.Errorf("failed to deploy blockchain: %w", err)
	}
	rootDir = clusterInfo.GetRootDataDir()

	d.app.Log.Debug(deployBlockchainsInfo.String())

	fmt.Println()
	ux.Logger.PrintToUser("Blockchain has been deployed. Wait until network acknowledges...")

	clusterInfo, err = WaitForHealthy(ctx, cli)
	if err != nil {
		utils.FindErrorLogs(rootDir, backendLogDir)
		pluginRemoveErr := d.removeInstalledPlugin(chainVMID)
		if pluginRemoveErr != nil {
			ux.Logger.PrintToUser("Failed to remove plugin binary: %s", pluginRemoveErr)
		}
		return ids.Empty, ids.Empty, fmt.Errorf("failed to query network health: %w", err)
	}

	endpoint := GetFirstEndpoint(clusterInfo, chain)

	fmt.Println()
	ux.Logger.PrintToUser("Network ready to use. Local network node endpoints:")
	ux.PrintTableEndpoints(clusterInfo)
	fmt.Println()

	ux.Logger.PrintToUser("Browser Extension connection details (any node URL from above works):")
	ux.Logger.PrintToUser("RPC URL:          %s", endpoint[strings.LastIndex(endpoint, "http"):])

	switch sc.VM {
	case models.SubnetEvm:
		if err := d.printExtraEvmInfo(chain, chainGenesis); err != nil {
			// not supposed to happen due to genesis pre validation
			return ids.Empty, ids.Empty, nil
		}
	case models.SpacesVM:
		if err := d.printExtraSpacesVMInfo(chainGenesis); err != nil {
			// not supposed to happen due to genesis pre validation
			return ids.Empty, ids.Empty, nil
		}
	}

	// we can safely ignore errors here as the subnets have already been generated
	subnetID, _ := ids.FromString(subnetIDStr)
	var blockchainID ids.ID
	for _, info := range clusterInfo.CustomChains {
		if info.VmId == chainVMID.String() {
			blockchainID, _ = ids.FromString(info.ChainId)
		}
	}
	return subnetID, blockchainID, nil
}

func (*LocalDeployer) printExtraSpacesVMInfo(chainGenesis []byte) error {
	var genesis spacesvmchain.Genesis
	if err := json.Unmarshal(chainGenesis, &genesis); err != nil {
		return fmt.Errorf("failed to unmarshall genesis: %w", err)
	}
	for _, alloc := range genesis.CustomAllocation {
		address := alloc.Address
		amount := alloc.Balance
		amountStr := fmt.Sprintf("%d", amount)
		if address == vm.PrefundedEwoqAddress {
			ux.Logger.PrintToUser("Funded address:   %s with %s - private key: %s", address, amountStr, vm.PrefundedEwoqPrivate)
		} else {
			ux.Logger.PrintToUser("Funded address:   %s with %s", address, amountStr)
		}
	}
	return nil
}

func (d *LocalDeployer) printExtraEvmInfo(chain string, chainGenesis []byte) error {
	var evmGenesis core.Genesis
	if err := json.Unmarshal(chainGenesis, &evmGenesis); err != nil {
		return fmt.Errorf("failed to unmarshall genesis: %w", err)
	}
	for address := range evmGenesis.Alloc {
		amount := evmGenesis.Alloc[address].Balance
		formattedAmount := new(big.Int).Div(amount, big.NewInt(params.Ether))
		if address == vm.PrefundedEwoqAddress {
			ux.Logger.PrintToUser("Funded address:   %s with %s (10^18) - private key: %s", address, formattedAmount.String(), vm.PrefundedEwoqPrivate)
		} else {
			ux.Logger.PrintToUser("Funded address:   %s with %s", address, formattedAmount.String())
		}
	}
	ux.Logger.PrintToUser("Network name:     %s", chain)
	ux.Logger.PrintToUser("Chain ID:         %s", evmGenesis.Config.ChainID)
	ux.Logger.PrintToUser("Currency Symbol:  %s", d.app.GetTokenName(chain))
	return nil
}

// SetupLocalEnv also does some heavy lifting:
// * sets up default snapshot if not installed
// * checks if avalanchego is installed in the local binary path
// * if not, it downloads it and installs it (os - and archive dependent)
// * returns the location of the avalanchego path
func (d *LocalDeployer) SetupLocalEnv() (string, error) {
	err := d.setDefaultSnapshot(d.app.GetSnapshotsDir(), false)
	if err != nil {
		return "", fmt.Errorf("failed setting up snapshots: %w", err)
	}

	avagoDir, err := d.setupLocalEnv()
	if err != nil {
		return "", fmt.Errorf("failed setting up local environment: %w", err)
	}

	pluginDir := d.app.GetPluginsDir()
	avalancheGoBinPath := filepath.Join(avagoDir, "avalanchego")

	if err := os.MkdirAll(pluginDir, constants.DefaultPerms755); err != nil {
		return "", fmt.Errorf("could not create pluginDir %s", pluginDir)
	}

	exists, err := storage.FolderExists(pluginDir)
	if !exists || err != nil {
		return "", fmt.Errorf("evaluated pluginDir to be %s but it does not exist", pluginDir)
	}

	// TODO: we need some better version management here
	// * compare latest to local version
	// * decide if force update or give user choice
	exists, err = storage.FileExists(avalancheGoBinPath)
	if !exists || err != nil {
		return "", fmt.Errorf(
			"evaluated avalancheGoBinPath to be %s but it does not exist", avalancheGoBinPath)
	}

	return avalancheGoBinPath, nil
}

func (d *LocalDeployer) setupLocalEnv() (string, error) {
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

func getExpectedDefaultSnapshotSHA256Sum() (string, error) {
	resp, err := http.Get(constants.BootstrapSnapshotSHA256URL)
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
	expectedSum, err := utils.SearchSHA256File(sha256FileBytes, constants.BootstrapSnapshotLocalPath)
	if err != nil {
		return "", fmt.Errorf("failed obtaining snapshot sha256 sum: %w", err)
	}
	return expectedSum, nil
}

// Initialize default snapshot with bootstrap snapshot archive
// If force flag is set to true, overwrite the default snapshot if it exists
func SetDefaultSnapshot(snapshotsDir string, force bool) error {
	bootstrapSnapshotArchivePath := filepath.Join(snapshotsDir, constants.BootstrapSnapshotArchiveName)
	// will download either if file not exists or if sha256 sum is not the same
	downloadSnapshot := false
	if _, err := os.Stat(bootstrapSnapshotArchivePath); os.IsNotExist(err) {
		downloadSnapshot = true
	} else {
		gotSum, err := utils.GetSHA256FromDisk(bootstrapSnapshotArchivePath)
		if err != nil {
			return err
		}
		expectedSum, err := getExpectedDefaultSnapshotSHA256Sum()
		if err != nil {
			ux.Logger.PrintToUser("Warning: failure verifying that the local snapshot is the latest one: %s", err)
		} else if gotSum != expectedSum {
			downloadSnapshot = true
		}
	}
	if downloadSnapshot {
		resp, err := http.Get(constants.BootstrapSnapshotURL)
		if err != nil {
			return fmt.Errorf("failed downloading bootstrap snapshot: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed downloading bootstrap snapshot: unexpected http status code: %d", resp.StatusCode)
		}
		defer resp.Body.Close()
		bootstrapSnapshotBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed downloading bootstrap snapshot: %w", err)
		}
		if err := os.WriteFile(bootstrapSnapshotArchivePath, bootstrapSnapshotBytes, WriteReadReadPerms); err != nil {
			return fmt.Errorf("failed writing down bootstrap snapshot: %w", err)
		}
	}
	defaultSnapshotPath := filepath.Join(snapshotsDir, "anr-snapshot-"+constants.DefaultSnapshotName)
	if force {
		if err := os.RemoveAll(defaultSnapshotPath); err != nil {
			return fmt.Errorf("failed removing default snapshot: %w", err)
		}
	}
	if _, err := os.Stat(defaultSnapshotPath); os.IsNotExist(err) {
		bootstrapSnapshotBytes, err := os.ReadFile(bootstrapSnapshotArchivePath)
		if err != nil {
			return fmt.Errorf("failed reading bootstrap snapshot: %w", err)
		}
		if err := binutils.InstallArchive("tar.gz", bootstrapSnapshotBytes, snapshotsDir); err != nil {
			return fmt.Errorf("failed installing bootstrap snapshot: %w", err)
		}
	}
	return nil
}

// start the network
func (d *LocalDeployer) startNetwork(
	ctx context.Context,
	cli client.Client,
	avalancheGoBinPath string,
	runDir string,
) error {
	loadSnapshotOpts := []client.OpOption{
		client.WithExecPath(avalancheGoBinPath),
		client.WithRootDataDir(runDir),
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

	pp, err := cli.LoadSnapshot(
		ctx,
		constants.DefaultSnapshotName,
		loadSnapshotOpts...,
	)
	if err != nil {
		return fmt.Errorf("failed to start network :%w", err)
	}
	ux.Logger.PrintToUser("Node log path: %s/node<i>/logs", pp.ClusterInfo.RootDataDir)
	ux.Logger.PrintToUser("Starting network...")
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

	ctx := binutils.GetAsyncContext()
	resp, err := cli.Status(ctx)
	if err != nil {
		return nil, err
	}

	for _, chain := range resp.GetClusterInfo().CustomChains {
		deployedNames[chain.ChainName] = struct{}{}
	}

	return deployedNames, nil
}
