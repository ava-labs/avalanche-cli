// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"encoding/json"
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

func getValidatorContractManagerAddr() ([]string, bool, error) {
	controllerAddrPrompt := "Enter Validator Manager Contract controller address"
	for {
		// ask in a loop so that if some condition is not met we can keep asking
		controlAddr, cancelled, err := getAddrLoop(controllerAddrPrompt, constants.ValidatorManagerController, models.UndefinedNetwork)
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
	addr, cancelled, err := getAddr()
	if err != nil {
		return nil, err
	}
	if cancelled {
		return nil, nil
	}
	return addr, nil
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

func promptProofOfPossession() (signer.Signer, error) {
	ux.Logger.PrintToUser("Next, we need the public key and proof of possession of the node's BLS")
	ux.Logger.PrintToUser("Check https://docs.avax.network/api-reference/info-api#infogetnodeid for instructions on calling info.getNodeID API")
	var err error
	txt := "What is the public key of the node's BLS?"
	publicKey, err := app.Prompt.CaptureValidatedString(txt, prompts.ValidateHexa)
	if err != nil {
		return nil, err
	}
	txt = "What is the proof of possession of the node's BLS?"
	proofOfPossesion, err := app.Prompt.CaptureValidatedString(txt, prompts.ValidateHexa)
	if err != nil {
		return nil, err
	}
	pop, err := getBLSInfo(publicKey, proofOfPossesion)
	if err != nil {
		return nil, err
	}
	return pop, nil
}

// TODO: add explain the difference for different validator management type
func promptValidatorManagementType(
	app *application.Avalanche,
	sidecar *models.Sidecar,
) error {
	proofOfAuthorityOption := "Proof of Authority"
	proofOfStakeOption := "Proof of Stake"
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

// TODO: find the min weight for bootstrap validator
func PromptWeightBootstrapValidator() (uint64, error) {
	txt := "What stake weight would you like to assign to the validator?"
	return app.Prompt.CaptureWeight(txt)
}

func PromptInitialBalance() (uint64, error) {
	defaultInitialBalance := fmt.Sprintf("Default (%d AVAX)", constants.MinInitialBalanceBootstrapValidator)
	txt := "What initial balance would you like to assign to the bootstrap validator (in AVAX)?"
	weightOptions := []string{defaultInitialBalance, "Custom"}
	weightOption, err := app.Prompt.CaptureList(txt, weightOptions)
	if err != nil {
		return 0, err
	}

	switch weightOption {
	case defaultInitialBalance:
		return constants.MinInitialBalanceBootstrapValidator, nil
	default:
		return app.Prompt.CaptureBootstrapInitialBalance(txt)
	}
}

func promptBootstrapValidators() ([]models.SubnetValidator, error) {
	var subnetValidators []models.SubnetValidator
	numBootstrapValidators, err := app.Prompt.CaptureInt(
		"How many bootstrap validators do you want to set up?",
	)
	if err != nil {
		return nil, err
	}
	previousAddr := ""
	for len(subnetValidators) < numBootstrapValidators {
		ux.Logger.PrintToUser("Getting info for bootstrap validator %d", len(subnetValidators)+1)
		nodeID, err := PromptNodeID()
		if err != nil {
			return nil, err
		}
		weight, err := PromptWeightBootstrapValidator()
		if err != nil {
			return nil, err
		}
		balance, err := PromptInitialBalance()
		if err != nil {
			return nil, err
		}
		proofOfPossession, err := promptProofOfPossession()
		changeAddr, err := getKeyForChangeOwner(previousAddr)
		if err != nil {
			return nil, err
		}
		addrs, err := address.ParseToIDs([]string{changeAddr})
		if err != nil {
			return nil, fmt.Errorf("failure parsing change owner address: %w", err)
		}
		changeOwner := &secp256k1fx.OutputOwners{
			Threshold: 1,
			Addrs:     addrs,
		}
		previousAddr = changeAddr
		subnetValidator := models.SubnetValidator{
			NodeID:      nodeID,
			Weight:      weight,
			Balance:     balance,
			Signer:      proofOfPossession,
			ChangeOwner: changeOwner,
		}
		subnetValidators = append(subnetValidators, subnetValidator)
		ux.Logger.GreenCheckmarkToUser("Bootstrap Validator %d:", len(subnetValidators))
		ux.Logger.PrintToUser("- Node ID: %s", nodeID)
		ux.Logger.PrintToUser("- Weight: %d", weight)
		ux.Logger.PrintToUser("- Initial Balance: %d AVAX", balance)
		ux.Logger.PrintToUser("- Change Address: %s", changeAddr)
	}
	return subnetValidators, nil
}

func validateBLS(publicKey, pop string) error {
	if err := prompts.ValidateHexa(publicKey); err != nil {
		return fmt.Errorf("format error in given public key: %s", err)
	}
	if err := prompts.ValidateHexa(pop); err != nil {
		return fmt.Errorf("format error in given proof of possession: %s", err)
	}
	return nil
}

func getBLSInfo(publicKey, proofOfPossesion string) (signer.Signer, error) {
	type jsonProofOfPossession struct {
		PublicKey         string
		ProofOfPossession string
	}
	jsonPop := jsonProofOfPossession{
		PublicKey:         publicKey,
		ProofOfPossession: proofOfPossesion,
	}
	popBytes, err := json.Marshal(jsonPop)
	if err != nil {
		return nil, err
	}
	pop := &signer.ProofOfPossession{}
	err = pop.UnmarshalJSON(popBytes)
	if err != nil {
		return nil, err
	}
	return pop, nil
}

func convertToSubnetValidators(validatorJSONS []models.SubnetValidatorJSON) ([]models.SubnetValidator, error) {
	subnetValidators := []models.SubnetValidator{}
	type jsonProofOfPossession struct {
		PublicKey         string
		ProofOfPossession string
	}
	for _, validatorJSON := range validatorJSONS {
		nodeID, err := ids.NodeIDFromString(validatorJSON.NodeID)
		if err != nil {
			return nil, fmt.Errorf("invalid node id %s", validatorJSON.NodeID)
		}
		if validatorJSON.Weight <= 0 {
			return nil, fmt.Errorf("bootstrap validator weight has to be greater than 0")
		}
		if validatorJSON.Balance <= 0 {
			return nil, fmt.Errorf("bootstrap validator balance has to be greater than 0")
		}
		if err = validateBLS(validatorJSON.BLSPublicKey, validatorJSON.BLSProofOfPossession); err != nil {
			return nil, err
		}
		//jsonPop := jsonProofOfPossession{
		//	PublicKey:         validatorJSON.BLSPublicKey,
		//	ProofOfPossession: validatorJSON.BLSProofOfPossession,
		//}
		//popBytes, err := json.Marshal(jsonPop)
		//if err != nil {
		//	return nil, err
		//}
		//pop := &signer.ProofOfPossession{}
		//err = pop.UnmarshalJSON(popBytes)
		//if err != nil {
		//	return nil, err
		//}
		pop, err := getBLSInfo(validatorJSON.BLSPublicKey, validatorJSON.BLSProofOfPossession)
		if err != nil {
			return nil, err
		}
		changeAddr, err := ids.ShortFromString(validatorJSON.ChangeOwnerAddr)
		if err != nil {
			return nil, err
		}
		changeOwner := &secp256k1fx.OutputOwners{
			Threshold: 1,
			Addrs:     []ids.ShortID{changeAddr},
		}
		subnetValidators = append(subnetValidators,
			models.SubnetValidator{
				NodeID:      nodeID,
				Weight:      validatorJSON.Weight,
				Balance:     validatorJSON.Balance,
				Signer:      pop,
				ChangeOwner: changeOwner,
			},
		)
	}
	return subnetValidators, nil
}
