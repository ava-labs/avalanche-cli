// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ux

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

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
	formattedMsg := fmt.Sprintf(msg, args...)
	if ul != nil {
		fmt.Fprintln(ul.Writer, formattedMsg)
		ul.log.Info(formattedMsg)
	} else {
		fmt.Println(formattedMsg)
	}
}

// Info prints to the log file
func (ul *UserLog) Info(msg string, args ...interface{}) {
	formattedMsg := fmt.Sprintf(msg, args...)
	ul.log.Info(formattedMsg)
}

// Error prints to the log file
func (ul *UserLog) Error(msg string, args ...interface{}) {
	formattedMsg := fmt.Sprintf(msg, args...)
	ul.log.Error(formattedMsg)
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

// PrintEndpointTables prints the endpoints coming from the healthy call
func PrintEndpointTables(clusterInfo *rpcpb.ClusterInfo) error {
	if err := PrintTableEndpoints(clusterInfo, false); err != nil {
		return err
	}
	Logger.PrintToUser("")
	if utils.InsideCodespace() {
		Logger.PrintToUser("Codespace node endpoints:")
		if err := PrintTableEndpoints(clusterInfo, true); err != nil {
			return err
		}
		Logger.PrintToUser("")
	}
	return nil
}

func PrintTableEndpoints(clusterInfo *rpcpb.ClusterInfo, codespaceURLs bool) error {
	table := tablewriter.NewWriter(os.Stdout)
	header := []string{"node", "VM", "URL", "ALIAS_URL"}
	table.SetHeader(header)
	table.SetRowLine(true)

	nodeInfos := map[string]*rpcpb.NodeInfo{}
	for _, nodeInfo := range clusterInfo.NodeInfos {
		nodeInfos[nodeInfo.Name] = nodeInfo
	}
	for _, nodeName := range clusterInfo.NodeNames {
		nodeInfo := nodeInfos[nodeName]
		for blockchainID, chainInfo := range clusterInfo.CustomChains {
			blockchainIDURL := fmt.Sprintf("%s/ext/bc/%s/rpc", nodeInfo.GetUri(), blockchainID)
			aliasedURL := fmt.Sprintf("%s/ext/bc/%s/rpc", nodeInfo.GetUri(), chainInfo.ChainName)
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
			}
			table.Append([]string{nodeInfo.Name, chainInfo.ChainName, blockchainIDURL, aliasedURL})
		}
	}
	table.Render()
	return nil
}

func ConvertToStringWithThousandSeparator(input uint64) string {
	p := message.NewPrinter(language.English)
	s := p.Sprintf("%d", input)
	return strings.ReplaceAll(s, ",", "_")
}
