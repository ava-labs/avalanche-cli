// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
)

func MigrateANRToTmpNet(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
) error {
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	clusterToReload := ""
	cli, _ := binutils.NewGRPCClientWithEndpoint(
		binutils.LocalClusterGRPCServerEndpoint,
		binutils.WithAvoidRPCVersionCheck(true),
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if cli != nil {
		// ANR is running
		status, _ := cli.Status(ctx)
		if status != nil && status.ClusterInfo != nil {
			// there is a local cluster up
			if status.ClusterInfo.NetworkId != constants.LocalNetworkID {
				clusterToReload = filepath.Base(status.ClusterInfo.RootDataDir)
				printFunc("Found running cluster %s. Will restart after migration.", clusterToReload)
			}
		}
	}
	if clusterToReload != "" {
		printFunc("Restarting cluster %s.", clusterToReload)
	}
	return nil
}
