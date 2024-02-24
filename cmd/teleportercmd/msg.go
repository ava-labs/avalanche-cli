// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"fmt"
	"strings"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/spf13/cobra"
)

var (
	useLocal   bool
	useDevnet  bool
	useFuji    bool
	useMainnet bool
	endpoint   string
)

// avalanche teleporter msg
func newMsgCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "msg [subnet1Name] [subnet2Name]",
		Short:        "Sends and wait reception for a teleporter msg between two subnets",
		Long:         `Sends and wait reception for a teleporter msg between two subnets.`,
		SilenceUsage: true,
		RunE:         msg,
		Args:         cobra.ExactArgs(2),
	}
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "use the given endpoint for network operations")
	cmd.Flags().BoolVarP(&useLocal, "local", "l", false, "operate on a local network")
	cmd.Flags().BoolVar(&useDevnet, "devnet", false, "operate on a devnet network")
	cmd.Flags().BoolVarP(&useFuji, "testnet", "t", false, "operate on testnet (alias to `fuji`)")
	cmd.Flags().BoolVarP(&useFuji, "fuji", "f", false, "operate on fuji (alias to `testnet`")
	cmd.Flags().BoolVarP(&useMainnet, "mainnet", "m", false, "operate on mainnet")
	return cmd
}

func msg(cmd *cobra.Command, args []string) error {
	network, err := subnetcmd.GetNetworkFromCmdLineFlags(
		useLocal,
		useDevnet,
		useFuji,
		useMainnet,
		"",
		false,
		[]models.NetworkKind{models.Local},
	)
	if err != nil {
		return err
	}

	subnet1 := strings.ToLower(args[0])
	subnet2 := strings.ToLower(args[1])

	chainID1, err := getSubnetParams(network, subnet1)
	if err != nil {
		return err
	}
	chainID2, err := getSubnetParams(network, subnet2)
	if err != nil {
		return err
	}

	fmt.Println(chainID1)
	fmt.Println(chainID2)

	return nil
}

func getSubnetParams(network models.Network, subnet string) (ids.ID, error) {
	if subnet == "c-chain" || subnet == "cchain" {
		subnet = "C"
	}

	chainID, err := getChainID(network.Endpoint, subnet)
	if err == nil {
		return chainID, err
	}
	sc, err := app.LoadSidecar(subnet)
	if err != nil {
		return ids.Empty, err
	}
	chainID = sc.Networks[network.Name()].BlockchainID
	if chainID == ids.Empty {
		return ids.Empty, fmt.Errorf("chainID for subnet %s not found on network %s", subnet, network.Name())
	}
	return chainID, err
}

func getChainID(endpoint string, chainName string) (ids.ID, error) {
	client := info.NewClient(endpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	return client.GetBlockchainID(ctx, chainName)
}
