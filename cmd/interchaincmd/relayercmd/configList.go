// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package relayercmd

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/olekukonko/tablewriter"
)

type SourceSpec struct {
	blockchainDesc      string
	rpcEndpoint         string
	wsEndpoint          string
	blockchainID        string
	subnetID            string
	rewardAddress       string
	icmMessengerAddress string
	icmRegistryAddress  string
}

type DestinationSpec struct {
	blockchainDesc string
	rpcEndpoint    string
	blockchainID   string
	subnetID       string
	privateKey     string
}

type ConfigSpec struct {
	sources      []SourceSpec
	destinations []DestinationSpec
}

const (
	explainOption = "Explain the difference"
	cancelOption  = "Cancel"
)

func preview(configSpec ConfigSpec) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	if len(configSpec.sources) > 0 {
		for _, source := range configSpec.sources {
			table.Append([]string{"Source", source.blockchainDesc})
		}
	}
	if len(configSpec.destinations) > 0 {
		for _, destination := range configSpec.destinations {
			table.Append([]string{"Destination", destination.blockchainDesc})
		}
	}
	table.Render()
	fmt.Println()
}

func addBoth(network models.Network, configSpec ConfigSpec, chainSpec contract.ChainSpec, defaultKey string) (ConfigSpec, error) {
	prompt := "Which blockchain do you want to set both as source and destination?"
	var err error
	if !chainSpec.Defined() {
		chainSpec, err = getBlockchain(network, prompt)
		if err != nil {
			return ConfigSpec{}, err
		}
	}
	rpcEndpoint, wsEndpoint, err := contract.GetBlockchainEndpoints(app, network, chainSpec, true, true)
	if err != nil {
		return ConfigSpec{}, err
	}
	configSpec, err = addSource(network, configSpec, chainSpec, rpcEndpoint, wsEndpoint, defaultKey)
	if err != nil {
		return ConfigSpec{}, err
	}
	configSpec, err = addDestination(network, configSpec, chainSpec, rpcEndpoint, defaultKey)
	if err != nil {
		return ConfigSpec{}, err
	}
	return configSpec, nil
}

func getBlockchain(network models.Network, prompt string) (contract.ChainSpec, error) {
	chainSpec := contract.ChainSpec{}
	chainSpec.SetEnabled(true, true, false, false, true)
	if cancel, err := contract.PromptChain(
		app,
		network,
		prompt,
		"",
		&chainSpec,
	); err != nil {
		return chainSpec, err
	} else if cancel {
		return chainSpec, fmt.Errorf("cancelled by user")
	}
	return chainSpec, nil
}

func addSource(
	network models.Network,
	configSpec ConfigSpec,
	chainSpec contract.ChainSpec,
	rpcEndpoint string,
	wsEndpoint string,
	defaultKey string,
) (ConfigSpec, error) {
	if !chainSpec.Defined() {
		prompt := "Which blockchain do you want to set as source?"
		var err error
		chainSpec, err = getBlockchain(network, prompt)
		if err != nil {
			return ConfigSpec{}, err
		}
		rpcEndpoint, wsEndpoint, err = contract.GetBlockchainEndpoints(app, network, chainSpec, true, true)
		if err != nil {
			return ConfigSpec{}, err
		}
	}
	blockchainID, err := contract.GetBlockchainID(app, network, chainSpec)
	if err != nil {
		return ConfigSpec{}, err
	}
	if foundSource := utils.Find(configSpec.sources, func(s SourceSpec) bool { return s.blockchainID == blockchainID.String() }); foundSource != nil {
		ux.Logger.PrintToUser("blockchain is already a source")
		return configSpec, nil
	}
	blockchainDesc, err := contract.GetBlockchainDesc(chainSpec)
	if err != nil {
		return ConfigSpec{}, err
	}
	subnetID, err := contract.GetSubnetID(app, network, chainSpec)
	if err != nil {
		return ConfigSpec{}, err
	}
	icmRegistryAddress, icmMessengerAddress, err := contract.GetICMInfo(app, network, chainSpec, true, true, false)
	if err != nil {
		return ConfigSpec{}, err
	}
	rewardAddress := ""
	if defaultKey != "" {
		k, err := app.GetKey(defaultKey, network, false)
		if err != nil {
			return ConfigSpec{}, err
		}
		rewardAddress = k.C()
	} else {
		genesisAddress, _, err := contract.GetEVMSubnetPrefundedKey(
			app,
			network,
			chainSpec,
		)
		if err != nil {
			return ConfigSpec{}, err
		}
		rewardAddress, err = prompts.PromptAddress(
			app.Prompt,
			fmt.Sprintf("receive relayer rewards on %s", blockchainDesc),
			app.GetKeyDir(),
			app.GetKey,
			genesisAddress,
			network,
			prompts.EVMFormat,
			"Address",
		)
		if err != nil {
			return ConfigSpec{}, err
		}
	}
	configSpec.sources = append(configSpec.sources, SourceSpec{
		blockchainDesc:      blockchainDesc,
		blockchainID:        blockchainID.String(),
		subnetID:            subnetID.String(),
		rewardAddress:       rewardAddress,
		icmRegistryAddress:  icmRegistryAddress,
		icmMessengerAddress: icmMessengerAddress,
		rpcEndpoint:         rpcEndpoint,
		wsEndpoint:          wsEndpoint,
	})
	return configSpec, nil
}

func addDestination(
	network models.Network,
	configSpec ConfigSpec,
	chainSpec contract.ChainSpec,
	rpcEndpoint string,
	defaultKey string,
) (ConfigSpec, error) {
	if !chainSpec.Defined() {
		prompt := "Which blockchain do you want to set as destination?"
		var err error
		chainSpec, err = getBlockchain(network, prompt)
		if err != nil {
			return ConfigSpec{}, err
		}
		rpcEndpoint, _, err = contract.GetBlockchainEndpoints(app, network, chainSpec, true, false)
		if err != nil {
			return ConfigSpec{}, err
		}
	}
	blockchainID, err := contract.GetBlockchainID(app, network, chainSpec)
	if err != nil {
		return ConfigSpec{}, err
	}
	if foundDestination := utils.Find(configSpec.destinations, func(s DestinationSpec) bool { return s.blockchainID == blockchainID.String() }); foundDestination != nil {
		ux.Logger.PrintToUser("blockchain is already a destination")
		return configSpec, nil
	}
	blockchainDesc, err := contract.GetBlockchainDesc(chainSpec)
	if err != nil {
		return ConfigSpec{}, err
	}
	subnetID, err := contract.GetSubnetID(app, network, chainSpec)
	if err != nil {
		return ConfigSpec{}, err
	}
	privateKey := ""
	if defaultKey != "" {
		k, err := app.GetKey(defaultKey, network, false)
		if err != nil {
			return ConfigSpec{}, err
		}
		privateKey = k.PrivKeyHex()
	} else {
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Please provide a key that is not going to be used for any other purpose on destination"))
		privateKey, err = prompts.PromptPrivateKey(
			app.Prompt,
			fmt.Sprintf("pay relayer fees on %s", blockchainDesc),
			app.GetKeyDir(),
			app.GetKey,
			"",
			"",
		)
		if err != nil {
			return ConfigSpec{}, err
		}
	}
	configSpec.destinations = append(configSpec.destinations, DestinationSpec{
		blockchainDesc: blockchainDesc,
		blockchainID:   blockchainID.String(),
		subnetID:       subnetID.String(),
		privateKey:     privateKey,
		rpcEndpoint:    rpcEndpoint,
	})
	return configSpec, nil
}

func removeSource(
	configSpec ConfigSpec,
) (ConfigSpec, bool, error) {
	if len(configSpec.sources) == 0 {
		ux.Logger.PrintToUser("There are no sources to remove")
		ux.Logger.PrintToUser("")
		return configSpec, true, nil
	}
	prompt := "Select the source you want to remove"
	options := utils.Map(configSpec.sources, func(s SourceSpec) string { return s.blockchainDesc })
	options = append(options, cancelOption)
	opt, err := app.Prompt.CaptureList(prompt, options)
	if err != nil {
		return configSpec, false, err
	}
	if opt != cancelOption {
		configSpec.sources = utils.Filter(configSpec.sources, func(s SourceSpec) bool { return s.blockchainDesc != opt })
		return configSpec, false, nil
	}
	return configSpec, true, nil
}

func removeDestination(
	configSpec ConfigSpec,
) (ConfigSpec, bool, error) {
	if len(configSpec.destinations) == 0 {
		ux.Logger.PrintToUser("There are no destinations to remove")
		ux.Logger.PrintToUser("")
		return configSpec, true, nil
	}
	prompt := "Select the destination you want to remove"
	options := utils.Map(configSpec.destinations, func(d DestinationSpec) string { return d.blockchainDesc })
	options = append(options, cancelOption)
	opt, err := app.Prompt.CaptureList(prompt, options)
	if err != nil {
		return configSpec, false, err
	}
	if opt != cancelOption {
		configSpec.destinations = utils.Filter(configSpec.destinations, func(d DestinationSpec) bool { return d.blockchainDesc != opt })
		return configSpec, false, nil
	}
	return configSpec, true, nil
}

func GenerateConfigSpec(
	network models.Network,
	relayCChain bool,
	blockchainsToRelay []string,
	defaultKey string,
) (ConfigSpec, bool, error) {
	configSpec := ConfigSpec{}
	var err error

	noPrompts := false
	if relayCChain {
		chainSpec := contract.ChainSpec{
			CChain: true,
		}
		chainSpec.SetEnabled(true, true, false, false, false)
		configSpec, err = addBoth(network, configSpec, chainSpec, defaultKey)
		if err != nil {
			return ConfigSpec{}, false, err
		}
		noPrompts = true
	}
	for _, blockchainName := range blockchainsToRelay {
		chainSpec := contract.ChainSpec{
			BlockchainName: blockchainName,
		}
		chainSpec.SetEnabled(true, true, false, false, false)
		configSpec, err = addBoth(network, configSpec, chainSpec, defaultKey)
		if err != nil {
			return ConfigSpec{}, false, err
		}
		noPrompts = true
	}
	if noPrompts {
		return configSpec, false, nil
	}

	prompt := "Configure the blockchains that will be interconnected by the relayer"

	addOption := "Add a blockchain"
	removeOption := "Remove a blockchain"
	previewOption := "Preview"
	confirmOption := "Confirm"

	for {
		options := []string{
			addOption,
			removeOption,
			previewOption,
			confirmOption,
			cancelOption,
		}
		if len(configSpec.sources) == 0 && len(configSpec.destinations) == 0 {
			options = utils.RemoveFromSlice(options, removeOption)
			options = utils.RemoveFromSlice(options, previewOption)
			options = utils.RemoveFromSlice(options, confirmOption)
		}
		option, err := app.Prompt.CaptureList(prompt, options)
		if err != nil {
			return ConfigSpec{}, false, err
		}
		switch option {
		case addOption:
			addPrompt := "What role should the blockchain have?"
			addBothOption := "Source and Destination"
			addSourceOption := "Source only"
			addDestinationOption := "Destination only"
			for {
				options := []string{addBothOption, addSourceOption, addDestinationOption, explainOption, cancelOption}
				roleOption, err := app.Prompt.CaptureList(addPrompt, options)
				if err != nil {
					return ConfigSpec{}, false, err
				}
				switch roleOption {
				case addBothOption:
					configSpec, err = addBoth(network, configSpec, contract.ChainSpec{}, "")
					if err != nil {
						return ConfigSpec{}, false, err
					}
				case addSourceOption:
					configSpec, err = addSource(network, configSpec, contract.ChainSpec{}, "", "", "")
					if err != nil {
						return ConfigSpec{}, false, err
					}
				case addDestinationOption:
					configSpec, err = addDestination(network, configSpec, contract.ChainSpec{}, "", "")
					if err != nil {
						return ConfigSpec{}, false, err
					}
				case explainOption:
					ux.Logger.PrintToUser("A source blockchain is going to be listened by the relayer to check for new")
					ux.Logger.PrintToUser("messages. You need to specify blockchain ID, ICM addresses.")
					ux.Logger.PrintToUser("A destination blockchain is going to be connected by the relayer in order")
					ux.Logger.PrintToUser("to deliver a message. You need to specify blockchain ID, private key")
					continue
				case cancelOption:
				}
				break
			}
		case removeOption:
			keepAsking := true
			for keepAsking {
				removePrompt := "Which role do you want to remove?"
				removeSourceOption := "Source"
				removeDestinationOption := "Destination"
				options := []string{}
				if len(configSpec.sources) != 0 {
					options = append(options, removeSourceOption)
				}
				if len(configSpec.destinations) != 0 {
					options = append(options, removeDestinationOption)
				}
				options = append(options, cancelOption)
				kindOption, err := app.Prompt.CaptureList(removePrompt, options)
				if err != nil {
					return ConfigSpec{}, false, err
				}
				switch kindOption {
				case removeSourceOption:
					configSpec, keepAsking, err = removeSource(configSpec)
					if err != nil {
						return ConfigSpec{}, false, err
					}
				case removeDestinationOption:
					configSpec, keepAsking, err = removeDestination(configSpec)
					if err != nil {
						return ConfigSpec{}, false, err
					}
				case cancelOption:
					keepAsking = false
				}
			}
		case previewOption:
			preview(configSpec)
		case confirmOption:
			preview(configSpec)
			confirmPrompt := "Confirm?"
			yesOption := "Yes"
			noOption := "No, keep editing"
			confirmOption, err := app.Prompt.CaptureList(
				confirmPrompt, []string{yesOption, noOption},
			)
			if err != nil {
				return ConfigSpec{}, false, err
			}
			if confirmOption == yesOption {
				return configSpec, false, nil
			}
		case cancelOption:
			return ConfigSpec{}, true, err
		}
	}
}
