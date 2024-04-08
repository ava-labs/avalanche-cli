// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

var (
	clusterFileName string
	force           bool
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "(ALPHA Warning) Export cluster configuration to a file",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node export command export clusters configuration including their nodes to a text file.
If no file is specified, the configuration is printed to the stdout.`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         export,
	}
	cmd.Flags().StringVar(&clusterFileName, "file", "", "specify the file to export the cluster configuration to")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite the file if it exists")
	return cmd
}

type exportNode struct {
	nodeConfig models.NodeConfig `json:"nodeConfig"`
	signerKey  string            `json:"signerKey"`
	stakerKey  string            `json:"stakerKey"`
	stakerCrt  string            `json:"stakerCrt"`
}
type exportCluster struct {
	clusterConfig models.ClusterConfig `json:"clusterConfig"`
	nodes         []exportNode         `json:"nodes"`
}

func export(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	if clusterFileName != "" && utils.FileExists(utils.ExpandHome(clusterFileName)) && !force {
		ux.Logger.RedXToUser("file already exists, use --force to overwrite")
		return nil
	}
	if err := checkCluster(clusterName); err != nil {
		ux.Logger.RedXToUser("cluster not found: %w", err)
		return err
	}
	clusterConf, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return err
	}
	nodes, err := utils.MapWithError(clusterConf.Nodes, func(nodeName string) (exportNode, error) {
		var err error
		nodeConf, err := app.LoadClusterNodeConfig(nodeName)
		if err != nil {
			return exportNode{}, err
		}
		signerKey, stakerKey, stakerCrt, err := readKeys(filepath.Join(app.GetNodesDir(), nodeConf.NodeID))
		if err != nil {
			return exportNode{}, err
		}
		return exportNode{
			nodeConfig: nodeConf,
			signerKey:  signerKey,
			stakerKey:  stakerKey,
			stakerCrt:  stakerCrt,
		}, nil
	})
	if err != nil {
		ux.Logger.RedXToUser("could not load node configuration: %s", err)
		return err
	}
	exportCluster := exportCluster{
		clusterConfig: clusterConf,
		nodes:         nodes,
	}
	if clusterFileName != "" {
		outFile, err := os.Create(utils.ExpandHome(clusterFileName))
		if err != nil {
			ux.Logger.RedXToUser("could not create file: %s", err)
			return err
		}
		defer outFile.Close()
		ux.Logger.GreenCheckmarkToUser("exported cluster [%s] configuration to %s", clusterName, clusterFileName)
		writeExportFile(exportCluster, outFile)
	} else {
		writeExportFile(exportCluster, os.Stdout)
	}
	return nil
}

// readKeys reads the keys from the node configuration
func readKeys(nodeConfPath string) (string, string, string, error) {
	signerKey, err := utils.ReadFile(filepath.Join(nodeConfPath, constants.BLSKeyFileName))
	if err != nil {
		return "", "", "", err
	}
	stakerKey, err := utils.ReadFile(filepath.Join(nodeConfPath, constants.StakerKeyFileName))
	if err != nil {
		return "", "", "", err
	}
	stakerCrt, err := utils.ReadFile(filepath.Join(nodeConfPath, constants.StakerCertFileName))
	if err != nil {
		return "", "", "", err
	}
	return signerKey, stakerKey, stakerCrt, nil
}

func writeExportFile(exportCluster exportCluster, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(exportCluster); err != nil {
		return err
	}
	return nil
}
