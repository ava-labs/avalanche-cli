// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [clusterName] [subnetName]",
		Short: "(ALPHA Warning) Deploy a subnet into a devnet cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node devnet deploy command deploys a subnet into a devnet cluster, creating subnet and blockchain txs for it.
It saves the deploy info both locally and remotely.
`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE:         deploySubnet,
	}
	return cmd
}

func deploySubnet(cmd *cobra.Command, args []string) error {
	clusterName := args[0]
	subnetName := args[1]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	if _, err := subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName}); err != nil {
		return err
	}
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	if clustersConfig.Clusters[clusterName].Network.Kind != models.Devnet {
		return fmt.Errorf("node deploy command must be applied to devnet clusters")
	}
	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	defer disconnectHosts(hosts)
	notHealthyNodes, err := checkHostsAreHealthy(hosts)
	if err != nil {
		return err
	}
	if len(notHealthyNodes) > 0 {
		return fmt.Errorf("node(s) %s are not healthy yet, please try again later", notHealthyNodes)
	}
	incompatibleNodes, err := checkAvalancheGoVersionCompatible(hosts, subnetName)
	if err != nil {
		return err
	}
	if len(incompatibleNodes) > 0 {
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Either modify your Avalanche Go version or modify your VM version")
		ux.Logger.PrintToUser("To modify your Avalanche Go version: https://docs.avax.network/nodes/maintain/upgrade-your-avalanchego-node")
		switch sc.VM {
		case models.SubnetEvm:
			ux.Logger.PrintToUser("To modify your Subnet-EVM version: https://docs.avax.network/build/subnet/upgrade/upgrade-subnet-vm")
		case models.CustomVM:
			ux.Logger.PrintToUser("To modify your Custom VM binary: avalanche subnet upgrade vm %s --config", subnetName)
		}
		ux.Logger.PrintToUser("Yoy can use \"avalanche node upgrade\" to upgrade Avalanche Go and/or Subnet-EVM to their latest versions")
		return fmt.Errorf("the Avalanche Go version of node(s) %s is incompatible with VM RPC version of %s", incompatibleNodes, subnetName)
	}

	deployLocal := false
	deployDevnet := true
	deployTestnet := false
	deployMainnet := false
	endpoint := clustersConfig.Clusters[clusterName].Network.Endpoint
	keyNameParam := ""
	useLedgerParam := false
	useEwoqParam := true
	sameControlKey := true

	if err := subnetcmd.CallDeploy(
		cmd,
		subnetName,
		deployLocal,
		deployDevnet,
		deployTestnet,
		deployMainnet,
		endpoint,
		keyNameParam,
		useLedgerParam,
		useEwoqParam,
		sameControlKey,
	); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Subnet successfully deployed into devnet!")
	return nil
}
