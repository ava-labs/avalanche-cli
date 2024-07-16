// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"strings"
)

func CallCleanup() error {
	return cleanup(nil, nil)
}

func newCleanupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "(ALPHA Warning) Destroys all existing clusters created by Avalanche CLI",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node cleanup command terminates all running nodes in cloud server previously created by
Avalanche CLI and deletes all storage disks.

If there is a static IP address attached, it will be released.`,
		Args: cobrautils.ExactArgs(0),
		RunE: cleanup,
	}
	cmd.Flags().StringVar(&awsProfile, "aws-profile", constants.AWSDefaultCredential, "aws profile to use")

	return cmd
}

func cleanup(_ *cobra.Command, _ []string) error {
	var err error
	clustersConfig := models.ClustersConfig{}
	if app.ClustersConfigExists() {
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return err
		}
	}
	clusterNames := maps.Keys(clustersConfig.Clusters)
	for _, clusterName := range clusterNames {
		if err = CallDestroyNode(clusterName, false); err != nil {
			if strings.Contains(err.Error(), "invalid cloud credentials") {
				return fmt.Errorf("invalid AWS credentials %s \n", clusterName)
			}
		}
	}
	return nil
}
