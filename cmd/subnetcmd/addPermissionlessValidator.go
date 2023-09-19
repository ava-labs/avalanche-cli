// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

// avalanche subnet deploy
func newAddPermissionlessValidatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addPermissionlessValidator",
		Short: "Add a permissionless validator to Primary Network",
		Long: `The subnet addPermissionlessValidator adds a node as permissionless validator 
in the Primary Network`,
		SilenceUsage: true,
		RunE:         addPermissionlessValidator,
		Args:         cobra.ExactArgs(0),
	}
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji deploy only]")
	cmd.Flags().StringVar(&nodeIDStr, "nodeID", "", "set the NodeID of the validator to add")
	cmd.Flags().Uint64Var(&weight, "weight", 0, "set the staking weight of the validator to add")
	cmd.Flags().StringVar(&startTimeStr, "start-time", "", "UTC start time when this validator starts validating, in 'YYYY-MM-DD HH:MM:SS' format")
	cmd.Flags().DurationVar(&duration, "staking-period", 0, "how long this validator will be staking")
	cmd.Flags().BoolVar(&deployTestnet, "fuji", false, "join on `fuji` (alias for `testnet`)")
	cmd.Flags().BoolVar(&deployTestnet, "testnet", false, "join on `testnet` (alias for `fuji`)")
	cmd.Flags().BoolVar(&deployMainnet, "mainnet", false, "join on `mainnet`")
	cmd.Flags().StringSliceVar(&subnetAuthKeys, "subnet-auth-keys", nil, "control keys that will be used to authenticate add validator tx")
	cmd.Flags().StringVar(&outputTxPath, "output-tx-path", "", "file path of the add validator tx")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	return cmd
}

func addPermissionlessValidator(_ *cobra.Command, args []string) error {
	var (
		nodeID ids.NodeID
		start  time.Time
		err    error
	)

	var network models.Network
	switch {
	case deployTestnet:
		network = models.Fuji
	case deployMainnet:
		network = models.Mainnet
	}

	if network == models.Undefined {
		networkStr, err := app.Prompt.CaptureList(
			"Choose a network to add validator to.",
			[]string{models.Fuji.String(), models.Mainnet.String()},
		)
		if err != nil {
			return err
		}
		network = models.NetworkFromString(networkStr)
	}

	if outputTxPath != "" {
		if _, err := os.Stat(outputTxPath); err == nil {
			return fmt.Errorf("outputTxPath %q already exists", outputTxPath)
		}
	}

	if len(ledgerAddresses) > 0 {
		useLedger = true
	}

	if useLedger && keyName != "" {
		return ErrMutuallyExlusiveKeyLedger
	}

	switch network {
	case models.Fuji:
		if !useLedger && keyName == "" {
			useLedger, keyName, err = prompts.GetFujiKeyOrLedger(app.Prompt, "pay transaction fees", app.GetKeyDir())
			if err != nil {
				return err
			}
		}
	case models.Mainnet:
		useLedger = true
		if keyName != "" {
			return ErrStoredKeyOnMainnet
		}
	default:
		return errors.New("unsupported network")
	}

	// used in E2E to simulate public network execution paths on a local network
	if os.Getenv(constants.SimulatePublicNetwork) != "" {
		network = models.Local
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

	if weight == 0 {
		weight, err = promptWeight()
		if err != nil {
			return err
		}
	} else if weight < constants.MinStakeWeight {
		return fmt.Errorf("illegal stake amount, must be greater than or equal to %d: %d", constants.MinStakeWeight, weight)
	}

	start, duration, err = getTimeParameters(network, nodeID, true)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("NodeID: %s", nodeID.String())
	ux.Logger.PrintToUser("Network: %s", network.String())
	ux.Logger.PrintToUser("Start time: %s", start.Format(constants.TimeParseLayout))
	ux.Logger.PrintToUser("End time: %s", start.Add(duration).Format(constants.TimeParseLayout))
	ux.Logger.PrintToUser("Stake Amount: %d", weight)
	ux.Logger.PrintToUser("Inputs complete, issuing transaction to add the provided validator information...")

	// get keychain accessory
	kc, err := GetKeychain(useLedger, ledgerAddresses, keyName, network)
	if err != nil {
		return err
	}
	deployer := subnet.NewPublicDeployer(app, useLedger, kc, network)
	recipientAddr := kc.Addresses().List()[0]
	txID, err := deployer.AddPermissionlessValidator(ids.Empty, ids.Empty, nodeID, weight, uint64(start.Unix()), uint64(start.Add(duration).Unix()), recipientAddr)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("TXID: %s", txID.String())
	return err
}
