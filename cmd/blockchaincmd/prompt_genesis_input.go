// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/staking"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
)

func getValidatorContractManagerAddr() ([]string, bool, error) {
	controllerAddrPrompt := "Enter Validator Manager Contract controller address"
	for {
		// ask in a loop so that if some condition is not met we can keep asking
		controlAddr, cancelled, err := prompts.CaptureListDecision(
			// we need this to be able to mock test
			app.Prompt,
			// the main prompt for entering address keys
			controllerAddrPrompt,
			// the Capture function to use
			func(_ string) (string, error) {
				return prompts.PromptAddress(
					app.Prompt,
					"enable as controller of ValidatorManager contract",
					app.GetKeyDir(),
					app.GetKey,
					"",
					models.UndefinedNetwork,
					prompts.EVMFormat,
					"Enter address",
				)
			},
			// the prompt for each address
			"",
			// label describes the entity we are prompting for (e.g. address, control key, etc.)
			"Validator Manager Controller",
			//TODO: add info here on what this validator manager controller is
			"",
		)
		if err != nil {
			return nil, false, err
		}
		if cancelled {
			return nil, cancelled, nil
		}
		if len(controlAddr) != 0 {
			return controlAddr, false, nil
		}
		ux.Logger.RedXToUser("An address to control Validator Manage Contract is required before proceeding")
	}
}

// Configure which addresses may make mint new native tokens
func getTokenMinterAddr() ([]string, error) {
	addTokenMinterAddrPrompt := "Currently only Validator Manager Contract can mint new native tokens"
	ux.Logger.PrintToUser(addTokenMinterAddrPrompt)
	yes, err := app.Prompt.CaptureNoYes("Add additional addresses that can mint new native tokens?")
	if err != nil {
		return nil, err
	}
	if !yes {
		return nil, nil
	}
	addresses, cancelled, err := getAddr()
	if err != nil {
		return nil, err
	}
	if cancelled {
		return nil, nil
	}
	return addresses, nil
}

func getAddr() ([]string, bool, error) {
	addrPrompt := "Enter addresses that can mint new native tokens"
	addr, cancelled, err := getAddrLoop(addrPrompt, constants.TokenMinter, models.UndefinedNetwork)
	if err != nil {
		return nil, false, err
	}
	if cancelled {
		return nil, cancelled, nil
	}
	return addr, false, nil
}

func promptProofOfPossession() (string, string, error) {
	ux.Logger.PrintToUser("Next, we need the public key and proof of possession of the node's BLS")
	ux.Logger.PrintToUser("Check https://docs.avax.network/api-reference/info-api#infogetnodeid for instructions on calling info.getNodeID API")
	var err error
	txt := "What is the node's BLS public key?"
	publicKey, err := app.Prompt.CaptureValidatedString(txt, prompts.ValidateHexa)
	if err != nil {
		return "", "", err
	}
	txt = "What is the node's BLS proof of possession?"
	proofOfPossesion, err := app.Prompt.CaptureValidatedString(txt, prompts.ValidateHexa)
	if err != nil {
		return "", "", err
	}
	return publicKey, proofOfPossesion, nil
}

// TODO: add explain the difference for different validator management type
func promptValidatorManagementType(
	app *application.Avalanche,
	sidecar *models.Sidecar,
) error {
	proofOfAuthorityOption := models.ProofOfAuthority
	proofOfStakeOption := models.ProofOfStake
	explainOption := "Explain the difference"
	if createFlags.proofOfStake {
		sidecar.ValidatorManagement = models.ValidatorManagementTypeFromString(proofOfStakeOption)
		return nil
	}
	if createFlags.proofOfAuthority {
		sidecar.ValidatorManagement = models.ValidatorManagementTypeFromString(proofOfAuthorityOption)
		return nil
	}
	options := []string{proofOfAuthorityOption, proofOfStakeOption, explainOption}
	var subnetTypeStr string
	for {
		option, err := app.Prompt.CaptureList(
			"Which validator management protocol would you like to use in your blockchain?",
			options,
		)
		if err != nil {
			return err
		}
		switch option {
		case proofOfAuthorityOption:
			subnetTypeStr = models.ProofOfAuthority
		case proofOfStakeOption:
			subnetTypeStr = models.ProofOfStake
		case explainOption:
			continue
		}
		break
	}
	sidecar.ValidatorManagement = models.ValidatorManagementTypeFromString(subnetTypeStr)
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
	blsSignerKey, err := bls.NewSecretKey()
	if err != nil {
		return "", "", "", err
	}
	p := signer.NewProofOfPossession(blsSignerKey)
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

func promptBootstrapValidators(network models.Network) ([]models.SubnetValidator, error) {
	var subnetValidators []models.SubnetValidator
	numBootstrapValidators, err := app.Prompt.CaptureInt(
		"How many bootstrap validators do you want to set up?",
	)
	if err != nil {
		return nil, err
	}
	var setUpNodes bool
	if generateNodeID {
		setUpNodes = true
	} else {
		setUpNodes, err = promptSetUpNodes()
		if err != nil {
			return nil, err
		}
	}
	previousAddr := ""
	for len(subnetValidators) < numBootstrapValidators {
		ux.Logger.PrintToUser("Getting info for bootstrap validator %d", len(subnetValidators)+1)
		var nodeID ids.NodeID
		var publicKey, pop string
		if setUpNodes {
			nodeID, err = PromptNodeID("add as bootstrap validator")
			if err != nil {
				return nil, err
			}
			publicKey, pop, err = promptProofOfPossession()
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
		changeAddr, err := getKeyForChangeOwner(previousAddr, network)
		if err != nil {
			return nil, err
		}
		previousAddr = changeAddr
		subnetValidator := models.SubnetValidator{
			NodeID:               nodeID.String(),
			Weight:               constants.DefaultBootstrapValidatorWeight,
			Balance:              constants.InitialBalanceBootstrapValidator,
			BLSPublicKey:         publicKey,
			BLSProofOfPossession: pop,
			ChangeOwnerAddr:      changeAddr,
		}
		subnetValidators = append(subnetValidators, subnetValidator)
		ux.Logger.GreenCheckmarkToUser("Bootstrap Validator %d:", len(subnetValidators))
		ux.Logger.PrintToUser("- Node ID: %s", nodeID)
		ux.Logger.PrintToUser("- Change Address: %s", changeAddr)
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
		if validatorJSON.Weight <= 0 {
			return fmt.Errorf("bootstrap validator weight has to be greater than 0")
		}
		if validatorJSON.Balance <= 0 {
			return fmt.Errorf("bootstrap validator balance has to be greater than 0")
		}
	}
	return nil
}

// promptProvideNodeID returns false if user doesn't have any Avalanche node set up yet to be
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
