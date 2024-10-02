// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"errors"
	"fmt"
	"sort"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"golang.org/x/exp/maps"
)

// PrintLocalNetworkEndpoints prints the endpoints coming from the status call
func PrintEndpoints(
	printFunc func(msg string, args ...interface{}),
	subnetName string,
) error {
	clusterInfo, err := GetClusterInfo()
	if err != nil {
		return err
	}
	for _, chainInfo := range clusterInfo.CustomChains {
		if subnetName == "" || chainInfo.ChainName == subnetName {
			if err := PrintSubnetEndpoints(printFunc, clusterInfo, chainInfo); err != nil {
				return err
			}
			printFunc("")
		}
	}
	if err := PrintNetworkEndpoints(printFunc, clusterInfo); err != nil {
		return err
	}
	return nil
}

func PrintSubnetEndpoints(
	printFunc func(msg string, args ...interface{}),
	clusterInfo *rpcpb.ClusterInfo,
	chainInfo *rpcpb.CustomChainInfo,
) error {
	nodeInfos := maps.Values(clusterInfo.NodeInfos)
	nodeUris := utils.Map(nodeInfos, func(nodeInfo *rpcpb.NodeInfo) string { return nodeInfo.GetUri() })
	if len(nodeUris) == 0 {
		return errors.New("network has no nodes")
	}
	sort.Strings(nodeUris)
	refNodeURI := nodeUris[0]
	nodeInfo := utils.Find(nodeInfos, func(nodeInfo *rpcpb.NodeInfo) bool { return nodeInfo.GetUri() == refNodeURI })
	if nodeInfo == nil {
		return errors.New("unexpected nil nodeInfo")
	}
	t := table.NewWriter()
	t.Style().Title.Align = text.AlignCenter
	t.Style().Title.Format = text.FormatUpper
	t.Style().Options.SeparateRows = true
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
	})
	t.SetTitle(chainInfo.ChainName + " RPC URLs")
	aliasedURL := fmt.Sprintf("%s/ext/bc/%s/rpc", (*nodeInfo).GetUri(), chainInfo.ChainName)
	blockchainIDURL := fmt.Sprintf("%s/ext/bc/%s/rpc", (*nodeInfo).GetUri(), chainInfo.ChainId)
	t.AppendRow(table.Row{"Localhost", aliasedURL})
	t.AppendRow(table.Row{"Localhost", blockchainIDURL})
	if utils.InsideCodespace() {
		var err error
		blockchainIDURL, err = utils.GetCodespaceURL(blockchainIDURL)
		if err != nil {
			return err
		}
		aliasedURL, err = utils.GetCodespaceURL(aliasedURL)
		if err != nil {
			return err
		}
		t.AppendRow(table.Row{"Codespace", aliasedURL})
		t.AppendRow(table.Row{"Codespace", blockchainIDURL})
	}
	printFunc(t.Render())
	return nil
}

func PrintNetworkEndpoints(
	printFunc func(msg string, args ...interface{}),
	clusterInfo *rpcpb.ClusterInfo,
) error {
	t := table.NewWriter()
	t.Style().Title.Align = text.AlignCenter
	t.Style().Title.Format = text.FormatUpper
	t.Style().Options.SeparateRows = true
	t.SetTitle("Nodes")
	header := table.Row{"Name", "Node ID", "Localhost Endpoint"}
	insideCodespace := utils.InsideCodespace()
	if insideCodespace {
		header = append(header, "Codespace Endpoint")
	}
	t.AppendHeader(header)
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
