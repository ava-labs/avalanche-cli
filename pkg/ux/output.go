// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ux

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/briandowns/spinner"
	"github.com/olekukonko/tablewriter"
)

const (
	green          = "\033[32m" // green color
	reset          = "\033[0m"  // reset color
	checkmark      = "✔"
	greenCheckmark = green + checkmark + reset + " "
)

var Logger *UserLog

type UserLog struct {
	log       logging.Logger
	spin      *spinner.Spinner
	spinID    string
	spinMutex sync.Mutex
	writer    io.Writer
}

func NewUserLog(log logging.Logger, userwriter io.Writer) {
	if Logger == nil {
		Logger = &UserLog{
			log:    log,
			spin:   spinner.New(spinner.CharSets[35], 150*time.Millisecond),
			spinID: "",
			writer: userwriter,
		}
	}
}

// PrintToUser prints msg directly on the screen, but also to log file
func (ul *UserLog) PrintToUser(msg string, args ...interface{}) {
	if ul.spin.Active() {
		ul.spin.Stop()
	}
	formattedMsg := fmt.Sprintf(msg, args...)
	fmt.Fprintln(ul.writer, formattedMsg)
	ul.log.Info(formattedMsg)
}

// SpinToUser updates the UserLog with a new message and starts the spinner.
func (ul *UserLog) SpinToUser(msg string, args ...interface{}) {
	ul.spin.Stop()
	formattedMsg := fmt.Sprintf(msg, args...)
	ul.spin.Suffix = formattedMsg
	ul.spin.FinalMSG = greenCheckmark + formattedMsg + "\n"
	ul.log.Info(formattedMsg)
	ul.spin.Start()
}

// SpinCombinedToUser updates the UserLog spin messages for a specific group.
func (ul *UserLog) SpinForParallelOperation(operationID string, hostID string) {
	if ul.spinID == operationID {
		// we are in the same operation, so we can just append
		ul.spinMutex.Lock()
		ul.spin.Suffix = fmt.Sprintf("%s [%s]", ul.spin.Suffix, hostID)
		ul.spin.FinalMSG = fmt.Sprintf("%s [%s]", ul.spin.FinalMSG, hostID)
		ul.spinMutex.Unlock()
		ul.log.Info(fmt.Sprintf("%s: [%s]", operationID, hostID))
	} else {
		if ul.spin.Active() {
			ul.spin.Stop()
			fmt.Fprintln(ul.writer, " ")
		}
		ul.spinMutex.Lock()
		ul.spinID = operationID
		ul.spin.Suffix = fmt.Sprintf("%s: [%s]", operationID, hostID)
		ul.spin.FinalMSG = greenCheckmark + fmt.Sprintf("%s [%s]", operationID, hostID)
		ul.spinMutex.Unlock()
		ul.log.Info(fmt.Sprintf("%s: [%s]", operationID, hostID))
		ul.spin.Start()
	}
}

func (ul *UserLog) StopSpinner() {
	ul.spin.Stop()
	fmt.Fprintln(ul.writer, "")
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

// PrintTableEndpoints prints the endpoints coming from the healthy call
func PrintTableEndpoints(clusterInfo *rpcpb.ClusterInfo) {
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
			table.Append([]string{nodeInfo.Name, chainInfo.ChainName, fmt.Sprintf("%s/ext/bc/%s/rpc", nodeInfo.GetUri(), blockchainID), fmt.Sprintf("%s/ext/bc/%s/rpc", nodeInfo.GetUri(), chainInfo.ChainName)})
		}
	}
	table.Render()
}

func ConvertToStringWithThousandSeparator(input uint64) string {
	p := message.NewPrinter(language.English)
	s := p.Sprintf("%d", input)
	return strings.ReplaceAll(s, ",", "_")
}
