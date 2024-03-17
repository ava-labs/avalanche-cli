// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package primarycmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/spf13/cobra"
)

const art = `
  _____      _                              _   _      _                      _      _____
 |  __ \    (_)                            | \ | |    | |                    | |    |  __ \
 | |__) | __ _ _ __ ___   __ _ _ __ _   _  |  \| | ___| |___      _____  _ __| | __ | |__) |_ _ _ __ __ _ _ __ ___  ___
 |  ___/ '__| | '_   _ \ / _  | '__| | | | | .   |/ _ \ __\ \ /\ / / _ \| '__| |/ / |  ___/ _  | '__/ _  | '_   _ \/ __|
 | |   | |  | | | | | | | (_| | |  | |_| | | |\  |  __/ |_ \ V  V / (_) | |  |   <  | |  | (_| | | | (_| | | | | | \__ \
 |_|   |_|  |_|_| |_| |_|\__,_|_|   \__, | |_| \_|\___|\__| \_/\_/ \___/|_|  |_|\_\ |_|   \__,_|_|  \__,_|_| |_| |_|___/
                                     __/ |
                                    |___/
`

var describeSupportedNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local, networkoptions.Cluster}

// avalanche primary describe
func newDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "describe",
		Short:        "Print details of the primary network configuration",
		Long:         `The subnet describe command prints details of the primary network configuration to the console.`,
		SilenceUsage: true,
		RunE:         describe,
		Args:         cobra.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, describeSupportedNetworkOptions)
	return cmd
}

func describe(_ *cobra.Command, _ []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		globalNetworkFlags,
		false,
		describeSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	_ = network
	fmt.Println(logging.LightBlue.Wrap(art))
	return nil
}
