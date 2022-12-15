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

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/localnetworkinterface"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	ledger "github.com/ava-labs/avalanche-ledger-go"
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/coreth/core"
	spacesvmchain "github.com/ava-labs/spacesvm/chain"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/mod/semver"
)

const numLedgerAddressesToSearch = 1000

var (
	deployLocal              bool
	deployTestnet            bool
	deployMainnet            bool
	sameControlKey           bool
	keyName                  string
	threshold                uint32
	controlKeys              []string
	subnetAuthKeys           []string
	userProvidedAvagoVersion string
	outputTxPath             string
	useLedger                bool
	ledgerAddresses          []string

	errMutuallyExlusiveNetworks    = errors.New("--local, --fuji (resp. --testnet) and --mainnet are mutually exclusive")
	errMutuallyExlusiveControlKeys = errors.New("--control-keys and --same-control-key are mutually exclusive")
	ErrMutuallyExlusiveKeyLedger   = errors.New("--key and --ledger,--ledger-addrs are mutually exclusive")
	ErrStoredKeyOnMainnet          = errors.New("--key is not available for mainnet operations")
)

// avalanche subnet deploy
func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [subnetName]",
		Short: "Deploys a subnet configuration",
		Long: `The subnet deploy command deploys your Subnet configuration locally, to Fuji Testnet, or to Mainnet.

At the end of the call, the command prints the RPC URL you can use to interact with the Subnet.

Avalanche-CLI only supports deploying an individual Subnet once per network. Subsequent
attempts to deploy the same Subnet to the same network (local, Fuji, Mainnet) aren't
allowed. If you'd like to redeploy a Subnet locally for testing, you must first call
avalanche network clean to reset all deployed chain state. Subsequent local deploys
redeploy the chain with fresh state. You can deploy the same Subnet to multiple networks,
so you can take your locally tested Subnet and deploy it on Fuji or Mainnet.`,
		SilenceUsage: true,
		RunE:         deploySubnet,
		Args:         cobra.ExactArgs(1),
	}
	cmd.Flags().BoolVarP(&deployLocal, "local", "l", false, "deploy to a local network")
	cmd.Flags().BoolVarP(&deployTestnet, "testnet", "t", false, "deploy to testnet (alias to `fuji`)")
	cmd.Flags().BoolVarP(&deployTestnet, "fuji", "f", false, "deploy to fuji (alias to `testnet`")
	cmd.Flags().BoolVarP(&deployMainnet, "mainnet", "m", false, "deploy to mainnet")
	cmd.Flags().StringVar(&userProvidedAvagoVersion, "avalanchego-version", "latest", "use this version of avalanchego (ex: v1.17.12)")
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji deploy only]")
	cmd.Flags().BoolVarP(&sameControlKey, "same-control-key", "s", false, "use creation key as control key")
	cmd.Flags().Uint32Var(&threshold, "threshold", 0, "required number of control key signatures to make subnet changes")
	cmd.Flags().StringSliceVar(&controlKeys, "control-keys", nil, "addresses that may make subnet changes")
	cmd.Flags().StringSliceVar(&subnetAuthKeys, "subnet-auth-keys", nil, "control keys that will be used to authenticate chain creation")
	cmd.Flags().StringVar(&outputTxPath, "output-tx-path", "", "file path of the blockchain creation tx")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	return cmd
}

func getChainsInSubnet(subnetName string) ([]string, error) {
	subnets, err := os.ReadDir(app.GetSubnetDir())
	if err != nil {
		return nil, fmt.Errorf("failed to read baseDir: %w", err)
	}

	chains := []string{}

	for _, s := range subnets {
		sidecarFile := filepath.Join(app.GetSubnetDir(), s.Name(), constants.SidecarFileName)
		if _, err := os.Stat(sidecarFile); err == nil {
			// read in sidecar file
			jsonBytes, err := os.ReadFile(sidecarFile)
			if err != nil {
				return nil, fmt.Errorf("failed reading file %s: %w", sidecarFile, err)
			}

			var sc models.Sidecar
			err = json.Unmarshal(jsonBytes, &sc)
			if err != nil {
				return nil, fmt.Errorf("failed unmarshaling file %s: %w", sidecarFile, err)
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

	if !flags.EnsureMutuallyExclusive([]bool{deployLocal, deployTestnet, deployMainnet}) {
		return errMutuallyExlusiveNetworks
	}

	if outputTxPath != "" {
		if _, err := os.Stat(outputTxPath); err == nil {
			return fmt.Errorf("outputTxPath %q already exists", outputTxPath)
		}
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

	if len(ledgerAddresses) > 0 {
		useLedger = true
	}

	if useLedger && keyName != "" {
		return ErrMutuallyExlusiveKeyLedger
	}

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

		// skip rpc check if using custom vm
		if sidecar.VM != models.CustomVM {
			// check if selected version matches what is currently running
			nc := localnetworkinterface.NewStatusChecker()
			userProvidedAvagoVersion, err = checkForInvalidDeployAndGetAvagoVersion(nc, sidecar.RPCVersion)
			if err != nil {
				return err
			}
		}

		deployer := subnet.NewLocalDeployer(app, userProvidedAvagoVersion, vmBin)
		subnetID, blockchainID, err := deployer.DeployToLocalNetwork(chain, chainGenesis, genesisPath)
		if err != nil {
			if deployer.BackendStartedHere() {
				if innerErr := binutils.KillgRPCServerProcess(app); innerErr != nil {
					app.Log.Warn("tried to kill the gRPC server process but it failed", zap.Error(innerErr))
				}
			}
			return err
		}
		return app.UpdateSidecarNetworks(&sidecar, network, subnetID, blockchainID)

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
		return errors.New("not implemented")
	}

	// used in E2E to simulate public network execution paths on a local network
	if os.Getenv(constants.SimulatePublicNetwork) != "" {
		network = models.Local
	}

	// from here on we are assuming a public deploy

	// get keychain accesor
	kc, err := GetKeychain(useLedger, ledgerAddresses, keyName, network)
	if err != nil {
		return err
	}

	// accept only one control keys specification
	if len(controlKeys) > 0 && sameControlKey {
		return errMutuallyExlusiveControlKeys
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
		threshold, err = getThreshold(len(controlKeys))
		if err != nil {
			return err
		}
	}

	// get keys for blockchain tx signing
	if subnetAuthKeys != nil {
		if err := prompts.CheckSubnetAuthKeys(subnetAuthKeys, controlKeys, threshold); err != nil {
			return err
		}
	} else {
		subnetAuthKeys, err = prompts.GetSubnetAuthKeys(app.Prompt, controlKeys, threshold)
		if err != nil {
			return err
		}
	}
	ux.Logger.PrintToUser("Your subnet auth keys for chain creation: %s", subnetAuthKeys)

	// deploy to public network
	deployer := subnet.NewPublicDeployer(app, useLedger, kc, network)
	isFullySigned, subnetID, blockchainID, tx, err := deployer.Deploy(controlKeys, subnetAuthKeys, threshold, chain, chainGenesis)
	if err != nil {
		return err
	}

	if err := PrintDeployResults(chain, subnetID, blockchainID, isFullySigned); err != nil {
		return err
	}

	if !isFullySigned {
		if err := SaveNotFullySignedTx(
			"Blockchain Creation",
			tx,
			network,
			chain,
			subnetID,
			subnetAuthKeys,
			outputTxPath,
			false,
		); err != nil {
			return err
		}
	}

	// update sidecar
	// TODO: need to do something for backwards compatibility?
	return app.UpdateSidecarNetworks(&sidecar, network, subnetID, blockchainID)
}

func getControlKeys(network models.Network, useLedger bool, kc keychain.Keychain) ([]string, bool, error) {
	controlKeysInitialPrompt := "Configure which addresses may make changes to the subnet.\n" +
		"These addresses are known as your control keys. You will also\n" +
		"set how many control keys are required to make a subnet change (the threshold)."
	moreKeysPrompt := "How would you like to set your control keys?"

	ux.Logger.PrintToUser(controlKeysInitialPrompt)

	const (
		useAll = "Use all stored keys"
		custom = "Custom list"
	)

	var creation string
	var listOptions []string
	if useLedger {
		creation = "Use ledger address"
	} else {
		creation = "Use creation key"
	}
	if network == models.Mainnet {
		listOptions = []string{creation, custom}
	} else {
		listOptions = []string{creation, useAll, custom}
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
	if maxLen == 1 {
		return uint32(1), nil
	}
	// create a list of indexes so the user only has the option to choose what is the theshold
	// instead of entering
	indexList := make([]string, maxLen)
	for i := 0; i < maxLen; i++ {
		indexList[i] = strconv.Itoa(i + 1)
	}
	threshold, err := app.Prompt.CaptureList("Select required number of control key signatures to make a subnet change", indexList)
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

func SaveNotFullySignedTx(
	txName string,
	tx *txs.Tx,
	network models.Network,
	chain string,
	subnetID ids.ID,
	subnetAuthKeys []string,
	outputTxPath string,
	forceOverwrite bool,
) error {
	remainingSubnetAuthKeys, err := txutils.GetRemainingSigners(tx, network, subnetID)
	if err != nil {
		return err
	}
	signedCount := len(subnetAuthKeys) - len(remainingSubnetAuthKeys)
	ux.Logger.PrintToUser("")
	if signedCount == len(subnetAuthKeys) {
		ux.Logger.PrintToUser("All %d required %s signatures have been signed. "+
			"Saving tx to disk to enable commit.", len(subnetAuthKeys), txName)
	} else {
		ux.Logger.PrintToUser("%d of %d required %s signatures have been signed. "+
			"Saving tx to disk to enable remaining signing.", signedCount, len(subnetAuthKeys), txName)
	}
	if outputTxPath == "" {
		ux.Logger.PrintToUser("")
		var err error
		if forceOverwrite {
			outputTxPath, err = app.Prompt.CaptureString("Path to export partially signed tx to")
		} else {
			outputTxPath, err = app.Prompt.CaptureNewFilepath("Path to export partially signed tx to")
		}
		if err != nil {
			return err
		}
	}
	if forceOverwrite {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Overwritting %s", outputTxPath)
	}
	if err := txutils.SaveToDisk(tx, outputTxPath, forceOverwrite); err != nil {
		return err
	}
	if signedCount == len(subnetAuthKeys) {
		PrintReadyToSignMsg(chain, outputTxPath)
	} else {
		PrintRemainingToSignMsg(chain, remainingSubnetAuthKeys, outputTxPath)
	}
	return nil
}

func PrintReadyToSignMsg(
	chain string,
	outputTxPath string,
) {
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Tx is fully signed, and ready to be committed")
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Commit command:")
	ux.Logger.PrintToUser("  avalanche transaction commit %s --input-tx-filepath %s", chain, outputTxPath)
}

func PrintRemainingToSignMsg(
	chain string,
	remainingSubnetAuthKeys []string,
	outputTxPath string,
) {
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Addresses remaining to sign the tx")
	for _, subnetAuthKey := range remainingSubnetAuthKeys {
		ux.Logger.PrintToUser("  %s", subnetAuthKey)
	}
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Connect a ledger with one of the remaining addresses or choose a stored key "+
		"and run the signing command, or send %q to another user for signing.", outputTxPath)
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Signing command:")
	ux.Logger.PrintToUser("  avalanche transaction sign %s --input-tx-filepath %s", chain, outputTxPath)
}

func GetKeychain(
	useLedger bool,
	ledgerAddresses []string,
	keyName string,
	network models.Network,
) (keychain.Keychain, error) {
	// get keychain accesor
	var kc keychain.Keychain
	networkID, err := network.NetworkID()
	if err != nil {
		return kc, err
	}
	if useLedger {
		ledgerDevice, err := ledger.New()
		if err != nil {
			return kc, err
		}
		// ask for addresses here to print user msg for ledger interaction
		ux.Logger.PrintToUser("*** Please provide extended public key on the ledger device ***")
		// set ledger indices
		var ledgerIndices []uint32
		if len(ledgerAddresses) == 0 {
			ledgerIndices = []uint32{0}
		} else {
			ledgerIndices, err = getLedgerIndices(ledgerDevice, ledgerAddresses)
			if err != nil {
				return kc, err
			}
		}
		// get formatted addresses for ux
		addresses, err := ledgerDevice.Addresses(ledgerIndices)
		if err != nil {
			return kc, err
		}
		addrStrs := []string{}
		for _, addr := range addresses {
			addrStr, err := address.Format("P", key.GetHRP(networkID), addr[:])
			if err != nil {
				return kc, err
			}
			addrStrs = append(addrStrs, addrStr)
		}
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Ledger addresses: "))
		for _, addrStr := range addrStrs {
			ux.Logger.PrintToUser(logging.Yellow.Wrap(fmt.Sprintf("  %s", addrStr)))
		}
		return keychain.NewLedgerKeychainFromIndices(ledgerDevice, ledgerIndices)
	}
	sf, err := key.LoadSoft(networkID, app.GetKeyPath(keyName))
	if err != nil {
		return kc, err
	}
	return sf.KeyChain(), nil
}

func getLedgerIndices(ledgerDevice ledger.Ledger, addressesStr []string) ([]uint32, error) {
	addresses, err := address.ParseToIDs(addressesStr)
	if err != nil {
		return []uint32{}, fmt.Errorf("failure parsing given ledger addresses: %w", err)
	}
	// maps the indices of addresses to their corresponding ledger indices
	indexMap := map[int]uint32{}
	// for all ledger indices to search for, find if the ledger address belongs to the input
	// addresses and, if so, add the index pair to indexMap, breaking the loop if
	// all addresses were found
	for ledgerIndex := uint32(0); ledgerIndex < numLedgerAddressesToSearch; ledgerIndex++ {
		ledgerAddress, err := ledgerDevice.Addresses([]uint32{ledgerIndex})
		if err != nil {
			return []uint32{}, err
		}
		for addressesIndex, addr := range addresses {
			if addr == ledgerAddress[0] {
				indexMap[addressesIndex] = ledgerIndex
			}
		}
		if len(indexMap) == len(addresses) {
			break
		}
	}
	// create ledgerIndices from indexMap
	ledgerIndices := []uint32{}
	for addressesIndex := range addresses {
		ledgerIndex, ok := indexMap[addressesIndex]
		if !ok {
			return []uint32{}, fmt.Errorf("address %s not found on ledger", addressesStr[addressesIndex])
		}
		ledgerIndices = append(ledgerIndices, ledgerIndex)
	}
	return ledgerIndices, nil
}

func PrintDeployResults(chain string, subnetID ids.ID, blockchainID ids.ID, isFullySigned bool) error {
	vmID, err := utils.VMID(chain)
	if err != nil {
		return fmt.Errorf("failed to create VM ID from %s: %w", chain, err)
	}
	header := []string{"Deployment results", ""}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetRowLine(true)
	table.SetAutoMergeCells(true)
	table.Append([]string{"Chain Name", chain})
	table.Append([]string{"Subnet ID", subnetID.String()})
	table.Append([]string{"VM ID", vmID.String()})
	if isFullySigned {
		table.Append([]string{"Blockchain ID", blockchainID.String()})
		table.Append([]string{"RPC URL", fmt.Sprintf("%s/ext/bc/%s/rpc", constants.DefaultNodeRunURL, blockchainID.String())})
		table.Append([]string{"P-Chain TXID", blockchainID.String()})
	}
	table.Render()
	return nil
}

// Determines the appropriate version of avalanchego to run with. Returns an error if
// that version conflicts with the current deployment.
func checkForInvalidDeployAndGetAvagoVersion(network localnetworkinterface.StatusChecker, configuredRPCVersion int) (string, error) {
	// get current network
	runningAvagoVersion, runningRPCVersion, networkRunning, err := network.GetCurrentNetworkVersion()
	if err != nil {
		return "", err
	}

	desiredAvagoVersion := userProvidedAvagoVersion

	// RPC Version was made available in the info API in avalanchego version v1.9.2. For prior versions,
	// we will need to skip this check.
	skipRPCCheck := false
	if semver.Compare(runningAvagoVersion, constants.AvalancheGoCompatibilityVersionAdded) == -1 {
		skipRPCCheck = true
	}

	if networkRunning {
		if userProvidedAvagoVersion == "latest" {
			if runningRPCVersion != configuredRPCVersion && !skipRPCCheck {
				return "", fmt.Errorf(
					"the current avalanchego deployment uses rpc version %d but your subnet has version %d and is not compatible",
					runningRPCVersion,
					configuredRPCVersion,
				)
			}
			desiredAvagoVersion = runningAvagoVersion
		} else if runningAvagoVersion != userProvidedAvagoVersion {
			// user wants a specific version
			return "", errors.New("incompatible avalanchego version selected")
		}
	} else if userProvidedAvagoVersion == "latest" {
		// find latest avago version for this rpc version
		desiredAvagoVersion, err = vm.GetLatestAvalancheGoByProtocolVersion(
			app, configuredRPCVersion, constants.AvalancheGoCompatibilityURL)
		if err != nil {
			return "", err
		}
	}
	return desiredAvagoVersion, nil
}
