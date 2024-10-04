// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/spf13/cobra"
)

// avalanche l1 convert
func newConvertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert [l1Name]",
		Short: "Converts an Avalanche L1 into a SOV (Subnet Only Validator) L1",
		Long: `The l1 convert command converts a non-SOV Avalanche L1 (which requires subnet 
validators to have at least 2000 AVAX staked in the Primary Network) into an SOV (Subnet Only 
Validator) L1.

Once an L1 is successfully converted into a an SOV, the Owner Keys can no longer be used to modify 
the L1's validator set. In addition, AddSubnetValidatorTx is disabled on the Subnet going forward. 
The only action that the Owner key is able to take is removing any L1 validators that were added 
using AddSubnetValidatorTx previously via RemoveSubnetValidatorTx. 

Unless removed by the Owner key, any Subnet Validators added previously with an AddSubnetValidatorTx 
will continue to validate the Subnet until their End time is reached. Once all Subnet Validators 
added with AddSubnetValidatorTx are no longer in the validator set, the Owner key is powerless. 
RegisterL1ValidatorTx and SetL1ValidatorWeightTx must be used to manage the Subnet's 
validator set going forward.`,
		RunE:              convertL1,
		PersistentPostRun: handlePostRun,
		Args:              cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, deploySupportedNetworkOptions)
	cmd.Flags().StringVar(&userProvidedAvagoVersion, "avalanchego-version", "latest", "use this version of avalanchego (ex: v1.17.12)")
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet deploy only]")
	cmd.Flags().BoolVarP(&sameControlKey, "same-control-key", "s", false, "use the fee-paying key as control key")
	cmd.Flags().Uint32Var(&threshold, "threshold", 0, "required number of control key signatures to make subnet changes")
	cmd.Flags().StringSliceVar(&controlKeys, "control-keys", nil, "addresses that may make subnet changes")
	cmd.Flags().StringSliceVar(&subnetAuthKeys, "subnet-auth-keys", nil, "control keys that will be used to authenticate chain creation")
	cmd.Flags().StringVar(&outputTxPath, "output-tx-path", "", "file path of the blockchain creation tx")
	cmd.Flags().BoolVarP(&useEwoq, "ewoq", "e", false, "use ewoq key [fuji/devnet deploy only]")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	cmd.Flags().StringVarP(&subnetIDStr, "subnet-id", "u", "", "do not create a subnet, deploy the blockchain into the given subnet id")
	cmd.Flags().Uint32Var(&mainnetChainID, "mainnet-chain-id", 0, "use different ChainID for mainnet deployment")
	cmd.Flags().StringVar(&avagoBinaryPath, "avalanchego-path", "", "use this avalanchego binary path")
	cmd.Flags().BoolVar(&subnetOnly, "subnet-only", false, "only create a subnet")
	cmd.Flags().BoolVar(&icmSpec.SkipICMDeploy, "skip-local-teleporter", false, "skip automatic teleporter deploy on local networks [to be deprecated]")
	cmd.Flags().BoolVar(&icmSpec.SkipICMDeploy, "skip-teleporter-deploy", false, "skip automatic teleporter deploy")
	cmd.Flags().BoolVar(&icmSpec.SkipRelayerDeploy, "skip-relayer", false, "skip relayer deploy")
	cmd.Flags().StringVar(&icmSpec.ICMVersion, "teleporter-version", "latest", "teleporter version to deploy")
	cmd.Flags().StringVar(&icmSpec.RelayerVersion, "relayer-version", "latest", "relayer version to deploy")
	cmd.Flags().StringVar(&icmSpec.MessengerContractAddressPath, "teleporter-messenger-contract-address-path", "", "path to an interchain messenger contract address file")
	cmd.Flags().StringVar(&icmSpec.MessengerDeployerAddressPath, "teleporter-messenger-deployer-address-path", "", "path to an interchain messenger deployer address file")
	cmd.Flags().StringVar(&icmSpec.MessengerDeployerTxPath, "teleporter-messenger-deployer-tx-path", "", "path to an interchain messenger deployer tx file")
	cmd.Flags().StringVar(&icmSpec.RegistryBydecodePath, "teleporter-registry-bytecode-path", "", "path to an interchain messenger registry bytecode file")
	cmd.Flags().StringVar(&bootstrapValidatorsJSONFilePath, "bootstrap-filepath", "", "JSON file path that provides details about bootstrap validators, leave Node-ID and BLS values empty if using --generate-node-id=true")
	cmd.Flags().BoolVar(&generateNodeID, "generate-node-id", false, "whether to create new node id for bootstrap validators (Node-ID and BLS values in bootstrap JSON file will be overridden if --bootstrap-filepath flag is used)")
	return cmd
}

// // convertL1 is the cobra command run for deploying subnets
func convertL1(cmd *cobra.Command, args []string) error {
	//blockchainName := args[0]
	//
	//if err := CreateBlockchainFirst(cmd, blockchainName, skipCreatePrompt); err != nil {
	//	return err
	//}
	//
	//chains, err := ValidateSubnetNameAndGetChains(args)
	//if err != nil {
	//	return err
	//}
	//
	//if icmSpec.MessengerContractAddressPath != "" || icmSpec.MessengerDeployerAddressPath != "" || icmSpec.MessengerDeployerTxPath != "" || icmSpec.RegistryBydecodePath != "" {
	//	if icmSpec.MessengerContractAddressPath == "" || icmSpec.MessengerDeployerAddressPath == "" || icmSpec.MessengerDeployerTxPath == "" || icmSpec.RegistryBydecodePath == "" {
	//		return fmt.Errorf("if setting any teleporter asset path, you must set all teleporter asset paths")
	//	}
	//}
	//
	//var bootstrapValidators []models.SubnetValidator
	//if bootstrapValidatorsJSONFilePath != "" {
	//	bootstrapValidators, err = LoadBootstrapValidator(bootstrapValidatorsJSONFilePath)
	//	if err != nil {
	//		return err
	//	}
	//}
	//
	//chain := chains[0]
	//
	//sidecar, err := app.LoadSidecar(chain)
	//if err != nil {
	//	return fmt.Errorf("failed to load sidecar for later update: %w", err)
	//}
	//
	//if sidecar.ImportedFromAPM {
	//	return errors.New("unable to deploy subnets imported from a repo")
	//}
	//
	//if outputTxPath != "" {
	//	if _, err := os.Stat(outputTxPath); err == nil {
	//		return fmt.Errorf("outputTxPath %q already exists", outputTxPath)
	//	}
	//}
	//
	//network, err := networkoptions.GetNetworkFromCmdLineFlags(
	//	app,
	//	"",
	//	globalNetworkFlags,
	//	true,
	//	false,
	//	deploySupportedNetworkOptions,
	//	"",
	//)
	//if err != nil {
	//	return err
	//}
	//
	//isEVMGenesis, validationErr, err := app.HasSubnetEVMGenesis(chain)
	//if err != nil {
	//	return err
	//}
	//if sidecar.VM == models.SubnetEvm && !isEVMGenesis {
	//	return fmt.Errorf("failed to validate SubnetEVM genesis format: %w", validationErr)
	//}
	//
	//chainGenesis, err := app.LoadRawGenesis(chain)
	//if err != nil {
	//	return err
	//}
	//
	//if isEVMGenesis {
	//	// is is a subnet evm or a custom vm based on subnet evm
	//	if network.Kind == models.Mainnet {
	//		err = getSubnetEVMMainnetChainID(&sidecar, chain)
	//		if err != nil {
	//			return err
	//		}
	//		chainGenesis, err = updateSubnetEVMGenesisChainID(chainGenesis, sidecar.SubnetEVMMainnetChainID)
	//		if err != nil {
	//			return err
	//		}
	//	}
	//	err = checkSubnetEVMDefaultAddressNotInAlloc(network, chain)
	//	if err != nil {
	//		return err
	//	}
	//}
	//
	//if bootstrapValidatorsJSONFilePath == "" {
	//	bootstrapValidators, err = promptBootstrapValidators(network)
	//	if err != nil {
	//		return err
	//	}
	//}
	//
	//ux.Logger.PrintToUser("Deploying %s to %s", chains, network.Name())
	//
	//if network.Kind == models.Local {
	//	app.Log.Debug("Deploy local")
	//
	//	genesisPath := app.GetGenesisPath(chain)
	//
	//	// copy vm binary to the expected location, first downloading it if necessary
	//	var vmBin string
	//	switch sidecar.VM {
	//	case models.SubnetEvm:
	//		_, vmBin, err = binutils.SetupSubnetEVM(app, sidecar.VMVersion)
	//		if err != nil {
	//			return fmt.Errorf("failed to install subnet-evm: %w", err)
	//		}
	//	case models.CustomVM:
	//		vmBin = binutils.SetupCustomBin(app, chain)
	//	default:
	//		return fmt.Errorf("unknown vm: %s", sidecar.VM)
	//	}
	//
	//	// check if selected version matches what is currently running
	//	nc := localnet.NewStatusChecker()
	//	avagoVersion, err := CheckForInvalidDeployAndGetAvagoVersion(nc, sidecar.RPCVersion)
	//	if err != nil {
	//		return err
	//	}
	//	if avagoBinaryPath == "" {
	//		userProvidedAvagoVersion = avagoVersion
	//	}
	//
	//	deployer := subnet.NewLocalDeployer(app, userProvidedAvagoVersion, avagoBinaryPath, vmBin)
	//	deployInfo, err := deployer.DeployToLocalNetwork(chain, genesisPath, icmSpec, subnetIDStr)
	//	if err != nil {
	//		if deployer.BackendStartedHere() {
	//			if innerErr := binutils.KillgRPCServerProcess(app); innerErr != nil {
	//				app.Log.Warn("tried to kill the gRPC server process but it failed", zap.Error(innerErr))
	//			}
	//		}
	//		return err
	//	}
	//	flags := make(map[string]string)
	//	flags[constants.MetricsNetwork] = network.Name()
	//	metrics.HandleTracking(cmd, constants.MetricsSubnetDeployCommand, app, flags)
	//	if err := app.UpdateSidecarNetworks(
	//		&sidecar,
	//		network,
	//		deployInfo.SubnetID,
	//		deployInfo.BlockchainID,
	//		deployInfo.ICMMessengerAddress,
	//		deployInfo.ICMRegistryAddress,
	//		bootstrapValidators,
	//	); err != nil {
	//		return err
	//	}
	//	return PrintSubnetInfo(blockchainName, true)
	//}
	//
	//// from here on we are assuming a public deploy
	//if subnetOnly && subnetIDStr != "" {
	//	return errMutuallyExlusiveSubnetFlags
	//}
	//
	//createSubnet := true
	//var subnetID ids.ID
	//if subnetIDStr != "" {
	//	subnetID, err = ids.FromString(subnetIDStr)
	//	if err != nil {
	//		return err
	//	}
	//	createSubnet = false
	//} else if !subnetOnly && sidecar.Networks != nil {
	//	model, ok := sidecar.Networks[network.Name()]
	//	if ok {
	//		if model.SubnetID != ids.Empty && model.BlockchainID == ids.Empty {
	//			subnetID = model.SubnetID
	//			createSubnet = false
	//		}
	//	}
	//}
	//
	//fee := uint64(0)
	//if !subnetOnly {
	//	fee += network.GenesisParams().TxFeeConfig.StaticFeeConfig.CreateBlockchainTxFee
	//}
	//if createSubnet {
	//	fee += network.GenesisParams().TxFeeConfig.StaticFeeConfig.CreateSubnetTxFee
	//}
	//
	//kc, err := keychain.GetKeychainFromCmdLineFlags(
	//	app,
	//	constants.PayTxsFeesMsg,
	//	network,
	//	keyName,
	//	useEwoq,
	//	useLedger,
	//	ledgerAddresses,
	//	fee,
	//)
	//if err != nil {
	//	return err
	//}
	//
	//network.HandlePublicNetworkSimulation()
	//
	//if createSubnet {
	//	controlKeys, threshold, err = promptOwners(
	//		kc,
	//		controlKeys,
	//		sameControlKey,
	//		threshold,
	//		subnetAuthKeys,
	//		true,
	//	)
	//	if err != nil {
	//		return err
	//	}
	//} else {
	//	ux.Logger.PrintToUser(logging.Blue.Wrap(
	//		fmt.Sprintf("Deploying into pre-existent subnet ID %s", subnetID.String()),
	//	))
	//	var isPermissioned bool
	//	isPermissioned, controlKeys, threshold, err = txutils.GetOwners(network, subnetID)
	//	if err != nil {
	//		return err
	//	}
	//	if !isPermissioned {
	//		return ErrNotPermissionedSubnet
	//	}
	//}
	//
	//// add control keys to the keychain whenever possible
	//if err := kc.AddAddresses(controlKeys); err != nil {
	//	return err
	//}
	//
	//kcKeys, err := kc.PChainFormattedStrAddresses()
	//if err != nil {
	//	return err
	//}
	//
	//// get keys for blockchain tx signing
	//if subnetAuthKeys != nil {
	//	if err := prompts.CheckSubnetAuthKeys(kcKeys, subnetAuthKeys, controlKeys, threshold); err != nil {
	//		return err
	//	}
	//} else {
	//	subnetAuthKeys, err = prompts.GetSubnetAuthKeys(app.Prompt, kcKeys, controlKeys, threshold)
	//	if err != nil {
	//		return err
	//	}
	//}
	//ux.Logger.PrintToUser("Your subnet auth keys for chain creation: %s", subnetAuthKeys)
	//
	//// deploy to public network
	//deployer := subnet.NewPublicDeployer(app, kc, network)
	//
	//if createSubnet {
	//	subnetID, err = deployer.DeploySubnet(controlKeys, threshold)
	//	if err != nil {
	//		return err
	//	}
	//	// get the control keys in the same order as the tx
	//	_, controlKeys, threshold, err = txutils.GetOwners(network, subnetID)
	//	if err != nil {
	//		return err
	//	}
	//}
	//
	//var (
	//	savePartialTx           bool
	//	blockchainID            ids.ID
	//	tx                      *txs.Tx
	//	remainingSubnetAuthKeys []string
	//	isFullySigned           bool
	//)
	//
	//if !subnetOnly {
	//	isFullySigned, blockchainID, tx, remainingSubnetAuthKeys, err = deployer.DeployBlockchain(
	//		controlKeys,
	//		subnetAuthKeys,
	//		subnetID,
	//		chain,
	//		chainGenesis,
	//	)
	//	if err != nil {
	//		ux.Logger.PrintToUser(logging.Red.Wrap(
	//			fmt.Sprintf("error deploying blockchain: %s. fix the issue and try again with a new deploy cmd", err),
	//		))
	//	}
	//
	//	savePartialTx = !isFullySigned && err == nil
	//}
	//
	//if err := PrintDeployResults(chain, subnetID, blockchainID); err != nil {
	//	return err
	//}
	//
	//if savePartialTx {
	//	if err := SaveNotFullySignedTx(
	//		"Blockchain Creation",
	//		tx,
	//		chain,
	//		subnetAuthKeys,
	//		remainingSubnetAuthKeys,
	//		outputTxPath,
	//		false,
	//	); err != nil {
	//		return err
	//	}
	//}
	//
	//// type ConvertSubnetTx struct {
	////		// Metadata, inputs and outputs
	////		BaseTx
	////		// ID of the Subnet to transform
	////		// Restrictions:
	////		// - Must not be the Primary Network ID
	////		Subnet ids.ID `json:"subnetID"`
	////		// BlockchainID where the Subnet manager lives
	////		ChainID ids.ID `json:"chainID"`
	////		// Address of the Subnet manager
	////		Address []byte `json:"address"`
	////		// Initial pay-as-you-go validators for the Subnet
	////		Validators []SubnetValidator `json:"validators"`
	////		// Authorizes this conversion
	////		SubnetAuth verify.Verifiable `json:"subnetAuthorization"`
	////	}
	//
	////avaGoBootstrapValidators, err := convertToAvalancheGoSubnetValidator(bootstrapValidators)
	////if err != nil {
	////	return err
	////}
	//// TODO: replace with avalanchego subnetValidators once implemented
	//isFullySigned, convertSubnetTxID, tx, remainingSubnetAuthKeys, err := deployer.ConvertL1(
	//	controlKeys,
	//	subnetAuthKeys,
	//	subnetID,
	//	blockchainID,
	//	// avaGoBootstrapValidators,
	//)
	//if err != nil {
	//	ux.Logger.PrintToUser(logging.Red.Wrap(
	//		fmt.Sprintf("error converting blockchain: %s. fix the issue and try again with a new convert cmd", err),
	//	))
	//}
	//
	//savePartialTx = !isFullySigned && err == nil
	//ux.Logger.PrintToUser("ConvertSubnetTx ID: %s", convertSubnetTxID)
	//
	//if savePartialTx {
	//	if err := SaveNotFullySignedTx(
	//		"ConvertSubnetTx",
	//		tx,
	//		chain,
	//		subnetAuthKeys,
	//		remainingSubnetAuthKeys,
	//		outputTxPath,
	//		false,
	//	); err != nil {
	//		return err
	//	}
	//}
	//
	//flags := make(map[string]string)
	//flags[constants.MetricsNetwork] = network.Name()
	//metrics.HandleTracking(cmd, constants.MetricsSubnetDeployCommand, app, flags)
	//
	//// update sidecar
	//// TODO: need to do something for backwards compatibility?
	//return app.UpdateSidecarNetworks(&sidecar, network, subnetID, blockchainID, "", "", bootstrapValidators)
	return nil
}
