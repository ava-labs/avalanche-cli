// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "(ALPHA Warning) Import cluster configuration from a file",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node import command export cluster configuration and nodes from a text file.
If no file is specified, the configuration is printed to the stdout.`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         importFile,
	}
	cmd.Flags().StringVar(&clusterFileName, "file", "", "specify the file to export the cluster configuration to")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite the cluster if it exists")
	return cmd
}

func importFile(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	clusterExists, err := checkClusterExists(clusterName)
	if err != nil {
		ux.Logger.RedXToUser("error checking cluster: %w", err)
		return err
	} else if clusterExists && !force {
		ux.Logger.RedXToUser("cluster already exists, use --force to overwrite")
		return nil
	}

	importCluster, err := readExportClusterFromFile(clusterFileName)
	if err != nil {
		ux.Logger.RedXToUser("error reading file: %w", err)
		return err
	}
	// check for existing nodes
	for _, node := range importCluster.Nodes {
		keyPath := filepath.Join(app.GetNodesDir(), node.NodeConfig.NodeID)
		if utils.DirectoryExists(keyPath) && !force {
			ux.Logger.RedXToUser("node %s already exists, use --force to overwrite", node.NodeConfig.NodeID)
			return nil
		}
	}
	if importCluster.ClusterConfig.MonitoringInstance != "" {
		keyPath := filepath.Join(app.GetNodesDir(), importCluster.MonitorNode.NodeConfig.NodeID)
		if utils.DirectoryExists(keyPath) && !force {
			ux.Logger.RedXToUser("monitor node %s already exists, use --force to overwrite", importCluster.MonitorNode.NodeConfig.NodeID)
			return nil
		}
	}

	// add nodes
	for _, node := range importCluster.Nodes {
		keyPath := filepath.Join(app.GetNodesDir(), node.NodeConfig.NodeID)
		nc := node.NodeConfig
		if err := app.CreateNodeCloudConfigFile(node.NodeConfig.NodeID, &nc); err != nil {
			ux.Logger.RedXToUser("error creating node config file: %w", err)
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
			ux.Logger.RedXToUser("error creating monitor node config file: %w", err)
			return err
		}
	}
	// add cluster
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		ux.Logger.RedXToUser("error loading clusters config: %w", err)
		return err
	}
	clustersConfig.Clusters[clusterName] = importCluster.ClusterConfig
	if err := app.WriteClustersConfigFile(&clustersConfig); err != nil {
		ux.Logger.RedXToUser("error saving clusters config: %w", err)
	}
	ux.Logger.GreenCheckmarkToUser("cluster %s imported successfully", clusterName)
	return nil
}

// readExportClusterFromFile  reads the export cluster configuration from a file
func readExportClusterFromFile(filename string) (exportCluster, error) {
	var cluster exportCluster
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
	if err := utils.WriteStringToFile(secret, filePath); err != nil {
		ux.Logger.RedXToUser("error writing %s file: %w", filePath, err)
		return err
	}
	return nil
}
