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
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/keychain"
	"github.com/ava-labs/coreth/core"
	spacesvmchain "github.com/ava-labs/spacesvm/chain"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	deployLocal    bool
	deployTestnet  bool
	deployMainnet  bool
	useLedger      bool
	sameControlKey bool
	keyName        string
	threshold      uint32
	controlKeys    []string
	avagoVersion   string

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
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key [fuji deploys]")
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji deploys]")
	cmd.Flags().BoolVarP(&sameControlKey, "same-control-key", "s", false, "use creation key as control key")
	cmd.Flags().Uint32Var(&threshold, "threshold", 0, "required number of control key signatures to add a validator [fuji deploys]")
	cmd.Flags().StringSliceVar(&controlKeys, "control-keys", nil, "addresses that may add new validators to the subnet [fuji deploys]")
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
	kc, err := getKeychainAccessor(useLedger, keyName, network)
	if err != nil {
		return err
	}

	// use creation key as control key
	if sameControlKey {
		controlKey, err := loadCreationKey(network, kc)
		if err != nil {
			return err
		}
		controlKeys = []string{controlKey}
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
	if int(threshold) > len(controlKeys) {
		return fmt.Errorf("given threshold is greater than number of control keys")
	}
	if len(controlKeys) > 0 && threshold == 0 {
		if len(controlKeys) == 1 {
			threshold = 1
		} else {
			threshold, err = getThreshold(len(controlKeys))
			if err != nil {
				return err
			}
		}
	}

	// deploy to public network
	deployer := subnet.NewPublicDeployer(app, useLedger, kc, network)
	subnetID, blockchainID, err := deployer.Deploy(controlKeys, threshold, chain, chainGenesis)
	if err != nil {
		return err
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

func getControlKeys(network models.Network, useLedger bool, kc keychain.Accessor) ([]string, bool, error) {
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
		var key string
		key, err = loadCreationKey(network, kc)
		keys = []string{key}
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

func loadCreationKey(network models.Network, kc keychain.Accessor) (string, error) {
	addrs := kc.Addresses().List()
	if len(addrs) == 0 {
		return "", fmt.Errorf("no creation addresses found")
	}
	addr := addrs[0]
	networkID, err := network.NetworkID()
	if err != nil {
		return "", err
	}
	hrp := key.GetHRP(networkID)
	addrStr, err := address.Format("P", hrp, addr[:])
	if err != nil {
		return "", err
	}

	return addrStr, nil
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
				ux.Logger.PrintToUser("No private keys have been found. Deployment to fuji without a private key or ledger is not possible. Create a new one with `avalanche key create`, or use a ledger device.")
			}
			return false, "", err
		}
		return false, keyName, nil
	} else {
		return true, "", nil
	}
}

func getKeychainAccessor(
	useLedger bool,
	keyName string,
	network models.Network,
) (keychain.Accessor, error) {
	// get keychain accesor
	var kc keychain.Accessor
	if useLedger {
		ledgerDevice, err := ledger.Connect()
		if err != nil {
			return kc, err
		}
		// ask for addresses here to print user msg for ledger interaction
		ux.Logger.PrintToUser("*** Please provide extended public key on the ledger device ***")
		_, err = ledgerDevice.Addresses(1)
		if err != nil {
			return kc, err
		}
		kc = keychain.NewLedgerKeychain(ledgerDevice)
	} else {
		networkID, err := network.NetworkID()
		if err != nil {
			return kc, err
		}
		sf, err := key.LoadSoft(networkID, app.GetKeyPath(keyName))
		if err != nil {
			return kc, err
		}
		kc = sf.KeyChain()
	}
	return kc, nil
}
