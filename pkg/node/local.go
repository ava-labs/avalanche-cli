// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package node

import (
	"fmt"
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
	anrutils "github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"os"
	"path/filepath"
)

func TrackSubnetWithLocalMachine(app *application.Avalanche, clusterName, blockchainName string) error {
	if ok, err := checkClusterIsLocal(app, clusterName); err != nil || !ok {
		return fmt.Errorf("local node %q is not found", clusterName)
	}
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	clusterConfig := clustersConfig.Clusters[clusterName]
	network := clusterConfig.Network
	if sc.Networks[network.Name()].BlockchainID == ids.Empty {
		return fmt.Errorf("blockchain %s has not been deployed to %s", blockchainName, network.Name())
	}
	subnetID := sc.Networks[network.Name()].SubnetID
	blockchainID := sc.Networks[network.Name()].BlockchainID
	vmID, err := anrutils.VMID(blockchainName)
	if err != nil {
		return fmt.Errorf("failed to create VM ID from %s: %w", blockchainName, err)
	}
	var vmBin string
	switch sc.VM {
	case models.SubnetEvm:
		_, vmBin, err = binutils.SetupSubnetEVM(app, sc.VMVersion)
		if err != nil {
			return fmt.Errorf("failed to install subnet-evm: %w", err)
		}
	case models.CustomVM:
		vmBin = binutils.SetupCustomBin(app, blockchainName)
	default:
		return fmt.Errorf("unknown vm: %s", sc.VM)
	}
	rootDir := app.GetLocalDir(clusterName)
	pluginPath := filepath.Join(rootDir, "node1", "plugins", vmID.String())
	if err := utils.FileCopy(vmBin, pluginPath); err != nil {
		return err
	}
	if err := os.Chmod(pluginPath, constants.DefaultPerms755); err != nil {
		return err
	}
	if app.ChainConfigExists(blockchainName) {
		inputChainConfigPath := app.GetChainConfigPath(blockchainName)
		outputChainConfigPath := filepath.Join(rootDir, "node1", "configs", "chains", blockchainID.String(), "config.json")
		if err := os.MkdirAll(filepath.Dir(outputChainConfigPath), 0o700); err != nil {
			return fmt.Errorf("could not create chain conf directory %s: %w", filepath.Dir(outputChainConfigPath), err)
		}
		if err := utils.FileCopy(inputChainConfigPath, outputChainConfigPath); err != nil {
			return err
		}
	}

	cli, err := binutils.NewGRPCClientWithEndpoint(
		binutils.LocalClusterGRPCServerEndpoint,
		binutils.WithAvoidRPCVersionCheck(true),
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if err != nil {
		return err
	}
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	status, err := cli.Status(ctx)
	if err != nil {
		return err
	}
	publicEndpoints := []string{}
	for _, nodeInfo := range status.ClusterInfo.NodeInfos {
		if _, err := cli.RestartNode(ctx, nodeInfo.Name, client.WithWhitelistedSubnets(subnetID.String())); err != nil {
			return err
		}
		publicEndpoints = append(publicEndpoints, nodeInfo.Uri)
	}
	networkInfo := sc.Networks[network.Name()]
	rpcEndpoints := set.Of(networkInfo.RPCEndpoints...)
	wsEndpoints := set.Of(networkInfo.WSEndpoints...)
	for _, publicEndpoint := range publicEndpoints {
		rpcEndpoints.Add(getRPCEndpoint(publicEndpoint, networkInfo.BlockchainID.String()))
		wsEndpoints.Add(getWSEndpoint(publicEndpoint, networkInfo.BlockchainID.String()))
	}
	networkInfo.RPCEndpoints = rpcEndpoints.List()
	networkInfo.WSEndpoints = wsEndpoints.List()
	sc.Networks[clusterConfig.Network.Name()] = networkInfo
	if err := app.UpdateSidecar(&sc); err != nil {
		return err
	}
	ux.Logger.GreenCheckmarkToUser("%s successfully tracking %s", clusterName, blockchainName)
	return nil
}

func checkClusterIsLocal(app *application.Avalanche, clusterName string) (bool, error) {
	clustersConfig, err := app.GetClustersConfig()
	if err != nil {
		return false, err
	}
	clusterConf, ok := clustersConfig.Clusters[clusterName]
	return ok && clusterConf.Local, nil
}

func StartLocalNode(app *application.Avalanche, clusterName string, useEtnaDevnet bool, avalanchegoBinaryPath string, anrSettings ANRSettings, avaGoVersionSetting AvalancheGoVersionSettings) error {
	network := models.UndefinedNetwork

	// check if this is existing cluster
	rootDir := app.GetLocalDir(clusterName)
	pluginDir := filepath.Join(rootDir, "node1", "plugins")
	// make sure rootDir exists
	if err := os.MkdirAll(rootDir, 0o700); err != nil {
		return fmt.Errorf("could not create root directory %s: %w", rootDir, err)
	}
	// make sure pluginDir exists
	if err := os.MkdirAll(pluginDir, 0o700); err != nil {
		return fmt.Errorf("could not create plugin directory %s: %w", pluginDir, err)
	}
	ctx, cancel := utils.GetANRContext()
	defer cancel()

	// starts server
	avalancheGoVersion := "latest"
	if avalanchegoBinaryPath == "" {
		avalancheGoVersion, err = getAvalancheGoVersion()
		if err != nil {
			return err
		} else {
			ux.Logger.PrintToUser("Using AvalancheGo version: %s", avalancheGoVersion)
		}
	}
	serverLogPath := filepath.Join(rootDir, "server.log")
	sd := subnet.NewLocalDeployer(app, avalancheGoVersion, avalanchegoBinaryPath, "")
	if err := sd.StartServer(
		constants.ServerRunFileLocalClusterPrefix,
		binutils.LocalClusterGRPCServerPort,
		binutils.LocalClusterGRPCGatewayPort,
		rootDir,
		serverLogPath,
	); err != nil {
		return err
	}
	_, avalancheGoBinPath, err := sd.SetupLocalEnv()
	if err != nil {
		return err
	}
	cli, err := binutils.NewGRPCClientWithEndpoint(binutils.LocalClusterGRPCServerEndpoint)
	if err != nil {
		return err
	}
	alreadyBootstrapped, err := localnet.CheckNetworkIsAlreadyBootstrapped(ctx, cli)
	if err != nil {
		return err
	}
	if alreadyBootstrapped {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("A local cluster is already executing")
		ux.Logger.PrintToUser("please stop it by calling 'node local stop'")
		return nil
	}

	if localClusterDataExists(clusterName) {
		ux.Logger.GreenCheckmarkToUser("Local cluster %s found. Booting up...", clusterName)
		loadSnapshotOpts := []client.OpOption{
			client.WithReassignPortsIfUsed(true),
			client.WithPluginDir(pluginDir),
			client.WithSnapshotPath(rootDir),
		}
		// load snapshot for existing network
		if _, err := cli.LoadSnapshot(
			ctx,
			clusterName,
			true, // in-place
			loadSnapshotOpts...,
		); err != nil {
			return fmt.Errorf("failed to load snapshot: %w", err)
		}
	} else {
		ux.Logger.GreenCheckmarkToUser("Local cluster %s not found. Creating...", clusterName)
		if useEtnaDevnet {
			network = models.NewNetwork(
				models.Devnet,
				constants.EtnaDevnetNetworkID,
				constants.EtnaDevnetEndpoint,
				clusterName,
			)
		} else {
			network, err = networkoptions.GetNetworkFromCmdLineFlags(
				app,
				"",
				globalNetworkFlags,
				false,
				true,
				createSupportedNetworkOptions,
				"",
			)
			if err != nil {
				return err
			}
		}
		if err := preLocalChecks(clusterName); err != nil {
			return err
		}
		if useEtnaDevnet {
			bootstrapIDs = constants.EtnaDevnetBootstrapNodeIDs
			bootstrapIPs = constants.EtnaDevnetBootstrapIPs
			// prepare genesis and upgrade files for anr
			genesisFile, err := os.CreateTemp("", "etna_devnet_genesis")
			if err != nil {
				return fmt.Errorf("could not create save Etna Devnet genesis file: %w", err)
			}
			if _, err := genesisFile.Write(constants.EtnaDevnetGenesisData); err != nil {
				return fmt.Errorf("could not write Etna Devnet genesis data: %w", err)
			}
			if err := genesisFile.Close(); err != nil {
				return fmt.Errorf("could not close Etna Devnet genesis file: %w", err)
			}
			genesisPath = genesisFile.Name()
			defer os.Remove(genesisPath)

			upgradeFile, err := os.CreateTemp("", "etna_devnet_upgrade")
			if err != nil {
				return fmt.Errorf("could not create save Etna Devnet upgrade file: %w", err)
			}
			if _, err := upgradeFile.Write(constants.EtnaDevnetUpgradeData); err != nil {
				return fmt.Errorf("could not write Etna Devnet upgrade data: %w", err)
			}
			upgradePath = upgradeFile.Name()
			if err := upgradeFile.Close(); err != nil {
				return fmt.Errorf("could not close Etna Devnet upgrade file: %w", err)
			}
			defer os.Remove(upgradePath)
		}

		if stakingTLSKeyPath != "" && stakingCertKeyPath != "" && stakingSignerKeyPath != "" {
			if err := os.MkdirAll(filepath.Join(rootDir, "node1", "staking"), 0o700); err != nil {
				return fmt.Errorf("could not create root directory %s: %w", rootDir, err)
			}
			if err := utils.FileCopy(stakingTLSKeyPath, filepath.Join(rootDir, "node1", "staking", "staker.key")); err != nil {
				return err
			}
			if err := utils.FileCopy(stakingCertKeyPath, filepath.Join(rootDir, "node1", "staking", "staker.crt")); err != nil {
				return err
			}
			if err := utils.FileCopy(stakingSignerKeyPath, filepath.Join(rootDir, "node1", "staking", "signer.key")); err != nil {
				return err
			}
		}

		anrOpts := []client.OpOption{
			client.WithNumNodes(1),
			client.WithNetworkID(network.ID),
			client.WithExecPath(avalancheGoBinPath),
			client.WithRootDataDir(rootDir),
			client.WithReassignPortsIfUsed(true),
			client.WithPluginDir(pluginDir),
			client.WithFreshStakingIds(true),
			client.WithZeroIP(false),
		}
		if genesisPath != "" && utils.FileExists(genesisPath) {
			anrOpts = append(anrOpts, client.WithGenesisPath(genesisPath))
		}
		if upgradePath != "" && utils.FileExists(upgradePath) {
			anrOpts = append(anrOpts, client.WithUpgradePath(upgradePath))
		}
		if bootstrapIDs != nil {
			anrOpts = append(anrOpts, client.WithBootstrapNodeIDs(bootstrapIDs))
		}
		if bootstrapIPs != nil {
			anrOpts = append(anrOpts, client.WithBootstrapNodeIPPortPairs(bootstrapIPs))
		}

		ux.Logger.PrintToUser("Starting local avalanchego node using root: %s ...", rootDir)
		spinSession := ux.NewUserSpinner()
		spinner := spinSession.SpinToUser("Booting Network. Wait until healthy...")
		if _, err := cli.Start(ctx, avalancheGoBinPath, anrOpts...); err != nil {
			ux.SpinFailWithError(spinner, "", err)
			localDestroyNode(nil, []string{clusterName})
			return fmt.Errorf("failed to start local avalanchego: %w", err)
		}
		ux.SpinComplete(spinner)
		// save cluster config for the new local cluster
		if err := addLocalClusterConfig(network); err != nil {
			return err
		}
	}

	ux.Logger.GreenCheckmarkToUser("Avalanchego started and ready to use from %s", rootDir)
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Node logs directory: %s/node1/logs", rootDir)
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Network ready to use.")
	ux.Logger.PrintToUser("")

	status, err := cli.Status(ctx)
	if err != nil {
		return err
	}
	for _, nodeInfo := range status.ClusterInfo.NodeInfos {
		ux.Logger.PrintToUser("URI: %s", nodeInfo.Uri)
		ux.Logger.PrintToUser("NodeID: %s", nodeInfo.Id)
		ux.Logger.PrintToUser("")
	}

	return nil
}
