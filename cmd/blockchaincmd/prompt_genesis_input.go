// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/cmd/flags"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/blockchain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-tooling-sdk-go/validatormanager/validatormanagertypes"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/staking"
	"github.com/ava-labs/avalanchego/utils/crypto/bls/signer/localsigner"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
)

func getValidatorContractManagerAddr() (string, error) {
	return prompts.PromptAddress(
		app.Prompt,
		"enable as controller of ValidatorManager contract",
		app.GetKeyDir(),
		app.GetKey,
		"",
		models.UndefinedNetwork,
		prompts.EVMFormat,
		"Enter address (C-Chain address)",
	)
}

func promptProofOfPossession(promptPublicKey, promptPop bool) (string, string, error) {
	if promptPublicKey || promptPop {
		ux.Logger.PrintToUser("Next, we need the public key and proof of possession of the node's BLS")
		ux.Logger.PrintToUser("Check https://docs.avax.network/api-reference/info-api#infogetnodeid for instructions on calling info.getNodeID API")
	}
	var err error
	publicKey := ""
	proofOfPossesion := ""
	if promptPublicKey {
		txt := "What is the node's BLS public key?"
		publicKey, err = app.Prompt.CaptureValidatedString(txt, prompts.ValidateHexa)
		if err != nil {
			return "", "", err
		}
	}
	if promptPop {
		txt := "What is the node's BLS proof of possession?"
		proofOfPossesion, err = app.Prompt.CaptureValidatedString(txt, prompts.ValidateHexa)
		if err != nil {
			return "", "", err
		}
	}
	return publicKey, proofOfPossesion, nil
}

// TODO: add explain the difference for different validator management type
func promptValidatorManagementType(
	app *application.Avalanche,
	sidecar *models.Sidecar,
) error {
	explainOption := "Explain the difference"
	if createFlags.proofOfStakeNative {
		sidecar.ValidatorManagement = validatormanagertypes.ProofOfStakeNative
		return nil
	}
	if createFlags.proofOfStakeERC20 {
		sidecar.ValidatorManagement = validatormanagertypes.ProofOfStakeERC20
		return nil
	}
	if createFlags.proofOfAuthority {
		sidecar.ValidatorManagement = validatormanagertypes.ProofOfAuthority
		return nil
	}

	options := []string{
		validatormanagertypes.ProofOfAuthority,
		validatormanagertypes.ProofOfStakeNative,
		validatormanagertypes.ProofOfStakeERC20,
		explainOption,
	}
	for {
		option, err := app.Prompt.CaptureList(
			"Which validator management type would you like to use in your blockchain?",
			options,
		)
		if err != nil {
			return err
		}
		switch option {
		case validatormanagertypes.ProofOfAuthority:
			sidecar.ValidatorManagement = validatormanagertypes.ValidatorManagementTypeFromString(option)
		case validatormanagertypes.ProofOfStakeNative:
			sidecar.ValidatorManagement = validatormanagertypes.ValidatorManagementTypeFromString(option)
		case validatormanagertypes.ProofOfStakeERC20:
			sidecar.ValidatorManagement = validatormanagertypes.ValidatorManagementTypeFromString(option)
		case explainOption:
			ux.Logger.PrintToUser("Validator Management Types:")
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser("  • Proof Of Authority (PoA)")
			ux.Logger.PrintToUser("    - Validators are managed by an owner/admin address")
			ux.Logger.PrintToUser("    - Owner can add/remove validators and set their weight")
			ux.Logger.PrintToUser("    - No staking required, fully permissioned")
			ux.Logger.PrintToUser("    - Best for: Permissioned networks")
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser("  • Proof Of Stake Native")
			ux.Logger.PrintToUser("    - Validators stake the blockchain's native token")
			ux.Logger.PrintToUser("    - Anyone can become a validator by staking minimum amount")
			ux.Logger.PrintToUser("    - Validator weight is based on stake amount")
			ux.Logger.PrintToUser("    - Supports delegation and staking rewards")
			ux.Logger.PrintToUser("    - Best for: Public networks using native token economics")
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser("  • Proof Of Stake ERC20")
			ux.Logger.PrintToUser("    - Validators stake a ERC20 token")
			ux.Logger.PrintToUser("    - Anyone can become a validator by staking minimum amount")
			ux.Logger.PrintToUser("    - Validator weight is based on ERC20 token stake amount")
			ux.Logger.PrintToUser("    - Supports delegation and staking rewards")
			ux.Logger.PrintToUser("    - Best for: Networks staking existing tokens, or custom token economics")
			ux.Logger.PrintToUser("")
			continue
		}
		break
	}
	return nil
}

// generateNewNodeAndBLS returns node id, bls public key and bls pop
func generateNewNodeAndBLS() (string, string, string, error) {
	certBytes, _, err := staking.NewCertAndKeyBytes()
	if err != nil {
		return "", "", "", err
	}
	nodeID, err := utils.ToNodeID(certBytes)
	if err != nil {
		return "", "", "", err
	}
	blsSignerKey, err := localsigner.New()
	if err != nil {
		return "", "", "", err
	}
	p, err := signer.NewProofOfPossession(blsSignerKey)
	if err != nil {
		return "", "", "", err
	}
	publicKey, err := formatting.Encode(formatting.HexNC, p.PublicKey[:])
	if err != nil {
		return "", "", "", err
	}
	pop, err := formatting.Encode(formatting.HexNC, p.ProofOfPossession[:])
	if err != nil {
		return "", "", "", err
	}
	return nodeID.String(), publicKey, pop, nil
}

func promptBootstrapValidators(
	network models.Network,
	validatorBalance uint64,
	availableBalance uint64,
	validatorWeight uint64,
	bootstrapValidatorFlags *flags.BootstrapValidatorFlags,
) ([]models.SubnetValidator, error) {
	var subnetValidators []models.SubnetValidator
	var err error
	if bootstrapValidatorFlags.NumBootstrapValidators == 0 {
		maxNumValidators := availableBalance / validatorBalance
		bootstrapValidatorFlags.NumBootstrapValidators, err = app.Prompt.CaptureInt(
			"How many bootstrap validators do you want to set up?",
			func(n int) error {
				if err := prompts.ValidatePositiveInt(n); err != nil {
					return err
				}
				if n > int(maxNumValidators) {
					return fmt.Errorf(
						"given available balance %d, the maximum number of validators with balance %d is %d",
						availableBalance,
						validatorBalance,
						maxNumValidators,
					)
				}
				return nil
			},
		)
	}
	if err != nil {
		return nil, err
	}
	var setUpNodes bool
	if bootstrapValidatorFlags.GenerateNodeID {
		setUpNodes = false
	} else {
		setUpNodes, err = promptSetUpNodes()
		if err != nil {
			return nil, err
		}
		bootstrapValidatorFlags.GenerateNodeID = !setUpNodes
	}
	if bootstrapValidatorFlags.ChangeOwnerAddress == "" {
		bootstrapValidatorFlags.ChangeOwnerAddress, err = blockchain.GetKeyForChangeOwner(app, network)
		if err != nil {
			return nil, err
		}
	}
	for len(subnetValidators) < bootstrapValidatorFlags.NumBootstrapValidators {
		ux.Logger.PrintToUser("Getting info for bootstrap validator %d", len(subnetValidators)+1)
		var nodeID ids.NodeID
		var publicKey, pop string
		if setUpNodes {
			nodeID, err = PromptNodeID("add as bootstrap validator")
			if err != nil {
				return nil, err
			}
			publicKey, pop, err = promptProofOfPossession(true, true)
			if err != nil {
				return nil, err
			}
		} else {
			nodeIDStr, publicKey, pop, err = generateNewNodeAndBLS()
			if err != nil {
				return nil, err
			}
			nodeID, err = ids.NodeIDFromString(nodeIDStr)
			if err != nil {
				return nil, err
			}
		}
		subnetValidator := models.SubnetValidator{
			NodeID:               nodeID.String(),
			Weight:               validatorWeight,
			Balance:              validatorBalance,
			BLSPublicKey:         publicKey,
			BLSProofOfPossession: pop,
			ChangeOwnerAddr:      bootstrapValidatorFlags.ChangeOwnerAddress,
		}
		subnetValidators = append(subnetValidators, subnetValidator)
		ux.Logger.GreenCheckmarkToUser("Bootstrap Validator %d:", len(subnetValidators))
		ux.Logger.PrintToUser("- Node ID: %s", nodeID)
		ux.Logger.PrintToUser("- Change Address: %s", bootstrapValidatorFlags.ChangeOwnerAddress)
	}
	return subnetValidators, nil
}

func validateBLS(publicKey, pop string) error {
	if err := prompts.ValidateHexa(publicKey); err != nil {
		return fmt.Errorf("format error in given public key: %w", err)
	}
	if err := prompts.ValidateHexa(pop); err != nil {
		return fmt.Errorf("format error in given proof of possession: %w", err)
	}
	return nil
}

func validateSubnetValidatorsJSON(generateNewNodeID bool, validatorJSONS []models.SubnetValidator) error {
	for _, validatorJSON := range validatorJSONS {
		if !generateNewNodeID {
			if validatorJSON.NodeID == "" || validatorJSON.BLSPublicKey == "" || validatorJSON.BLSProofOfPossession == "" {
				return fmt.Errorf("no Node ID or BLS info provided, use --generate-node-id flag to generate new Node ID and BLS info")
			}
			_, err := ids.NodeIDFromString(validatorJSON.NodeID)
			if err != nil {
				return fmt.Errorf("invalid node id %s", validatorJSON.NodeID)
			}
			if err = validateBLS(validatorJSON.BLSPublicKey, validatorJSON.BLSProofOfPossession); err != nil {
				return err
			}
		}
		if validatorJSON.Weight == 0 {
			return fmt.Errorf("bootstrap validator weight has to be greater than 0")
		}
		if validatorJSON.Balance == 0 {
			return fmt.Errorf("bootstrap validator balance has to be greater than 0")
		}
	}
	return nil
}

// promptSetUpNodes returns false if user doesn't have any Avalanche node set up yet to be
// bootstrap validators
func promptSetUpNodes() (bool, error) {
	ux.Logger.PrintToUser("If you have set up your own Avalanche Nodes, you can provide the Node ID and BLS Key from those nodes in the next step.")
	ux.Logger.PrintToUser("Otherwise, we will generate new Node IDs and BLS Key for you.")
	setUpNodes, err := app.Prompt.CaptureYesNo("Have you set up your own Avalanche Nodes?")
	if err != nil {
		return false, err
	}
	return setUpNodes, nil
}
