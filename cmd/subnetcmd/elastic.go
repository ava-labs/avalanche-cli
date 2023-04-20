// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/ava-labs/avalanchego/genesis"

	es "github.com/ava-labs/avalanche-cli/pkg/elasticsubnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	subnet "github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const (
	localDeployment   = "Existing local deployment"
	fujiDeployment    = "Fuji (coming soon)"
	mainnetDeployment = "Mainnet (coming soon)"
)

var (
	transformLocal      bool
	tokenNameFlag       string
	tokenSymbolFlag     string
	useDefaultConfig    bool
	overrideWarning     bool
	transformValidators bool
)

// avalanche subnet elastic
func newElasticCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "elastic [subnetName]",
		Short: "Transforms a subnet into elastic subnet",
		Long: `The elastic command enables anyone to be a validator of a Subnet by simply staking its token on the 
P-Chain. When enabling Elastic Validation, the creator permanently locks the Subnet from future modification 
(they relinquish their control keys), specifies an Avalanche Native Token (ANT) that validators must use for staking 
and that will be distributed as staking rewards, and provides a set of parameters that govern how the Subnet’s staking 
mechanics will work.`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         transformElasticSubnet,
	}
	cmd.Flags().BoolVarP(&transformLocal, "local", "l", false, "transform a subnet on a local network")
	cmd.Flags().StringVar(&tokenNameFlag, "tokenName", "", "specify the token name")
	cmd.Flags().StringVar(&tokenSymbolFlag, "tokenSymbol", "", "specify the token symbol")
	cmd.Flags().BoolVar(&useDefaultConfig, "default", false, "use default elastic subnet config values")
	cmd.Flags().BoolVar(&overrideWarning, "force", false, "override transform into elastic subnet warning")
	cmd.Flags().Uint64Var(&stakeAmount, "stake-amount", 0, "amount of tokens to stake on validator")
	cmd.Flags().StringVar(&startTimeStr, "start-time", "", "start time that validator starts validating")
	cmd.Flags().DurationVar(&duration, "staking-period", 0, "how long validator validates for after start time")
	cmd.Flags().BoolVar(&transformValidators, "transform-validators", false, "transform validators to permissionless validators")
	return cmd
}

func checkIfSubnetIsElasticOnLocal(sc models.Sidecar) bool {
	if _, ok := sc.ElasticSubnet[models.Local.String()]; ok {
		return true
	}
	return false
}

func transformElasticSubnet(_ *cobra.Command, args []string) error {
	subnetName := args[0]

	if !app.SubnetConfigExists(subnetName) {
		return errors.New("subnet does not exist")
	}

	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return fmt.Errorf("unable to load sidecar: %w", err)
	}

	var network models.Network
	if transformLocal {
		network = models.Local
	}

	if network == models.Undefined {
		networkToUpgrade, err := promptNetworkElastic(sc, "Which network should transform into an elastic Subnet?")
		if err != nil {
			return err
		}
		switch networkToUpgrade {
		case fujiDeployment:
			return errors.New("elastic subnet transformation is not yet supported on Fuji network")
		case mainnetDeployment:
			return errors.New("elastic subnet transformation is not yet supported on Mainnet")
		}
	}

	if checkIfSubnetIsElasticOnLocal(sc) {
		return fmt.Errorf("%s is already an elastic subnet", subnetName)
	}

	subnetID := sc.Networks[models.Local.String()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	if !overrideWarning {
		yes, err := app.Prompt.CaptureNoYes("WARNING: Transforming a Permissioned Subnet into an Elastic Subnet is an irreversible operation. Continue?")
		if err != nil {
			return err
		}
		if !yes {
			return nil
		}
	}

	tokenName := ""
	if tokenNameFlag == "" {
		tokenName, err = getTokenName()
		if err != nil {
			return err
		}
	} else {
		tokenName = tokenNameFlag
	}

	tokenSymbol := ""
	if tokenSymbolFlag == "" {
		tokenSymbol, err = getTokenSymbol()
		if err != nil {
			return err
		}
	} else {
		tokenSymbol = tokenSymbolFlag
	}
	elasticSubnetConfig, err := es.GetElasticSubnetConfig(app, tokenSymbol, useDefaultConfig)
	if err != nil {
		return err
	}
	elasticSubnetConfig.SubnetID = subnetID
	ux.Logger.PrintToUser("Starting Elastic Subnet Transformation")
	cancel := make(chan struct{})
	go ux.PrintWait(cancel)
	testKey := genesis.EWOQKey
	keyChain := secp256k1fx.NewKeychain(testKey)
	txID, assetID, err := subnet.IssueTransformSubnetTx(elasticSubnetConfig, keyChain, subnetID, tokenName, tokenSymbol, elasticSubnetConfig.MaxSupply)
	close(cancel)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Subnet Successfully Transformed To Elastic Subnet!")

	elasticSubnetConfig.AssetID = assetID
	if err = app.CreateElasticSubnetConfig(subnetName, &elasticSubnetConfig); err != nil {
		return err
	}
	if err = app.UpdateSidecarElasticSubnet(&sc, models.Local, subnetID, assetID, txID, tokenName, tokenSymbol); err != nil {
		return fmt.Errorf("elastic subnet transformation was successful, but failed to update sidecar: %w", err)
	}
	if !transformValidators {
		yes, err := app.Prompt.CaptureNoYes("Do you want to transform existing validators to permissionless validators with equal weight? " +
			"Press <No> if you want to customize the structure of your permissionless validators")
		if err != nil {
			return err
		}
		if !yes {
			return nil
		}
		ux.Logger.PrintToUser("Transforming validators to permissionless validators")
		if err = transformValidatorsToPermissionlessLocal(sc, subnetID, subnetName); err != nil {
			return err
		}
	} else {
		ux.Logger.PrintToUser("Transforming validators to permissionless validators")
		if err = transformValidatorsToPermissionlessLocal(sc, subnetID, subnetName); err != nil {
			return err
		}
	}
	PrintTransformResults(subnetName, txID, subnetID, tokenName, tokenSymbol, assetID)
	return nil
}

// select which network to transform to elastic subnet
func promptNetworkElastic(sc models.Sidecar, prompt string) (string, error) {
	var networkOptions []string
	for network := range sc.Networks {
		switch network {
		case models.Local.String():
			networkOptions = append(networkOptions, localDeployment)
		case models.Fuji.String():
			networkOptions = append(networkOptions, fujiDeployment)
		case models.Mainnet.String():
			networkOptions = append(networkOptions, mainnetDeployment)
		}
	}

	if len(networkOptions) == 0 {
		return "", errors.New("no deployment target available, please first deploy created subnet")
	}

	selectedDeployment, err := app.Prompt.CaptureList(prompt, networkOptions)
	if err != nil {
		return "", err
	}
	return selectedDeployment, nil
}

func PrintTransformResults(chain string, txID ids.ID, subnetID ids.ID, tokenName string, tokenSymbol string, assetID ids.ID) {
	const art = "\n  ______ _           _   _         _____       _                _     _______                   __                        _____                              __       _ " +
		"\n |  ____| |         | | (_)       / ____|     | |              | |   |__   __|                 / _|                      / ____|                            / _|     | |" +
		"\n | |__  | | __ _ ___| |_ _  ___  | (___  _   _| |__  _ __   ___| |_     | |_ __ __ _ _ __  ___| |_ ___  _ __ _ __ ___   | (___  _   _  ___ ___ ___  ___ ___| |_ _   _| |" +
		"\n |  __| | |/ _` / __| __| |/ __|  \\___ \\| | | | '_ \\| '_ \\ / _ \\ __|    | | '__/ _` | '_ \\/ __|  _/ _ \\| '__| '_ ` _ \\   \\___ \\| | | |/ __/ __/ _ \\/ __/ __|  _| | | | |" +
		"\n | |____| | (_| \\__ \\ |_| | (__   ____) | |_| | |_) | | | |  __/ |_     | | | | (_| | | | \\__ \\ || (_) | |  | | | | | |  ____) | |_| | (_| (_|  __/\\__ \\__ \\ | | |_| | |" +
		"\n |______|_|\\__,_|___/\\__|_|\\___| |_____/ \\__,_|_.__/|_| |_|\\___|\\__|    |_|_|  \\__,_|_| |_|___/_| \\___/|_|  |_| |_| |_| |_____/ \\__,_|\\___\\___\\___||___/___/_|  \\__,_|_|" +
		"\n"
	fmt.Print(art)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetRowLine(true)
	table.SetAutoMergeCells(true)
	table.Append([]string{"Token Name", tokenName})
	table.Append([]string{"Token Symbol", tokenSymbol})
	table.Append([]string{"Asset ID", assetID.String()})
	table.Append([]string{"Chain Name", chain})
	table.Append([]string{"Subnet ID", subnetID.String()})
	table.Append([]string{"P-Chain TXID", txID.String()})
	table.Render()
}

func getTokenName() (string, error) {
	ux.Logger.PrintToUser("Select a name for your subnet's native token")
	tokenName, err := app.Prompt.CaptureString("Token name")
	if err != nil {
		return "", err
	}
	return tokenName, nil
}

func getTokenSymbol() (string, error) {
	ux.Logger.PrintToUser("Select a symbol for your subnet's native token")
	tokenSymbol, err := app.Prompt.CaptureString("Token symbol")
	if err != nil {
		return "", err
	}
	return tokenSymbol, nil
}

func transformValidatorsToPermissionlessLocal(sc models.Sidecar, subnetID ids.ID, subnetName string) error {
	stakedTokenAmount, err := promptStakeAmount(subnetName)
	if err != nil {
		return err
	}

	validators, err := subnet.GetSubnetValidators(subnetID)
	if err != nil {
		return err
	}

	validatorList := make([]ids.NodeID, len(validators))
	for i, v := range validators {
		validatorList[i] = v.NodeID
	}

	start, stakeDuration, err := getTimeParameters(models.Local, validatorList[0])
	if err != nil {
		return err
	}
	endTime := start.Add(stakeDuration)

	for _, validator := range validatorList {
		testKey := genesis.EWOQKey
		keyChain := secp256k1fx.NewKeychain(testKey)
		_, err = subnet.IssueRemoveSubnetValidatorTx(keyChain, subnetID, validator)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser(fmt.Sprintf("Validator %s removed", validator.String()))
		assetID := sc.ElasticSubnet[models.Local.String()].AssetID
		subnetID := sc.Networks[models.Local.String()].SubnetID
		txID, err := subnet.IssueAddPermissionlessValidatorTx(keyChain, subnetID, validator, stakedTokenAmount, assetID, uint64(start.Unix()), uint64(endTime.Unix()))
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser(fmt.Sprintf("%s successfully joined elastic subnet as permissionless validator!", validator.String()))
		if err = app.UpdateSidecarPermissionlessValidator(&sc, models.Local, validator.String(), txID); err != nil {
			return fmt.Errorf("joining permissionless subnet was successful, but failed to update sidecar: %w", err)
		}
	}
	return nil
}
