// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package l1cmd

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var validatorsSupportedNetworkOptions = []networkoptions.NetworkOption{
	networkoptions.Local,
	networkoptions.Devnet,
	networkoptions.Fuji,
	networkoptions.Mainnet,
}

// avalanche blockchain validators
func newValidatorsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validators [blockchainName]",
		Short: "List subnets validators of a blockchain",
		Long: `The blockchain validators command lists the validators of a blockchain's subnet and provides
several statistics about them.`,
		RunE: printValidators,
		Args: cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, validatorsSupportedNetworkOptions)
	return cmd
}

func printValidators(_ *cobra.Command, args []string) error {
	blockchainName := args[0]

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		true,
		false,
		validatorsSupportedNetworkOptions,
		blockchainName,
	)
	if err != nil {
		return err
	}

	// get the subnetID
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}

	deployInfo, ok := sc.Networks[network.Name()]
	if !ok {
		return errors.New("no deployment found for subnet")
	}

	subnetID := deployInfo.SubnetID

	if network.Kind == models.Local {
		return printLocalValidators(subnetID)
	} else {
		return printPublicValidators(subnetID, network)
	}
}

func printLocalValidators(subnetID ids.ID) error {
	validators, err := subnet.GetSubnetValidators(subnetID)
	if err != nil {
		return err
	}

	return printValidatorsFromList(validators)
}

func printPublicValidators(subnetID ids.ID, network models.Network) error {
	validators, err := subnet.GetPublicSubnetValidators(subnetID, network)
	if err != nil {
		return err
	}

	return printValidatorsFromList(validators)
}

func printValidatorsFromList(validators []platformvm.ClientPermissionlessValidator) error {
	header := []string{"NodeID", "Stake Amount", "Delegator Weight", "Start Time", "End Time", "Type"}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetRowLine(true)

	for _, validator := range validators {
		var delegatorWeight uint64
		if validator.DelegatorWeight != nil {
			delegatorWeight = *validator.DelegatorWeight
		}

		validatorType := "permissioned"
		if validator.PotentialReward != nil && *validator.PotentialReward > 0 {
			validatorType = "elastic"
		}

		table.Append([]string{
			validator.NodeID.String(),
			strconv.FormatUint(*validator.StakeAmount, 10),
			strconv.FormatUint(delegatorWeight, 10),
			formatUnixTime(validator.StartTime),
			formatUnixTime(validator.EndTime),
			validatorType,
		})
	}

	table.Render()

	return nil
}

func formatUnixTime(unixTime uint64) string {
	return time.Unix(int64(unixTime), 0).Format(time.RFC3339)
}
