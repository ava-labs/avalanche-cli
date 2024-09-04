// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package relayercmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/mitchellh/go-wordwrap"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
)

var (
	logsNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local, networkoptions.Fuji}
	raw                bool
	last               uint
	first              uint
)

// avalanche teleporter relayer logs
func newLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "shows pretty formatted AWM relayer logs",
		Long:  "Shows pretty formatted AWM relayer logs",
		RunE:  logs,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, logsNetworkOptions)
	cmd.Flags().BoolVar(&raw, "raw", false, "raw logs output")
	cmd.Flags().UintVar(&last, "last", 0, "output last N log lines")
	cmd.Flags().UintVar(&first, "first", 0, "output first N log lines")
	return cmd
}

func logs(_ *cobra.Command, _ []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		false,
		false,
		logsNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	var logLines []string
	switch {
	case network.Kind == models.Local || network.Kind == models.Fuji:
		logsPath := app.GetLocalRelayerLogPath(network.Kind)
		bs, err := os.ReadFile(logsPath)
		if err != nil {
			return err
		}
		logs := string(bs)
		logLines = strings.Split(logs, "\n")
	default:
		return fmt.Errorf("unsupported network")
	}
	if first != 0 {
		if len(logLines) > int(first) {
			logLines = logLines[:first]
		}
	}
	if last != 0 {
		if len(logLines) > int(last) {
			logLines = logLines[len(logLines)-1-int(last):]
		}
	}
	if raw {
		for _, logLine := range logLines {
			logLine = strings.TrimSpace(logLine)
			if len(logLine) != 0 {
				fmt.Println(logLine)
			}
		}
		return nil
	}
	blockchainIDToBlockchainName, err := getBlockchainIDToBlockchainNameMap(network)
	if err != nil {
		return err
	}
	t := table.NewWriter()
	t.AppendHeader(table.Row{"", "Time", "Chain", "Log"})
	for _, logLine := range logLines {
		logLine = strings.TrimSpace(logLine)
		if len(logLine) != 0 {
			logMap := map[string]interface{}{}
			err := json.Unmarshal([]byte(logLine), &logMap)
			if err != nil {
				return err
			}
			levelEmoji := ""
			levelStr, b := logMap["level"].(string)
			if b {
				levelEmoji, err = logLevelToEmoji(levelStr)
				if err != nil {
					return err
				}
			}
			timeStampStr, b := logMap["timestamp"].(string)
			timeStr := ""
			if b {
				t, err := time.Parse("2006-01-02T15:04:05.000Z0700", timeStampStr)
				if err != nil {
					return err
				}
				timeStr = t.Format("15:04:05")
			}
			msg, b := logMap["msg"].(string)
			if !b {
				continue
			}
			logMsg := wordwrap.WrapString(msg, 80)
			logMsgLines := strings.Split(logMsg, "\n")
			logMsgLines = utils.Map(logMsgLines, func(s string) string { return logging.Green.Wrap(s) })
			logMsg = strings.Join(logMsgLines, "\n")
			keys := maps.Keys(logMap)
			sort.Strings(keys)
			for _, k := range keys {
				if !utils.Belongs([]string{"logger", "caller", "level", "timestamp", "msg"}, k) {
					logMsg = addAditionalInfo(
						logMsg,
						logMap,
						k,
						k,
						blockchainIDToBlockchainName,
					)
				}
			}
			subnet := getLogSubnet(logMap, blockchainIDToBlockchainName)
			t.AppendRow(table.Row{levelEmoji, timeStr, subnet, logMsg})
		}
	}
	fmt.Println(t.Render())

	return nil
}

func addAditionalInfo(
	logMsg string,
	logMap map[string]interface{},
	key string,
	outputName string,
	blockchainIDToBlockchainName map[string]string,
) string {
	value, b := logMap[key].(string)
	if b {
		blockchainName := blockchainIDToBlockchainName[value]
		if blockchainName != "" {
			value = blockchainName
		}
		logMsg = fmt.Sprintf("%s\n  %s=%s", logMsg, outputName, value)
	}
	return logMsg
}

func getLogSubnet(
	logMap map[string]interface{},
	blockchainIDToBlockchainName map[string]string,
) string {
	for _, key := range []string{
		"blockchainID",
		"originBlockchainID",
		"sourceBlockchainID",
		"destinationBlockchainID",
	} {
		value, b := logMap[key].(string)
		if b {
			blockchainName := blockchainIDToBlockchainName[value]
			if blockchainName != "" {
				return blockchainName
			}
		}
	}
	return ""
}

func getBlockchainIDToBlockchainNameMap(network models.Network) (map[string]string, error) {
	blockchainNames, err := app.GetBlockchainNamesOnNetwork(network)
	if err != nil {
		return nil, err
	}
	blockchainIDToBlockchainName := map[string]string{}
	for _, blockchainName := range blockchainNames {
		blockchainID, err := contract.GetBlockchainID(app, network, contract.ChainSpec{BlockchainName: blockchainName})
		if err != nil {
			return nil, err
		}
		blockchainIDToBlockchainName[blockchainID.String()] = blockchainName
	}
	blockchainID, err := contract.GetBlockchainID(app, network, contract.ChainSpec{CChain: true})
	if err != nil {
		return nil, err
	}
	blockchainIDToBlockchainName[blockchainID.String()] = "c-chain"
	return blockchainIDToBlockchainName, nil
}

func logLevelToEmoji(logLevel string) (string, error) {
	levelEmoji := ""
	level, err := logging.ToLevel(logLevel)
	if err != nil {
		return "", err
	}
	switch level {
	case logging.Info:
		levelEmoji = "ℹ️"
	case logging.Debug:
		levelEmoji = "🪲"
	case logging.Warn:
		levelEmoji = "⚠️"
	case logging.Error:
		levelEmoji = "⛔"
	case logging.Fatal:
		levelEmoji = "💀"
	}
	return levelEmoji, nil
}
