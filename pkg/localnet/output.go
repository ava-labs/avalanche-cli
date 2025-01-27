// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"fmt"
	"sort"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/jedib0t/go-pretty/v6/table"
)

// PrintEndpoints prints the network endpoints
func PrintEndpoints(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
	blockchainName string,
) error {
	if isBoostrapped, err := LocalNetworkIsBootstrapped(app); err != nil {
		return err
	} else if isBoostrapped {
		networkDir, err := GetLocalNetworkDir(app)
		if err != nil {
			return err
		}
		blockchains, err := GetLocalNetworkBlockchainInfo(app)
		if err != nil {
			return err
		}
		for _, blockchain := range blockchains {
			if blockchainName == "" || blockchain.Name == blockchainName {
				if err := PrintSubnetEndpoints(app, printFunc, networkDir, blockchain); err != nil {
					return err
				}
				printFunc("")
			}
		}
		if err := PrintNetworkEndpoints("Primary Nodes", printFunc, networkDir); err != nil {
			return err
		}
		clusterInfo, err := GetClusterInfoWithEndpoint(binutils.LocalClusterGRPCServerEndpoint)
		if err == nil {
			printFunc("")
			if err := PrintNetworkEndpointsFromClusterInfo("L1 Nodes", printFunc, clusterInfo); err != nil {
				return err
			}
		}
	}
	return nil
}

func PrintSubnetEndpoints(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
	networkDir string,
	blockchain BlockchainInfo,
) error {
	node, err := GetTmpNetFirstNode(networkDir)
	if err != nil {
		return err
	}
	t := ux.DefaultTable(fmt.Sprintf("%s RPC URLs", blockchain.Name), nil)
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
	})
	blockchainIDURL := fmt.Sprintf("%s/ext/bc/%s/rpc", FixTmpNetURI(node.URI), blockchain.ID)
	sc, err := app.LoadSidecar(blockchain.Name)
	if err == nil {
		rpcEndpoints := sc.Networks[models.NewLocalNetwork().Name()].RPCEndpoints
		if len(rpcEndpoints) > 0 {
			blockchainIDURL = rpcEndpoints[0]
		}
	}
	t.AppendRow(table.Row{"Localhost", blockchainIDURL})
	if utils.InsideCodespace() {
		codespaceURL, err := utils.GetCodespaceURL(blockchainIDURL)
		if err != nil {
			return err
		}
		t.AppendRow(table.Row{"Codespace", codespaceURL})
	}
	printFunc(t.Render())
	return nil
}

func PrintNetworkEndpointsFromClusterInfo(
	title string,
	printFunc func(msg string, args ...interface{}),
	clusterInfo *rpcpb.ClusterInfo,
) error {
	header := table.Row{"Node ID", "Localhost Endpoint"}
	insideCodespace := utils.InsideCodespace()
	if insideCodespace {
		header = append(header, "Codespace Endpoint")
	}
	t := ux.DefaultTable(title, header)
	nodeNames := clusterInfo.NodeNames
	sort.Strings(nodeNames)
	nodeInfos := map[string]*rpcpb.NodeInfo{}
	for _, nodeInfo := range clusterInfo.NodeInfos {
		nodeInfos[nodeInfo.Name] = nodeInfo
	}
	var err error
	for _, nodeName := range nodeNames {
		nodeInfo := nodeInfos[nodeName]
		nodeURL := nodeInfo.GetUri()
		row := table.Row{nodeInfo.Id, nodeURL}
		if insideCodespace {
			nodeURL, err = utils.GetCodespaceURL(nodeURL)
			if err != nil {
				return err
			}
			row = append(row, nodeURL)
		}
		t.AppendRow(row)
	}
	printFunc(t.Render())
	return nil
}

func PrintNetworkEndpoints(
	title string,
	printFunc func(msg string, args ...interface{}),
	networkDir string,
) error {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return err
	}
	header := table.Row{"Node ID", "Localhost Endpoint"}
	insideCodespace := utils.InsideCodespace()
	if insideCodespace {
		header = append(header, "Codespace Endpoint")
	}
	t := ux.DefaultTable(title, header)
	for _, node := range network.Nodes {
		row := table.Row{node.NodeID, FixTmpNetURI(node.URI)}
		if insideCodespace {
			if codespaceURL, err := utils.GetCodespaceURL(FixTmpNetURI(node.URI)); err != nil {
				return err
			} else {
				row = append(row, codespaceURL)
			}
		}
		t.AppendRow(row)
	}
	printFunc(t.Render())
	return nil
}
