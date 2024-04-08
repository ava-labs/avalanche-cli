// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/spf13/cobra"
)

func newAddDashboardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addDashboard [clusterName]",
		Short: "(ALPHA Warning) Adds custom dashboard for existing devnet cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode. 

The node addDashboard command adds custom dashboard to the Grafana monitoring dashboard for the 
cluster.`,

		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         addDashboard,
	}
	cmd.Flags().StringVar(&customGrafanaDashboardPath, "add-grafana-dashboard", "", "path to additional grafana dashboard json file")
	cmd.Flags().StringVar(&subnetName, "subnet", "", "subnet that the dasbhoard is intended for (if any)")
	return cmd
}

func addDashboard(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	if customGrafanaDashboardPath != "" {
		if err := addCustomDashboard(clusterName, subnetName); err != nil {
			return err
		}
	}
	return nil
}

func addCustomDashboard(clusterName, subnetName string) error {
	monitoringInventoryPath := app.GetMonitoringInventoryDir(clusterName)
	monitoringHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(monitoringInventoryPath)
	if err != nil {
		return err
	}
	_, chainID, err := getDeployedSubnetInfo(clusterName, subnetName)
	if err != nil {
		return err
	}
	return ssh.RunSSHUpdateMonitoringDashboards(monitoringHosts[0], app.GetMonitoringDashboardDir()+"/", customGrafanaDashboardPath, chainID)
}
