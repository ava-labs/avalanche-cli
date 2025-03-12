// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package node

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
)

func LocalNodeTrackSubnet(
	ctx context.Context,
	cli client.Client,
	app *application.Avalanche,
	rootDir string,
	avalancheGoBinaryPath string,
	blockchainName string,
	blockchainID ids.ID,
	subnetID ids.ID,
	nodeName string,
) error {
	if app.ChainConfigExists(blockchainName) {
		inputChainConfigPath := app.GetChainConfigPath(blockchainName)
		outputChainConfigPath := filepath.Join(rootDir, nodeName, "configs", "chains", blockchainID.String(), "config.json")
		ux.Logger.Info("Creating chain conf directory %s", filepath.Dir(outputChainConfigPath))
		if err := os.MkdirAll(filepath.Dir(outputChainConfigPath), 0o700); err != nil {
			return fmt.Errorf("could not create chain conf directory %s: %w", filepath.Dir(outputChainConfigPath), err)
		}
		ux.Logger.Info("Copying %s to %s", inputChainConfigPath, outputChainConfigPath)
		if err := utils.FileCopy(inputChainConfigPath, outputChainConfigPath); err != nil {
			return err
		}
	}

	opts := []client.OpOption{
		client.WithWhitelistedSubnets(subnetID.String()),
		client.WithRootDataDir(rootDir),
		client.WithExecPath(avalancheGoBinaryPath),
	}
	ux.Logger.Info("Using client options: %v", opts)
	ux.Logger.Info("Restarting node %s", nodeName)
	if _, err := cli.RestartNode(ctx, nodeName, opts...); err != nil {
		return err
	}

	return nil
}

func setupAvalancheGo(
	app *application.Avalanche,
	avalancheGoBinaryPath string,
	avaGoVersionSetting AvalancheGoVersionSettings,
	printFunc func(msg string, args ...interface{}),
) (string, error) {
	var err error
	avalancheGoVersion := ""
	if avalancheGoBinaryPath == "" {
		avalancheGoVersion, err = GetAvalancheGoVersion(app, avaGoVersionSetting)
		if err != nil {
			return "", err
		}
		printFunc("Using AvalancheGo version: %s", avalancheGoVersion)
	}
	avalancheGoBinaryPath, err = localnet.SetupAvalancheGoBinary(app, avalancheGoVersion, avalancheGoBinaryPath)
	if err != nil {
		return "", err
	}
	printFunc("AvalancheGo path: %s\n", avalancheGoBinaryPath)
	return avalancheGoBinaryPath, err
}

func StartLocalNode(
	app *application.Avalanche,
	clusterName string,
	avalancheGoBinaryPath string,
	numNodes uint32,
	defaultFlags map[string]interface{},
	connectionSettings localnet.ConnectionSettings,
	nodeSettings localnet.NodeSettings,
	avaGoVersionSetting AvalancheGoVersionSettings,
	network models.Network,
	networkFlags networkoptions.NetworkFlags,
	supportedNetworkOptions []networkoptions.NetworkOption,
) error {
	// initializes directories
	networkDir := localnet.GetLocalClusterDir(app, clusterName)
	pluginDir := filepath.Join(networkDir, "plugins")
	if err := os.MkdirAll(networkDir, constants.DefaultPerms755); err != nil {
		return fmt.Errorf("could not create network directory %s: %w", networkDir, err)
	}
	if err := os.MkdirAll(pluginDir, constants.DefaultPerms755); err != nil {
		return fmt.Errorf("could not create plugin directory %s: %w", pluginDir, err)
	}
	// setup avalanchego
	var err error
	avalancheGoBinaryPath, err = setupAvalancheGo(
		app,
		avalancheGoBinaryPath,
		avaGoVersionSetting,
		ux.Logger.PrintToUser,
	)
	if err != nil {
		return err
	}

	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	if localnet.LocalClusterExists(app, clusterName) {
		ux.Logger.GreenCheckmarkToUser("Local cluster %s found. Booting up...", clusterName)
		network, err := localnet.GetClusterNetworkKind(app, clusterName)
		if err != nil {
			return err
		}
		ctx, cancel = network.BootstrappingContext()
		defer cancel()
		if _, err := localnet.TmpNetLoad(ctx, app.Log, networkDir, avalancheGoBinaryPath); err != nil {
			return err
		}
	} else {
		ux.Logger.GreenCheckmarkToUser("Local cluster %s not found. Creating...", clusterName)
		if network.Kind == models.Undefined {
			network, err = networkoptions.GetNetworkFromCmdLineFlags(
				app,
				"",
				networkFlags,
				false,
				true,
				supportedNetworkOptions,
				"",
			)
			if err != nil {
				return err
			}
		}
		network.ClusterName = clusterName

		switch {
		case network.Kind == models.Fuji:
			ux.Logger.PrintToUser(logging.Yellow.Wrap("Warning: Fuji Bootstrapping can take several minutes"))
			connectionSettings.NetworkID = network.ID
		case network.Kind == models.Mainnet:
			ux.Logger.PrintToUser(logging.Yellow.Wrap("Warning: Mainnet Bootstrapping can take 6-24 hours"))
			connectionSettings.NetworkID = network.ID
		case network.Kind == models.Local:
			connectionSettings, err = localnet.GetLocalNetworkConnectionInfo(app)
			if err != nil {
				return err
			}
		}

		if defaultFlags == nil {
			defaultFlags = map[string]interface{}{}
		}
		defaultFlags[config.NetworkAllowPrivateIPsKey] = true
		defaultFlags[config.IndexEnabledKey] = false
		defaultFlags[config.IndexAllowIncompleteKey] = true

		ctx, cancel = network.BootstrappingContext()
		defer cancel()

		ux.Logger.PrintToUser("Starting local avalanchego node using root: %s ...", networkDir)
		spinSession := ux.NewUserSpinner()
		spinner := spinSession.SpinToUser("Booting Network. Wait until healthy...")

		_, err := localnet.CreateLocalCluster(
			app,
			ctx,
			ux.Logger.PrintToUser,
			clusterName,
			avalancheGoBinaryPath,
			pluginDir,
			defaultFlags,
			connectionSettings,
			numNodes,
			[]localnet.NodeSettings{nodeSettings},
			network,
		)
		if err != nil {
			ux.SpinFailWithError(spinner, "", err)
			return fmt.Errorf("failed to start local avalanchego: %w", err)
		}

		ux.SpinComplete(spinner)
		spinSession.Stop()
	}

	ux.Logger.PrintToUser("Waiting for P-Chain to be bootstrapped")
	if err := localnet.WaitLocalClusterBlockchainBootstrapped(app, ctx, clusterName, "P", ids.Empty); err != nil {
		return fmt.Errorf("failure waiting for local cluster P-Chain bootstrapping: %w", err)
	}

	ux.Logger.GreenCheckmarkToUser("Avalanchego started and ready to use from %s", networkDir)
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Node logs directory: %s/<NodeID>/logs", networkDir)
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Network ready to use.")
	ux.Logger.PrintToUser("")

	cluster, err := localnet.GetLocalCluster(app, clusterName)
	if err != nil {
		return err
	}
	for _, node := range cluster.Nodes {
		ux.Logger.PrintToUser("URI: %s", node.URI)
		ux.Logger.PrintToUser("NodeID: %s", node.NodeID)
		ux.Logger.PrintToUser("")
	}

	return nil
}

// add additional validator to local node
func UpsizeLocalNode(
	app *application.Avalanche,
	network models.Network,
	blockchainName string,
	blockchainID ids.ID,
	subnetID ids.ID,
	avalancheGoBinaryPath string,
	nodeConfig map[string]interface{},
	connectionSettings localnet.ConnectionSettings,
) (
	string, // added nodeName
	error,
) {
	clusterName, err := GetRunnningLocalNodeClusterName(app)
	if err != nil {
		return "", err
	}
	rootDir := app.GetLocalClusterDir(clusterName)
	pluginDir := filepath.Join(rootDir, "node1", "plugins")

	if nodeConfig == nil {
		nodeConfig = map[string]interface{}{}
	}
	nodeConfig[config.NetworkAllowPrivateIPsKey] = true
	if network.Kind == models.Fuji {
		nodeConfig[config.IndexEnabledKey] = false // disable index for Fuji
	}
	nodeConfigBytes, err := json.Marshal(nodeConfig)
	if err != nil {
		return "", err
	}
	nodeConfigStr := string(nodeConfigBytes)

	// we will remove this code soon, so it can be not DRY
	if network.Kind == models.Local {
		connectionSettings, err = localnet.GetLocalNetworkConnectionInfo(app)
		if err != nil {
			return "", err
		}
	}
	// end of code to be removed
	anrOpts := []client.OpOption{
		client.WithNetworkID(network.ID),
		client.WithExecPath(avalancheGoBinaryPath),
		client.WithRootDataDir(rootDir),
		client.WithWhitelistedSubnets(subnetID.String()),
		client.WithReassignPortsIfUsed(true),
		client.WithPluginDir(pluginDir),
		client.WithFreshStakingIds(true),
		client.WithZeroIP(false),
		client.WithGlobalNodeConfig(nodeConfigStr),
	}
	/*
		if connectionSettings.GenesisPath != "" && utils.FileExists(connectionSettings.GenesisPath) {
			anrOpts = append(anrOpts, client.WithGenesisPath(connectionSettings.GenesisPath))
		}
		if connectionSettings.UpgradePath != "" && utils.FileExists(connectionSettings.UpgradePath) {
			anrOpts = append(anrOpts, client.WithUpgradePath(connectionSettings.UpgradePath))
		}
	*/
	if connectionSettings.BootstrapIDs != nil {
		anrOpts = append(anrOpts, client.WithBootstrapNodeIDs(connectionSettings.BootstrapIDs))
	}
	if connectionSettings.BootstrapIPs != nil {
		anrOpts = append(anrOpts, client.WithBootstrapNodeIPPortPairs(connectionSettings.BootstrapIPs))
	}

	cli, err := binutils.NewGRPCClientWithEndpoint(
		binutils.LocalClusterGRPCServerEndpoint,
		binutils.WithAvoidRPCVersionCheck(true),
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if err != nil {
		return "", err
	}

	ctx, cancel := network.BootstrappingContext()
	defer cancel()

	newNodeName, err := GetNextNodeName()
	if err != nil {
		return "", fmt.Errorf("failed to generate a new node name: %w", err)
	}

	spinSession := ux.NewUserSpinner()
	spinner := spinSession.SpinToUser("Creating new node with name %s on local machine", newNodeName)
	err = DownloadPublicArchive(network, rootDir, []string{newNodeName})
	ux.Logger.Info("seeding public archive data finished with error: %v. Ignored if any", err)
	// add new local node
	if _, err := cli.AddNode(ctx, newNodeName, avalancheGoBinaryPath, anrOpts...); err != nil {
		ux.SpinFailWithError(spinner, "", err)
		return newNodeName, fmt.Errorf("failed to add local validator: %w", err)
	}
	ux.Logger.Info("Waiting for node: %s to be bootstrapping P-Chain", newNodeName)
	if err := WaitBootstrapped(ctx, cli, "P"); err != nil {
		return newNodeName, fmt.Errorf("failure waiting for local cluster P-Chain bootstrapping: %w", err)
	}
	ux.Logger.Info("Waiting for node: %s to be healthy", newNodeName)
	_, err = subnet.WaitForHealthy(ctx, cli)
	if err != nil {
		return newNodeName, fmt.Errorf("failed waiting for node %s to be healthy: %w", newNodeName, err)
	}
	ux.SpinComplete(spinner)
	spinner = spinSession.SpinToUser("Tracking blockchain %s", blockchainName)
	time.Sleep(10 * time.Second) // delay before restarting new node
	if err := LocalNodeTrackSubnet(ctx,
		cli,
		app,
		rootDir,
		avalancheGoBinaryPath,
		blockchainName,
		blockchainID,
		subnetID,
		newNodeName); err != nil {
		ux.SpinFailWithError(spinner, "", err)
		return newNodeName, fmt.Errorf("failed to track blockchain: %w", err)
	}
	// wait until cluster is healthy
	ux.Logger.Info("Waiting for node: %s to be bootstrapping %s", newNodeName, blockchainName)
	if err := WaitBootstrapped(ctx, cli, blockchainID.String()); err != nil {
		return newNodeName, fmt.Errorf("failure waiting for local cluster blockchain bootstrapping: %w", err)
	}
	spinner = spinSession.SpinToUser("Waiting for blockchain to be healthy")
	clusterInfo, err := subnet.WaitForHealthy(ctx, cli)
	if err != nil {
		return newNodeName, fmt.Errorf("failed waiting for blockchain to become healthy: %w", err)
	}
	ux.SpinComplete(spinner)
	spinSession.Stop()

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Node logs directory: %s/%s/logs", rootDir, newNodeName)
	ux.Logger.PrintToUser("")

	nodeInfo := clusterInfo.NodeInfos[newNodeName]
	ux.Logger.PrintToUser("Node name: %s ", newNodeName)
	ux.Logger.PrintToUser("URI: %s", nodeInfo.Uri)
	ux.Logger.PrintToUser("Node-ID: %s", nodeInfo.Id)
	ux.Logger.PrintToUser("")
	return newNodeName, nil
}

func LocalStatus(
	app *application.Avalanche,
	clusterName string,
	blockchainName string,
) error {
	var localClusters []string
	if clusterName != "" {
		if !localnet.LocalClusterExists(app, clusterName) {
			return fmt.Errorf("local node %q is not found", clusterName)
		}
		localClusters = []string{clusterName}
	} else {
		var err error
		localClusters, err = localnet.GetClusters(app)
		if err != nil {
			return fmt.Errorf("failed to list local clusters: %w", err)
		}
	}
	if clusterName != "" {
		ux.Logger.PrintToUser("%s %s", logging.Blue.Wrap("Local cluster:"), logging.Green.Wrap(clusterName))
	} else if len(localClusters) > 0 {
		ux.Logger.PrintToUser(logging.Blue.Wrap("Local clusters:"))
	}
	for _, clusterName := range localClusters {
		currenlyRunning := ""
		healthStatus := ""
		avagoURIOuput := ""

		network, err := localnet.GetClusterNetworkKind(app, clusterName)
		if err != nil {
			return fmt.Errorf("failed to get cluster network: %w", err)
		}
		networkKind := fmt.Sprintf(" [%s]", logging.Orange.Wrap(network.Name()))

		// load sidecar and cluster config for the cluster  if blockchainName is not empty
		blockchainID := ids.Empty
		if blockchainName != "" {
			sc, err := app.LoadSidecar(blockchainName)
			if err != nil {
				return err
			}
			blockchainID = sc.Networks[network.Name()].BlockchainID
		}
		isRunning, err := localnet.ClusterIsRunning(app, clusterName)
		if err != nil {
			return err
		}
		if isRunning {
			pChainHealth, l1Health, err := localnet.LocalClusterHealth(app, clusterName)
			if err != nil {
				return err
			}
			currenlyRunning = fmt.Sprintf(" [%s]", logging.Blue.Wrap("Running"))
			if pChainHealth && l1Health {
				healthStatus = fmt.Sprintf(" [%s]", logging.Green.Wrap("Healthy"))
			} else {
				healthStatus = fmt.Sprintf(" [%s]", logging.Red.Wrap("Unhealthy"))
			}
			runningAvagoURIs, err := localnet.GetLocalClusterURIs(app, clusterName)
			if err != nil {
				return err
			}
			for _, avagoURI := range runningAvagoURIs {
				nodeID, nodePOP, isBoot, err := GetInfo(avagoURI, blockchainID.String())
				if err != nil {
					ux.Logger.RedXToUser("failed to get node  %s info: %v", avagoURI, err)
					continue
				}
				nodePOPPubKey := "0x" + hex.EncodeToString(nodePOP.PublicKey[:])
				nodePOPProof := "0x" + hex.EncodeToString(nodePOP.ProofOfPossession[:])

				isBootStr := "Primary:" + logging.Red.Wrap("Not Bootstrapped")
				if isBoot {
					isBootStr = "Primary:" + logging.Green.Wrap("Bootstrapped")
				}

				blockchainStatus := ""
				if blockchainID != ids.Empty {
					blockchainStatus, _ = GetBlockchainStatus(avagoURI, blockchainID.String()) // silence errors
				}

				avagoURIOuput += fmt.Sprintf("   - %s [%s] [%s]\n     publicKey: %s \n     proofOfPossession: %s \n",
					logging.LightBlue.Wrap(avagoURI),
					nodeID,
					strings.TrimRight(strings.Join([]string{isBootStr, "L1:" + logging.Orange.Wrap(blockchainStatus)}, " "), " "),
					nodePOPPubKey,
					nodePOPProof,
				)
			}
		} else {
			currenlyRunning = fmt.Sprintf(" [%s]", logging.LightGray.Wrap("Stopped"))
		}
		networkDir := localnet.GetLocalClusterDir(app, clusterName)
		ux.Logger.PrintToUser("- %s: %s %s %s %s", clusterName, networkDir, networkKind, currenlyRunning, healthStatus)
		ux.Logger.PrintToUser(avagoURIOuput)
	}

	return nil
}

func GetInfo(uri string, blockchainID string) (
	ids.NodeID, // nodeID
	*signer.ProofOfPossession, // nodePOP
	bool, // isBootstrapped
	error, // error
) {
	client := info.NewClient(uri)
	ctx, cancel := utils.GetAPILargeContext()
	defer cancel()
	nodeID, nodePOP, err := client.GetNodeID(ctx)
	if err != nil {
		return ids.EmptyNodeID, &signer.ProofOfPossession{}, false, err
	}
	isBootstrapped, err := client.IsBootstrapped(ctx, blockchainID)
	if err != nil {
		return nodeID, nodePOP, isBootstrapped, err
	}
	return nodeID, nodePOP, isBootstrapped, err
}

func GetBlockchainStatus(uri string, blockchainID string) (
	string, // status
	error, // error
) {
	client := platformvm.NewClient(uri)
	ctx, cancel := utils.GetAPILargeContext()
	defer cancel()
	status, err := client.GetBlockchainStatus(ctx, blockchainID)
	if err != nil {
		return "", err
	}
	if status.String() == "" {
		return "Not Syncing", nil
	}
	return status.String(), nil
}

func WaitBootstrapped(ctx context.Context, cli client.Client, blockchainID string) error {
	blockchainBootstrapCheckFrequency := time.Second
	status, err := cli.Status(ctx)
	if err != nil {
		return err
	}
	for _, nodeInfo := range status.ClusterInfo.NodeInfos {
		for {
			infoClient := info.NewClient(nodeInfo.GetUri())
			boostrapped, err := infoClient.IsBootstrapped(ctx, blockchainID)
			if err != nil && !strings.Contains(err.Error(), "there is no chain with alias/ID") {
				return err
			}
			if boostrapped {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(blockchainBootstrapCheckFrequency):
			}
		}
	}
	return err
}
