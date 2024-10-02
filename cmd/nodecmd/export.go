// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var (
	clusterFileName string
	force           bool
	includeSecrets  bool
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export [clusterName]",
		Short: "(ALPHA Warning) Export cluster configuration to a file",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node export command exports cluster configuration and its nodes config to a text file.

If no file is specified, the configuration is printed to the stdout.

Use --include-secrets to include keys in the export. In this case please keep the file secure as it contains sensitive information.

Exported cluster configuration without secrets can be imported by another user using node import command.`,
		Args: cobrautils.ExactArgs(1),
		RunE: exportFile,
	}
	cmd.Flags().StringVar(&clusterFileName, "file", "", "specify the file to export the cluster configuration to")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite the file if it exists")
	cmd.Flags().BoolVar(&includeSecrets, "include-secrets", false, "include keys in the export")
	return cmd
}

func exportFile(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	if clusterFileName != "" && utils.FileExists(utils.ExpandHome(clusterFileName)) && !force {
		ux.Logger.RedXToUser("file already exists, use --force to overwrite")
		return nil
	}
	if err := checkCluster(clusterName); err != nil {
		ux.Logger.RedXToUser("cluster not found: %v", err)
		return err
	}
	clusterConf, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return err
	}
	clusterConf.Network.ClusterName = "" // hide cluster name
	clusterConf.External = true          // mark cluster as external
	nodes, err := utils.MapWithError(clusterConf.Nodes, func(nodeName string) (models.ExportNode, error) {
		var err error
		nodeConf, err := app.LoadClusterNodeConfig(nodeName)
		nodeConf.CertPath, nodeConf.SecurityGroup, nodeConf.KeyPair = "", "", "" // hide cert path and sg id
		if err != nil {
			return models.ExportNode{}, err
		}
		signerKey, stakerKey, stakerCrt, err := readKeys(filepath.Join(app.GetNodesDir(), nodeConf.NodeID))
		if err != nil {
			return models.ExportNode{}, err
		}
		return models.ExportNode{
			NodeConfig: nodeConf,
			SignerKey:  signerKey,
			StakerKey:  stakerKey,
			StakerCrt:  stakerCrt,
		}, nil
	})
	if err != nil {
		ux.Logger.RedXToUser("could not load node configuration: %v", err)
		return err
	}
	// monitoring instance
	monitor := models.ExportNode{}
	if clusterConf.MonitoringInstance != "" {
		monitoringHost, err := app.LoadClusterNodeConfig(clusterConf.MonitoringInstance)
		if err != nil {
			ux.Logger.RedXToUser("could not load monitoring host configuration: %v", err)
			return err
		}
		monitoringHost.CertPath, monitoringHost.SecurityGroup, monitoringHost.KeyPair = "", "", "" // hide cert path and sg id
		monitor = models.ExportNode{
			NodeConfig: monitoringHost,
			SignerKey:  "",
			StakerKey:  "",
			StakerCrt:  "",
		}
	}
	// loadtest nodes
	loadTestNodes := []models.ExportNode{}
	for _, loadTestNode := range clusterConf.LoadTestInstance {
		loadTestNodeConf, err := app.LoadClusterNodeConfig(loadTestNode)
		if err != nil {
			ux.Logger.RedXToUser("could not load load test node configuration: %v", err) //nolint:dupword
			return err
		}
		loadTestNodeConf.CertPath, loadTestNodeConf.SecurityGroup, loadTestNodeConf.KeyPair = "", "", "" // hide cert path and sg id
		loadTestNodes = append(loadTestNodes, models.ExportNode{
			NodeConfig: loadTestNodeConf,
			SignerKey:  "",
			StakerKey:  "",
			StakerCrt:  "",
		})
	}

	exportCluster := models.ExportCluster{
		ClusterConfig: clusterConf,
		Nodes:         nodes,
		MonitorNode:   monitor,
		LoadTestNodes: loadTestNodes,
	}
	if clusterFileName != "" {
		outFile, err := os.Create(utils.ExpandHome(clusterFileName))
		if err != nil {
			ux.Logger.RedXToUser("could not create file: %w", err)
			return err
		}
		defer outFile.Close()
		if err := writeExportFile(exportCluster, outFile); err != nil {
			ux.Logger.RedXToUser("could not write to file: %v", err)
			return err
		}
		ux.Logger.GreenCheckmarkToUser("exported cluster [%s] configuration to %s", clusterName, utils.ExpandHome(outFile.Name()))
	} else {
		if err := writeExportFile(exportCluster, os.Stdout); err != nil {
			ux.Logger.RedXToUser("could not write to stdout: %v", err)
			return err
		}
	}
	return nil
}

// readKeys reads the keys from the node configuration
func readKeys(nodeConfPath string) (string, string, string, error) {
	stakerCrt, err := utils.ReadFile(filepath.Join(nodeConfPath, constants.StakerCertFileName))
	if err != nil {
		return "", "", "", err
	}
	if !includeSecrets {
		return "", "", stakerCrt, nil // return only the certificate
	}
	signerKey, err := utils.ReadFile(filepath.Join(nodeConfPath, constants.BLSKeyFileName))
	if err != nil {
		return "", "", "", err
	}
	stakerKey, err := utils.ReadFile(filepath.Join(nodeConfPath, constants.StakerKeyFileName))
	if err != nil {
		return "", "", "", err
	}

	return signerKey, stakerKey, stakerCrt, nil
}

// writeExportFile writes the exportCluster to the out writer
func writeExportFile(exportCluster models.ExportCluster, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(exportCluster); err != nil {
		return err
	}
	return nil
}
