// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

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
	useTestDefaults               bool
	useProductionDefaults         bool
	useWarp                       bool
	useTeleporter                 bool
	vmVersion                     string
	useLatestReleasedVMVersion    bool
	useLatestPreReleasedVMVersion bool
	useExternalGasToken           bool
	addICMRegistryToGenesis       bool
	proofOfStake                  bool
	proofOfAuthority              bool
	poaValidatorManagerOwner      string
}

var (
	createFlags             CreateFlags
	forceCreate             bool
	genesisFile             string
	vmFile                  string
	useRepo                 bool
	errIllegalNameCharacter = errors.New(
		"illegal name character: only letters, no special characters allowed")
	errMutuallyExlusiveVersionOptions             = errors.New("version flags --latest,--pre-release,vm-version are mutually exclusive")
	errMutuallyExclusiveVMConfigOptions           = errors.New("--genesis flag disables --evm-chain-id,--evm-defaults,--production-defaults,--test-defaults")
	errMutuallyExlusiveValidatorManagementOptions = errors.New("validator management type flags --proof-of-authority,--proof-of-stake are mutually exclusive")
)

// avalanche blockchain create
func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [blockchainName]",
		Short: "Create a new blockchain configuration",
		Long: `The blockchain create command builds a new genesis file to configure your Blockchain.
By default, the command runs an interactive wizard. It walks you through
all the steps you need to create your first Blockchain.

The tool supports deploying Subnet-EVM, and custom VMs. You
can create a custom, user-generated genesis with a custom VM by providing
the path to your genesis and VM binaries with the --genesis and --vm flags.

By default, running the command with a blockchainName that already exists
causes the command to fail. If you'd like to overwrite an existing
configuration, pass the -f flag.`,
		Args:              cobrautils.ExactArgs(1),
		RunE:              createBlockchainConfig,
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
	cmd.Flags().BoolVar(&createFlags.useProductionDefaults, "evm-defaults", false, "deprecation notice: use '--production-defaults'")
	cmd.Flags().BoolVar(&createFlags.useProductionDefaults, "production-defaults", false, "use default production settings for your blockchain")
	cmd.Flags().BoolVar(&createFlags.useTestDefaults, "test-defaults", false, "use default test settings for your blockchain")
	cmd.Flags().BoolVarP(&forceCreate, forceFlag, "f", false, "overwrite the existing configuration if one exists")
	cmd.Flags().StringVar(&vmFile, "vm", "", "file path of custom vm to use. alias to custom-vm-path")
	cmd.Flags().StringVar(&vmFile, "custom-vm-path", "", "file path of custom vm to use")
	cmd.Flags().StringVar(&customVMRepoURL, "custom-vm-repo-url", "", "custom vm repository url")
	cmd.Flags().StringVar(&customVMBranch, "custom-vm-branch", "", "custom vm branch or commit")
	cmd.Flags().StringVar(&customVMBuildScript, "custom-vm-build-script", "", "custom vm build-script")
	cmd.Flags().BoolVar(&useRepo, "from-github-repo", false, "generate custom VM binary from github repository")
	cmd.Flags().BoolVar(&createFlags.useWarp, "warp", true, "generate a vm with warp support (needed for teleporter)")
	cmd.Flags().BoolVar(&createFlags.useTeleporter, "teleporter", false, "interoperate with other blockchains using teleporter")
	cmd.Flags().BoolVar(&createFlags.useExternalGasToken, "external-gas-token", false, "use a gas token from another blockchain")
	cmd.Flags().BoolVar(&createFlags.addICMRegistryToGenesis, "icm-registry-at-genesis", false, "setup ICM registry smart contract on genesis [experimental]")
	cmd.Flags().BoolVar(&createFlags.proofOfAuthority, "proof-of-authority", false, "use proof of authority for validator management")
	cmd.Flags().BoolVar(&createFlags.proofOfStake, "proof-of-stake", false, "(coming soon) use proof of stake for validator management")
	cmd.Flags().StringVar(&createFlags.poaValidatorManagerOwner, "poa-manager-owner", "", "EVM address that controls Validator Manager Owner (for Proof of Authority only)")
	return cmd
}

func CallCreate(
	cmd *cobra.Command,
	blockchainName string,
	forceCreateParam bool,
	genesisFileParam string,
	useSubnetEvmParam bool,
	useCustomParam bool,
	vmVersionParam string,
	evmChainIDParam uint64,
	tokenSymbolParam string,
	useProductionDefaultsParam bool,
	useTestDefaultsParam bool,
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
	createFlags.useProductionDefaults = useProductionDefaultsParam
	createFlags.useTestDefaults = useTestDefaultsParam
	createFlags.useLatestReleasedVMVersion = useLatestReleasedVMVersionParam
	createFlags.useLatestPreReleasedVMVersion = useLatestPreReleasedVMVersionParam
	createFlags.useCustomVM = useCustomParam
	customVMRepoURL = customVMRepoURLParam
	customVMBranch = customVMBranchParam
	customVMBuildScript = customVMBuildScriptParam
	return createBlockchainConfig(cmd, []string{blockchainName})
}

// override postrun function from root.go, so that we don't double send metrics for the same command
func handlePostRun(_ *cobra.Command, _ []string) {}

func createBlockchainConfig(cmd *cobra.Command, args []string) error {
	blockchainName := args[0]

	if app.GenesisExists(blockchainName) && !forceCreate {
		return errors.New("configuration already exists. Use --" + forceFlag + " parameter to overwrite")
	}

	if err := checkInvalidSubnetNames(blockchainName); err != nil {
		return fmt.Errorf("subnet name %q is invalid: %w", blockchainName, err)
	}

	// version flags exclusiveness
	if !flags.EnsureMutuallyExclusive([]bool{
		createFlags.useLatestReleasedVMVersion,
		createFlags.useLatestPreReleasedVMVersion,
		createFlags.vmVersion != "",
	}) {
		return errMutuallyExlusiveVersionOptions
	}

	defaultsKind := vm.NoDefaults
	if createFlags.useTestDefaults {
		defaultsKind = vm.TestDefaults
	}
	if createFlags.useProductionDefaults {
		defaultsKind = vm.ProductionDefaults
	}

	// genesis flags exclusiveness
	if genesisFile != "" && (createFlags.chainID != 0 || defaultsKind != vm.NoDefaults) {
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

	// validator management type exclusiveness
	if !flags.EnsureMutuallyExclusive([]bool{createFlags.proofOfAuthority, createFlags.proofOfStake}) {
		return errMutuallyExlusiveValidatorManagementOptions
	}

	// get vm kind
	vmType, err := vm.PromptVMType(app, createFlags.useSubnetEvm, createFlags.useCustomVM)
	if err != nil {
		return err
	}

	var (
		genesisBytes        []byte
		useTeleporterFlag   *bool
		deployTeleporter    bool
		useExternalGasToken bool
	)

	// get teleporter flag as a pointer (3 values: undef/true/false)
	flagName := "teleporter"
	if flag := cmd.Flags().Lookup(flagName); flag != nil && flag.Changed {
		useTeleporterFlag = &createFlags.useTeleporter
	}

	// get teleporter info
	teleporterInfo, err := teleporter.GetInfo(app)
	if err != nil {
		return err
	}

	sc := &models.Sidecar{}

	if err = promptValidatorManagementType(app, sc); err != nil {
		return err
	}

	if !sc.PoA() && createFlags.poaValidatorManagerOwner != "" {
		return errors.New("--poa-manager-owner flag cannot be used when blockchain validator management type is not Proof of Authority")
	}

	if vmType == models.SubnetEvm {
		if sc.PoA() {
			if createFlags.poaValidatorManagerOwner == "" {
				createFlags.poaValidatorManagerOwner, err = getValidatorContractManagerAddr()
				if err != nil {
					return err
				}
			}
			sc.PoAValidatorManagerOwner = createFlags.poaValidatorManagerOwner
			ux.Logger.GreenCheckmarkToUser("Validator Manager Contract owner address %s", createFlags.poaValidatorManagerOwner)
		}

		if genesisFile == "" {
			// Default
			defaultsKind, err = vm.PromptDefaults(app, defaultsKind)
			if err != nil {
				return err
			}
		}

		// get vm version
		vmVersion := createFlags.vmVersion
		if createFlags.useLatestReleasedVMVersion || defaultsKind != vm.NoDefaults {
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

		var tokenSymbol string

		if genesisFile != "" {
			if evmCompatibleGenesis, err := utils.FileIsSubnetEVMGenesis(genesisFile); err != nil {
				return err
			} else if !evmCompatibleGenesis {
				return fmt.Errorf("the provided genesis file has no proper Subnet-EVM format")
			}
			tokenSymbol, err = vm.PromptTokenSymbol(app, createFlags.tokenSymbol)
			if err != nil {
				return err
			}
			deployTeleporter, err = vm.PromptInterop(app, useTeleporterFlag, defaultsKind, false)
			if err != nil {
				return err
			}
			ux.Logger.PrintToUser("importing genesis for blockchain %s", blockchainName)
			genesisBytes, err = os.ReadFile(genesisFile)
			if err != nil {
				return err
			}
		} else {
			var params vm.SubnetEVMGenesisParams
			params, tokenSymbol, err = vm.PromptSubnetEVMGenesisParams(
				app,
				sc,
				vmVersion,
				createFlags.chainID,
				createFlags.tokenSymbol,
				blockchainName,
				useTeleporterFlag,
				defaultsKind,
				createFlags.useWarp,
				createFlags.useExternalGasToken,
			)
			if err != nil {
				return err
			}
			deployTeleporter = params.UseTeleporter
			useExternalGasToken = params.UseExternalGasToken
			genesisBytes, err = vm.CreateEVMGenesis(
				params,
				teleporterInfo,
				createFlags.addICMRegistryToGenesis,
			)
			if err != nil {
				return err
			}
		}
		if sc, err = vm.CreateEvmSidecar(
			sc,
			app,
			blockchainName,
			vmVersion,
			tokenSymbol,
			true,
		); err != nil {
			return err
		}
	} else {
		genesisBytes, err = vm.LoadCustomGenesis(app, genesisFile)
		if err != nil {
			return err
		}
		var tokenSymbol string
		if evmCompatibleGenesis := utils.ByteSliceIsSubnetEvmGenesis(genesisBytes); evmCompatibleGenesis {
			tokenSymbol, err = vm.PromptTokenSymbol(app, createFlags.tokenSymbol)
			if err != nil {
				return err
			}
			deployTeleporter, err = vm.PromptInterop(app, useTeleporterFlag, defaultsKind, false)
			if err != nil {
				return err
			}
		}
		if sc, err = vm.CreateCustomSidecar(
			sc,
			app,
			blockchainName,
			useRepo,
			customVMRepoURL,
			customVMBranch,
			customVMBuildScript,
			vmFile,
			tokenSymbol,
		); err != nil {
			return err
		}
	}

	if deployTeleporter || useExternalGasToken {
		sc.TeleporterReady = true
		sc.RunRelayer = true // TODO: remove this once deploy asks if deploying relayer
		sc.ExternalToken = useExternalGasToken
		sc.TeleporterKey = constants.ICMKeyName
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

	if err = app.WriteGenesisFile(blockchainName, genesisBytes); err != nil {
		return err
	}

	if err = app.CreateSidecar(sc); err != nil {
		return err
	}
	if vmType == models.SubnetEvm {
		err = sendMetrics(cmd, vmType.RepoName(), blockchainName)
		if err != nil {
			return err
		}
	}
	ux.Logger.GreenCheckmarkToUser("Successfully created blockchain configuration")
	ux.Logger.PrintToUser("Run 'avalanche blockchain describe' to view all created addresses and what their roles are")
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

func sendMetrics(cmd *cobra.Command, repoName, blockchainName string) error {
	flags := make(map[string]string)
	flags[constants.SubnetType] = repoName
	genesis, err := app.LoadEvmGenesis(blockchainName)
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
