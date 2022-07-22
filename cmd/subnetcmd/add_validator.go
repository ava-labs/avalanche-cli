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
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
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
		Short: "Allow a validator to stake on the given subnet",
		Long: `The addValidator command prompts for start time, duration and weight for validating this subnet.
It also prompts for the NodeID of the node which will be validating this subnet.`,
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
	networkStr, err := prompts.CaptureList(
		"Choose a network to deploy on (this command only supports public networks)",
		[]string{models.Fuji.String(), models.Mainnet.String()},
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
			return fmt.Errorf("time should be at least start from now + %s", constants.StakingStartLeadTime)
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
		d, err := prompts.CaptureDuration(txt)
		if err != nil {
			return 0, err
		}
		end := start.Add(d)
		confirm := fmt.Sprintf("Your validator will complete staking by %s", end.Format(constants.TimeParseLayout))
		yes, err := prompts.CaptureYesNo(confirm)
		if err != nil {
			return 0, err
		}
		if yes {
			return d, nil
		}
	}
}

func promptStart() (time.Time, error) {
	txt := "When will the validator start validating? Enter a date in 'YYYY-MM-DD HH:MM:SS' format"
	return prompts.CaptureDate(txt)
}

func promptNodeID() (ids.NodeID, error) {
	txt := "What is the NodeID of the validator?"
	return prompts.CaptureNodeID(txt)
}

func promptWeight() (uint64, error) {
	txt := "What is the staking weight of the validator?"
	return prompts.CaptureWeight(txt)
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

	keyName, err = prompts.CaptureList("Which private key should be used to issue the transaction?", keys)
	if err != nil {
		return "", err
	}

	return keyName, nil
}
