// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"fmt"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"os"

	avmtx "github.com/ava-labs/avalanchego/vms/avm/txs"
	"github.com/ava-labs/avalanchego/vms/components/verify"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"

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
	fujiDeployment    = "Fuji"
	mainnetDeployment = "Mainnet (coming soon)"
)

var (
	transformLocal   bool
	tokenNameFlag    string
	denominationFlag int
	tokenSymbolFlag  string
	useDefaultConfig bool
	overrideWarning  bool
	recipientKeys    []string
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
	cmd.Flags().IntVar(&denominationFlag, "denomination", 0, "specify the token denomination")
	cmd.Flags().BoolVar(&useDefaultConfig, "default", false, "use default elastic subnet config values")
	cmd.Flags().BoolVar(&overrideWarning, "force", false, "override transform into elastic subnet warning")
	cmd.Flags().StringSliceVar(&recipientKeys, "recipient-keys", nil, "addresses that will receive tokens when subnet is transformed")

	return cmd
}

func checkIfSubnetIsElasticOnLocal(sc models.Sidecar) bool {
	if _, ok := sc.ElasticSubnet[models.Local.String()]; ok {
		return true
	}
	return false
}

func createAssetID(deployer *subnet.PublicDeployer,
	subnetAuthAddrStr []string,
	maxSupply uint64,
	subnetID ids.ID,
	tokenName string,
	tokenSymbol string,
	tokenDenomination byte,
	recipientAddr ids.ShortID,
) (bool, ids.ID, error) {
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs: []ids.ShortID{
			recipientAddr,
		},
	}
	fmt.Printf("obtained public key address create asset %s \n", recipientAddr)
	initialState := map[uint32][]verify.State{
		0: {
			&secp256k1fx.TransferOutput{
				Amt:          maxSupply,
				OutputOwners: *owner,
			},
		},
	}

	isFullySigned, txID, err := deployer.CreateAssetTx(subnetAuthAddrStr, subnetID, tokenName, tokenSymbol, tokenDenomination, initialState)
	if err != nil {
		return false, ids.Empty, err
	}
	//fmt.Printf("obtained txID %s \n", tx.ID().String())
	//if !isFullySigned {
	//	if err := SaveNotFullySignedAVMTx(
	//		"Add Validator",
	//		tx,
	//		network,
	//		subnetName,
	//		subnetID,
	//		subnetAuthKeys,
	//		outputTxPath,
	//		false,
	//	); err != nil {
	//		return err
	//	}
	//}
	return isFullySigned, txID, nil
}
func exportToPChain(deployer *subnet.PublicDeployer,
	subnetAuthKeysStrs []string,
	subnetID ids.ID,
	subnetAssetID ids.ID,
	recipientAddr ids.ShortID,
	maxSupply uint64) (bool, *avmtx.Tx, error) {
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs: []ids.ShortID{
			recipientAddr,
		},
	}
	isFullySigned, tx, err := deployer.ExportToPChainTx(subnetAuthKeysStrs, subnetID, subnetAssetID, owner, maxSupply)
	if err != nil {
		return false, nil, err
	}
	return isFullySigned, tx, nil
}
func importFromXChain(deployer *subnet.PublicDeployer,
	subnetAuthKeysStrs []string,
	subnetID ids.ID,
	recipientAddr ids.ShortID) (bool, *txs.Tx, error) {
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs: []ids.ShortID{
			recipientAddr,
		},
	}
	isFullySigned, tx, err := deployer.ImportFromXChain(subnetAuthKeysStrs, subnetID, owner)
	if err != nil {
		return false, nil, err
	}
	return isFullySigned, tx, nil
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
		networkToUpgrade, err := selectNetworkToTransform(sc)
		if err != nil {
			return err
		}
		if networkToUpgrade == localDeployment {
			network = models.Local
		} else if networkToUpgrade == fujiDeployment {
			network = models.Fuji
		} else {
			return errors.New("elastic subnet transformation is not yet supported on Mainnet")
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

	//tokenDenomination := 0
	//if network != models.Local {
	//	if denominationFlag == 0 {
	//		tokenDenomination, err = getTokenDenomination()
	//		if err != nil {
	//			return err
	//		}
	//	} else {
	//		tokenDenomination = denominationFlag
	//	}
	//}

	subnetID := sc.Networks[network.String()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	elasticSubnetConfig, err := es.GetElasticSubnetConfig(app, tokenSymbol, useDefaultConfig)
	if err != nil {
		return err
	}
	elasticSubnetConfig.SubnetID = subnetID

	switch network {
	case models.Local:
		return transformElasticSubnetLocal(sc, subnetName, tokenName, tokenSymbol, elasticSubnetConfig)
	case models.Fuji:
		if !useLedger && keyName == "" {
			useLedger, keyName, err = prompts.GetFujiKeyOrLedger(app.Prompt, app.GetKeyDir())
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

	// get keychain accessor
	kc, err := GetKeychain(useLedger, ledgerAddresses, keyName, network)
	if err != nil {
		return err
	}
	//fmt.Printf("obtained kcPrivaddress %s \n", kcPrivAddr.String())
	//// accept only one control keys specification
	//if len(controlKeys) > 0 && sameControlKey {
	//	return errMutuallyExlusiveControlKeys
	//}
	//
	//// use creation key as control key
	//if sameControlKey {
	//	controlKeys, err = loadCreationKeys(network, kc)
	//	if err != nil {
	//		return err
	//	}
	//}
	//
	//// prompt for control keys
	//if controlKeys == nil {
	//	var cancelled bool
	//	controlKeys, cancelled, err = getControlKeys(network, useLedger, kc)
	//	if err != nil {
	//		return err
	//	}
	//	if cancelled {
	//		ux.Logger.PrintToUser("User cancelled. No subnet deployed")
	//		return nil
	//	}
	//}

	//controlKeys, threshold, err := subnet.GetOwners(network, subnetID)
	//if err != nil {
	//	return err
	//}
	//
	//// get keys for add validator tx signing
	//if subnetAuthKeys != nil {
	//	if err := prompts.CheckSubnetAuthKeys(subnetAuthKeys, controlKeys, threshold); err != nil {
	//		return err
	//	}
	//} else {
	//	subnetAuthKeys, err = prompts.GetSubnetAuthKeys(app.Prompt, controlKeys, threshold)
	//	if err != nil {
	//		return err
	//	}
	//}
	//ux.Logger.PrintToUser("Your subnet auth keys for transform elastic subnet tx creation: %s", subnetAuthKeys)
	//
	//kc, err := GetKeychain(useLedger, ledgerAddresses, keyName, network)
	//if err != nil {
	//	return err
	//}
	var subnetAuthAddrs []string
	var subnetAuthAddrsPubKey []ids.ShortID

	recipientAddr := kc.Addresses().List()
	fmt.Printf("obtained subnetAuthAddrs list %s \n", recipientAddr)

	for _, addr := range recipientAddr {
		subnetAuthAddrs = append(subnetAuthAddrs, addr.String())
		subnetAuthAddrsPubKey = append(subnetAuthAddrsPubKey, addr)
	}

	subnetAuthKeys, err := address.ParseToIDs([]string{"P-fuji1r5mkuktrr4l9hncga4qnukn2jx38llzy8mdxs9", "P-fuji165tam4age2lyznusd8zu08fell4u6scuqp0svz", "P-fuji15t53knd9l003f8q4ap0x2wrs3uwaeqa5yg00ap"})
	fmt.Printf("obtained subnetAuthAddrs list %s \n", subnetAuthKeys)

	// prompt for control keys
	if recipientKeys == nil {
		var cancelled bool
		recipientKeys, cancelled, err = getRecipientAddr(network, useLedger, kc, keyName)
		if err != nil {
			return err
		}
		if cancelled {
			ux.Logger.PrintToUser("User cancelled. Subnet is not transformed into elastic subnet")
			return nil
		}
	}

	if threshold == 0 {
		threshold, err = getThreshold(len(recipientKeys))
		if err != nil {
			return err
		}
	}

	recipientAddrStr, err := prompts.GetSubnetAuthKeys(app.Prompt, recipientKeys, threshold)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("Your subnet auth keys for chain creation: %s", subnetAuthKeys)

	fmt.Printf("obtained recipientAddrStr %s \n ", recipientAddrStr)
	//deployer := subnet.NewPublicDeployer(app, useLedger, kc, network)
	//isFullySigned, assetID, err := createAssetID(deployer, subnetAuthAddrs, elasticSubnetConfig.MaxSupply, subnetID, tokenName, tokenSymbol, byte(tokenDenomination), subnetAuthAddrsPubKey[0])
	//if err != nil {
	//	return err
	//}
	//if !isFullySigned {
	//	return errors.New("not fully signed createAssetTx")
	//}
	//fmt.Printf("obtained assetID %s \n", assetID)
	////assetID, err := ids.FromString("ZAP9junNhhkCZri5i4PU5k2vSinL8XzVvXjbKbwDfSocMRjp7")
	////if err != nil {
	////	return err
	////}
	//isFullySigned, _, err = exportToPChain(deployer, subnetAuthAddrs, subnetID, assetID, subnetAuthAddrsPubKey[0], elasticSubnetConfig.MaxSupply)
	//if err != nil {
	//	return err
	//}
	//if !isFullySigned {
	//	return errors.New("not fully signed exportToPChain")
	//}
	//
	//isFullySigned, _, err = importFromXChain(deployer, subnetAuthAddrs, subnetID, subnetAuthAddrsPubKey[0])
	//if err != nil {
	//	return err
	//}
	//if !isFullySigned {
	//	return errors.New("not fully signed importFromXChain")
	//}
	//
	//isFullySigned, _, err = deployer.TransformSubnetTx(subnetAuthAddrs, elasticSubnetConfig, subnetID, assetID)
	//if err != nil {
	//	return errors.New("not fully signed TransformSubnetTx")
	//}
	//if !isFullySigned {
	//	return errors.New("not fully signed TransformSubnetTx")
	//}
	//fmt.Printf("obtainedTxID is %s", txID.ID().String())

	//isFullySigned, txID, err := createAssetID(deployer, elasticSubnetConfig.MaxSupply, subnetID, tokenName, tokenSymbol, byte(tokenDenomination))
	//if err != nil {
	//	return err
	//}
	//if !isFullySigned {
	//	return errors.New("not fully signed createAssetTx")
	//}
	return nil
}

func getRecipientAddr(network models.Network, useLedger bool, kc keychain.Keychain, keyName string) ([]string, bool, error) {
	recipientAddrPrompt := "Configure which addresses you want to receive the created assets on the elastic subnet."
	moreKeysPrompt := "How would you like to set your recipient address(es)?"

	ux.Logger.PrintToUser(recipientAddrPrompt)
	useAll := "Choose 1 or more keys from locally stored keys"
	var creation string
	var listOptions []string
	if useLedger {
		creation = "Use ledger address"
	} else {
		creation = fmt.Sprintf("Use %s", keyName)
	}
	if network == models.Mainnet {
		listOptions = []string{creation}
	} else {
		listOptions = []string{creation, useAll}
	}

	listDecision, err := app.Prompt.CaptureList(moreKeysPrompt, listOptions)
	if err != nil {
		return nil, false, err
	}

	var (
		keys      []string
		cancelled bool
	)

	switch listDecision {
	case creation:
		keys, err = loadCreationKeys(network, kc)
	case useAll:
		keys, err = useAllKeys(network)
	}
	if err != nil {
		return nil, false, err
	}
	if cancelled {
		return nil, true, nil
	}
	return keys, false, nil
}

func transformElasticSubnetLocal(sc models.Sidecar, subnetName string, tokenName string, tokenSymbol string, elasticSubnetConfig models.ElasticSubnetConfig) error {
	if checkIfSubnetIsElasticOnLocal(sc) {
		return fmt.Errorf("%s is already an elastic subnet", subnetName)
	}
	var err error
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

	ux.Logger.PrintToUser("Starting Elastic Subnet Transformation")
	cancel := make(chan struct{})
	defer close(cancel)
	go ux.PrintWait(cancel)
	testKey := genesis.EWOQKey
	keyChain := secp256k1fx.NewKeychain(testKey)
	txID, assetID, err := subnet.IssueTransformSubnetTx(elasticSubnetConfig, keyChain, subnetID, tokenName, tokenSymbol, elasticSubnetConfig.MaxSupply)
	if err != nil {
		return err
	}
	elasticSubnetConfig.AssetID = assetID
	PrintTransformResults(subnetName, txID, subnetID, tokenName, tokenSymbol, assetID)
	if err = app.CreateElasticSubnetConfig(subnetName, &elasticSubnetConfig); err != nil {
		return err
	}
	if err = app.UpdateSidecarElasticSubnet(&sc, models.Local, subnetID, assetID, txID, tokenName, tokenSymbol); err != nil {
		return fmt.Errorf("elastic subnet transformation was successful, but failed to update sidecar: %w", err)
	}
	return nil
}

// select which network to transform to elastic subnet
func selectNetworkToTransform(sc models.Sidecar) (string, error) {
	var networkOptions []string
	networkPrompt := "Which network should transform into an elastic Subnet?"
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

	selectedDeployment, err := app.Prompt.CaptureList(networkPrompt, networkOptions)
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

func getTokenDenomination() (int, error) {
	ux.Logger.PrintToUser("What's the denomination for your token?")
	tokenDenomination, err := app.Prompt.CaptureUint64Compare(
		"Token Denomination",
		[]prompts.Comparator{
			{
				Label: "Min Denomination Value",
				Type:  prompts.MoreThanEq,
				Value: 0,
			},
		},
	)
	if err != nil {
		return 0, err
	}
	return int(tokenDenomination), nil
}

//
//func SaveNotFullySignedAVMTx(
//	txName string,
//	tx *avmtx.Tx,
//	network models.Network,
//	chain string,
//	subnetID ids.ID,
//	subnetAuthKeys []string,
//	outputTxPath string,
//	forceOverwrite bool,
//) error {
//	remainingSubnetAuthKeys, err := txutils.GetRemainingSigners(tx, network, subnetID)
//	if err != nil {
//		return err
//	}
//	signedCount := len(subnetAuthKeys) - len(remainingSubnetAuthKeys)
//	ux.Logger.PrintToUser("")
//	if signedCount == len(subnetAuthKeys) {
//		ux.Logger.PrintToUser("All %d required %s signatures have been signed. "+
//			"Saving tx to disk to enable commit.", len(subnetAuthKeys), txName)
//	} else {
//		ux.Logger.PrintToUser("%d of %d required %s signatures have been signed. "+
//			"Saving tx to disk to enable remaining signing.", signedCount, len(subnetAuthKeys), txName)
//	}
//	if outputTxPath == "" {
//		ux.Logger.PrintToUser("")
//		var err error
//		if forceOverwrite {
//			outputTxPath, err = app.Prompt.CaptureString("Path to export partially signed tx to")
//		} else {
//			outputTxPath, err = app.Prompt.CaptureNewFilepath("Path to export partially signed tx to")
//		}
//		if err != nil {
//			return err
//		}
//	}
//	if forceOverwrite {
//		ux.Logger.PrintToUser("")
//		ux.Logger.PrintToUser("Overwritting %s", outputTxPath)
//	}
//	if err := txutils.SaveToDisk(tx, outputTxPath, forceOverwrite); err != nil {
//		return err
//	}
//	if signedCount == len(subnetAuthKeys) {
//		PrintReadyToSignMsg(chain, outputTxPath)
//	} else {
//		PrintRemainingToSignMsg(chain, remainingSubnetAuthKeys, outputTxPath)
//	}
//	return nil
//}
