// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/metrics"
	"github.com/ava-labs/avalanche-cli/pkg/models"
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

type CreateFlags struct {
	useSubnetEvm                  bool
	useCustomVM                   bool
	chainID                       uint64
	tokenSymbol                   string
	useDefaults                   bool
	useWarp                       bool
	useTeleporter                 bool
	vmVersion                     string
	useLatestReleasedVMVersion    bool
	useLatestPreReleasedVMVersion bool
}

var (
	createFlags CreateFlags
	forceCreate bool
	genesisFile string
	vmFile      string
	useRepo     bool

	errIllegalNameCharacter = errors.New(
		"illegal name character: only letters, no special characters allowed")
	errMutuallyExlusiveVersionOptions   = errors.New("version flags --latest,--pre-release,vm-version are mutually exclusive")
	errMutuallyExclusiveVMConfigOptions = errors.New("specifying --genesis flag disable flags --evm-chain-id,--evm-token,--evm-defaults")
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
causes the command to fail. If you'd like to overwrite an existing
configuration, pass the -f flag.`,
		Args:              cobrautils.ExactArgs(1),
		RunE:              createSubnetConfig,
		PersistentPostRun: handlePostRun,
	}
	cmd.Flags().StringVar(&genesisFile, "genesis", "", "file path of genesis to use")
	cmd.Flags().BoolVar(&createFlags.useSubnetEvm, "evm", false, "use the Subnet-EVM as the base template")
	cmd.Flags().BoolVar(&createFlags.useCustomVM, "custom", false, "use a custom VM template")
	cmd.Flags().StringVar(&createFlags.vmVersion, "vm-version", "", "version of Subnet-EVM template to use")
	cmd.Flags().BoolVar(&createFlags.useLatestPreReleasedVMVersion, preRelease, false, "use latest Subnet-EVM pre-released version, takes precedence over --vm-version")
	cmd.Flags().BoolVar(&createFlags.useLatestReleasedVMVersion, latest, false, "use latest Subnet-EVM released version, takes precedence over --vm-version")
	cmd.Flags().Uint64Var(&createFlags.chainID, "evm-chain-id", 0, "chain ID to use with Subnet-EVM")
	cmd.Flags().StringVar(&createFlags.tokenSymbol, "evm-token", "", "token symbol to use with Subnet-EVM")
	cmd.Flags().BoolVar(&createFlags.useDefaults, "evm-defaults", false, "use default settings for fees/airdrop/precompiles/teleporter with Subnet-EVM")
	cmd.Flags().BoolVarP(&forceCreate, forceFlag, "f", false, "overwrite the existing configuration if one exists")
	cmd.Flags().StringVar(&vmFile, "vm", "", "file path of custom vm to use. alias to custom-vm-path")
	cmd.Flags().StringVar(&vmFile, "custom-vm-path", "", "file path of custom vm to use")
	cmd.Flags().StringVar(&customVMRepoURL, "custom-vm-repo-url", "", "custom vm repository url")
	cmd.Flags().StringVar(&customVMBranch, "custom-vm-branch", "", "custom vm branch or commit")
	cmd.Flags().StringVar(&customVMBuildScript, "custom-vm-build-script", "", "custom vm build-script")
	cmd.Flags().BoolVar(&useRepo, "from-github-repo", false, "generate custom VM binary from github repository")
	cmd.Flags().BoolVar(&createFlags.useWarp, "warp", true, "generate a vm with warp support (needed for teleporter)")
	cmd.Flags().BoolVar(&createFlags.useTeleporter, "teleporter", false, "interoperate with other blockchains using teleporter")
	return cmd
}

func CallCreate(
	cmd *cobra.Command,
	subnetName string,
	forceCreateParam bool,
	genesisFileParam string,
	useSubnetEvmParam bool,
	useCustomParam bool,
	vmVersionParam string,
	evmChainIDParam uint64,
	tokenSymbolParam string,
	useDefaultsParam bool,
	useLatestReleasedVMVersionParam bool,
	useLatestPreReleasedVMVersionParam bool,
	customVMRepoURLParam string,
	customVMBranchParam string,
	customVMBuildScriptParam string,
) error {
	forceCreate = forceCreateParam
	genesisFile = genesisFileParam
	createFlags.useSubnetEvm = useSubnetEvmParam
	createFlags.vmVersion = vmVersionParam
	createFlags.chainID = evmChainIDParam
	createFlags.tokenSymbol = tokenSymbolParam
	createFlags.useDefaults = useDefaultsParam
	createFlags.useLatestReleasedVMVersion = useLatestReleasedVMVersionParam
	createFlags.useLatestPreReleasedVMVersion = useLatestPreReleasedVMVersionParam
	createFlags.useCustomVM = useCustomParam
	customVMRepoURL = customVMRepoURLParam
	customVMBranch = customVMBranchParam
	customVMBuildScript = customVMBuildScriptParam
	return createSubnetConfig(cmd, []string{subnetName})
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

	// version flags exclusiveness
	if !flags.EnsureMutuallyExclusive([]bool{
		createFlags.useLatestReleasedVMVersion,
		createFlags.useLatestPreReleasedVMVersion,
		createFlags.vmVersion != "",
	}) {
		return errMutuallyExlusiveVersionOptions
	}

	// genesis flags exclusiveness
	if genesisFile != "" && (createFlags.chainID != 0 || createFlags.tokenSymbol != "" || createFlags.useDefaults) {
		return errMutuallyExclusiveVMConfigOptions
	}

	// if given custom repo info, assumes custom VM
	if vmFile != "" || customVMRepoURL != "" || customVMBranch != "" || customVMBuildScript != "" {
		createFlags.useCustomVM = true
	}

	// vm type exclusiveness
	if !flags.EnsureMutuallyExclusive([]bool{createFlags.useSubnetEvm, createFlags.useCustomVM}) {
		return errors.New("flags --evm,--custom are mutually exclusive")
	}

	// get vm kind
	vmType, err := vm.PromptVMType(app, createFlags.useSubnetEvm, createFlags.useCustomVM)
	if err != nil {
		return err
	}

	var (
		genesisBytes []byte
		sc           *models.Sidecar
	)

	var teleporterInfo *teleporter.Info

	if vmType == models.SubnetEvm {
		// get vm version
		vmVersion := createFlags.vmVersion
		if createFlags.useLatestReleasedVMVersion {
			vmVersion = latest
		}
		if createFlags.useLatestPreReleasedVMVersion {
			vmVersion = preRelease
		}
		if vmVersion != latest && vmVersion != preRelease && vmVersion != "" && !semver.IsValid(vmVersion) {
			return fmt.Errorf("invalid version string, should be semantic version (ex: v1.1.1): %s", vmVersion)
		}
		vmVersion, err = vm.PromptVMVersion(app, constants.SubnetEVMRepoName, vmVersion)
		if err != nil {
			return err
		}

		if genesisFile != "" {
			// load given genesis
			if evmCompatibleGenesis, err := utils.FileIsSubnetEVMGenesis(genesisFile); err != nil {
				return err
			} else if !evmCompatibleGenesis {
				return fmt.Errorf("the provided genesis file has no proper Subnet-EVM format")
			}
			ux.Logger.PrintToUser("importing genesis for subnet %s", subnetName)
			genesisBytes, err = os.ReadFile(genesisFile)
			if err != nil {
				return err
			}
		} else {
			var useTeleporterFlag *bool
			flagName := "teleporter"
			if flag := cmd.Flags().Lookup(flagName); flag != nil && flag.Changed {
				useTeleporterFlag = &createFlags.useTeleporter
			}
			params, tokenSymbol, err := vm.PromptSubnetEVMGenesisParams(
				app,
				vmVersion,
				createFlags.chainID,
				createFlags.tokenSymbol,
				useTeleporterFlag,
				createFlags.useDefaults,
				createFlags.useWarp,
			)
			if err != nil {
				return err
			}

			if params.UseTeleporter || params.UseExternalGasToken {
				teleporterInfo, err = teleporter.GetInfo(app)
				if err != nil {
					return err
				}
			}

			genesisBytes, sc, err = vm.CreateEvmSubnetConfig(
				app,
				subnetName,
				genesisFile,
				vmVersion,
				true,
				createFlags.chainID,
				tokenSymbol,
				createFlags.useDefaults,
				createFlags.useWarp,
				teleporterInfo,
			)
			if err != nil {
				return err
			}
		}
	} else {
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
	}

	if teleporterInfo != nil {
		sc.TeleporterReady = true
		sc.TeleporterKey = constants.TeleporterKeyName
		sc.TeleporterVersion = teleporterInfo.Version
		if genesisFile != "" {
			if evmCompatibleGenesis, err := utils.FileIsSubnetEVMGenesis(genesisFile); err != nil {
				return err
			} else if !evmCompatibleGenesis {
				// evm genesis file was given. make appropriate checks and customizations for teleporter
				genesisBytes, err = addSubnetEVMGenesisPrefundedAddress(genesisBytes, teleporterInfo.FundedAddress, teleporterInfo.FundedBalance.String())
				if err != nil {
					return err
				}
			}
		}
	}

	if err = app.WriteGenesisFile(subnetName, genesisBytes); err != nil {
		return err
	}

	sc.ImportedFromAPM = false
	if err = app.CreateSidecar(sc); err != nil {
		return err
	}
	if vmType == models.SubnetEvm {
		err = sendMetrics(cmd, vmType.RepoName(), subnetName)
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
	metrics.HandleTracking(cmd, constants.MetricsSubnetCreateCommand, app, flags)
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
