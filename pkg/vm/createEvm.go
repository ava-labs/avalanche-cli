// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/statemachine"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/precompile/contracts/txallowlist"
	"github.com/ava-labs/subnet-evm/utils"
	"github.com/ethereum/go-ethereum/common"
)

var versionComments = map[string]string{
	"v0.6.0-fuji": " (recommended for fuji durango)",
}

func CreateEvmSubnetConfig(
	app *application.Avalanche,
	subnetName string,
	genesisPath string,
	subnetEVMVersion string,
	getRPCVersionFromBinary bool,
	subnetEVMChainID uint64,
	subnetEVMTokenSymbol string,
	useSubnetEVMDefaults bool,
	useWarp bool,
	teleporterInfo *teleporter.Info,
) ([]byte, *models.Sidecar, error) {
	var (
		genesisBytes []byte
		sc           *models.Sidecar
		err          error
		rpcVersion   int
	)

	if getRPCVersionFromBinary {
		_, vmBin, err := binutils.SetupSubnetEVM(app, subnetEVMVersion)
		if err != nil {
			return nil, &models.Sidecar{}, fmt.Errorf("failed to install subnet-evm: %w", err)
		}
		rpcVersion, err = GetVMBinaryProtocolVersion(vmBin)
		if err != nil {
			return nil, &models.Sidecar{}, fmt.Errorf("unable to get RPC version: %w", err)
		}
	} else {
		rpcVersion, err = GetRPCProtocolVersion(app, models.SubnetEvm, subnetEVMVersion)
		if err != nil {
			return nil, &models.Sidecar{}, err
		}
	}

	if genesisPath == "" {
		genesisBytes, sc, err = createEvmGenesis(
			app,
			subnetName,
			subnetEVMVersion,
			rpcVersion,
			subnetEVMChainID,
			subnetEVMTokenSymbol,
			useSubnetEVMDefaults,
			useWarp,
			teleporterInfo,
		)
		if err != nil {
			return nil, &models.Sidecar{}, err
		}
	} else {
		ux.Logger.PrintToUser("importing genesis for subnet %s", subnetName)
		genesisBytes, err = os.ReadFile(genesisPath)
		if err != nil {
			return nil, &models.Sidecar{}, err
		}

		sc = &models.Sidecar{
			Name:       subnetName,
			VM:         models.SubnetEvm,
			VMVersion:  subnetEVMVersion,
			RPCVersion: rpcVersion,
			Subnet:     subnetName,
		}
	}

	return genesisBytes, sc, nil
}

func createEvmGenesis(
	app *application.Avalanche,
	subnetName string,
	subnetEVMVersion string,
	rpcVersion int,
	subnetEVMChainID uint64,
	subnetEVMTokenSymbol string,
	useSubnetEVMDefaults bool,
	useWarp bool,
	teleporterInfo *teleporter.Info,
) ([]byte, *models.Sidecar, error) {
	ux.Logger.PrintToUser("creating genesis for subnet %s", subnetName)

	genesis := core.Genesis{}
	genesis.Timestamp = *utils.TimeToNewUint64(time.Now())

	conf := params.SubnetEVMDefaultChainConfig
	conf.NetworkUpgrades = params.NetworkUpgrades{}

	const (
		descriptorsState = "descriptors"
		feeState         = "fee"
		airdropState     = "airdrop"
		precompilesState = "precompiles"
	)

	var (
		chainID     *big.Int
		tokenSymbol string
		allocation  core.GenesisAlloc
		direction   statemachine.StateDirection
		err         error
	)

	subnetEvmState, err := statemachine.NewStateMachine(
		[]string{descriptorsState, feeState, airdropState, precompilesState},
	)
	if err != nil {
		return nil, nil, err
	}
	for subnetEvmState.Running() {
		switch subnetEvmState.CurrentState() {
		case descriptorsState:
			chainID, tokenSymbol, direction, err = getDescriptors(
				app,
				subnetEVMChainID,
				subnetEVMTokenSymbol,
			)
		case feeState:
			*conf, direction, err = GetFeeConfig(*conf, app, useSubnetEVMDefaults)
		case airdropState:
			allocation, direction, err = getAllocation(
				app,
				subnetName,
				defaultEvmAirdropAmount,
				oneAvax,
				fmt.Sprintf("Amount to airdrop (in %s units)", tokenSymbol),
				useSubnetEVMDefaults,
			)
			if teleporterInfo != nil {
				allocation = addTeleporterAddressToAllocations(
					allocation,
					teleporterInfo.FundedAddress,
					teleporterInfo.FundedBalance,
				)
			}
		case precompilesState:
			*conf, direction, err = getPrecompiles(*conf, app, &genesis.Timestamp, useSubnetEVMDefaults, useWarp)
			if teleporterInfo != nil {
				*conf = addTeleporterAddressesToAllowLists(
					*conf,
					teleporterInfo.FundedAddress,
					teleporterInfo.MessengerDeployerAddress,
					teleporterInfo.RelayerAddress,
				)
			}
		default:
			err = errors.New("invalid creation stage")
		}
		if err != nil {
			return nil, nil, err
		}
		subnetEvmState.NextState(direction)
	}

	if conf != nil && conf.GenesisPrecompiles[txallowlist.ConfigKey] != nil {
		allowListCfg, ok := conf.GenesisPrecompiles[txallowlist.ConfigKey].(*txallowlist.Config)
		if !ok {
			return nil, nil, fmt.Errorf(
				"expected config of type txallowlist.AllowListConfig, but got %T",
				allowListCfg,
			)
		}

		if err := ensureAdminsHaveBalance(
			allowListCfg.AdminAddresses,
			allocation); err != nil {
			return nil, nil, err
		}
	}

	conf.ChainID = chainID

	genesis.Alloc = allocation
	genesis.Config = conf
	genesis.Difficulty = Difficulty
	genesis.GasLimit = conf.FeeConfig.GasLimit.Uint64()

	jsonBytes, err := genesis.MarshalJSON()
	if err != nil {
		return nil, nil, err
	}

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonBytes, "", "    ")
	if err != nil {
		return nil, nil, err
	}

	sc := &models.Sidecar{
		Name:        subnetName,
		VM:          models.SubnetEvm,
		VMVersion:   subnetEVMVersion,
		RPCVersion:  rpcVersion,
		Subnet:      subnetName,
		TokenSymbol: tokenSymbol,
		TokenName:   tokenSymbol + " Token",
	}

	return prettyJSON.Bytes(), sc, nil
}

func ensureAdminsHaveBalance(admins []common.Address, alloc core.GenesisAlloc) error {
	if len(admins) < 1 {
		return nil
	}

	for _, admin := range admins {
		// we can break at the first admin who has a non-zero balance
		if bal, ok := alloc[admin]; ok &&
			bal.Balance != nil &&
			bal.Balance.Uint64() > uint64(0) {
			return nil
		}
	}
	return errors.New(
		"none of the addresses in the transaction allow list precompile have any tokens allocated to them. Currently, no address can transact on the network. Airdrop some funds to one of the allow list addresses to continue",
	)
}

func GetVMVersion(
	app *application.Avalanche,
	vmName string,
	repoName string,
	vmVersion string,
) (string, error) {
	var err error
	switch vmVersion {
	case "latest":
		vmVersion, err = app.Downloader.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
			constants.AvaLabsOrg,
			repoName,
		))
		if err != nil {
			return "", err
		}
	case "pre-release":
		vmVersion, err = app.Downloader.GetLatestPreReleaseVersion(
			constants.AvaLabsOrg,
			repoName,
		)
		if err != nil {
			return "", err
		}
	case "":
		vmVersion, err = askForVMVersion(app, vmName, repoName)
		if err != nil {
			return "", err
		}
	}
	return vmVersion, nil
}

func askForVMVersion(
	app *application.Avalanche,
	vmName string,
	repoName string,
) (string, error) {
	latestReleaseVersion, err := app.Downloader.GetLatestReleaseVersion(
		binutils.GetGithubLatestReleaseURL(
			constants.AvaLabsOrg,
			repoName,
		),
	)
	if err != nil {
		return "", err
	}
	latestPreReleaseVersion, err := app.Downloader.GetLatestPreReleaseVersion(
		constants.AvaLabsOrg,
		repoName,
	)
	if err != nil {
		return "", err
	}

	useCustom := "Specify custom version"
	useLatestRelease := "Use latest release version" + versionComments[latestReleaseVersion]
	useLatestPreRelease := "Use latest pre-release version" + versionComments[latestPreReleaseVersion]

	defaultPrompt := fmt.Sprintf("What version of %s would you like?", vmName)

	versionOptions := []string{useLatestRelease, useCustom}
	if latestPreReleaseVersion != latestReleaseVersion {
		versionOptions = []string{useLatestPreRelease, useLatestRelease, useCustom}
	}

	versionOption, err := app.Prompt.CaptureList(
		defaultPrompt,
		versionOptions,
	)
	if err != nil {
		return "", err
	}

	if versionOption == useLatestPreRelease {
		return latestPreReleaseVersion, err
	}

	if versionOption == useLatestRelease {
		return latestReleaseVersion, err
	}

	// prompt for version
	versions, err := app.Downloader.GetAllReleasesForRepo(
		constants.AvaLabsOrg,
		constants.SubnetEVMRepoName,
	)
	if err != nil {
		return "", err
	}
	version, err := app.Prompt.CaptureList("Pick the version for this VM", versions)
	if err != nil {
		return "", err
	}

	return version, nil
}
