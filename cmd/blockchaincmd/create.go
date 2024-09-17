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

	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"

	"github.com/ava-labs/avalanche-cli/pkg/application"

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
	proofOfStake                  bool
	proofOfAuthority              bool
	validatorManagerMintOnly      bool
	tokenMinterAddress            []string
	validatorManagerController    []string
	bootstrapValidators           []models.SubnetValidator
}

var (
	createFlags CreateFlags
	forceCreate bool
	genesisFile string
	vmFile      string
	useRepo     bool

	errIllegalNameCharacter = errors.New(
		"illegal name character: only letters, no special characters allowed")
	errMutuallyExlusiveVersionOptions             = errors.New("version flags --latest,--pre-release,vm-version are mutually exclusive")
	errMutuallyExclusiveVMConfigOptions           = errors.New("--genesis flag disables --evm-chain-id,--evm-defaults,--production-defaults,--test-defaults")
	errMutuallyExlusiveValidatorManagementOptions = errors.New("validator management type flags --proof-of-authority,--proof-of-stake are mutually exclusive")
	errTokenMinterAddressConflict                 = errors.New("--validator-manager-mint-only means that no additional addresses can be provided in --token-minter-address")
	errTokenMinterAddressForPoS                   = errors.New("--token-minter-address is only applicable to proof of authority")
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
	cmd.Flags().BoolVar(&createFlags.proofOfAuthority, "proof-of-authority", false, "use proof of authority for validator management")
	cmd.Flags().BoolVar(&createFlags.proofOfStake, "proof-of-stake", false, "use proof of stake for validator management")
	cmd.Flags().BoolVar(&createFlags.validatorManagerMintOnly, "validator-manager-mint-only", false, "only enable validator manager contract to mint new native tokens")
	cmd.Flags().StringSliceVar(&createFlags.tokenMinterAddress, "token-minter-address", nil, "addresses that can mint new native tokens (for proof of authority validator management only)")
	cmd.Flags().StringSliceVar(&createFlags.validatorManagerController, "validator-manager-controller", nil, "addresses that will control Validator Manager contract")
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

	if createFlags.proofOfAuthority {
		return errMutuallyExlusiveValidatorManagementOptions
	}

	if len(createFlags.tokenMinterAddress) > 0 {
		if createFlags.proofOfStake {
			return errTokenMinterAddressForPoS
		}
		if createFlags.validatorManagerMintOnly {
			return errTokenMinterAddressConflict
		}
	}

	//if len(createFlags.bootstrapValidatorInitialBalance) > 0 {
	//	for _, balance := range createFlags.bootstrapValidatorInitialBalance {
	//		if balance < constants.MinInitialBalanceBootstrapValidator {
	//			return fmt.Errorf("initial bootstrap validator balance must be at least %d AVAX", constants.MinInitialBalanceBootstrapValidator)
	//		}
	//	}
	//}

	// get vm kind
	vmType, err := vm.PromptVMType(app, createFlags.useSubnetEvm, createFlags.useCustomVM)
	if err != nil {
		return err
	}

	var (
		genesisBytes        []byte
		sc                  *models.Sidecar
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

	if vmType == models.SubnetEvm {
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
				blockchainName,
				params,
				teleporterInfo,
			)
			if err != nil {
				return err
			}
		}
		sc, err = vm.CreateEvmSidecar(
			app,
			blockchainName,
			vmVersion,
			tokenSymbol,
			true,
		)
		if err != nil {
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
		sc, err = vm.CreateCustomSidecar(
			app,
			blockchainName,
			useRepo,
			customVMRepoURL,
			customVMBranch,
			customVMBuildScript,
			vmFile,
			tokenSymbol,
		)
		if err != nil {
			return err
		}
	}

	if deployTeleporter || useExternalGasToken {
		sc.TeleporterReady = true
		sc.RunRelayer = true // TODO: remove this once deploy asks if deploying relayer
		sc.ExternalToken = useExternalGasToken
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
	if err = promptValidatorManagementType(app, sc); err != nil {
		return err
	}
	if sc.ValidatorManagement == models.ProofOfAuthority {
		if !createFlags.validatorManagerMintOnly && createFlags.tokenMinterAddress == nil {
			createFlags.tokenMinterAddress, err = getTokenMinterAddr()
			if err != nil {
				return err
			}
		}
	}
	if !createFlags.validatorManagerMintOnly {
		if len(createFlags.tokenMinterAddress) > 0 {
			ux.Logger.GreenCheckmarkToUser("Addresses added as new native token minter %s", createFlags.tokenMinterAddress)
		} else {
			ux.Logger.GreenCheckmarkToUser("No additional addresses added as new native token minter")
		}
	}
	sc.NewNativeTokenMinter = createFlags.tokenMinterAddress
	if createFlags.validatorManagerController == nil {
		var cancelled bool
		createFlags.validatorManagerController, cancelled, err = getValidatorContractManagerAddr()
		if err != nil {
			return err
		}
		if cancelled {
			return fmt.Errorf("user cancelled operation")
		}
	}
	sc.ValidatorManagerController = createFlags.validatorManagerController
	// TODO: add description of what Validator Manager Contract controller does
	ux.Logger.GreenCheckmarkToUser("Validator Manager Contract controller %s", createFlags.validatorManagerController)
	if err = app.WriteGenesisFile(blockchainName, genesisBytes); err != nil {
		return err
	}

	bootstrapValidators, err := promptBootstrapValidators()
	if err != nil {
		return err
	}
	sc.BootstrapValidators = bootstrapValidators

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
	return nil
}

func getValidatorContractManagerAddr() ([]string, bool, error) {
	controllerAddrPrompt := "Enter Validator Manager Contract controller address"
	for {
		// ask in a loop so that if some condition is not met we can keep asking
		controlAddr, cancelled, err := getAddrLoop(controllerAddrPrompt, constants.ValidatorManagerController, models.UndefinedNetwork)
		if err != nil {
			return nil, false, err
		}
		if cancelled {
			return nil, cancelled, nil
		}
		if len(controlAddr) != 0 {
			return controlAddr, false, nil
		}
		ux.Logger.RedXToUser("An address to control Validator Manage Contract is required before proceeding")
	}
}

// Configure which addresses may make mint new native tokens
func getTokenMinterAddr() ([]string, error) {
	addTokenMinterAddrPrompt := "Currently only Validator Manager Contract can mint new native tokens"
	ux.Logger.PrintToUser(addTokenMinterAddrPrompt)
	yes, err := app.Prompt.CaptureNoYes("Add additional addresses that can mint new native tokens?")
	if err != nil {
		return nil, err
	}
	if !yes {
		return nil, nil
	}
	addr, cancelled, err := getAddr()
	if err != nil {
		return nil, err
	}
	if cancelled {
		return nil, nil
	}
	return addr, nil
}

func getAddr() ([]string, bool, error) {
	addrPrompt := "Enter addresses that can mint new native tokens"
	addr, cancelled, err := getAddrLoop(addrPrompt, constants.TokenMinter, models.UndefinedNetwork)
	if err != nil {
		return nil, false, err
	}
	if cancelled {
		return nil, cancelled, nil
	}
	return addr, false, nil
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

// TODO: find the min weight for bootstrap validator
func PromptWeightBootstrapValidator() (uint64, error) {
	txt := "What stake weight would you like to assign to the validator?"
	return app.Prompt.CaptureWeight(txt)
}

func PromptInitialBalance() (uint64, error) {
	defaultInitialBalance := fmt.Sprintf("Default (%d AVAX)", constants.MinInitialBalanceBootstrapValidator)
	txt := "What initial balance would you like to assign to the bootstrap validator (in AVAX)?"
	weightOptions := []string{defaultInitialBalance, "Custom"}
	weightOption, err := app.Prompt.CaptureList(txt, weightOptions)
	if err != nil {
		return 0, err
	}

	switch weightOption {
	case defaultInitialBalance:
		return constants.MinInitialBalanceBootstrapValidator, nil
	default:
		return app.Prompt.CaptureBootstrapInitialBalance(txt)
	}
}

func promptBootstrapValidators() ([]models.SubnetValidator, error) {
	var subnetValidators []models.SubnetValidator
	numBootstrapValidators, err := app.Prompt.CaptureInt(
		"How many bootstrap validators do you want to set up?",
	)
	if err != nil {
		return nil, err
	}
	previousAddr := ""
	for len(subnetValidators) < numBootstrapValidators {
		ux.Logger.PrintToUser("Getting info for bootstrap validator %d", len(subnetValidators)+1)
		nodeID, err := PromptNodeID()
		if err != nil {
			return nil, err
		}
		weight, err := PromptWeightBootstrapValidator()
		if err != nil {
			return nil, err
		}
		balance, err := PromptInitialBalance()
		if err != nil {
			return nil, err
		}
		proofOfPossession, err := promptProofOfPossession()
		changeAddr, err := getKeyForChangeOwner(previousAddr)
		if err != nil {
			return nil, err
		}
		addrs, err := address.ParseToIDs([]string{changeAddr})
		if err != nil {
			return nil, fmt.Errorf("failure parsing change owner address: %w", err)
		}
		changeOwner := &secp256k1fx.OutputOwners{
			Threshold: 1,
			Addrs:     addrs,
		}
		previousAddr = changeAddr
		subnetValidator := models.SubnetValidator{
			NodeID:      nodeID,
			Weight:      weight,
			Balance:     balance,
			Signer:      proofOfPossession,
			ChangeOwner: changeOwner,
		}
		subnetValidators = append(subnetValidators, subnetValidator)
		ux.Logger.GreenCheckmarkToUser("Bootstrap Validator %d:", len(subnetValidators))
		ux.Logger.PrintToUser("- Node ID: %s", nodeID)
		ux.Logger.PrintToUser("- Weight: %d", weight)
		ux.Logger.PrintToUser("- Initial Balance: %d AVAX", balance)
		ux.Logger.PrintToUser("- Change Address: %s", changeAddr)
	}
	return subnetValidators, nil
}

func validateProofOfPossession(publicKey, pop string) {
	if publicKey != "" {
		err := prompts.ValidateHexa(publicKey)
		if err != nil {
			ux.Logger.PrintToUser("Format error in given public key: %s", err)
			publicKey = ""
		}
	}
	if pop != "" {
		err := prompts.ValidateHexa(pop)
		if err != nil {
			ux.Logger.PrintToUser("Format error in given proof of possession: %s", err)
			pop = ""
		}
	}
}

func promptProofOfPossession() (signer.Signer, error) {
	ux.Logger.PrintToUser("Next, we need the public key and proof of possession of the node's BLS")
	ux.Logger.PrintToUser("Check https://docs.avax.network/api-reference/info-api#infogetnodeid for instructions on calling info.getNodeID API")
	var err error
	txt := "What is the public key of the node's BLS?"
	publicKey, err := app.Prompt.CaptureValidatedString(txt, prompts.ValidateHexa)
	if err != nil {
		return nil, err
	}
	txt = "What is the proof of possession of the node's BLS?"
	proofOfPossesion, err := app.Prompt.CaptureValidatedString(txt, prompts.ValidateHexa)
	if err != nil {
		return nil, err
	}
	var proofOfPossession signer.Signer
	pop := &signer.ProofOfPossession{
		PublicKey:         [48]byte([]byte(publicKey)),
		ProofOfPossession: [96]byte([]byte(proofOfPossesion)),
	}
	proofOfPossession = pop
	return proofOfPossession, nil
}

// TODO: add explain the difference for different validator management type
func promptValidatorManagementType(
	app *application.Avalanche,
	sidecar *models.Sidecar,
) error {
	proofOfAuthorityOption := "Proof of Authority"
	proofOfStakeOption := "Proof of Stake"
	explainOption := "Explain the difference"
	if createFlags.proofOfStake {
		sidecar.ValidatorManagement = models.ValidatorManagementTypeFromString(proofOfStakeOption)
		return nil
	}
	if createFlags.proofOfAuthority {
		sidecar.ValidatorManagement = models.ValidatorManagementTypeFromString(proofOfAuthorityOption)
		return nil
	}
	options := []string{proofOfAuthorityOption, proofOfStakeOption, explainOption}
	var subnetTypeStr string
	for {
		option, err := app.Prompt.CaptureList(
			"Which validator management protocol would you like to use in your blockchain?",
			options,
		)
		if err != nil {
			return err
		}
		switch option {
		case proofOfAuthorityOption:
			subnetTypeStr = models.ProofOfAuthority
		case proofOfStakeOption:
			subnetTypeStr = models.ProofOfStake
		case explainOption:
			continue
		}
		break
	}
	sidecar.ValidatorManagement = models.ValidatorManagementTypeFromString(subnetTypeStr)
	return nil
}
