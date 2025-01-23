// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"fmt"
	"sort"

	"golang.org/x/exp/maps"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
)

// PrintEndpoints prints the network endpoints
func PrintEndpoints(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
	subnetName string,
) error {
	if InfoExists(app) {
		networkDir, err := ReadInfo(app)
		if err != nil {
			return err
		}
		network, err := tmpnet.ReadNetwork(networkDir)
		if err != nil {
			return err
		}
		if err := PrintNetworkEndpoints("Primary Nodes", printFunc, network); err != nil {
			return err
		}
	}
	/*
		if err := PrintNetworkEndpoints("Primary Nodes", printFunc, clusterInfo); err != nil {
			return err
		}
	*/
	return nil
}

// PrintEndpoints prints the network endpoints
func PrintEndpointsOld(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
	subnetName string,
) error {
	clusterInfo, err := GetClusterInfo()
	if err != nil {
		return err
	}
	for _, chainInfo := range clusterInfo.CustomChains {
		if subnetName == "" || chainInfo.ChainName == subnetName {
			if err := PrintSubnetEndpoints(app, printFunc, clusterInfo, chainInfo); err != nil {
				return err
			}
			printFunc("")
		}
	}
	if err := PrintNetworkEndpointsOld("Primary Nodes", printFunc, clusterInfo); err != nil {
		return err
	}
	clusterInfo, err = GetClusterInfoWithEndpoint(binutils.LocalClusterGRPCServerEndpoint)
	if err == nil {
		printFunc("")
		if err := PrintNetworkEndpointsOld("L1 Nodes", printFunc, clusterInfo); err != nil {
			return err
		}
	}
	return nil
}

func PrintSubnetEndpoints(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
	clusterInfo *rpcpb.ClusterInfo,
	chainInfo *rpcpb.CustomChainInfo,
) error {
	nodeInfos := maps.Values(clusterInfo.NodeInfos)
	nodeUris := utils.Map(nodeInfos, func(nodeInfo *rpcpb.NodeInfo) string { return nodeInfo.GetUri() })
	if len(nodeUris) == 0 {
		return fmt.Errorf("network has no nodes")
	}
	sort.Strings(nodeUris)
	refNodeURI := nodeUris[0]
	nodeInfo := utils.Find(nodeInfos, func(nodeInfo *rpcpb.NodeInfo) bool { return nodeInfo.GetUri() == refNodeURI })
	if nodeInfo == nil {
		return fmt.Errorf("unexpected nil nodeInfo")
	}
	t := ux.DefaultTable(fmt.Sprintf("%s RPC URLs", chainInfo.ChainName), nil)
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
	})
	blockchainIDURL := fmt.Sprintf("%s/ext/bc/%s/rpc", (*nodeInfo).GetUri(), chainInfo.ChainId)
	sc, err := app.LoadSidecar(chainInfo.ChainName)
	if err == nil {
		rpcEndpoints := sc.Networks[models.NewLocalNetwork().Name()].RPCEndpoints
		if len(rpcEndpoints) > 0 {
			blockchainIDURL = rpcEndpoints[0]
		}
	}
	t.AppendRow(table.Row{"Localhost", blockchainIDURL})
	if utils.InsideCodespace() {
		var err error
		blockchainIDURL, err = utils.GetCodespaceURL(blockchainIDURL)
		if err != nil {
			return err
		}
		t.AppendRow(table.Row{"Codespace", blockchainIDURL})
	}
	printFunc(t.Render())
	return nil
}

func PrintNetworkEndpointsOld(
	title string,
	printFunc func(msg string, args ...interface{}),
	clusterInfo *rpcpb.ClusterInfo,
) error {
	header := table.Row{"Name", "Node ID", "Localhost Endpoint"}
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
		row := table.Row{nodeInfo.Name, nodeInfo.Id, nodeURL}
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
	network *tmpnet.Network,
) error {
	header := table.Row{"Node ID", "Localhost Endpoint"}
	insideCodespace := utils.InsideCodespace()
	if insideCodespace {
		header = append(header, "Codespace Endpoint")
	}
	t := ux.DefaultTable(title, header)
	for _, node := range network.Nodes {
		row := table.Row{node.NodeID, node.URI}
		if insideCodespace {
			if codespaceURL, err := utils.GetCodespaceURL(node.URI); err != nil {
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
