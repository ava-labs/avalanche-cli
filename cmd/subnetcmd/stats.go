// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/api"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// avalanche subnet stats
func newStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "stats [subnetName]",
		Short:        "Show validator statistics for the given subnet",
		Long:         ``,
		Args:         cobra.ExactArgs(1),
		RunE:         stats,
		SilenceUsage: true,
	}
	cmd.Flags().BoolVar(&deployTestnet, "fuji", false, "join on `fuji` (alias for `testnet`)")
	cmd.Flags().BoolVar(&deployTestnet, "testnet", false, "join on `testnet` (alias for `fuji`)")
	cmd.Flags().BoolVar(&deployMainnet, "mainnet", false, "join on `mainnet`")
	return cmd
}

func stats(cmd *cobra.Command, args []string) error {
	var network models.Network
	switch {
	case deployTestnet:
		network = models.Fuji
	case deployMainnet:
		network = models.Mainnet
	}

	if network == models.Undefined {
		networkStr, err := app.Prompt.CaptureList(
			"Choose a network from which you want to get the statistics (this command only supports public networks)",
			[]string{models.Fuji.String(), models.Mainnet.String()},
		)
		if err != nil {
			return err
		}
		// flag provided
		networkStr = strings.Title(networkStr)
		// as we are allowing a flag, we need to check if a supported network has been provided
		if !(networkStr == models.Fuji.String() || networkStr == models.Mainnet.String()) {
			return errors.New("unsupported network")
		}
		network = models.NetworkFromString(networkStr)
	}

	chains, err := validateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}
	subnetName := chains[0]

	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}

	subnetID := sc.Networks[network.String()].SubnetID
	if subnetID == ids.Empty {
		return errors.New("no subnetID found for the provided subnet name; has this subnet actually been deployed to this network?")
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

	table = tablewriter.NewWriter(os.Stdout)
	rows, err = buildPendingValidatorStats(pClient, infoClient, table, subnetID)
	if err != nil {
		return err
	}

	if len(rows) == 0 {
		return nil
	}
	for _, row := range rows {
		table.Append(row)
	}
	table.Render()
	return nil
}

func buildPendingValidatorStats(pClient platformvm.Client, infoClient info.Client, table *tablewriter.Table, subnetID ids.ID) ([][]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 2nd param are delegators, we don't need them here?
	pendingValidatorsIface, _, err := pClient.GetPendingValidators(ctx, subnetID, []ids.NodeID{})
	if err != nil {
		return nil, fmt.Errorf("failed to query the API endpoint for the pending validators: %w", err)
	}

	pendingValidators := make([]api.PermissionlessValidator, len(pendingValidatorsIface))
	var ok bool
	for i, v := range pendingValidatorsIface {
		pendingValidators[i], ok = v.(api.PermissionlessValidator)
		if !ok {
			return nil, fmt.Errorf("expected type api.PermissionlessValidator, but got %T", v)
		}
	}

	rows := [][]string{}

	if len(pendingValidators) == 0 {
		ux.Logger.PrintToUser("No pending validators found.")
		return rows, nil
	}

	ux.Logger.PrintToUser("Pending validators (not yet validating the subnet)")
	ux.Logger.PrintToUser("==================================================")

	header := []string{"nodeID", "stake-amount", "weight", "start-time", "end-time", "vmversion"}
	table.SetHeader(header)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	table.SetAutoMergeCells(true)
	table.SetRowLine(true)

	var (
		startTime, endTime          time.Time
		localNodeID                 ids.NodeID
		stakeAmount, weight         string
		localVersionStr, versionStr string
	)

	// try querying the local node for its node version
	reply, err := infoClient.GetNodeVersion(ctx)
	if err == nil {
		// we can ignore err here; if it worked, we have a non-zero node ID
		localNodeID, _ = infoClient.GetNodeID(ctx)
		for k, v := range reply.VMVersions {
			localVersionStr = fmt.Sprintf("%s: %s\n", k, v)
		}
	}

	for _, v := range pendingValidators {
		startTime = time.Unix(int64(v.StartTime), 0)
		endTime = time.Unix(int64(v.EndTime), 0)

		if v.StakeAmount != nil {
			stakeAmount = strconv.FormatUint(uint64(*v.StakeAmount), 10)
		} else {
			stakeAmount = constants.NotAvailableLabel
		}
		if v.Weight != nil {
			weight = strconv.FormatUint(uint64(*v.Weight), 10)
		} else {
			weight = constants.NotAvailableLabel
		}
		// if retrieval of localNodeID failed, it will be empty,
		// and this comparison fails
		if v.NodeID == localNodeID {
			versionStr = localVersionStr
		}
		// query peers for IP address of this NodeID...
		rows = append(rows, []string{
			v.NodeID.String(),
			stakeAmount,
			weight,
			// TODO: Print in local time zone vs UTC?
			startTime.UTC().String(),
			endTime.UTC().String(),
			versionStr,
		})
	}

	return rows, nil
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

	header := []string{"nodeID", "connected", "stake-amount", "weight", "remaining", "vmversion"}
	table.SetHeader(header)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	table.SetAutoMergeCells(true)
	table.SetRowLine(true)
	rows := [][]string{}

	var (
		startTime, endTime                        time.Time
		localNodeID                               ids.NodeID
		remaining, connected, stakeAmount, weight string
		localVersionStr, versionStr               string
	)

	// try querying the local node for its node version
	reply, err := infoClient.GetNodeVersion(ctx)
	if err == nil {
		// we can ignore err here; if it worked, we have a non-zero node ID
		localNodeID, _ = infoClient.GetNodeID(ctx)
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
		if v.StakeAmount != nil {
			stakeAmount = strconv.FormatUint(*v.StakeAmount, 10)
		} else {
			stakeAmount = constants.NotAvailableLabel
		}
		if v.Weight != nil {
			weight = strconv.FormatUint(*v.Weight, 10)
		} else {
			weight = constants.NotAvailableLabel
		}
		// if retrieval of localNodeID failed, it will be empty,
		// and this comparison fails
		if v.NodeID == localNodeID {
			versionStr = localVersionStr
		}
		// query peers for IP address of this NodeID...
		rows = append(rows, []string{
			v.NodeID.String(),
			connected,
			stakeAmount,
			weight,
			// TODO: Print in local time zone vs UTC?
			// startTime.UTC().String(),
			// endTime.UTC().String(),
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
		_, err := i.GetNodeID(ctx)
		if err == nil {
			return c, i
		}
	}

	var url string
	// try public APIs
	switch network {
	case models.Fuji:
		url = constants.FujiAPIEndpoint
	case models.Mainnet:
		url = constants.MainnetAPIEndpoint
	}
	// unsupported network
	if url == "" {
		return nil, nil
	}

	// create client to public API
	c = platformvm.NewClient(url)
	// try calling it to make sure it actually worked
	_, err = c.GetHeight(ctx)
	if err == nil {
		// also try to get a local client
		i = info.NewClient(constants.LocalAPIEndpoint)
	}
	return c, i
}
