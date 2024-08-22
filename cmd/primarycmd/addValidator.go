// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package primarycmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/ava-labs/avalanche-cli/cmd/blockchaincmd"
	"github.com/ava-labs/avalanche-cli/cmd/nodecmd"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

var (
	globalNetworkFlags                  networkoptions.NetworkFlags
	addValidatorSupportedNetworkOptions = []networkoptions.NetworkOption{networkoptions.Fuji, networkoptions.Mainnet}
	keyName                             string
	useLedger                           bool
	ledgerAddresses                     []string
	nodeIDStr                           string
	weight                              uint64
	delegationFee                       uint32
	startTimeStr                        string
	duration                            time.Duration
	publicKey                           string
	pop                                 string
	ErrMutuallyExlusiveKeyLedger        = errors.New("--key and --ledger,--ledger-addrs are mutually exclusive")
	ErrStoredKeyOnMainnet               = errors.New("--key is not available for mainnet operations")
)

type jsonProofOfPossession struct {
	PublicKey         string `json:"publicKey"`
	ProofOfPossession string `json:"proofOfPossession"`
}

// avalanche primary addValidator
func newAddValidatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addValidator",
		Short: "Add a validator to Primary Network",
		Long: `The primary addValidator command adds a node as a validator 
in the Primary Network`,
		RunE: addValidator,
		Args: cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, addValidatorSupportedNetworkOptions)
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji only]")
	cmd.Flags().StringVar(&nodeIDStr, "nodeID", "", "set the NodeID of the validator to add")
	cmd.Flags().Uint64Var(&weight, "weight", 0, "set the staking weight of the validator to add")
	cmd.Flags().StringVar(&startTimeStr, "start-time", "", "UTC start time when this validator starts validating, in 'YYYY-MM-DD HH:MM:SS' format")
	cmd.Flags().DurationVar(&duration, "staking-period", 0, "how long this validator will be staking")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	cmd.Flags().StringVar(&publicKey, "public-key", "", "set the BLS public key of the validator to add")
	cmd.Flags().StringVar(&pop, "proof-of-possession", "", "set the BLS proof of possession of the validator to add")
	cmd.Flags().Uint32Var(&delegationFee, "delegation-fee", 0, "set the delegation fee (20 000 is equivalent to 2%)")
	return cmd
}

func promptProofOfPossession() (jsonProofOfPossession, error) {
	if publicKey != "" {
		err := prompts.ValidateHexa(publicKey)
		if err != nil {
			ux.Logger.PrintToUser("Format error in given public key: %s", err)
			publicKey = ""
		}
	}
	if pop != "" {
		err := prompts.ValidateHexa(pop)
		if err != nil {
			ux.Logger.PrintToUser("Format error in given proof of possession: %s", err)
			pop = ""
		}
	}
	if publicKey == "" || pop == "" {
		ux.Logger.PrintToUser("Next, we need the public key and proof of possession of the node's BLS")
		ux.Logger.PrintToUser("SSH into the node and call info.getNodeID API to get the node's BLS info")
		ux.Logger.PrintToUser("Check https://docs.avax.network/apis/avalanchego/apis/info#infogetnodeid for instructions on calling info.getNodeID API")
	}
	var err error
	if publicKey == "" {
		txt := "What is the public key of the node's BLS?"
		publicKey, err = app.Prompt.CaptureValidatedString(txt, prompts.ValidateHexa)
		if err != nil {
			return jsonProofOfPossession{}, err
		}
	}
	if pop == "" {
		txt := "What is the proof of possession of the node's BLS?"
		pop, err = app.Prompt.CaptureValidatedString(txt, prompts.ValidateHexa)
		if err != nil {
			return jsonProofOfPossession{}, err
		}
	}
	return jsonProofOfPossession{PublicKey: publicKey, ProofOfPossession: pop}, nil
}

func addValidator(_ *cobra.Command, _ []string) error {
	var (
		nodeID ids.NodeID
		start  time.Time
		err    error
	)

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		false,
		false,
		addValidatorSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	if len(ledgerAddresses) > 0 {
		useLedger = true
	}

	if useLedger && keyName != "" {
		return ErrMutuallyExlusiveKeyLedger
	}

	switch network.Kind {
	case models.Fuji:
		if !useLedger && keyName == "" {
			useLedger, keyName, err = prompts.GetKeyOrLedger(app.Prompt, constants.PayTxsFeesMsg, app.GetKeyDir(), false)
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

	if nodeIDStr == "" {
		nodeID, err = blockchaincmd.PromptNodeID()
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

	fee := network.GenesisParams().TxFeeConfig.StaticFeeConfig.AddPrimaryNetworkValidatorFee
	kc, err := keychain.GetKeychain(app, false, useLedger, ledgerAddresses, keyName, network, fee)
	if err != nil {
		return err
	}

	network.HandlePublicNetworkSimulation()

	jsonPop, err := promptProofOfPossession()
	if err != nil {
		return err
	}
	popBytes, err := json.Marshal(jsonPop)
	if err != nil {
		return err
	}
	start, duration, err = nodecmd.GetTimeParametersPrimaryNetwork(network, 0, duration, startTimeStr, false)
	if err != nil {
		return err
	}
	deployer := subnet.NewPublicDeployer(app, kc, network)
	nodecmd.PrintNodeJoinPrimaryNetworkOutput(nodeID, weight, network, start)
	recipientAddr := kc.Addresses().List()[0]
	if delegationFee == 0 {
		delegationFee, err = getDelegationFeeOption(app, network)
		if err != nil {
			return err
		}
	} else {
		defaultFee := network.GenesisParams().MinDelegationFee
		if delegationFee < defaultFee {
			return fmt.Errorf("delegation fee has to be larger than %d", defaultFee)
		}
	}
	_, err = deployer.AddPermissionlessValidator(ids.Empty, ids.Empty, nodeID, weight, uint64(start.Unix()), uint64(start.Add(duration).Unix()), recipientAddr, delegationFee, popBytes, nil)
	return err
}

func getDelegationFeeOption(app *application.Avalanche, network models.Network) (uint32, error) {
	ux.Logger.PrintToUser("What would you like to set the delegation fee to?")
	defaultFee := network.GenesisParams().MinDelegationFee
	defaultOption := fmt.Sprintf("Default Delegation Fee (%d%%)", defaultFee/10000)
	delegationFeePrompt := "Delegation Fee"
	feeOption, err := app.Prompt.CaptureList(
		delegationFeePrompt,
		[]string{defaultOption, "Custom"},
	)
	if err != nil {
		return 0, err
	}
	if feeOption != defaultOption {
		ux.Logger.PrintToUser("Note that 20 000 is equivalent to 2%%")
		delegationFee, err := app.Prompt.CapturePositiveInt(
			delegationFeePrompt,
			[]prompts.Comparator{
				{
					Label: "Min Delegation Fee",
					Type:  prompts.MoreThanEq,
					Value: uint64(defaultFee),
				},
			},
		)
		if err != nil {
			return 0, err
		}
		if delegationFee > 0 && delegationFee <= math.MaxUint32 {
			return uint32(delegationFee), nil
		}
		return 0, fmt.Errorf("invalid delegation fee")
	}
	return defaultFee, nil
}
