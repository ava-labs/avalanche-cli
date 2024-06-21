// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package relayercmd

import (
	"bufio"
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"

	"github.com/spf13/cobra"
)

var (
	logsNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local, networkoptions.Cluster}
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
	switch {
	case network.Kind == models.Local:
		logsPath := app.GetAWMRelayerLogPath()
		file, err := os.Open(logsPath)
		if err != nil {
			return err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return err
		}
	case network.ClusterName != "":
	}
	return nil
}
