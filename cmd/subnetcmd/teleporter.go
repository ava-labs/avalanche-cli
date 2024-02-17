// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/vms/platformvm"

	"github.com/spf13/cobra"
)

// avalanche subnet teleporter
func newTeleporterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "teleporter",
		Short:             "Deploys teleporter to local network cchain",
		Long:              `Deploys teleporter to a local network cchain.`,
		SilenceUsage:      true,
		RunE:              deployTeleporter,
		PersistentPostRun: handlePostRun,
		Args:              cobra.ExactArgs(0),
	}
	return cmd
}

/*
Loading EWOQ key
Teleporter Messenger successfully deployed to c-chain (0xF7cBd95f1355f0d8d659864b92e2e9fbfaB786f7)
Teleporter Registry successfully deployed to c-chain (0x17aB05351fC94a1a67Bf3f56DdbB941aE6c63E25)

Loading pp1-teleporter-thexl key
Teleporter Messenger successfully deployed to pp1 (0xF7cBd95f1355f0d8d659864b92e2e9fbfaB786f7)
Teleporter Registry successfully deployed to pp1 (0xcb65EF152B10ae00500EfDC7E4CD20358e64b233)

AWM-Relayer v0.2.12 is already installed

Blockchain ready to use. Local network node endpoints:
+-------+-----+-------------------------------------------------------------------------------------+--------------------------------------+
| NODE  | VM  |                                         URL                                         |              ALIAS URL               |
+-------+-----+-------------------------------------------------------------------------------------+--------------------------------------+
| node1 | pp1 | http://127.0.0.1:9650/ext/bc/2iH9UhEo9JhV68VhkNQ7kbp3vhTLaGBs8JbLBy4QkMtbZveNCe/rpc | http://127.0.0.1:9650/ext/bc/pp1/rpc |
+-------+-----+-------------------------------------------------------------------------------------+--------------------------------------+
| node2 | pp1 | http://127.0.0.1:9652/ext/bc/2iH9UhEo9JhV68VhkNQ7kbp3vhTLaGBs8JbLBy4QkMtbZveNCe/rpc | http://127.0.0.1:9652/ext/bc/pp1/rpc |
+-------+-----+-------------------------------------------------------------------------------------+--------------------------------------+
| node3 | pp1 | http://127.0.0.1:9654/ext/bc/2iH9UhEo9JhV68VhkNQ7kbp3vhTLaGBs8JbLBy4QkMtbZveNCe/rpc | http://127.0.0.1:9654/ext/bc/pp1/rpc |
+-------+-----+-------------------------------------------------------------------------------------+--------------------------------------+
| node4 | pp1 | http://127.0.0.1:9656/ext/bc/2iH9UhEo9JhV68VhkNQ7kbp3vhTLaGBs8JbLBy4QkMtbZveNCe/rpc | http://127.0.0.1:9656/ext/bc/pp1/rpc |
+-------+-----+-------------------------------------------------------------------------------------+--------------------------------------+
| node5 | pp1 | http://127.0.0.1:9658/ext/bc/2iH9UhEo9JhV68VhkNQ7kbp3vhTLaGBs8JbLBy4QkMtbZveNCe/rpc | http://127.0.0.1:9658/ext/bc/pp1/rpc |
+-------+-----+-------------------------------------------------------------------------------------+--------------------------------------+
*/

func getSubnetInfos(endpoint string, registryMap map[string]string) ([]teleporter.AWMRelayerSubnetInfo, error) {
	subnetsInfo := []teleporter.AWMRelayerSubnetInfo{}
	pClient := platformvm.NewClient(endpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	blockChains, err := pClient.GetBlockchains(ctx)
	if err != nil {
		return nil, err
	}
	for _, chain := range blockChains {
		if chain.Name == "X-Chain" {
			continue
		}
		subnetsInfo = append(subnetsInfo, teleporter.AWMRelayerSubnetInfo{
			SubnetID:                  chain.SubnetID.String(),
			BlockchainID:              chain.ID.String(),
			TeleporterRegistryAddress: registryMap[chain.Name],
		})
	}
	return subnetsInfo, nil
}

func deployTeleporter(cmd *cobra.Command, args []string) error {
	subnetsInfo, err := getSubnetInfos(constants.LocalAPIEndpoint, map[string]string{})
	if err != nil {
		return err
	}
	return teleporter.DeployAWMRelayer(app, "v0.2.12", models.LocalNetwork, subnetsInfo, "0xF7cBd95f1355f0d8d659864b92e2e9fbfaB786f7")
}
