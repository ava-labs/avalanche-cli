// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/metrics"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

const (
	forceFlag  = "force"
	latest     = "latest"
	preRelease = "pre-release"
)

var (
	forceCreate                    bool
	useSubnetEvm                   bool
	genesisFile                    string
	vmFile                         string
	useCustom                      bool
	evmVersion                     string
	evmChainID                     uint64
	evmToken                       string
	evmDefaults                    bool
	useLatestReleasedEvmVersion    bool
	useLatestPreReleasedEvmVersion bool
	useRepo                        bool
	teleporterReady                bool
	runRelayer                     bool
	useWarp                        bool

	errIllegalNameCharacter = errors.New(
		"illegal name character: only letters, no special characters allowed")
	errMutuallyExlusiveVersionOptions = errors.New("version flags --latest,--pre-release,vm-version are mutually exclusive")
	errMutuallyVMConfigOptions        = errors.New("specifying --genesis flag disables SubnetEVM config flags --evm-chain-id,--evm-token,--evm-defaults")
)

// avalanche subnet create
func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [subnetName]",
		Short: "Create a new subnet configuration",
		Long: `The subnet create command builds a new genesis file to configure your Subnet.
By default, the command runs an interactive wizard. It walks you through
all the steps you need to create your first Subnet.

The tool supports deploying Subnet-EVM, and custom VMs. You
can create a custom, user-generated genesis with a custom VM by providing
the path to your genesis and VM binaries with the --genesis and --vm flags.

By default, running the command with a subnetName that already exists
causes the command to fail. If you’d like to overwrite an existing
configuration, pass the -f flag.`,
		SilenceUsage:      true,
		Args:              cobra.ExactArgs(1),
		RunE:              createSubnetConfig,
		PersistentPostRun: handlePostRun,
	}
	cmd.Flags().StringVar(&genesisFile, "genesis", "", "file path of genesis to use")
	cmd.Flags().BoolVar(&useSubnetEvm, "evm", false, "use the Subnet-EVM as the base template")
	cmd.Flags().StringVar(&evmVersion, "vm-version", "", "version of Subnet-EVM template to use")
	cmd.Flags().Uint64Var(&evmChainID, "evm-chain-id", 0, "chain ID to use with Subnet-EVM")
	cmd.Flags().StringVar(&evmToken, "evm-token", "", "token name to use with Subnet-EVM")
	cmd.Flags().BoolVar(&evmDefaults, "evm-defaults", false, "use default settings for fees/airdrop/precompiles/teleporter with Subnet-EVM")
	cmd.Flags().BoolVar(&useCustom, "custom", false, "use a custom VM template")
	cmd.Flags().BoolVar(&useLatestPreReleasedEvmVersion, preRelease, false, "use latest Subnet-EVM pre-released version, takes precedence over --vm-version")
	cmd.Flags().BoolVar(&useLatestReleasedEvmVersion, latest, false, "use latest Subnet-EVM released version, takes precedence over --vm-version")
	cmd.Flags().BoolVarP(&forceCreate, forceFlag, "f", false, "overwrite the existing configuration if one exists")
	cmd.Flags().StringVar(&vmFile, "vm", "", "file path of custom vm to use. alias to custom-vm-path")
	cmd.Flags().StringVar(&vmFile, "custom-vm-path", "", "file path of custom vm to use")
	cmd.Flags().StringVar(&customVMRepoURL, "custom-vm-repo-url", "", "custom vm repository url")
	cmd.Flags().StringVar(&customVMBranch, "custom-vm-branch", "", "custom vm branch or commit")
	cmd.Flags().StringVar(&customVMBuildScript, "custom-vm-build-script", "", "custom vm build-script")
	cmd.Flags().BoolVar(&useRepo, "from-github-repo", false, "generate custom VM binary from github repository")
	cmd.Flags().BoolVar(&useWarp, "warp", true, "generate a vm with warp support (needed for teleporter)")
	cmd.Flags().BoolVar(&teleporterReady, "teleporter", false, "generate a teleporter-ready vm")
	cmd.Flags().BoolVar(&runRelayer, "relayer", false, "run AWM relayer when deploying the vm")
	return cmd
}

func CallCreate(
	cmd *cobra.Command,
	subnetName string,
	forceCreateParam bool,
	genesisFileParam string,
	useSubnetEvmParam bool,
	useCustomParam bool,
	evmVersionParam string,
	evmChainIDParam uint64,
	evmTokenParam string,
	evmDefaultsParam bool,
	useLatestReleasedEvmVersionParam bool,
	useLatestPreReleasedEvmVersionParam bool,
	customVMRepoURLParam string,
	customVMBranchParam string,
	customVMBuildScriptParam string,
) error {
	forceCreate = forceCreateParam
	genesisFile = genesisFileParam
	useSubnetEvm = useSubnetEvmParam
	evmVersion = evmVersionParam
	evmChainID = evmChainIDParam
	evmToken = evmTokenParam
	evmDefaults = evmDefaultsParam
	useLatestReleasedEvmVersion = useLatestReleasedEvmVersionParam
	useLatestPreReleasedEvmVersion = useLatestPreReleasedEvmVersionParam
	useCustom = useCustomParam
	customVMRepoURL = customVMRepoURLParam
	customVMBranch = customVMBranchParam
	customVMBuildScript = customVMBuildScriptParam
	return createSubnetConfig(cmd, []string{subnetName})
}

func detectVMTypeFromFlags() {
	// assumes custom
	if customVMRepoURL != "" || customVMBranch != "" || customVMBuildScript != "" {
		useCustom = true
	}
}

func moreThanOneVMSelected() bool {
	vmVars := []bool{useSubnetEvm, useCustom}
	firstSelect := false
	for _, val := range vmVars {
		if firstSelect && val {
			return true
		} else if val {
			firstSelect = true
		}
	}
	return false
}

func getVMFromFlag() models.VMType {
	if useSubnetEvm {
		return models.SubnetEvm
	}
	if useCustom {
		return models.CustomVM
	}
	return ""
}

// override postrun function from root.go, so that we don't double send metrics for the same command
func handlePostRun(_ *cobra.Command, _ []string) {}

func createSubnetConfig(cmd *cobra.Command, args []string) error {
	subnetName := args[0]
	if app.GenesisExists(subnetName) && !forceCreate {
		return errors.New("configuration already exists. Use --" + forceFlag + " parameter to overwrite")
	}

	if err := checkInvalidSubnetNames(subnetName); err != nil {
		return fmt.Errorf("subnet name %q is invalid: %w", subnetName, err)
	}

	detectVMTypeFromFlags()

	if moreThanOneVMSelected() {
		return errors.New("too many VMs selected. Provide at most one VM selection flag")
	}

	if !flags.EnsureMutuallyExclusive([]bool{useLatestReleasedEvmVersion, useLatestPreReleasedEvmVersion, evmVersion != ""}) {
		return errMutuallyExlusiveVersionOptions
	}

	if genesisFile != "" && (evmChainID != 0 || evmToken != "" || evmDefaults) {
		return errMutuallyVMConfigOptions
	}

	subnetType := getVMFromFlag()

	if subnetType == "" {
		subnetTypeStr, err := app.Prompt.CaptureList(
			"Choose your VM",
			[]string{models.SubnetEvm, models.CustomVM},
		)
		if err != nil {
			return err
		}
		subnetType = models.VMTypeFromString(subnetTypeStr)
	}

	var (
		genesisBytes []byte
		sc           *models.Sidecar
		err          error
	)

	if useLatestReleasedEvmVersion {
		evmVersion = latest
	}

	if useLatestPreReleasedEvmVersion {
		evmVersion = preRelease
	}

	if evmVersion != latest && evmVersion != preRelease && evmVersion != "" && !semver.IsValid(evmVersion) {
		return fmt.Errorf("invalid version string, should be semantic version (ex: v1.1.1): %s", evmVersion)
	}

	switch subnetType {
	case models.SubnetEvm:
		genesisBytes, sc, err = vm.CreateEvmSubnetConfig(
			app,
			subnetName,
			genesisFile,
			evmVersion,
			true,
			evmChainID,
			evmToken,
			evmDefaults,
			useWarp,
		)
		if err != nil {
			return err
		}
	case models.CustomVM:
		genesisBytes, sc, err = vm.CreateCustomSubnetConfig(
			app,
			subnetName,
			genesisFile,
			useRepo,
			customVMRepoURL,
			customVMBranch,
			customVMBuildScript,
			vmFile,
		)
		if err != nil {
			return err
		}
	default:
		return errors.New("not implemented")
	}

	if isSubnetEVMGenesis := jsonIsSubnetEVMGenesis(genesisBytes); isSubnetEVMGenesis {
		if evmDefaults {
			teleporterReady = true
			runRelayer = true
		}
		teleporterReady, err = prompts.CaptureBoolFlag(
			app.Prompt,
			cmd,
			"teleporter",
			teleporterReady,
			"Would you like to enable Teleporter on your VM?",
		)
		if err != nil {
			return err
		}
		if teleporterReady && !useWarp {
			return fmt.Errorf("warp should be enabled for teleporter to work")
		}
		if teleporterReady {
			runRelayer, err = prompts.CaptureBoolFlag(
				app.Prompt,
				cmd,
				"relayer",
				runRelayer,
				"Would you like to run AMW Relayer when deploying your VM?",
			)
			if err != nil {
				return err
			}
			keyPath := app.GetKeyPath(constants.TeleporterKeyName)
			var k *key.SoftKey
			if utils.FileExists(keyPath) {
				ux.Logger.PrintToUser("loading stored key %q for teleporter deploys", constants.TeleporterKeyName)
				k, err = key.LoadSoft(models.NewLocalNetwork().ID, keyPath)
				if err != nil {
					return err
				}
			} else {
				ux.Logger.PrintToUser("generating stored key %q for teleporter deploys", constants.TeleporterKeyName)
				k, err = key.NewSoft(0)
				if err != nil {
					return err
				}
				if err := k.Save(keyPath); err != nil {
					return err
				}
			}
			ux.Logger.PrintToUser("  (evm address, genesis balance) = (%s, %v)", k.C(), teleporter.TeleporterPrefundedAddressBalance)
			genesisBytes, err = addSubnetEVMGenesisPrefundedAddress(genesisBytes, k.C(), teleporter.TeleporterPrefundedAddressBalance.String())
			if err != nil {
				return err
			}
			// let's use latest versions for teleporter contract
			teleporterVersion, err := app.Downloader.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(constants.AvaLabsOrg, constants.TeleporterRepoName))
			if err != nil {
				return err
			}
			ux.Logger.PrintToUser("using latest teleporter version (%s)", teleporterVersion)
			sc.TeleporterReady = true
			sc.TeleporterKey = constants.TeleporterKeyName
			sc.TeleporterVersion = teleporterVersion
			sc.RunRelayer = runRelayer
		}
	}

	if err = app.WriteGenesisFile(subnetName, genesisBytes); err != nil {
		return err
	}

	sc.ImportedFromAPM = false
	if err = app.CreateSidecar(sc); err != nil {
		return err
	}
	if subnetType == models.SubnetEvm {
		err = sendMetrics(cmd, subnetType.RepoName(), subnetName)
		if err != nil {
			return err
		}
	}
	ux.Logger.GreenCheckmarkToUser("Successfully created subnet configuration")
	return nil
}

func addSubnetEVMGenesisPrefundedAddress(genesisBytes []byte, address string, balance string) ([]byte, error) {
	var genesisMap map[string]interface{}
	if err := json.Unmarshal(genesisBytes, &genesisMap); err != nil {
		return nil, err
	}
	allocI, ok := genesisMap["alloc"]
	if !ok {
		return nil, fmt.Errorf("alloc field not found on genesis")
	}
	alloc, ok := allocI.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected genesis alloc field to be map[string]interface, found %T", allocI)
	}
	trimmedAddress := strings.TrimPrefix(address, "0x")
	alloc[trimmedAddress] = map[string]interface{}{
		"balance": balance,
	}
	genesisMap["alloc"] = alloc
	return json.MarshalIndent(genesisMap, "", "  ")
}

func sendMetrics(cmd *cobra.Command, repoName, subnetName string) error {
	flags := make(map[string]string)
	flags[constants.SubnetType] = repoName
	genesis, err := app.LoadEvmGenesis(subnetName)
	if err != nil {
		return err
	}
	conf := genesis.Config.GenesisPrecompiles
	precompiles := make([]string, 6)
	for precompileName := range conf {
		precompileTag := "precompile-" + precompileName
		flags[precompileTag] = precompileName
		precompiles = append(precompiles, precompileName)
	}
	numAirdropAddresses := len(genesis.Alloc)
	for address := range genesis.Alloc {
		if address.String() != vm.PrefundedEwoqAddress.String() {
			precompileTag := "precompile-" + constants.CustomAirdrop
			flags[precompileTag] = constants.CustomAirdrop
			precompiles = append(precompiles, constants.CustomAirdrop)
			break
		}
	}
	sort.Strings(precompiles)
	precompilesJoined := strings.Join(precompiles, ",")
	flags[constants.PrecompileType] = precompilesJoined
	flags[constants.NumberOfAirdrops] = strconv.Itoa(numAirdropAddresses)
	metrics.HandleTracking(cmd, app, flags)
	return nil
}

func checkInvalidSubnetNames(name string) error {
	// this is currently exactly the same code as in avalanchego/vms/platformvm/create_chain_tx.go
	for _, r := range name {
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == ' ') {
			return errIllegalNameCharacter
		}
	}

	return nil
}
