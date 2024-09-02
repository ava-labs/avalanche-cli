// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
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
	"github.com/olekukonko/tablewriter"
)

type SourceEsp struct {
	blockchainDesc      string
	rpcEndpoint         string
	wsEndpoint          string
	blockchainID        string
	subnetID            string
	rewardAddress       string
	icmMessengerAddress string
	icmRegistryAddress  string
}

type DestinationEsp struct {
	blockchainDesc string
	rpcEndpoint    string
	blockchainID   string
	subnetID       string
	privateKey     string
}

type ConfigEsp struct {
	sources      []SourceEsp
	destinations []DestinationEsp
}

const (
	explainOption = "Explain the difference"
	cancelOption  = "Cancel"
)

func preview(configEsp ConfigEsp) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	if len(configEsp.sources) > 0 {
		for _, source := range configEsp.sources {
			table.Append([]string{"Source", source.blockchainDesc})
		}
	}
	if len(configEsp.destinations) > 0 {
		for _, destination := range configEsp.destinations {
			table.Append([]string{"Destination", destination.blockchainDesc})
		}
	}
	table.Render()
	fmt.Println()
}

func addBoth(network models.Network, configEsp ConfigEsp) (ConfigEsp, error) {
	prompt := "Which blockchain do you want to set both as source and destination?"
	var err error
	chainSpec, err := getBlockchain(network, prompt)
	if err != nil {
		return ConfigEsp{}, err
	}
	rpcEndpoint, wsEndpoint, err := contract.GetBlockchainEndpoints(app, network, chainSpec, true, true)
	if err != nil {
		return ConfigEsp{}, err
	}
	configEsp, err = addSource(network, configEsp, chainSpec, rpcEndpoint, wsEndpoint)
	if err != nil {
		return ConfigEsp{}, err
	}
	configEsp, err = addDestination(network, configEsp, chainSpec, rpcEndpoint)
	if err != nil {
		return ConfigEsp{}, err
	}
	return configEsp, nil
}

func getBlockchain(network models.Network, prompt string) (contract.ChainSpec, error) {
	chainSpec := contract.ChainSpec{}
	if cancel, err := contract.PromptChain(
		app,
		network,
		prompt,
		false,
		"",
		true,
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
	configEsp ConfigEsp,
	chainSpec contract.ChainSpec,
	rpcEndpoint string,
	wsEndpoint string,
) (ConfigEsp, error) {
	if !contract.DefinedChainSpec(chainSpec) {
		prompt := "Which blockchain do you want to set as source?"
		var err error
		chainSpec, err = getBlockchain(network, prompt)
		if err != nil {
			return ConfigEsp{}, err
		}
		rpcEndpoint, wsEndpoint, err = contract.GetBlockchainEndpoints(app, network, chainSpec, true, true)
		if err != nil {
			return ConfigEsp{}, err
		}
	}
	blockchainID, err := contract.GetBlockchainID(app, network, chainSpec)
	if err != nil {
		return ConfigEsp{}, err
	}
	if foundSource := utils.Find(configEsp.sources, func(s SourceEsp) bool { return s.blockchainID == blockchainID.String() }); foundSource != nil {
		ux.Logger.PrintToUser("blockchain is already a source")
		return configEsp, nil
	}
	blockchainDesc, err := contract.GetBlockchainDesc(chainSpec)
	if err != nil {
		return ConfigEsp{}, err
	}
	subnetID, err := contract.GetSubnetID(app, network, chainSpec)
	if err != nil {
		return ConfigEsp{}, err
	}
	icmRegistryAddress, icmMessengerAddress, err := contract.GetICMInfo(app, network, chainSpec, true, true, false)
	if err != nil {
		return ConfigEsp{}, err
	}
	genesisAddress, _, err := contract.GetEVMSubnetPrefundedKey(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return ConfigEsp{}, err
	}
	rewardAddress, err := prompts.PromptAddress(
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
		return ConfigEsp{}, err
	}
	configEsp.sources = append(configEsp.sources, SourceEsp{
		blockchainDesc:      blockchainDesc,
		blockchainID:        blockchainID.String(),
		subnetID:            subnetID.String(),
		rewardAddress:       rewardAddress,
		icmRegistryAddress:  icmRegistryAddress,
		icmMessengerAddress: icmMessengerAddress,
		rpcEndpoint:         rpcEndpoint,
		wsEndpoint:          wsEndpoint,
	})
	return configEsp, nil
}

func addDestination(
	network models.Network,
	configEsp ConfigEsp,
	chainSpec contract.ChainSpec,
	rpcEndpoint string,
) (ConfigEsp, error) {
	if !contract.DefinedChainSpec(chainSpec) {
		prompt := "Which blockchain do you want to set as destination?"
		var err error
		chainSpec, err = getBlockchain(network, prompt)
		if err != nil {
			return ConfigEsp{}, err
		}
		rpcEndpoint, _, err = contract.GetBlockchainEndpoints(app, network, chainSpec, true, false)
		if err != nil {
			return ConfigEsp{}, err
		}
	}
	blockchainID, err := contract.GetBlockchainID(app, network, chainSpec)
	if err != nil {
		return ConfigEsp{}, err
	}
	if foundDestination := utils.Find(configEsp.destinations, func(s DestinationEsp) bool { return s.blockchainID == blockchainID.String() }); foundDestination != nil {
		ux.Logger.PrintToUser("blockchain is already a destination")
		return configEsp, nil
	}
	blockchainDesc, err := contract.GetBlockchainDesc(chainSpec)
	if err != nil {
		return ConfigEsp{}, err
	}
	subnetID, err := contract.GetSubnetID(app, network, chainSpec)
	if err != nil {
		return ConfigEsp{}, err
	}
	genesisAddress, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return ConfigEsp{}, err
	}
	privateKey, err := prompts.PromptPrivateKey(
		app.Prompt,
		fmt.Sprintf("pay relayer fees on %s", blockchainDesc),
		app.GetKeyDir(),
		app.GetKey,
		genesisAddress,
		genesisPrivateKey,
	)
	if err != nil {
		return ConfigEsp{}, err
	}
	configEsp.destinations = append(configEsp.destinations, DestinationEsp{
		blockchainDesc: blockchainDesc,
		blockchainID:   blockchainID.String(),
		subnetID:       subnetID.String(),
		privateKey:     privateKey,
		rpcEndpoint:    rpcEndpoint,
	})
	return configEsp, nil
}

func removeSource(
	configEsp ConfigEsp,
) (ConfigEsp, bool, error) {
	if len(configEsp.sources) == 0 {
		ux.Logger.PrintToUser("There are no sources to remove")
		ux.Logger.PrintToUser("")
		return configEsp, true, nil
	}
	prompt := "Select the source you want to remove"
	options := utils.Map(configEsp.sources, func(s SourceEsp) string { return s.blockchainDesc })
	options = append(options, cancelOption)
	opt, err := app.Prompt.CaptureList(prompt, options)
	if err != nil {
		return configEsp, false, err
	}
	if opt != cancelOption {
		configEsp.sources = utils.Filter(configEsp.sources, func(s SourceEsp) bool { return s.blockchainDesc != opt })
		return configEsp, false, nil
	}
	return configEsp, true, nil
}

func removeDestination(
	configEsp ConfigEsp,
) (ConfigEsp, bool, error) {
	if len(configEsp.destinations) == 0 {
		ux.Logger.PrintToUser("There are no destinations to remove")
		ux.Logger.PrintToUser("")
		return configEsp, true, nil
	}
	prompt := "Select the destination you want to remove"
	options := utils.Map(configEsp.destinations, func(d DestinationEsp) string { return d.blockchainDesc })
	options = append(options, cancelOption)
	opt, err := app.Prompt.CaptureList(prompt, options)
	if err != nil {
		return configEsp, false, err
	}
	if opt != cancelOption {
		configEsp.destinations = utils.Filter(configEsp.destinations, func(d DestinationEsp) bool { return d.blockchainDesc != opt })
		return configEsp, false, nil
	}
	return configEsp, true, nil
}

func GenerateConfigEsp(network models.Network) (ConfigEsp, bool, error) {
	configEsp := ConfigEsp{}

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
		if len(configEsp.sources) == 0 && len(configEsp.destinations) == 0 {
			options = utils.RemoveFromSlice(options, removeOption)
			options = utils.RemoveFromSlice(options, previewOption)
			options = utils.RemoveFromSlice(options, confirmOption)
		}
		option, err := app.Prompt.CaptureList(prompt, options)
		if err != nil {
			return ConfigEsp{}, false, err
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
					return ConfigEsp{}, false, err
				}
				switch roleOption {
				case addBothOption:
					configEsp, err = addBoth(network, configEsp)
					if err != nil {
						return ConfigEsp{}, false, err
					}
				case addSourceOption:
					configEsp, err = addSource(network, configEsp, contract.ChainSpec{}, "", "")
					if err != nil {
						return ConfigEsp{}, false, err
					}
				case addDestinationOption:
					configEsp, err = addDestination(network, configEsp, contract.ChainSpec{}, "")
					if err != nil {
						return ConfigEsp{}, false, err
					}
				case explainOption:
					ux.Logger.PrintToUser("A source blockchain is going to be listened by the relayer to check for new")
					ux.Logger.PrintToUser("messages. You need to specify blockchain ID, teleporter addresses.")
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
				if len(configEsp.sources) != 0 {
					options = append(options, removeSourceOption)
				}
				if len(configEsp.destinations) != 0 {
					options = append(options, removeDestinationOption)
				}
				options = append(options, cancelOption)
				kindOption, err := app.Prompt.CaptureList(removePrompt, options)
				if err != nil {
					return ConfigEsp{}, false, err
				}
				switch kindOption {
				case removeSourceOption:
					configEsp, keepAsking, err = removeSource(configEsp)
					if err != nil {
						return ConfigEsp{}, false, err
					}
				case removeDestinationOption:
					configEsp, keepAsking, err = removeDestination(configEsp)
					if err != nil {
						return ConfigEsp{}, false, err
					}
				case cancelOption:
					keepAsking = false
				}
			}
		case previewOption:
			preview(configEsp)
		case confirmOption:
			preview(configEsp)
			confirmPrompt := "Confirm?"
			yesOption := "Yes"
			noOption := "No, keep editing"
			confirmOption, err := app.Prompt.CaptureList(
				confirmPrompt, []string{yesOption, noOption},
			)
			if err != nil {
				return ConfigEsp{}, false, err
			}
			if confirmOption == yesOption {
				return configEsp, false, nil
			}
		case cancelOption:
			return ConfigEsp{}, true, err
		}
	}
}
