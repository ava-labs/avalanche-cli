// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

var (
	nodeIDStr    string
	weightStr    string
	startTimeStr string
	duration     time.Duration

	errNoSubnetID    = errors.New("failed to find the subnet ID for this subnet, has it been deployed/created on this network?")
	startTimeDefault = time.Now().Add(constants.StakingStartLeadTime)
)

// avalanche subnet deploy
func newAddValidatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addValidator [subnetName]",
		Short: "Allow a validator to validate your subnet",
		Long: `The subnet addValidator command whitelists a primary network validator to
validate the provided deployed subnet.

To add the validator to the subnet's allow list, you first need to provide
the subnetName and the validator's unique NodeID. The command then prompts
for the validation start time, duration and stake weight. These values can
all be collected with flags instead of prompts.

This command currently only works on subnets deployed to the Fuji testnet.`,
		SilenceUsage: true,
		RunE:         addValidator,
		Args:         cobra.ExactArgs(1),
	}
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use")
	cmd.Flags().StringVar(&nodeIDStr, "nodeID", "", "set the NodeID of the validator to add")
	cmd.Flags().StringVar(&weightStr, "weight", "", "set the staking weight of the validator to add")
	cmd.Flags().StringVar(&startTimeStr, "start-time", startTimeDefault.Format(constants.TimeParseLayout), "start time when this validator starts validating, in 'YYYY-MM-DD HH:MM:SS' format")
	cmd.Flags().DurationVar(&duration, "staking-period", constants.MaxStakeDuration, "how long this validator will be staking")
	return cmd
}

func addValidator(cmd *cobra.Command, args []string) error {
	var (
		nodeID ids.NodeID
		weight uint64
		start  time.Time
		err    error
	)

	if keyName == "" {
		keyName, err = captureKeyName()
		if err != nil {
			return err
		}
	}

	var network models.Network
	networkStr, err := app.Prompt.CaptureList(
		"Choose a network to deploy on. This command only supports Fuji currently.",
		[]string{models.Fuji.String(), models.Mainnet.String() + " (coming soon)"},
	)
	if err != nil {
		return err
	}
	network = models.NetworkFromString(networkStr)

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
		return errNoSubnetID
	}

	if nodeIDStr == "" {
		nodeID, err = promptNodeID()
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
		weight, err = promptWeight()
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
		start, err = promptStart()
		if err != nil {
			return err
		}
	} else {
		start, err = time.Parse(constants.TimeParseLayout, startTimeStr)
		if err != nil {
			return err
		}
		if start.Before(time.Now().Add(constants.StakingStartLeadTime)) {
			return fmt.Errorf("time should be at least %s in the future ", constants.StakingStartLeadTime)
		}
	}

	if duration == 0 {
		duration, err = promptDuration(start)
		if err != nil {
			return err
		}
	}
	// TODO validate this duration?

	ux.Logger.PrintToUser("Inputs complete, issuing transaction to add the provided validator information...")
	deployer := subnet.NewPublicDeployer(app, app.GetKeyPath(keyName), network)
	return deployer.AddValidator(subnetID, nodeID, weight, start, duration)
}

func promptDuration(start time.Time) (time.Duration, error) {
	for {
		txt := "How long should this validator be validating? Enter a duration, e.g. 8760h"
		d, err := app.Prompt.CaptureDuration(txt)
		if err != nil {
			return 0, err
		}
		end := start.Add(d)
		confirm := fmt.Sprintf("Your validator will finish staking by %s", end.Format(constants.TimeParseLayout))
		yes, err := app.Prompt.CaptureYesNo(confirm)
		if err != nil {
			return 0, err
		}
		if yes {
			return d, nil
		}
	}
}

func promptStart() (time.Time, error) {
	txt := "When should the validator start validating? Enter a date in 'YYYY-MM-DD HH:MM:SS' format"
	return app.Prompt.CaptureDate(txt)
}

func promptNodeID() (ids.NodeID, error) {
	txt := "What is the NodeID of the validator you'd like to whitelist?"
	return app.Prompt.CaptureNodeID(txt)
}

func promptWeight() (uint64, error) {
	txt := "What stake weight would you like to assign to the validator?"
	return app.Prompt.CaptureWeight(txt)
}

func captureKeyName() (string, error) {
	files, err := os.ReadDir(app.GetKeyDir())
	if err != nil {
		return "", err
	}

	keys := make([]string, len(files))

	for i, f := range files {
		if strings.HasSuffix(f.Name(), constants.KeySuffix) {
			keys[i] = strings.TrimSuffix(f.Name(), constants.KeySuffix)
		}
	}

	keyName, err = app.Prompt.CaptureList("Which private key should be used to issue the transaction?", keys)
	if err != nil {
		return "", err
	}

	return keyName, nil
}
