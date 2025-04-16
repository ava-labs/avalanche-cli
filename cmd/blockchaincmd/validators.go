// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blockchaincmd

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	validatorsdk "github.com/ava-labs/avalanche-cli/sdk/validator"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// avalanche blockchain validators
func newValidatorsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validators [blockchainName]",
		Short: "List subnets validators of a blockchain",
		Long: `The blockchain validators command lists the validators of a blockchain and provides
several statistics about them.`,
		RunE: printValidators,
		Args: cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, networkoptions.DefaultSupportedNetworkOptions)
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
		networkoptions.DefaultSupportedNetworkOptions,
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
		return printLocalValidators(network, subnetID)
	} else {
		return printPublicValidators(network, subnetID)
	}
}

func printLocalValidators(network models.Network, subnetID ids.ID) error {
	validators, err := subnet.GetSubnetValidators(subnetID)
	if err != nil {
		return err
	}

	return printValidatorsFromList(network, subnetID, validators)
}

func printPublicValidators(network models.Network, subnetID ids.ID) error {
	validators, err := subnet.GetPublicSubnetValidators(subnetID, network)
	if err != nil {
		return err
	}

	return printValidatorsFromList(network, subnetID, validators)
}

func printValidatorsFromList(network models.Network, subnetID ids.ID, validators []platformvm.ClientPermissionlessValidator) error {
	header := []string{"NodeID", "Weight", "Delegator Weight", "Start Time", "End Time", "Type"}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetRowLine(true)

	for _, validator := range validators {
		var delegatorWeight uint64
		if validator.DelegatorWeight != nil {
			delegatorWeight = *validator.DelegatorWeight
		}

		validatorKind, err := validatorsdk.GetValidatorKind(network.SDKNetwork(), subnetID, validator.NodeID)
		if err != nil {
			return err
		}
		validatorType := "permissioned"
		if validatorKind == validatorsdk.SovereignValidator {
			validatorType = "sovereign"
		}

		table.Append([]string{
			validator.NodeID.String(),
			strconv.FormatUint(validator.Weight, 10),
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
