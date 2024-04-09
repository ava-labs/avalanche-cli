// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

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
