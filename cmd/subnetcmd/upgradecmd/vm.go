// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/plugins"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ava-labs/avalanche-network-runner/server"
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/spf13/cobra"
)

const (
	futureDeployment  = "Update config for future deployments"
	localDeployment   = "Existing local deployment"
	fujiDeployment    = "Fuji"
	mainnetDeployment = "Mainnet (coming soon)"
)

var (
	pluginDir string

	useFuji       bool
	useMainnet    bool
	useLocal      bool
	useConfig     bool
	useManual     bool
	useLatest     bool
	targetVersion string
	binaryPathArg string
)

// avalanche subnet update vm
func newUpgradeVMCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "vm [subnetName]",
		Short:        "Upgrade a subnet's binary",
		Long:         "",
		RunE:         upgradeVM,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}

	cmd.Flags().BoolVar(&useConfig, "config", false, "upgrade config for future subnet deployments")
	cmd.Flags().BoolVar(&useLocal, "local", false, "upgrade existing `local` deployment")
	cmd.Flags().BoolVar(&useFuji, "fuji", false, "upgrade existing `fuji` deployment (alias for `testnet`)")
	cmd.Flags().BoolVar(&useFuji, "testnet", false, "upgrade existing `testnet` deployment (alias for `fuji`)")
	cmd.Flags().BoolVar(&useMainnet, "mainnet", false, "upgrade existing `mainnet` deployment")

	cmd.Flags().BoolVar(&useManual, "print", false, "print instructions for upgrading")
	cmd.Flags().StringVar(&pluginDir, "plugin-dir", "", "plugin directory to automatically upgrade VM")

	cmd.Flags().BoolVar(&useLatest, "latest", false, "upgrade to latest version")
	cmd.Flags().StringVar(&targetVersion, "version", "", "Upgrade to custom version")
	cmd.Flags().StringVar(&binaryPathArg, "binary", "", "Upgrade to custom binary")

	return cmd
}

func atMostOneNetworkSelected() bool {
	return !(useConfig && useLocal || useConfig && useFuji || useConfig && useMainnet || useLocal && useFuji ||
		useLocal && useMainnet || useFuji && useMainnet)
}

func atMostOneVersionSelected() bool {
	return !(useLatest && targetVersion != "" || useLatest && binaryPathArg != "" || targetVersion != "" && binaryPathArg != "")
}

func atMostOneAutomationSelected() bool {
	return !(useManual && pluginDir != "")
}

func upgradeVM(_ *cobra.Command, args []string) error {
	// Check flag preconditions
	if !atMostOneNetworkSelected() {
		return errors.New("too many networks selected")
	}

	if !atMostOneVersionSelected() {
		return errors.New("too many versions selected")
	}

	if !atMostOneAutomationSelected() {
		return errors.New("--print and --plugin-dir are mutually exclusive")
	}

	subnetName := args[0]

	if !app.SubnetConfigExists(subnetName) {
		return errors.New("subnet does not exist")
	}

	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return fmt.Errorf("unable to load sidecar: %w", err)
	}

	upgradeOptions := []string{futureDeployment}
	networkToUpgrade, err := selectNetworkToUpgrade(sc, upgradeOptions)
	if err != nil {
		return err
	}

	vmType := sc.VM
	if vmType == models.SubnetEvm || vmType == models.SpacesVM {
		return selectUpdateOption(vmType, sc, networkToUpgrade)
	}

	// Must be a custom update
	return updateToCustomBin(sc, networkToUpgrade, binaryPathArg)
}

// select which network to upgrade
// optionally provide a list of options to preload
func selectNetworkToUpgrade(sc models.Sidecar, upgradeOptions []string) (string, error) {
	switch {
	case useConfig:
		return futureDeployment, nil
	case useLocal:
		return localDeployment, nil
	case useFuji:
		return fujiDeployment, nil
	case useMainnet:
		return mainnetDeployment, nil
	}

	updatePrompt := "What deployment would you like to upgrade"
	if upgradeOptions == nil {
		upgradeOptions = []string{}
	}

	// check if subnet already deployed locally
	locallyDeployedSubnets, err := subnet.GetLocallyDeployedSubnets()
	if err != nil {
		// ignore error if we can't reach the server, assume subnet isn't deployed
		app.Log.Warn("Unable to reach server to get deployed subnets")
	}
	if _, ok := locallyDeployedSubnets[sc.Subnet]; ok {
		upgradeOptions = append(upgradeOptions, localDeployment)
	}

	// check if subnet deployed on fuji
	if _, ok := sc.Networks[models.Fuji.String()]; ok {
		upgradeOptions = append(upgradeOptions, fujiDeployment)
	}

	// check if subnet deployed on mainnet
	if _, ok := sc.Networks[models.Mainnet.String()]; ok {
		upgradeOptions = append(upgradeOptions, mainnetDeployment)
	}

	if len(upgradeOptions) == 0 {
		return "", errors.New("no deployment target available")
	}

	selectedDeployment, err := app.Prompt.CaptureList(updatePrompt, upgradeOptions)
	if err != nil {
		return "", err
	}
	return selectedDeployment, nil
}

func selectUpdateOption(vmType models.VMType, sc models.Sidecar, networkToUpgrade string) error {
	switch {
	case useLatest:
		return updateToLatestVersion(vmType, sc, networkToUpgrade)
	case targetVersion != "":
		return updateToSpecificVersion(sc, networkToUpgrade)
	case binaryPathArg != "":
		return updateToCustomBin(sc, networkToUpgrade, binaryPathArg)
	}

	latestVersionUpdate := "Update to latest version"
	specificVersionUpdate := "Update to a specific version"
	customBinaryUpdate := "Update to a custom binary"

	updateOptions := []string{latestVersionUpdate, specificVersionUpdate, customBinaryUpdate}

	updatePrompt := "How would you like to update your subnet's virtual machine"
	updateDecision, err := app.Prompt.CaptureList(updatePrompt, updateOptions)
	if err != nil {
		return err
	}

	switch updateDecision {
	case latestVersionUpdate:
		return updateToLatestVersion(vmType, sc, networkToUpgrade)
	case specificVersionUpdate:
		return updateToSpecificVersion(sc, networkToUpgrade)
	case customBinaryUpdate:
		return updateToCustomBin(sc, networkToUpgrade, binaryPathArg)
	default:
		return errors.New("invalid option")
	}
}

func updateToLatestVersion(vmType models.VMType, sc models.Sidecar, networkToUpgrade string) error {
	// pull in current version
	currentVersion := sc.VMVersion

	// check latest version
	latestVersion, err := app.Downloader.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
		constants.AvaLabsOrg,
		vmType.RepoName(),
	))
	if err != nil {
		return err
	}

	// check if current version equals latest
	if currentVersion == "latest" || currentVersion == latestVersion {
		ux.Logger.PrintToUser("VM already up-to-date")
		return nil
	}

	return updateVMByNetwork(sc, latestVersion, networkToUpgrade)
}

func updateToSpecificVersion(sc models.Sidecar, networkToUpgrade string) error {
	// pull in current version
	currentVersion := sc.VMVersion

	// Get version to update to
	var err error
	if targetVersion == "" {
		targetVersion, err = app.Prompt.CaptureVersion("Enter version")
		if err != nil {
			return err
		}
	}

	// check if current version equals chosen version
	if currentVersion == targetVersion {
		ux.Logger.PrintToUser("VM already up-to-date")
		return nil
	}

	return updateVMByNetwork(sc, targetVersion, networkToUpgrade)
}

func updateVMByNetwork(sc models.Sidecar, targetVersion string, networkToUpgrade string) error {
	switch networkToUpgrade {
	case futureDeployment:
		return updateFutureVM(sc, targetVersion)
	case localDeployment:
		return updateExistingLocalVM(sc, targetVersion)
	case fujiDeployment:
		return chooseManualOrAutomatic(sc, targetVersion, networkToUpgrade)
	case mainnetDeployment:
		return updateMainnetVM()
	default:
		return errors.New("unknown deployment")
	}
}

func updateToCustomBin(sc models.Sidecar, networkToUpgrade, binaryPath string) error {
	var err error
	if binaryPath == "" {
		binaryPath, err = app.Prompt.CaptureExistingFilepath("Enter path to custom binary")
		if err != nil {
			return err
		}
	}

	if err := vm.CopyCustomVM(app, sc.Name, binaryPath); err != nil {
		return err
	}

	sc.VM = models.CustomVM
	targetVersion = ""

	return updateVMByNetwork(sc, targetVersion, networkToUpgrade)
}

func updateFutureVM(sc models.Sidecar, targetVersion string) error {
	// to switch to new version, just need to update sidecar
	sc.VMVersion = targetVersion
	if err := app.UpdateSidecar(&sc); err != nil {
		return err
	}
	ux.Logger.PrintToUser("VM updated for future deployments. Update will apply next time subnet is deployed.")
	return nil
}

func updateExistingLocalVM(sc models.Sidecar, targetVersion string) error {
	// check network has been stopped
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return err
	}
	ctx := binutils.GetAsyncContext()

	_, err = cli.Status(ctx)

	if err == nil || !server.IsServerError(err, server.ErrNotBootstrapped) {
		ux.Logger.PrintToUser("Please stop network before upgrading local VMs")
	}

	vmid, err := utils.VMID(sc.Name)
	if err != nil {
		return err
	}
	var vmBin string
	switch sc.VM {
	// download the binary and prepare to copy it
	case models.SubnetEvm:
		vmBin, err = binutils.SetupSubnetEVM(app, targetVersion)
		if err != nil {
			return fmt.Errorf("failed to install subnet-evm: %w", err)
		}
	case models.SpacesVM:
		// download the binary and prepare to copy it
		vmBin, err = binutils.SetupSpacesVM(app, targetVersion)
		if err != nil {
			return fmt.Errorf("failed to install spaces-vm: %w", err)
		}
	case models.CustomVM:
		// get the path to the already copied binary
		vmBin = binutils.SetupCustomBin(app, sc.Name)
	default:
		return errors.New("unknown VM type")
	}

	// Update the binary in the plugin directory
	if err := binutils.UpgradeVM(app, vmid.String(), vmBin); err != nil {
		return err
	}

	ux.Logger.PrintToUser("Upgrade complete. Ready to restart the network.")

	return nil
}

func chooseManualOrAutomatic(sc models.Sidecar, targetVersion string, _ string) error {
	switch {
	case useManual:
		return plugins.ManualUpgrade(app, sc, targetVersion)
	case pluginDir != "":
		return plugins.AutomatedUpgrade(app, sc, targetVersion, pluginDir)
	}

	const (
		choiceManual    = "Manual"
		choiceAutomatic = "Automatic (Make sure your node isn't running)"
	)
	choice, err := app.Prompt.CaptureList(
		"How would you like to update the avalanchego config?",
		[]string{choiceAutomatic, choiceManual},
	)
	if err != nil {
		return err
	}

	if choice == choiceManual {
		return plugins.ManualUpgrade(app, sc, targetVersion)
	}
	return plugins.AutomatedUpgrade(app, sc, targetVersion, pluginDir)
}

func updateMainnetVM() error {
	ux.Logger.PrintToUser("Coming soon. For now, please upgrade your mainnet deployments manually.")
	return nil
}
