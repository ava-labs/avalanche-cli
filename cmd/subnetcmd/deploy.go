// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	ledger "github.com/ava-labs/avalanche-ledger-go"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/keychain"
	"github.com/ava-labs/coreth/core"
	spacesvmchain "github.com/ava-labs/spacesvm/chain"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const numLedgerAddressesToDerive = 1

var (
	deployLocal    bool
	deployTestnet  bool
	deployMainnet  bool
	useLedger      bool
	sameControlKey bool
	keyName        string
	threshold      uint32
	controlKeys    []string
	subnetAuthKeys []string
	avagoVersion   string
	outputTxPath   string

	errMutuallyExlusive = errors.New("--local, --fuji (resp. --testnet) and --mainnet are mutually exclusive")
)

// avalanche subnet deploy
func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [subnetName]",
		Short: "Deploys a subnet configuration",
		Long: `The subnet deploy command deploys your subnet configuration locally, to
Fuji Testnet, or to Mainnet. Currently, the beta release only supports
local and Fuji deploys.

At the end of the call, the command will print the RPC URL you can use
to interact with the subnet.

Subnets may only be deployed once. Subsequent calls of deploy to the
same network (local, Fuji, Mainnet) are not allowed. If you'd like to
redeploy a subnet locally for testing, you must first call avalanche
network clean to reset all deployed chain state. Subsequent local
deploys will redeploy the chain with fresh state. The same subnet can
be deployed to multiple networks, so you can take your locally tested
subnet and deploy it on Fuji or Mainnet.`,
		SilenceUsage: true,
		RunE:         deploySubnet,
		Args:         cobra.ExactArgs(1),
	}
	cmd.Flags().BoolVarP(&deployLocal, "local", "l", false, "deploy to a local network")
	cmd.Flags().BoolVarP(&deployTestnet, "testnet", "t", false, "deploy to testnet (alias to `fuji`)")
	cmd.Flags().BoolVarP(&deployTestnet, "fuji", "f", false, "deploy to fuji (alias to `testnet`")
	cmd.Flags().BoolVarP(&deployMainnet, "mainnet", "m", false, "deploy to mainnet")
	cmd.Flags().StringVar(&avagoVersion, "avalanchego-version", "latest", "use this version of avalanchego (ex: v1.17.12)")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji)")
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji deploy only]")
	cmd.Flags().BoolVarP(&sameControlKey, "same-control-key", "s", false, "use creation key as control key")
	cmd.Flags().Uint32Var(&threshold, "threshold", 0, "required number of control key signatures to add a validator")
	cmd.Flags().StringSliceVar(&controlKeys, "control-keys", nil, "addresses that may add new validators to the subnet")
	cmd.Flags().StringSliceVar(&subnetAuthKeys, "subnet-auth-keys", nil, "control keys that will be used to authenticate chain creation")
	cmd.Flags().StringVar(&outputTxPath, "output-tx-path", "", "file path of the blockchain creation tx")
	return cmd
}

func getChainsInSubnet(subnetName string) ([]string, error) {
	files, err := os.ReadDir(app.GetBaseDir())
	if err != nil {
		return nil, fmt.Errorf("failed to read baseDir: %w", err)
	}

	chains := []string{}

	for _, f := range files {
		if strings.Contains(f.Name(), constants.SidecarSuffix) {
			// read in sidecar file
			path := filepath.Join(app.GetBaseDir(), f.Name())
			jsonBytes, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed reading file %s: %w", path, err)
			}

			var sc models.Sidecar
			err = json.Unmarshal(jsonBytes, &sc)
			if err != nil {
				return nil, fmt.Errorf("failed unmarshaling file %s: %w", path, err)
			}
			if sc.Subnet == subnetName {
				chains = append(chains, sc.Name)
			}
		}
	}
	return chains, nil
}

// deploySubnet is the cobra command run for deploying subnets
func deploySubnet(cmd *cobra.Command, args []string) error {
	chains, err := validateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}

	chain := chains[0]

	sc, err := app.LoadSidecar(chain)
	if err != nil {
		return fmt.Errorf("failed to load sidecar for later update: %w", err)
	}

	if sc.ImportedFromAPM {
		return errors.New("unable to deploy subnets imported from a repo")
	}

	// get the network to deploy to
	var network models.Network

	if err := checkMutuallyExclusive(deployLocal, deployTestnet, deployMainnet); err != nil {
		return err
	}

	switch {
	case deployLocal:
		network = models.Local
	case deployTestnet:
		network = models.Fuji
	case deployMainnet:
		network = models.Mainnet
	}

	if network == models.Undefined {
		// no flag was set, prompt user
		networkStr, err := app.Prompt.CaptureList(
			"Choose a network to deploy on",
			[]string{models.Local.String(), models.Fuji.String(), models.Mainnet.String()},
		)
		if err != nil {
			return err
		}
		network = models.NetworkFromString(networkStr)
	}

	// deploy based on chosen network
	ux.Logger.PrintToUser("Deploying %s to %s", chains, network.String())
	chainGenesis, err := app.LoadRawGenesis(chain)
	if err != nil {
		return err
	}

	sidecar, err := app.LoadSidecar(chain)
	if err != nil {
		return err
	}

	// validate genesis as far as possible previous to deploy
	switch sidecar.VM {
	case models.SubnetEvm:
		var genesis core.Genesis
		err = json.Unmarshal(chainGenesis, &genesis)
	case models.SpacesVM:
		var genesis spacesvmchain.Genesis
		err = json.Unmarshal(chainGenesis, &genesis)
	default:
		var genesis map[string]interface{}
		err = json.Unmarshal(chainGenesis, &genesis)
	}
	if err != nil {
		return fmt.Errorf("failed to validate genesis format: %w", err)
	}

	genesisPath := app.GetGenesisPath(chain)

	switch network {
	case models.Local:
		app.Log.Debug("Deploy local")

		// copy vm binary to the expected location, first downloading it if necessary
		var vmBin string
		switch sidecar.VM {
		case models.SubnetEvm:
			vmBin, err = binutils.SetupSubnetEVM(app, sidecar.VMVersion)
			if err != nil {
				return fmt.Errorf("failed to install subnet-evm: %w", err)
			}
		case models.SpacesVM:
			vmBin, err = binutils.SetupSpacesVM(app, sidecar.VMVersion)
			if err != nil {
				return fmt.Errorf("failed to install spacesvm: %w", err)
			}
		case models.CustomVM:
			vmBin = binutils.SetupCustomBin(app, chain)
		default:
			return fmt.Errorf("unknown vm: %s", sidecar.VM)
		}

		deployer := subnet.NewLocalDeployer(app, avagoVersion, vmBin)
		subnetID, blockchainID, err := deployer.DeployToLocalNetwork(chain, chainGenesis, genesisPath)
		if err != nil {
			if deployer.BackendStartedHere() {
				if innerErr := binutils.KillgRPCServerProcess(app); innerErr != nil {
					app.Log.Warn("tried to kill the gRPC server process but it failed", zap.Error(innerErr))
				}
			}
			return err
		}
		if sidecar.Networks == nil {
			sidecar.Networks = make(map[string]models.NetworkData)
		}
		sidecar.Networks[models.Local.String()] = models.NetworkData{
			SubnetID:     subnetID,
			BlockchainID: blockchainID,
		}
		if err := app.UpdateSidecar(&sidecar); err != nil {
			return fmt.Errorf("creation of chains and subnet was successful, but failed to update sidecar: %w", err)
		}
		return nil

	case models.Fuji:
		if !useLedger && keyName == "" {
			useLedger, keyName, err = getFujiKeyOrLedger()
			if err != nil {
				return err
			}
		}

	case models.Mainnet:
		useLedger = true

	default:
		return errors.New("not implemented")
	}

	// used in E2E to simulate public network execution paths on a local network
	if os.Getenv(constants.SimulatePublicNetwork) != "" {
		network = models.Local
	}

	// from here on we are assuming a public deploy

	// get keychain accesor
	kc, err := getKeychain(useLedger, keyName, network)
	if err != nil {
		return err
	}

	// use creation key as control key
	if sameControlKey {
		controlKeys, err = loadCreationKeys(network, kc)
		if err != nil {
			return err
		}
	}

	// prompt for control keys
	if controlKeys == nil {
		var cancelled bool
		controlKeys, cancelled, err = getControlKeys(network, useLedger, kc)
		if err != nil {
			return err
		}
		if cancelled {
			ux.Logger.PrintToUser("User cancelled. No subnet deployed")
			return nil
		}
	}

	ux.Logger.PrintToUser("Your Subnet's control keys: %s", controlKeys)

	// validate and prompt for threshold
	if threshold == 0 && subnetAuthKeys != nil {
		threshold = uint32(len(subnetAuthKeys))
	}
	if int(threshold) > len(controlKeys) {
		return fmt.Errorf("given threshold is greater than number of control keys")
	}
	if threshold == 0 {
		if len(controlKeys) == 1 {
			threshold = 1
		} else {
			threshold, err = getThreshold(len(controlKeys))
			if err != nil {
				return err
			}
		}
	}

	// get keys for blockchain tx signing
	if subnetAuthKeys != nil {
		if err := checkSubnetAuthKeys(subnetAuthKeys, controlKeys, threshold); err != nil {
			return err
		}
	}
	if subnetAuthKeys == nil {
		subnetAuthKeys, err = getSubnetAuthKeys(controlKeys, threshold)
	}
	ux.Logger.PrintToUser("Your subnet auth keys for chain creation: %s", subnetAuthKeys)

	// deploy to public network
	deployer := subnet.NewPublicDeployer(app, useLedger, kc, network)
	subnetID, blockchainDeployed, blockchainID, tx, err := deployer.Deploy(controlKeys, subnetAuthKeys, threshold, chain, chainGenesis)
	if err != nil {
		return err
	}

	if !blockchainDeployed {
		if err := saveTxToDisk(tx, outputTxPath); err != nil {
			return err
		}
		if err := printPartialSigningMsg(deployer, subnetAuthKeys, outputTxPath); err != nil {
			return err
		}
		tx2, err := loadTxFromDisk(outputTxPath)
		if err != nil {
			return err
		}
		fmt.Printf("%#v", tx)
		fmt.Printf("%#v", tx2)
	}

	// update sidecar
	// TODO: need to do something for backwards compatibility?
	nets := sidecar.Networks
	if nets == nil {
		nets = make(map[string]models.NetworkData)
	}
	nets[network.String()] = models.NetworkData{
		SubnetID:     subnetID,
		BlockchainID: blockchainID,
	}
	sidecar.Networks = nets
	return app.UpdateSidecar(&sidecar)
}

func getControlKeys(network models.Network, useLedger bool, kc keychain.Keychain) ([]string, bool, error) {
	controlKeysInitialPrompt := "Configure which addresses may add new validators to the subnet.\n" +
		"These addresses are known as your control keys. You will also\n" +
		"set how many control keys are required to add a validator (the threshold)."
	moreKeysPrompt := "How would you like to set your control keys?"

	ux.Logger.PrintToUser(controlKeysInitialPrompt)

	const (
		creation = "Use creation key"
		useAll   = "Use all stored keys"
		custom   = "Custom list"
	)

	listDecision, err := app.Prompt.CaptureList(
		moreKeysPrompt, []string{creation, useAll, custom},
	)
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
	case custom:
		keys, cancelled, err = enterCustomKeys(network)
	}
	if err != nil {
		return nil, false, err
	}
	if cancelled {
		return nil, true, nil
	}
	return keys, false, nil
}

func useAllKeys(network models.Network) ([]string, error) {
	networkID, err := network.NetworkID()
	if err != nil {
		return nil, err
	}

	existing := []string{}

	files, err := os.ReadDir(app.GetKeyDir())
	if err != nil {
		return nil, err
	}

	keyPaths := make([]string, 0, len(files))

	for _, f := range files {
		if strings.HasSuffix(f.Name(), constants.KeySuffix) {
			keyPaths = append(keyPaths, filepath.Join(app.GetKeyDir(), f.Name()))
		}
	}

	for _, kp := range keyPaths {
		k, err := key.LoadSoft(networkID, kp)
		if err != nil {
			return nil, err
		}

		existing = append(existing, k.P()...)
	}

	return existing, nil
}

func loadCreationKeys(network models.Network, kc keychain.Keychain) ([]string, error) {
	addrs := kc.Addresses().List()
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no creation addresses found")
	}
	networkID, err := network.NetworkID()
	if err != nil {
		return nil, err
	}
	hrp := key.GetHRP(networkID)
	addrsStr := []string{}
	for _, addr := range addrs {
		addrStr, err := address.Format("P", hrp, addr[:])
		if err != nil {
			return nil, err
		}
		addrsStr = append(addrsStr, addrStr)
	}

	return addrsStr, nil
}

func enterCustomKeys(network models.Network) ([]string, bool, error) {
	controlKeysPrompt := "Enter control keys"
	for {
		// ask in a loop so that if some condition is not met we can keep asking
		controlKeys, cancelled, err := controlKeysLoop(controlKeysPrompt, network)
		if err != nil {
			return nil, false, err
		}
		if cancelled {
			return nil, cancelled, nil
		}
		if len(controlKeys) == 0 {
			ux.Logger.PrintToUser("This tool does not allow to proceed without any control key set")
		} else {
			return controlKeys, false, nil
		}
	}
}

// controlKeysLoop asks as many controlkeys the user requires, until Done or Cancel is selected
func controlKeysLoop(controlKeysPrompt string, network models.Network) ([]string, bool, error) {
	label := "Control key"
	info := "Control keys are P-Chain addresses which have admin rights on the subnet.\n" +
		"Only private keys which control such addresses are allowed to make changes on the subnet"
	addressPrompt := "Enter P-Chain address (Example: P-...)"
	return prompts.CaptureListDecision(
		// we need this to be able to mock test
		app.Prompt,
		// the main prompt for entering address keys
		controlKeysPrompt,
		// the Capture function to use
		func(s string) (string, error) { return app.Prompt.CapturePChainAddress(s, network) },
		// the prompt for each address
		addressPrompt,
		// label describes the entity we are prompting for (e.g. address, control key, etc.)
		label,
		// optional parameter to allow the user to print the info string for more information
		info,
	)
}

// getThreshold prompts for the threshold of addresses as a number
func getThreshold(maxLen int) (uint32, error) {
	// create a list of indexes so the user only has the option to choose what is the theshold
	// instead of entering
	indexList := make([]string, maxLen)
	for i := 0; i < maxLen; i++ {
		indexList[i] = strconv.Itoa(i + 1)
	}
	threshold, err := app.Prompt.CaptureList("Select required number of control key signatures to add a validator", indexList)
	if err != nil {
		return 0, err
	}
	intTh, err := strconv.ParseUint(threshold, 0, 32)
	if err != nil {
		return 0, err
	}
	// this now should technically not happen anymore, but let's leave it as a double stitch
	if int(intTh) > maxLen {
		return 0, fmt.Errorf("the threshold can't be bigger than the number of control keys")
	}
	return uint32(intTh), err
}

func validateSubnetNameAndGetChains(args []string) ([]string, error) {
	// this should not be necessary but some bright guy might just be creating
	// the genesis by hand or something...
	if err := checkInvalidSubnetNames(args[0]); err != nil {
		return nil, fmt.Errorf("subnet name %s is invalid: %w", args[0], err)
	}
	// Check subnet exists
	// TODO create a file that lists chains by subnet for fast querying
	chains, err := getChainsInSubnet(args[0])
	if err != nil {
		return nil, fmt.Errorf("failed to getChainsInSubnet: %w", err)
	}

	if len(chains) == 0 {
		return nil, errors.New("Invalid subnet " + args[0])
	}

	return chains, nil
}

func checkMutuallyExclusive(flagA bool, flagB bool, flagC bool) error {
	if flagA && flagB || flagB && flagC || flagA && flagC {
		return errMutuallyExlusive
	}
	return nil
}

func getFujiKeyOrLedger() (bool, string, error) {
	useStoredKey, err := app.Prompt.ChooseKeyOrLedger()
	if err != nil {
		return false, "", err
	}
	if useStoredKey {
		keyName, err := captureKeyName()
		if err != nil {
			if err == errNoKeys {
				ux.Logger.PrintToUser("No private keys have been found. Deployment to fuji without a private key " +
					"or ledger is not possible. Create a new one with `avalanche key create`, or use a ledger device.")
			}
			return false, "", err
		}
		return false, keyName, nil
	}
	return true, "", nil
}

func getKeychain(
	useLedger bool,
	keyName string,
	network models.Network,
) (keychain.Keychain, error) {
	// get keychain accesor
	var kc keychain.Keychain
	if useLedger {
		ledgerDevice, err := ledger.New()
		if err != nil {
			return kc, err
		}
		// ask for addresses here to print user msg for ledger interaction
		ux.Logger.PrintToUser("*** Please provide extended public key on the ledger device ***")
		addresses, err := ledgerDevice.Addresses(1)
		if err != nil {
			return kc, err
		}
		addr := addresses[0]
		networkID, err := network.NetworkID()
		if err != nil {
			return kc, err
		}
		addrStr, err := address.Format("P", key.GetHRP(networkID), addr[:])
		if err != nil {
			return kc, err
		}
		ux.Logger.PrintToUser(logging.Yellow.Wrap(fmt.Sprintf("Ledger address: %s", addrStr)))
		return keychain.NewLedgerKeychain(ledgerDevice, numLedgerAddressesToDerive)
	}
	networkID, err := network.NetworkID()
	if err != nil {
		return kc, err
	}
	sf, err := key.LoadSoft(networkID, app.GetKeyPath(keyName))
	if err != nil {
		return kc, err
	}
	return sf.KeyChain(), nil
}

func checkSubnetAuthKeys(subnetAuthKeys []string, controlKeys []string, threshold uint32) error {
	// get keys for blockchain creation
	if subnetAuthKeys != nil && len(subnetAuthKeys) != int(threshold) {
		return fmt.Errorf("number of given chain creation keys differs from the threshold")
	}
	if subnetAuthKeys != nil {
		for _, subnetAuthKey := range subnetAuthKeys {
			found := false
			for _, controlKey := range controlKeys {
				if subnetAuthKey == controlKey {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("subnet auth key %s does not belong to control keys", subnetAuthKey)
			}
		}
	}
	return nil
}

func getSubnetAuthKeys(controlKeys []string, threshold uint32) ([]string, error) {
	subnetAuthKeys := []string{}
	if len(controlKeys) == int(threshold) {
		subnetAuthKeys = controlKeys[:]
	} else {
		subnetAuthKeys = []string{}
		filteredControlKeys := controlKeys[:]
		for len(subnetAuthKeys) != int(threshold) {
			subnetAuthKey, err := app.Prompt.CaptureList(
				"Choose a chain creation key",
				filteredControlKeys,
			)
			if err != nil {
				return nil, err
			}
			subnetAuthKeys = append(subnetAuthKeys, subnetAuthKey)
			filteredControlKeysTmp := []string{}
			for _, controlKey := range filteredControlKeys {
				if controlKey != subnetAuthKey {
					filteredControlKeysTmp = append(filteredControlKeysTmp, controlKey)
				}
			}
			filteredControlKeys = filteredControlKeysTmp
		}
	}
	return subnetAuthKeys, nil
}

func saveTxToDisk(tx *txs.Tx, outputTxPath string) error {
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Partial signing was done on blockchain tx. Saving tx to disk to enable remaining signing.")
	// get path
	if outputTxPath == "" {
		ux.Logger.PrintToUser("")
		var err error
		outputTxPath, err = app.Prompt.CaptureString("Path to export partially signed tx to")
		if err != nil {
			return err
		}
	}
	// Serialize the signed tx
	txBytes, err := txs.Codec.Marshal(txs.Version, tx)
	if err != nil {
		return fmt.Errorf("couldn't marshal signed tx: %w", err)
	}

	// Get the encoded (in hex + checksum) signed tx
	txStr, err := formatting.Encode(formatting.Hex, txBytes)
	if err != nil {
		return fmt.Errorf("couldn't encode signed tx: %w", err)
	}
	// save
	if _, err := os.Stat(outputTxPath); err == nil {
		return fmt.Errorf("couldn't create file to write tx to: file exists")
	}
	f, err := os.Create(outputTxPath)
	if err != nil {
		return fmt.Errorf("couldn't create file to write tx to: %w", err)
	}
	defer f.Close()
	_, err = f.WriteString(txStr)
	if err != nil {
		return fmt.Errorf("couldn't write tx into file: %w", err)
	}
	return nil
}

func loadTxFromDisk(outputTxPath string) (*txs.Tx, error) {
	txEncodedBytes, err := os.ReadFile(outputTxPath)
	if err != nil {
		return nil, err
	}
	txBytes, err := formatting.Decode(formatting.Hex, string(txEncodedBytes))
	if err != nil {
		return nil, fmt.Errorf("couldn't decode signed tx: %w", err)
	}
	var tx txs.Tx
	if _, err := txs.Codec.Unmarshal(txBytes, &tx); err != nil {
		return nil, fmt.Errorf("error unmarshaling signed tx: %w", err)
	}
	tx.Initialize(nil, txBytes)
	return &tx, nil
}

func printPartialSigningMsg(deployer *subnet.PublicDeployer, subnetAuthKeys []string, outputTxPath string) error {
	// final msg
	walletSubnetAuthKeys, err := deployer.GetWalletSubnetAuthAddresses(subnetAuthKeys)
	if err != nil {
		return err
	}
	filteredSubnetAuthKeys := []string{}
	for _, subnetAuthKey := range subnetAuthKeys {
		found := false
		for _, walletSubnetAuthKey := range walletSubnetAuthKeys {
			if subnetAuthKey == walletSubnetAuthKey {
				found = true
			}
		}
		if !found {
			filteredSubnetAuthKeys = append(filteredSubnetAuthKeys, subnetAuthKey)
		}
	}
	ux.Logger.PrintToUser("")
	if len(filteredSubnetAuthKeys) == 1 {
		ux.Logger.PrintToUser("One address remaining to sign the tx: %s", filteredSubnetAuthKeys[0])
	} else {
		ux.Logger.PrintToUser("%d addresses remaining to sign the tx:", len(filteredSubnetAuthKeys))
		for _, subnetAuthKey := range filteredSubnetAuthKeys {
			ux.Logger.PrintToUser("  %s", subnetAuthKey)
		}
	}
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Signing command:")
	ux.Logger.PrintToUser("  avalanche transaction sign %s", outputTxPath)
	return nil
}
