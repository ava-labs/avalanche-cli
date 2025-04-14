// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package node

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
)

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
	nodeSettings []localnet.NodeSetting,
	avaGoVersionSetting AvalancheGoVersionSettings,
	network models.Network,
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

	if localnet.LocalClusterExists(app, clusterName) {
		ux.Logger.GreenCheckmarkToUser("Local cluster %s found. Booting up...", clusterName)
		if err := localnet.LoadLocalCluster(app, clusterName, avalancheGoBinaryPath); err != nil {
			return err
		}
	} else {
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

		ux.Logger.GreenCheckmarkToUser("Local cluster %s not found. Creating...", clusterName)
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

		ux.Logger.PrintToUser("Starting local avalanchego node using root: %s ...", networkDir)
		spinSession := ux.NewUserSpinner()
		spinner := spinSession.SpinToUser("Booting Network. Wait until healthy...")

		_, err = localnet.CreateLocalCluster(
			app,
			ux.Logger.PrintToUser,
			clusterName,
			avalancheGoBinaryPath,
			pluginDir,
			defaultFlags,
			connectionSettings,
			numNodes,
			nodeSettings,
			[]ids.ID{},
			network,
			true, // Download DB
			true, // Bootstrap
		)
		if err != nil {
			ux.SpinFailWithError(spinner, "", err)
			return fmt.Errorf("failed to start local avalanchego: %w", err)
		}

		ux.SpinComplete(spinner)
		spinSession.Stop()
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
		localClusters, err = localnet.GetLocalClusters(app)
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

		network, err := localnet.GetLocalClusterNetworkModel(app, clusterName)
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
		isRunning, err := localnet.LocalClusterIsRunning(app, clusterName)
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
				nodeID, nodePOP, isBoot, err := getInfo(avagoURI, blockchainID.String())
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
					blockchainStatus, _ = getBlockchainStatus(avagoURI, blockchainID.String()) // silence errors
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

func getInfo(uri string, blockchainID string) (
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

func getBlockchainStatus(uri string, blockchainID string) (
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
