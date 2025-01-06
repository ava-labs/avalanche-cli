// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var statsSupportedNetworkOptions = []networkoptions.NetworkOption{
	networkoptions.Local,
	networkoptions.Devnet,
	networkoptions.Fuji,
	networkoptions.Mainnet,
}

// avalanche blockchain stats
func newStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats [blockchainName]",
		Short: "Show validator statistics for the given blockchain",
		Long:  `The blockchain stats command prints validator statistics for the given Blockchain.`,
		Args:  cobrautils.ExactArgs(1),
		RunE:  stats,
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, statsSupportedNetworkOptions)
	return cmd
}

func stats(_ *cobra.Command, args []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		true,
		false,
		statsSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	chains, err := ValidateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}
	blockchainName := chains[0]

	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}

	subnetID := sc.Networks[network.Name()].SubnetID
	if subnetID == ids.Empty {
		return errors.New("no subnetID found for the provided blockchain name; has this blockchain actually been deployed to this network?")
	}

	pClient, infoClient := findAPIEndpoint(network)
	if pClient == nil {
		return errors.New("failed to create a client to an API endpoint")
	}

	table := tablewriter.NewWriter(os.Stdout)
	rows, err := buildCurrentValidatorStats(pClient, infoClient, table, subnetID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		table.Append(row)
	}
	table.Render()

	return nil
}

func buildCurrentValidatorStats(pClient platformvm.Client, infoClient info.Client, table *tablewriter.Table, subnetID ids.ID) ([][]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	currValidators, err := pClient.GetCurrentValidators(ctx, subnetID, []ids.NodeID{})
	if err != nil {
		return nil, fmt.Errorf("failed to query the API endpoint for the current validators: %w", err)
	}

	ux.Logger.PrintToUser("Current validators (already validating the subnet)")
	ux.Logger.PrintToUser("==================================================")

	header := []string{"nodeID", "connected", "weight", "remaining", "vmversion"}
	table.SetHeader(header)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	table.SetAutoMergeCells(true)
	table.SetRowLine(true)
	rows := [][]string{}

	var (
		startTime, endTime           time.Time
		localNodeID                  ids.NodeID
		remaining, connected, weight string
		localVersionStr, versionStr  string
	)

	// try querying the local node for its node version
	reply, err := infoClient.GetNodeVersion(ctx)
	if err == nil {
		// we can ignore err here; if it worked, we have a non-zero node ID
		localNodeID, _, _ = infoClient.GetNodeID(ctx)
		for k, v := range reply.VMVersions {
			localVersionStr = fmt.Sprintf("%s: %s\n", k, v)
		}
	}

	for _, v := range currValidators {
		startTime = time.Unix(int64(v.StartTime), 0)
		endTime = time.Unix(int64(v.EndTime), 0)
		remaining = ux.FormatDuration(endTime.Sub(startTime))

		// some members of the returned object are pointers
		// so we need to check the pointer is actually valid
		if v.Connected != nil {
			connected = strconv.FormatBool(*v.Connected)
		} else {
			connected = constants.NotAvailableLabel
		}

		uint64Weight := v.Weight
		delegators := v.Delegators
		for _, d := range delegators {
			uint64Weight += d.Weight
		}
		weight = strconv.FormatUint(uint64Weight, 10)

		// if retrieval of localNodeID failed, it will be empty,
		// and this comparison fails
		if v.NodeID == localNodeID {
			versionStr = localVersionStr
		}
		// query peers for IP address of this NodeID...
		rows = append(rows, []string{
			v.NodeID.String(),
			connected,
			weight,
			remaining,
			versionStr,
		})
	}

	return rows, nil
}

// findAPIEndpoint tries first to create a client to a local node
// if it doesn't find one, it tries public APIs
func findAPIEndpoint(network models.Network) (platformvm.Client, info.Client) {
	var i info.Client

	// first try local node
	ctx := context.Background()
	c := platformvm.NewClient(constants.LocalAPIEndpoint)
	_, err := c.GetHeight(ctx)
	if err == nil {
		i = info.NewClient(constants.LocalAPIEndpoint)
		// try calling it to make sure it actually worked
		_, _, err := i.GetNodeID(ctx)
		if err == nil {
			return c, i
		}
	}

	// create client to public API
	c = platformvm.NewClient(network.Endpoint)
	// try calling it to make sure it actually worked
	_, err = c.GetHeight(ctx)
	if err == nil {
		// also try to get a local client
		i = info.NewClient(constants.LocalAPIEndpoint)
	}
	return c, i
}
