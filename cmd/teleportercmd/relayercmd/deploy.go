// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package relayercmd

import (
	"fmt"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/spf13/cobra"
)

type DeployFlags struct {
	Network                      networkoptions.NetworkFlags
	SubnetName                   string
	BlockchainID                 string
	CChain                       bool
	KeyName                      string
	GenesisKey                   bool
	DeployMessenger              bool
	DeployRegistry               bool
	RPCURL                       string
	Version                      string
	MessengerContractAddressPath string
	MessengerDeployerAddressPath string
	MessengerDeployerTxPath      string
	RegistryBydecodePath         string
	PrivateKeyFlags              contract.PrivateKeyFlags
}

var (
	deploySupportedNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Devnet,
		networkoptions.Fuji,
	}
	deployFlags DeployFlags
)

// avalanche teleporter relayer deploy
func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys an ICM Relayer for the given Network",
		Long:  `Deploys an ICM Relayer for the given Network.`,
		RunE:  deploy,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &deployFlags.Network, true, deploySupportedNetworkOptions)
	cmd.Flags().StringVar(&deployFlags.Version, "version", "latest", "version to deploy")
	// flag for binary to use
	// flag for local process vs cloud
	// flag for provided config file (may need to change tmp dir)
	return cmd
}

func deploy(_ *cobra.Command, args []string) error {
	return CallDeploy(args, deployFlags)
}

func CallDeploy(_ []string, flags DeployFlags) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"In which Network will operate the Relayer?",
		flags.Network,
		true,
		false,
		deploySupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	deployToRemote := false
	if network.Kind != models.Local {
		prompt := "Do you want to deploy the relayer to a remote or a local host?"
		remoteHostOption := "I want to deploy the relayer into a remote node in the cloud"
		localHostOption := "I prefer to deploy into a localhost process"
		options := []string{remoteHostOption, localHostOption, explainOption}
		for {
			option, err := app.Prompt.CaptureList(
				prompt,
				options,
			)
			if err != nil {
				return err
			}
			switch option {
			case remoteHostOption:
				deployToRemote = true
			case localHostOption:
			case explainOption:
				ux.Logger.PrintToUser("A local host relayer is for temporary networks, won't survive a host restart")
				ux.Logger.PrintToUser("or a relayer transient failure (but anyway can be manually restarted by cmd)")
				ux.Logger.PrintToUser("A remote relayer is deployed into a new cloud node, and will recover from")
				ux.Logger.PrintToUser("temporary relayer failures and from host restarts.")
				continue
			}
			break
		}
	}

	// TODO: put in prompts
	prompt := "Which log level do you prefer for your relayer?"
	options := []string{
		logging.Info.String(),
		logging.Warn.String(),
		logging.Error.String(),
		logging.Off.String(),
		logging.Fatal.String(),
		logging.Debug.String(),
		logging.Trace.String(),
		logging.Verbo.String(),
	}
	logLevel, err := app.Prompt.CaptureList(
		prompt,
		options,
	)
	if err != nil {
		return err
	}

	networkUP := true
	_, err = utils.GetChainID(network.Endpoint, "C")
	if err != nil {
		if !strings.Contains(err.Error(), "connection refused") {
			return err
		}
		networkUP = false
	}

	configureBlockchains := false
	if networkUP {
		prompt = "Do you want to add blockchain information to your relayer?"
		yesOption := "Yes, I want to configure source and destination blockchains"
		noOption := "No, I prefer to configure the relayer later on"
		options = []string{yesOption, noOption, explainOption}
		for {
			option, err := app.Prompt.CaptureList(
				prompt,
				options,
			)
			if err != nil {
				return err
			}
			switch option {
			case yesOption:
				configureBlockchains = true
			case noOption:
			case explainOption:
				ux.Logger.PrintToUser("You can configure a list of source and destination blockchains, so that the")
				ux.Logger.PrintToUser("relayer will listen for new messages on each source, and deliver them to the")
				ux.Logger.PrintToUser("destinations.")
				ux.Logger.PrintToUser("Or you can not configure those later on, by using the 'relayer config' cmd.")
				continue
			}
			break
		}
	}

	var configEsp ConfigEsp
	if configureBlockchains {
		// TODO: this is the base for a 'relayer config' cmd
		// that should load the current config, generate a configEsp for that,
		// and use this to change the config, before saving it
		// most probably, also, relayer config should restart the relayer
		var cancel bool
		configEsp, cancel, err = GenerateConfigEsp(network)
		if cancel {
			return nil
		}
		if err != nil {
			return err
		}
	}

	fundBlockchains := false
	if networkUP && len(configEsp.destinations) > 0 {
		// TODO: this (and the next section) are the base for a 'relayer fund' cmd
		// it must be based on relayer conf, and try to gather a nice blockchain desc
		// from the blockchain id (as relayer logs cmd)
		ux.Logger.PrintToUser("")
		for _, destination := range configEsp.destinations {
			pk, err := crypto.HexToECDSA(destination.privateKey)
			if err != nil {
				return err
			}
			addr := crypto.PubkeyToAddress(pk.PublicKey)
			client, err := evm.GetClient(network.BlockchainEndpoint(destination.blockchainID))
			if err != nil {
				return err
			}
			balance, err := evm.GetAddressBalance(client, addr.Hex())
			if err != nil {
				return err
			}
			ux.Logger.PrintToUser("Relayer private key on destination %s has a balance of %s", destination.blockchainDesc, balance)
		}
		ux.Logger.PrintToUser("")

		prompt = "Do you want to fund relayer destinations?"
		yesOption := "Yes, I want to fund destination blockchains"
		noOption := "No, I prefer to fund the relayer later on"
		options = []string{yesOption, noOption, explainOption}
		for {
			option, err := app.Prompt.CaptureList(
				prompt,
				options,
			)
			if err != nil {
				return err
			}
			switch option {
			case yesOption:
				fundBlockchains = true
			case noOption:
			case explainOption:
				ux.Logger.PrintToUser("You need to set some balance on the destination addresses")
				ux.Logger.PrintToUser("so the relayer can pay for fees when delivering messages.")
				continue
			}
			break
		}
	}

	if fundBlockchains {
		for _, destination := range configEsp.destinations {
			pk, err := crypto.HexToECDSA(destination.privateKey)
			if err != nil {
				return err
			}
			addr := crypto.PubkeyToAddress(pk.PublicKey)
			client, err := evm.GetClient(network.BlockchainEndpoint(destination.blockchainID))
			if err != nil {
				return err
			}
			balance, err := evm.GetAddressBalance(client, addr.Hex())
			if err != nil {
				return err
			}
			prompt = fmt.Sprintf("Do you want to fund relayer for destination %s (balance=%s)?", destination.blockchainDesc, balance)
			yesOption := "Yes, I will send funds to it"
			noOption := "Not yet"
			options = []string{yesOption, noOption}
			doPay := false
			option, err := app.Prompt.CaptureList(
				prompt,
				options,
			)
			if err != nil {
				return err
			}
			switch option {
			case yesOption:
				doPay = true
			case noOption:
			}
			if doPay {
				genesisAddress, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
					app,
					network,
					contract.ChainSpec{
						BlockchainID: destination.blockchainID,
					},
				)
				if err != nil {
					return err
				}
				privateKey, err := prompts.PromptPrivateKey(
					app.Prompt,
					fmt.Sprintf("fund the relayer destination %s", destination.blockchainDesc),
					app.GetKeyDir(),
					app.GetKey,
					genesisAddress,
					genesisPrivateKey,
				)
				if err != nil {
					return err
				}
				amount, err := app.Prompt.CapturePositiveBigInt("Amount to transfer")
				if err != nil {
					return err
				}
				if err := evm.FundAddress(client, privateKey, addr.Hex(), amount); err != nil {
					return err
				}
			}
		}
	}

	if !deployToRemote {
		// download if needed. copy if needed.
		// save version of filename in run file or whichever
	} else {
		// ask for relayer name
		// create cluster
		// set conf
	}

	_ = configEsp
	_ = logLevel

	return nil
}
