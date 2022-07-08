// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

var (
	nodeIDStr    string
	weightStr    string
	startTimeStr string
	duration     time.Duration

	errNoSubnetID = errors.New("failed to find the subnet ID for this subnet, has it been deployed/created?")
)

// avalanche subnet deploy
func newAddValidatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addValidator [subnetName]",
		Short: "Allow a validator to stake on the given subnet",
		Long: `The addValidator command prompts for start time, duration and weight for validating this subnet.
It also prompts for the NodeID of the node which will be validating this subnet.`,
		SilenceUsage: true,
		RunE:         addValidatorCmd,
		Args:         cobra.ExactArgs(1),
	}
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use")
	cmd.Flags().StringVar(&nodeIDStr, "nodeID", "", "set the NodeID of the validator to add")
	cmd.Flags().StringVar(&weightStr, "weight", "", "set the staking weight of the validator to add")
	cmd.Flags().StringVar(&startTimeStr, "start-time", "", "start time when this validator starts validating, in 'YYYY-MM-DD HH:MM:SS' format")
	cmd.Flags().DurationVar(&duration, "staking-period", 0, "how long this validator will be staking")
	return cmd
}

func addValidatorCmd(cmd *cobra.Command, args []string) error {
	return addValidator(args, prompts.NewPrompter, prompts.NewSelector, subnet.NewPublicSubnetDeployer)
}

func addValidator(args []string,
	prompter prompts.PromptCreateFunc,
	selector prompts.SelectCreateFunc,
	newDeployer subnet.NewDeployerFunc,
) error {
	var (
		nodeID ids.NodeID
		weight uint64
		start  time.Time
		err    error
	)

	chains, err := validateSubnetName(args)
	if err != nil {
		return err
	}
	subnetName := chains[0]
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}

	subnetID := sc.SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	if nodeIDStr == "" {
		nodeID, err = promptNodeID(prompter)
		if err != nil {
			return err
		}
	} else {
		nodeID, err = ids.NodeIDFromString(nodeIDStr)
		if err != nil {
			return err
		}
	}

	if weightStr == "" {
		weight, err = promptWeight(prompter)
		if err != nil {
			return err
		}
	} else {
		weight, err = strconv.ParseUint(weightStr, 10, 64)
		if err != nil {
			return err
		}
	}

	if startTimeStr == "" {
		start, err = promptStart(prompter)
		if err != nil {
			return err
		}
	} else {
		start, err = time.Parse(constants.TimeParseLayout, startTimeStr)
		if err != nil {
			return err
		}
		// TODO validate this start time
	}

	if duration == 0 {
		duration, err = promptDuration(prompter, selector, start)
		if err != nil {
			return err
		}
	}
	// TODO validate this duration

	var network models.Network
	networkStr, err := prompts.CaptureList(
		selector(
			"Choose a network to deploy on",
			[]string{models.Fuji.String(), models.Mainnet.String()},
		),
	)
	if err != nil {
		return err
	}
	network = models.NetworkFromString(networkStr)

	deployer := newDeployer(app, app.GetKeyPath(keyName), network)
	return deployer.AddValidator(subnetID, nodeID, weight, start, duration)
}

func promptDuration(p prompts.PromptCreateFunc, s prompts.SelectCreateFunc, start time.Time) (time.Duration, error) {
	for {
		txt := "How long should this validator be validating? Enter a duration, e.g. 8760h"
		prompt := p(txt)
		d, err := prompts.CaptureDuration(prompt)
		if err != nil {
			return 0, err
		}
		end := start.Add(d)
		confirm := fmt.Sprintf("Your validator will complete staking by %s", end.Format(constants.TimeParseLayout))
		yes, err := prompts.CaptureYesNo(s, confirm)
		if err != nil {
			return 0, err
		}
		if yes {
			return d, nil
		}
	}
}

func promptStart(f prompts.PromptCreateFunc) (time.Time, error) {
	txt := "When will the validator start validating? Enter a date in 'YYYY-MM-DD HH:MM:SS' format"
	prompt := f(txt)
	return prompts.CaptureDate(prompt)
}

func promptNodeID(f prompts.PromptCreateFunc) (ids.NodeID, error) {
	txt := "What is the NodeID of the validator?"
	prompt := f(txt)
	return prompts.CaptureNodeID(prompt)
}

func promptWeight(f prompts.PromptCreateFunc) (uint64, error) {
	txt := "What is the staking weight of the validator?"
	prompt := f(txt)
	return prompts.CaptureWeight(prompt)
}
