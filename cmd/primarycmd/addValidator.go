// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package primarycmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ava-labs/avalanche-cli/cmd/nodecmd"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/genesis"
	"os"
	"time"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

var (
	deployTestnet                bool
	deployMainnet                bool
	keyName                      string
	subnetAuthKeys               []string
	outputTxPath                 string
	useLedger                    bool
	ledgerAddresses              []string
	nodeIDStr                    string
	weight                       uint64
	startTimeStr                 string
	duration                     time.Duration
	ErrMutuallyExlusiveKeyLedger = errors.New("--key and --ledger,--ledger-addrs are mutually exclusive")
	ErrStoredKeyOnMainnet        = errors.New("--key is not available for mainnet operations")
)

type jsonProofOfPossession struct {
	PublicKey         string `json:"publicKey"`
	ProofOfPossession string `json:"proofOfPossession"`
}

// avalanche subnet deploy
func newAddValidatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addValidator",
		Short: "Add a validator to Primary Network",
		Long: `The primary addValidator command adds a node as a validator 
in the Primary Network`,
		SilenceUsage: true,
		RunE:         addValidator,
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

func promptProofOfPossession() (jsonProofOfPossession, error) {
	ux.Logger.PrintToUser("Next, we need the public key and proof of possession of the node's BLS")
	ux.Logger.PrintToUser("SSH into the node and call info.getNodeID API to get the node's BLS info")
	ux.Logger.PrintToUser("Check https://docs.avax.network/apis/avalanchego/apis/info#infogetnodeid for instructions on calling info.getNodeID API")
	txt := "What is the public key of the node's BLS?"
	publicKey, err := app.Prompt.CaptureString(txt)
	if err != nil {
		return jsonProofOfPossession{}, err
	}
	txt = "What is the proof of possession of the node's BLS?"
	pop, err := app.Prompt.CaptureString(txt)
	if err != nil {
		return jsonProofOfPossession{}, err
	}
	return jsonProofOfPossession{PublicKey: publicKey, ProofOfPossession: pop}, nil
}

func addValidator(_ *cobra.Command, _ []string) error {
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
		nodeID, err = subnetcmd.PromptNodeID()
		if err != nil {
			return err
		}
	} else {
		nodeID, err = ids.NodeIDFromString(nodeIDStr)
		if err != nil {
			return err
		}
	}

	minValStake, err := nodecmd.GetMinStakingAmount(network)
	if err != nil {
		return err
	}
	if weight == 0 {
		weight, err = nodecmd.PromptWeightPrimaryNetwork(network)
		if err != nil {
			return err
		}
	}
	if weight < minValStake {
		return fmt.Errorf("illegal weight, must be greater than or equal to %d: %d", minValStake, weight)
	}

	kc, err := subnetcmd.GetKeychain(useLedger, ledgerAddresses, keyName, network)
	if err != nil {
		return err
	}
	jsonPop, err := promptProofOfPossession()
	if err != nil {
		return err
	}
	popBytes, err := json.Marshal(jsonPop)
	if err != nil {
		return err
	}
	start, duration, err = nodecmd.GetTimeParametersPrimaryNetwork(network, 0)
	if err != nil {
		return err
	}
	deployer := subnet.NewPublicDeployer(app, useLedger, kc, network)
	nodecmd.PrintNodeJoinPrimaryNetworkOutput(nodeID, weight, network, start)
	recipientAddr := kc.Addresses().List()[0]
	delegationFee := genesis.FujiParams.MinDelegationFee
	if network == models.Mainnet {
		delegationFee = genesis.MainnetParams.MinDelegationFee
	}
	_, err = deployer.AddPermissionlessValidator(ids.Empty, ids.Empty, nodeID, weight, uint64(start.Unix()), uint64(start.Add(duration).Unix()), recipientAddr, delegationFee, popBytes)
	return err
}
