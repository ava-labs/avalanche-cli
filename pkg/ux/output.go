// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ux

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"golang.org/x/exp/maps"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
)

var Logger *UserLog

type UserLog struct {
	log    logging.Logger
	Writer io.Writer
}

func NewUserLog(log logging.Logger, userwriter io.Writer) {
	if Logger == nil {
		Logger = &UserLog{
			log:    log,
			Writer: userwriter,
		}
	}
}

// PrintToUser prints msg directly on the screen, but also to log file
func (ul *UserLog) PrintToUser(msg string, args ...interface{}) {
	fmt.Print("\r\033[K") // Clear the line from the cursor position to the end
	ul.print(fmt.Sprintf(msg, args...) + "\n")
}

func (ul *UserLog) print(msg string) {
	if ul != nil {
		fmt.Fprint(ul.Writer, msg)
		ul.log.Info(msg)
	} else {
		fmt.Print(msg)
	}
}

// Info prints to the log file
func (ul *UserLog) Info(msg string, args ...interface{}) {
	ul.log.Info(fmt.Sprintf(msg, args...) + "\n")
}

// Error prints to the log file
func (ul *UserLog) Error(msg string, args ...interface{}) {
	ul.log.Error(fmt.Sprintf(msg, args...))
}

// GreenCheckmarkToUser prints a green checkmark to the user before the message
func (ul *UserLog) GreenCheckmarkToUser(msg string, args ...interface{}) {
	checkmark := "\u2713" // Unicode for checkmark symbol
	green := color.New(color.FgHiGreen).SprintFunc()
	ul.PrintToUser(green(checkmark)+" "+msg, args...)
}

func (ul *UserLog) RedXToUser(msg string, args ...interface{}) {
	xmark := "\u2717" // Unicode for X symbol
	red := color.New(color.FgHiRed).SprintFunc()
	ul.PrintToUser(red(xmark)+" "+msg, args...)
}

func (ul *UserLog) PrintLineSeparator() {
	ul.PrintToUser("==============================================")
}

// PrintWait does some dot printing to entertain the user
func PrintWait(cancel chan struct{}) {
	for {
		select {
		case <-time.After(1 * time.Second):
			fmt.Print(".")
		case <-cancel:
			return
		}
	}
}

// PrintLocalNetworkEndpointsInfo prints the endpoints coming from the status call
func PrintLocalNetworkEndpointsInfo(clusterInfo *rpcpb.ClusterInfo) error {
	for _, chainInfo := range clusterInfo.CustomChains {
		if err := PrintSubnetEndpoints(clusterInfo, chainInfo, utils.InsideCodespace()); err != nil {
			return err
		}
		Logger.PrintToUser("")
	}
	if err := PrintNetworkEndpoints(clusterInfo, utils.InsideCodespace()); err != nil {
		return err
	}
	return nil
}

func PrintSubnetEndpoints(clusterInfo *rpcpb.ClusterInfo, chainInfo *rpcpb.CustomChainInfo, codespaceURLs bool) error {
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
	strBuilder := strings.Builder{}
	table := tablewriter.NewWriter(&strBuilder)
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	aliasedURL := fmt.Sprintf("%s/ext/bc/%s/rpc", (*nodeInfo).GetUri(), chainInfo.ChainName)
	blockchainIDURL := fmt.Sprintf("%s/ext/bc/%s/rpc", (*nodeInfo).GetUri(), chainInfo.ChainId)
	table.Append([]string{fmt.Sprintf("%s RPC URLs", chainInfo.ChainName)})
	table.ClearRows()
	table.Append([]string{"Localhost", aliasedURL})
	table.Append([]string{"Localhost", blockchainIDURL})
	if codespaceURLs {
		var err error
		blockchainIDURL, err = utils.GetCodespaceURL(blockchainIDURL)
		if err != nil {
			return err
		}
		aliasedURL, err = utils.GetCodespaceURL(aliasedURL)
		if err != nil {
			return err
		}
		table.Append([]string{"Codespace", aliasedURL})
		table.Append([]string{"Codespace", blockchainIDURL})
	}
	table.Render()
	tableStr := strBuilder.String()
	var err error
	tableStr, err = addTitleToTable(tableStr, fmt.Sprintf("%s RPC URLs", chainInfo.ChainName))
	if err != nil {
		return err
	}
	Logger.print(tableStr)
	return nil
}

func addTitleToTable(tableStr string, title string) (string, error) {
	newLineIdx := strings.Index(tableStr, "\n")
	if newLineIdx == -1 {
		return "", fmt.Errorf("expected to found newline in table output")
	}
	titleStr := tableStr[:newLineIdx] + "\n"
	availableLen := newLineIdx - 2
	if availableLen < len(title) {
		title = title[:availableLen-1]
	}
	spacesCount := availableLen - len(title)
	spaces1 := spacesCount / 2
	spaces2 := spacesCount - spaces1
	titleStr = titleStr + "|" + strings.Repeat(" ", spaces1) + title + strings.Repeat(" ", spaces2) + "|" + "\n"
	return titleStr + tableStr, nil
}

func PrintNetworkEndpoints(clusterInfo *rpcpb.ClusterInfo, codespaceURLs bool) error {
	strBuilder := strings.Builder{}
	table := tablewriter.NewWriter(&strBuilder)
	table.SetRowLine(true)
	header := []string{"Name", "Node ID", "Localhost Endpoint"}
	if codespaceURLs {
		header = append(header, "Codespace Endpoint")
	}
	table.Append(header)
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
		row := []string{nodeInfo.Name, nodeInfo.Id, nodeURL}
		if codespaceURLs {
			nodeURL, err = utils.GetCodespaceURL(nodeURL)
			if err != nil {
				return err
			}
			row = append(row, nodeURL)
		}
		table.Append(row)
	}
	table.Render()
	tableStr := strBuilder.String()
	tableStr, err = addTitleToTable(tableStr, "Nodes")
	if err != nil {
		return err
	}
	Logger.print(tableStr)
	return nil
}

func ConvertToStringWithThousandSeparator(input uint64) string {
	p := message.NewPrinter(language.English)
	s := p.Sprintf("%d", input)
	return strings.ReplaceAll(s, ",", "_")
}
