// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"fmt"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/jedib0t/go-pretty/v6/table"
)

// PrintEndpoints prints the endpoint information for the executing local network,
// including primary nodes, l1 nodes, and blockchain URLs for all blockchains in the
// network
// If [blockchainName] is given, only prints information for it
func PrintEndpoints(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
	blockchainName string,
) error {
	if isRunning, err := IsLocalNetworkRunning(app); err != nil {
		return err
	} else if isRunning {
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
				if err := PrintBlockchainEndpoints(app, printFunc, networkDir, blockchain); err != nil {
					return err
				}
				printFunc("")
			}
		}
		if err := PrintNetworkEndpoints("Primary Nodes", printFunc, networkDir); err != nil {
			return err
		}
		if err := PrintL1Endpoints(app, printFunc); err != nil {
			return err
		}
	}
	return nil
}

// PrintBlockchainEndpoints prints out a table of (RPC Kind, RPC) for the given
// [blockchain] associated to the the given tmpnet [networkDir]
// RPC Kind to be in [Localhost, Codespace] where the latest
// is used only if inside a codespace environment
func PrintBlockchainEndpoints(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
	networkDir string,
	blockchain BlockchainInfo,
) error {
	network, err := GetTmpNetNetworkWithURIFix(networkDir)
	if err != nil {
		return err
	}
	node, err := GetTmpNetFirstRunningNode(network)
	if err != nil {
		return err
	}
	t := ux.DefaultTable(fmt.Sprintf("%s RPC URLs", blockchain.Name), nil)
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
	})
	blockchainIDURL := fmt.Sprintf("%s/ext/bc/%s/rpc", node.URI, blockchain.ID)
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

// PrintNetworkEndpoints prints out a table of (Node ID, Node URI) for a given
// tmpnet [networkDir], with a given [title]
// If the environment is codespace based, It also adds a node codespace URI
func PrintNetworkEndpoints(
	title string,
	printFunc func(msg string, args ...interface{}),
	networkDir string,
) error {
	network, err := GetTmpNetNetworkWithURIFix(networkDir)
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

// PrintL1Endpoints prints out a table of (Node ID, Endpoint, L1) for all running clusters
// connected to the local network
// If the environment is codespace based, It also adds a node codespace URI
func PrintL1Endpoints(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
) error {
	clusters, err := GetRunningLocalClustersConnectedToLocalNetwork(app)
	if err != nil {
		return err
	}
	if len(clusters) == 0 {
		return nil
	}
	header := table.Row{"Node ID", "Localhost Endpoint"}
	insideCodespace := utils.InsideCodespace()
	if insideCodespace {
		header = append(header, "Codespace Endpoint")
	}
	header = append(header, "L1")
	t := ux.DefaultTable("L1 NODES", header)
	for _, clusterName := range clusters {
		validatedBlockchainsInfo, err := GetLocalClusterValidatedBlockchains(app, clusterName)
		if err != nil {
			return err
		}
		validatedBlockchains := utils.Map(validatedBlockchainsInfo, func(i BlockchainInfo) string { return i.Name })
		networkDir := GetLocalClusterDir(app, clusterName)
		network, err := GetTmpNetNetworkWithURIFix(networkDir)
		if err != nil {
			return err
		}
		for _, node := range network.Nodes {
			row := table.Row{node.NodeID, node.URI}
			if insideCodespace {
				if codespaceURL, err := utils.GetCodespaceURL(node.URI); err != nil {
					return err
				} else {
					row = append(row, codespaceURL)
				}
			}
			row = append(row, strings.Join(validatedBlockchains, ","))
			t.AppendRow(row)
		}
	}
	printFunc(t.Render())
	return nil
}
