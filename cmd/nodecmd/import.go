// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	sdkutils "github.com/ava-labs/avalanche-tooling-sdk-go/utils"

	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import [clusterName]",
		Short: "(ALPHA Warning) Import cluster configuration from a file",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node import command imports cluster configuration and its nodes configuration from a text file
created from the node export command.

Prior to calling this command, call node whitelist command to have your SSH public key and IP whitelisted by
the cluster owner. This will enable you to use avalanche-cli commands to manage the imported cluster.

Please note, that this imported cluster will be considered as EXTERNAL by avalanche-cli, so some commands
affecting cloud nodes like node create or node destroy will be not applicable to it.`,
		Args: cobrautils.ExactArgs(1),
		RunE: importFile,
	}
	cmd.Flags().StringVar(&clusterFileName, "file", "", "specify the file to export the cluster configuration to")
	return cmd
}

func importFile(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	if clusterExists, err := node.CheckClusterExists(app, clusterName); clusterExists || err != nil {
		ux.Logger.RedXToUser("cluster %s already exists, please use a different name", clusterName)
		return nil
	}

	importCluster, err := readExportClusterFromFile(clusterFileName)
	if err != nil {
		ux.Logger.RedXToUser("error reading file: %v", err)
		return err
	}
	importCluster.ClusterConfig.External = true // mark cluster as external
	// check for existing nodes
	nodestoCheck := importCluster.Nodes
	nodestoCheck = append(nodestoCheck, importCluster.LoadTestNodes...)
	if importCluster.ClusterConfig.MonitoringInstance != "" {
		nodestoCheck = append(nodestoCheck, importCluster.MonitorNode)
	}
	for _, node := range nodestoCheck {
		keyPath := filepath.Join(app.GetNodesDir(), node.NodeConfig.NodeID)
		if sdkutils.DirExists(keyPath) {
			ux.Logger.RedXToUser("node %s already exists and belongs to the existing cluster, can't import", node.NodeConfig.NodeID)
			ux.Logger.RedXToUser("you can use destroy command to remove the cluster it belongs to and then retry import")
			return nil
		}
	}
	// add nodes
	for _, node := range importCluster.Nodes {
		keyPath := filepath.Join(app.GetNodesDir(), node.NodeConfig.NodeID)
		nc := node.NodeConfig
		if err := app.CreateNodeCloudConfigFile(node.NodeConfig.NodeID, &nc); err != nil {
			ux.Logger.RedXToUser("error creating node config file: %v", err)
			return err
		}
		if err := writeSecretToFile(node.StakerKey, filepath.Join(keyPath, constants.StakerKeyFileName)); err != nil {
			return err
		}
		if err := writeSecretToFile(node.StakerCrt, filepath.Join(keyPath, constants.StakerCertFileName)); err != nil {
			return err
		}
		if err := writeSecretToFile(node.SignerKey, filepath.Join(keyPath, constants.BLSKeyFileName)); err != nil {
			return err
		}
	}
	if importCluster.ClusterConfig.MonitoringInstance != "" {
		if err := app.CreateNodeCloudConfigFile(importCluster.MonitorNode.NodeConfig.NodeID, &importCluster.MonitorNode.NodeConfig); err != nil {
			ux.Logger.RedXToUser("error creating monitor node config file: %v", err)
			return err
		}
	}
	// add inventory
	inventoryPath := app.GetAnsibleInventoryDirPath(clusterName)
	nodes := sdkutils.Map(importCluster.Nodes, func(node models.ExportNode) models.NodeConfig { return node.NodeConfig })
	if err := ansible.WriteNodeConfigsToAnsibleInventory(inventoryPath, nodes); err != nil {
		ux.Logger.RedXToUser("error writing inventory file: %v", err)
		return err
	}
	if importCluster.ClusterConfig.MonitoringInstance != "" {
		monitoringInventoryPath := app.GetMonitoringInventoryDir(clusterName)
		if err := ansible.WriteNodeConfigsToAnsibleInventory(monitoringInventoryPath, []models.NodeConfig{importCluster.MonitorNode.NodeConfig}); err != nil {
			ux.Logger.RedXToUser("error writing monitoring inventory file: %v", err)
			return err
		}
	}

	// add cluster
	clustersConfig := models.ClustersConfig{}
	clustersConfig.Clusters = make(map[string]models.ClusterConfig)
	clustersConfig, err = app.GetClustersConfig()
	if err != nil {
		ux.Logger.RedXToUser("error loading clusters config: %v", err)
		return err
	}

	importCluster.ClusterConfig.Network.ClusterName = clusterName
	clustersConfig.Clusters[clusterName] = importCluster.ClusterConfig
	if err := app.WriteClustersConfigFile(&clustersConfig); err != nil {
		ux.Logger.RedXToUser("error saving clusters config: %v", err)
	}
	ux.Logger.GreenCheckmarkToUser("cluster [%s] imported successfully", clusterName)
	return nil
}

// readExportClusterFromFile  reads the export cluster configuration from a file
func readExportClusterFromFile(filename string) (models.ExportCluster, error) {
	var cluster models.ExportCluster
	if !utils.FileExists(utils.ExpandHome(filename)) {
		return cluster, fmt.Errorf("file does not exist")
	} else {
		file, err := os.Open(filename)
		if err != nil {
			return cluster, err
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			return cluster, err
		}
		err = json.Unmarshal(data, &cluster)
		if err != nil {
			return cluster, err
		}
		return cluster, nil
	}
}

// writeSecretToFile writes a secret to a file
func writeSecretToFile(secret, filePath string) error {
	if secret == "" {
		return nil // nothing to write(no error)
	}
	if err := utils.WriteStringToFile(filePath, secret); err != nil {
		ux.Logger.RedXToUser("error writing %s file: %w", filePath, err)
		return err
	}
	return nil
}
